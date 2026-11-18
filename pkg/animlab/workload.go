package animlab

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// TestRunSpec is a single concrete test scenario expanded from YAML.
type TestRunSpec struct {
	RunID            string
	Family           string
	Method           string
	WorkloadName     string
	Count            int
	Isolation        string // "independent-svg-roots" | "shared-scene"
	VisibleCount     int
	SuspendOffscreen bool
	IsMixture        bool
	Seed             int64
	Instances        []InstanceParams
	Limitations      []string
}

var palette = []string{"#38bdf8", "#22c55e", "#f59e0b", "#ec4899", "#a855f7", "#06b6d4"}

// ExpandScenarios expands a validated Config into concrete TestRunSpecs.
// If alternate_method_order is true, repetition index can rotate method order in Runner.
func ExpandScenarios(cfg *Config) []TestRunSpec {
	reg := DefaultRegistry()
	var specs []TestRunSpec
	rng := rand.New(rand.NewSource(cfg.Execution.RandomSeed))

	for _, count := range cfg.Instances.Counts {
		for _, wl := range cfg.Workloads {
			// 1. Per-effect scenarios
			for _, eff := range cfg.Effects {
				for _, method := range eff.Methods {
					gen := reg[eff.Family]
					instances := buildEffectInstances(eff, method, count, rng)
					var lims []string
					limSeen := map[string]bool{}
					for _, inst := range instances {
						re := gen.Render(inst, 180, 120)
						if re.Limitation != "" && !limSeen[re.Limitation] {
							limSeen[re.Limitation] = true
							lims = append(lims, re.Limitation)
						}
					}
					specs = append(specs, TestRunSpec{
						RunID:            fmt.Sprintf("%s__%s__%s__n%d", eff.Family, method, wl.Name, count),
						Family:           eff.Family,
						Method:           method,
						WorkloadName:     wl.Name,
						Count:            count,
						Isolation:        cfg.Instances.Isolation,
						VisibleCount:     wl.VisibleCount,
						SuspendOffscreen: wl.SuspendOffscreen,
						Seed:             cfg.Execution.RandomSeed,
						Instances:        instances,
						Limitations:      lims,
					})
				}
			}

			// 2. Mixed workload scenarios (if configured)
			if cfg.Mixture != nil {
				for _, method := range cfg.Mixture.Methods {
					instances := buildMixtureInstances(cfg, method, count, rng)
					mixName := cfg.Mixture.Name
					if mixName == "" {
						mixName = "mixed"
					}
					specs = append(specs, TestRunSpec{
						RunID:            fmt.Sprintf("%s__%s__%s__n%d", mixName, method, wl.Name, count),
						Family:           mixName,
						Method:           method,
						WorkloadName:     wl.Name,
						Count:            count,
						Isolation:        cfg.Instances.Isolation,
						VisibleCount:     wl.VisibleCount,
						SuspendOffscreen: wl.SuspendOffscreen,
						IsMixture:        true,
						Seed:             cfg.Execution.RandomSeed,
						Instances:        instances,
					})
				}
			}
		}
	}
	return specs
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildEffectInstances(eff EffectSpec, method string, count int, rng *rand.Rand) []InstanceParams {
	sizeKeys := sortedMapKeys(eff.Sizes)
	speedKeys := sortedMapKeys(eff.Speeds)
	out := make([]InstanceParams, 0, count)
	for i := 0; i < count; i++ {
		shape := eff.Shapes[i%len(eff.Shapes)]
		sz := eff.Sizes[sizeKeys[i%len(sizeKeys)]]
		spd := eff.Speeds[speedKeys[i%len(speedKeys)]]
		seg := 2
		if len(eff.SegmentCounts) > 0 {
			seg = eff.SegmentCounts[i%len(eff.SegmentCounts)]
		}
		ratio := "1:3"
		if len(eff.LineGapRatios) > 0 {
			ratio = eff.LineGapRatios[i%len(eff.LineGapRatios)]
		}
		dir := "cw"
		if i%2 == 1 {
			dir = "ccw"
		}
		pm := "scale-opacity"
		if len(eff.PulseModes) > 0 {
			pm = eff.PulseModes[i%len(eff.PulseModes)]
		}
		pAmp := 0.2
		if len(eff.PulseAmplitudes) > 0 {
			pAmp = eff.PulseAmplitudes[i%len(eff.PulseAmplitudes)]
		}
		fm := "repeating"
		if len(eff.FlashModes) > 0 {
			fm = eff.FlashModes[i%len(eff.FlashModes)]
		}
		stagger := 0.0
		if fm == "staggered" {
			stagger = float64(i%8) * 0.15
		}
		// Add slight deterministic variation using rng so instances are genuinely independent
		_ = rng.Intn(10)
		out = append(out, InstanceParams{
			ID:             fmt.Sprintf("lab_%s_%d", strings.ReplaceAll(method, "-", "_"), i+1),
			Family:         eff.Family,
			Method:         method,
			Shape:          shape,
			Width:          sz[0],
			Height:         sz[1],
			Speed:          spd,
			Segments:       seg,
			LineGapRatio:   ratio,
			Direction:      dir,
			PulseMode:      pm,
			PulseAmplitude: pAmp,
			FlashMode:      fm,
			FlashDurMs:     eff.FlashDurationMs,
			FlashIntervalS: eff.FlashIntervalS,
			StaggerDelayS:  stagger,
			Color:          palette[i%len(palette)],
		})
	}
	return out
}

func buildMixtureInstances(cfg *Config, method string, count int, rng *rand.Rand) []InstanceParams {
	fams := sortedMapKeys(cfg.Mixture.Weights)
	totalW := 0.0
	for _, f := range fams {
		totalW += cfg.Mixture.Weights[f]
	}
	effMap := map[string]EffectSpec{}
	for _, e := range cfg.Effects {
		effMap[e.Family] = e
	}
	reg := DefaultRegistry()

	// Build proportional family list, then deterministically shuffle with rng
	assigned := make([]string, 0, count)
	for _, f := range fams {
		n := int((cfg.Mixture.Weights[f] / totalW) * float64(count))
		for j := 0; j < n && len(assigned) < count; j++ {
			assigned = append(assigned, f)
		}
	}
	for len(assigned) < count {
		assigned = append(assigned, fams[len(assigned)%len(fams)])
	}
	rng.Shuffle(len(assigned), func(i, j int) { assigned[i], assigned[j] = assigned[j], assigned[i] })

	out := make([]InstanceParams, 0, count)
	for i, fam := range assigned {
		eff, ok := effMap[fam]
		if !ok {
			eff = EffectSpec{
				Family:          fam,
				Shapes:          defaultShapesForFamily(fam),
				Sizes:           map[string][2]int{"medium": {110, 70}},
				Speeds:          map[string]float64{"medium": 2.5},
				SegmentCounts:   []int{2},
				LineGapRatios:   []string{"1:3"},
				PulseModes:      []string{"scale-opacity"},
				PulseAmplitudes: []float64{0.2},
				FlashModes:      []string{"repeating"},
				FlashDurationMs: 120,
				FlashIntervalS:  1.2,
			}
		}
		m := method
		if gen := reg[fam]; gen != nil && !gen.SupportsMethod(m) {
			m = gen.SupportedMethods()[0]
		}
		one := buildEffectInstances(eff, m, 1, rng)[0]
		one.ID = fmt.Sprintf("mix_%d", i+1)
		out = append(out, one)
	}
	return out
}

// BuildTestPageHTML generates the self-contained HTML harness page for a TestRunSpec.
func BuildTestPageHTML(spec TestRunSpec) string {
	reg := DefaultRegistry()
	var cssBuf, bodyBuf, jsBuf strings.Builder
	const cellW, cellH = 180, 120

	if spec.Isolation == "shared-scene" {
		cols := 6
		rows := (len(spec.Instances) + cols - 1) / cols
		if rows < 1 {
			rows = 1
		}
		totalW := cols * cellW
		totalH := rows * cellH
		var inner strings.Builder
		for i, inst := range spec.Instances {
			gen := reg[inst.Family]
			if gen == nil {
				continue
			}
			re := gen.Render(inst, cellW, cellH)
			if re.CSS != "" {
				cssBuf.WriteString(re.CSS + "\n")
			}
			if re.JSDriver != "" {
				jsBuf.WriteString(re.JSDriver + "\n")
			}
			col := i % cols
			row := i / cols
			inner.WriteString(fmt.Sprintf(`<g id="%s" class="lab-instance" data-lab-idx="%d" transform="translate(%d,%d)">%s</g>`,
				inst.ID, i, col*cellW, row*cellH, re.SVGInner))
		}
		bodyBuf.WriteString(fmt.Sprintf(`<div class="lab-shared-wrap"><svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg></div>`,
			totalW, totalH, totalW, totalH, inner.String()))
	} else {
		for i, inst := range spec.Instances {
			gen := reg[inst.Family]
			if gen == nil {
				continue
			}
			re := gen.Render(inst, cellW, cellH)
			if re.CSS != "" {
				cssBuf.WriteString(re.CSS + "\n")
			}
			if re.JSDriver != "" {
				jsBuf.WriteString(re.JSDriver + "\n")
			}
			bodyBuf.WriteString(fmt.Sprintf(
				`<div class="lab-card" data-lab-idx="%d"><div class="lab-meta">#%d %s (%s/%s)</div><svg id="%s" class="lab-svg-root" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg></div>`,
				i, i+1, inst.Family, inst.Method, inst.Shape, inst.ID, cellW, cellH, cellW, cellH, re.SVGInner))
		}
	}

	visHeightCSS := ""
	if spec.VisibleCount > 0 {
		// Force a scrollable container showing ~VisibleCount cards in 3 columns
		rowsVis := (spec.VisibleCount + 2) / 3
		if rowsVis < 1 {
			rowsVis = 1
		}
		visHeightCSS = fmt.Sprintf("#labGrid { max-height: %dpx; overflow-y: auto; }", rowsVis*164)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab — %s</title>
<style>
  body { margin: 0; padding: 12px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  #labGrid { display: grid; grid-template-columns: repeat(3, 196px); gap: 10px; }
  .lab-card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 6px; }
  .lab-meta { font-size: 11px; color: #8b949e; margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .lab-card.lab-suspended, .lab-card.lab-suspended * { animation-play-state: paused !important; }
  %s
  %s
</style>
</head>
<body>
<div id="labHeader" style="font-size:12px;margin-bottom:8px;color:#8b949e;">Run: %s | Instances: %d | Isolation: %s | Workload: %s</div>
<div id="labGrid">%s</div>
<script>
window.__labRafDrivers = [];
window.__labLongTasks = [];
window.__labSuspendedCount = 0;
window.__labVisibleCount = %d;
if (typeof PerformanceObserver !== 'undefined') {
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        window.__labLongTasks.push({ start: e.startTime, dur: e.duration });
      }
    }).observe({ entryTypes: ['longtask'] });
  } catch (_) {}
}
%s
(function() {
  const suspendOffscreen = %v;
  const suspendedSet = new Set();
  if (suspendOffscreen && typeof IntersectionObserver !== 'undefined') {
    const rootEl = document.getElementById('labGrid');
    const io = new IntersectionObserver((entries) => {
      for (const ent of entries) {
        const card = ent.target;
        const svg = card.querySelector('svg');
        const id = svg ? svg.id : '';
        if (ent.isIntersecting) {
          card.classList.remove('lab-suspended');
          suspendedSet.delete(id);
          if (svg && svg.unpauseAnimations) try { svg.unpauseAnimations(); } catch (_) {}
        } else {
          card.classList.add('lab-suspended');
          suspendedSet.add(id);
          if (svg && svg.pauseAnimations) try { svg.pauseAnimations(); } catch (_) {}
        }
      }
      window.__labSuspendedCount = suspendedSet.size;
      window.__labVisibleCount = %d - suspendedSet.size;
    }, { root: rootEl, threshold: 0.01 });
    document.querySelectorAll('.lab-card').forEach(c => io.observe(c));
  }

  // Unified JS rAF driver for javascript-raf method comparisons
  if (window.__labRafDrivers.length > 0) {
    const startT = performance.now();
    const resolved = window.__labRafDrivers.map(d => {
      const root = document.getElementById(d.id);
      const el = root ? (root.querySelector('.lab-payload') || root) : null;
      return Object.assign({ el }, d);
    }).filter(d => d.el);
    function step(now) {
      const elapsed = (now - startT) / 1000;
      for (let i = 0; i < resolved.length; i++) {
        const d = resolved[i];
        if (suspendedSet.has(d.id)) continue;
        if (d.kind === 'tracer') {
          const p = (elapsed %% d.dur) / d.dur;
          const off = (d.dir === 'ccw' ? 1 : -1) * (p * 1000);
          d.el.style.strokeDashoffset = off.toFixed(2) + 'px';
        } else if (d.kind === 'pulse') {
          const phase = (elapsed %% d.dur) / d.dur;
          const wave = 0.5 - 0.5 * Math.cos(phase * Math.PI * 2);
          const sc = d.minS + (d.maxS - d.minS) * wave;
          const op = d.minOp + (1 - d.minOp) * wave;
          d.el.setAttribute('transform', 'scale(' + sc.toFixed(4) + ')');
          d.el.setAttribute('opacity', op.toFixed(3));
        } else if (d.kind === 'flash') {
          const t = Math.max(0, elapsed - (d.stagger || 0));
          const cyc = d.single ? t : (t %% d.interval);
          const op = (!d.single || t <= d.dur) && cyc <= d.dur ? Math.sin((cyc / d.dur) * Math.PI) : 0;
          d.el.setAttribute('opacity', Math.max(0, op).toFixed(3));
        }
      }
      requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }
})();
</script>
</body>
</html>`, spec.RunID, visHeightCSS, cssBuf.String(), spec.RunID, spec.Count, spec.Isolation, spec.WorkloadName, bodyBuf.String(), spec.Count, jsBuf.String(), spec.SuspendOffscreen, spec.Count)
}

// BuildStandaloneSampleSVG generates a clean standalone SVG sample for a single InstanceParams.
func BuildStandaloneSampleSVG(p InstanceParams) string {
	reg := DefaultRegistry()
	gen := reg[p.Family]
	if gen == nil {
		return ""
	}
	const w, h = 220, 140
	re := gen.Render(p, w, h)
	style := ""
	if re.CSS != "" {
		style = fmt.Sprintf("\n  <style>\n%s\n  </style>", re.CSS)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg id="%s" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s
  <rect width="%d" height="%d" fill="#0d1117" rx="8" />
  %s
</svg>`, p.ID, w, h, w, h, style, w, h, re.SVGInner)
}
