package animlab

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// TestRunSpec is a single concrete test scenario expanded from YAML.
type TestRunSpec struct {
	RunID              string
	Family             string
	Method             string
	SchedulingStrategy string
	VariationStrategy  string
	VariationTag       string
	WorkloadName       string
	WorkloadVisibility string
	Count              int
	Isolation          string // "independent-svg-roots" | "shared-scene" | "shared-svg-scene" | "shared-dt-scene" | "stage16c-widgets"
	VisibleCount       int
	SuspendOffscreen   bool
	IsMixture          bool
	MixtureAllocation  map[string]int
	NonEquivalent      bool
	Unsupported        bool
	Seed               int64
	Scene              *SceneConfig
	WidgetRuntime      *WidgetRuntimeConfig
	ConfigDir          string
	Instances          []InstanceParams
	Limitations        []string
}

var palette = []string{"#38bdf8", "#22c55e", "#f59e0b", "#ec4899", "#a855f7", "#06b6d4"}

func resolveSchedulingStrategy(eff EffectSpec, method string) string {
	switch method {
	case "css":
		return "native-css"
	case "smil":
		return "native-smil"
	case "javascript-raf":
		if eff.RAFScheduling == "per-instance" {
			return "per-instance-raf"
		}
		return "shared-raf"
	case "data-driven":
		return "data-driven-interval"
	default:
		return method
	}
}

type effectVariant struct {
	tag string
	eff EffectSpec
}

func cloneEffectSingle(eff EffectSpec) EffectSpec {
	out := eff
	out.Shapes = append([]string(nil), eff.Shapes...)
	out.Sizes = map[string][2]int{}
	for k, v := range eff.Sizes {
		out.Sizes[k] = v
	}
	out.Speeds = map[string]float64{}
	for k, v := range eff.Speeds {
		out.Speeds[k] = v
	}
	out.SegmentCounts = append([]int(nil), eff.SegmentCounts...)
	out.LineGapRatios = append([]string(nil), eff.LineGapRatios...)
	out.Directions = append([]string(nil), eff.Directions...)
	out.PulseModes = append([]string(nil), eff.PulseModes...)
	out.PulseAmplitudes = append([]float64(nil), eff.PulseAmplitudes...)
	out.FlashModes = append([]string(nil), eff.FlashModes...)
	return out
}

func expandEffectVariations(eff EffectSpec) []effectVariant {
	strat := eff.VariationStrategy
	if strat == "" || strat == "representative" {
		return []effectVariant{{tag: "", eff: eff}}
	}

	sizeKeys := sortedMapKeys(eff.Sizes)
	speedKeys := sortedMapKeys(eff.Speeds)
	segs := eff.SegmentCounts
	if len(segs) == 0 {
		segs = []int{2}
	}
	ratios := eff.LineGapRatios
	if len(ratios) == 0 {
		ratios = []string{"1:3"}
	}
	pModes := eff.PulseModes
	if len(pModes) == 0 {
		pModes = []string{"scale-opacity"}
	}
	pAmps := eff.PulseAmplitudes
	if len(pAmps) == 0 {
		pAmps = []float64{0.2}
	}
	fModes := eff.FlashModes
	if len(fModes) == 0 {
		fModes = []string{"repeating"}
	}

	if strat == "isolated" {
		// Hold baseline constant (first element of each dimension), vary one factor at a time
		baseShape := eff.Shapes[0]
		baseSizeK := sizeKeys[0]
		baseSpeedK := speedKeys[0]
		baseSeg := segs[0]
		baseRatio := ratios[0]
		basePM := pModes[0]
		basePA := pAmps[0]
		baseFM := fModes[0]

		makeSingle := func(sh, szK, spK string, seg int, rat, pm string, pa float64, fm string) EffectSpec {
			v := cloneEffectSingle(eff)
			v.Shapes = []string{sh}
			v.Sizes = map[string][2]int{szK: eff.Sizes[szK]}
			v.Speeds = map[string]float64{spK: eff.Speeds[spK]}
			v.SegmentCounts = []int{seg}
			v.LineGapRatios = []string{rat}
			v.PulseModes = []string{pm}
			v.PulseAmplitudes = []float64{pa}
			v.FlashModes = []string{fm}
			return v
		}

		var out []effectVariant
		seen := map[string]bool{}
		add := func(tag string, v EffectSpec) {
			if !seen[tag] {
				seen[tag] = true
				out = append(out, effectVariant{tag: tag, eff: v})
			}
		}
		add("baseline", makeSingle(baseShape, baseSizeK, baseSpeedK, baseSeg, baseRatio, basePM, basePA, baseFM))
		for _, sh := range eff.Shapes[1:] {
			add("shape_"+sh, makeSingle(sh, baseSizeK, baseSpeedK, baseSeg, baseRatio, basePM, basePA, baseFM))
		}
		for _, szK := range sizeKeys[1:] {
			add("size_"+szK, makeSingle(baseShape, szK, baseSpeedK, baseSeg, baseRatio, basePM, basePA, baseFM))
		}
		for _, spK := range speedKeys[1:] {
			add("speed_"+spK, makeSingle(baseShape, baseSizeK, spK, baseSeg, baseRatio, basePM, basePA, baseFM))
		}
		if eff.Family == "perimeter-tracer" {
			for _, seg := range segs[1:] {
				add(fmt.Sprintf("segs_%d", seg), makeSingle(baseShape, baseSizeK, baseSpeedK, seg, baseRatio, basePM, basePA, baseFM))
			}
			for _, rat := range ratios[1:] {
				add("ratio_"+strings.ReplaceAll(rat, ":", "x"), makeSingle(baseShape, baseSizeK, baseSpeedK, baseSeg, rat, basePM, basePA, baseFM))
			}
		}
		if eff.Family == "pulse" {
			for _, pm := range pModes[1:] {
				add("pmode_"+pm, makeSingle(baseShape, baseSizeK, baseSpeedK, baseSeg, baseRatio, pm, basePA, baseFM))
			}
			for _, pa := range pAmps[1:] {
				add(fmt.Sprintf("pamp_%.2f", pa), makeSingle(baseShape, baseSizeK, baseSpeedK, baseSeg, baseRatio, basePM, pa, baseFM))
			}
		}
		if eff.Family == "flash" {
			for _, fm := range fModes[1:] {
				add("fmode_"+fm, makeSingle(baseShape, baseSizeK, baseSpeedK, baseSeg, baseRatio, basePM, basePA, fm))
			}
		}
		return out
	}

	// "factorial": full Cartesian product across relevant family dimensions
	if eff.Family != "perimeter-tracer" {
		segs = segs[:1]
		ratios = ratios[:1]
	}
	if eff.Family != "pulse" {
		pModes = pModes[:1]
		pAmps = pAmps[:1]
	}
	if eff.Family != "flash" {
		fModes = fModes[:1]
	}

	var out []effectVariant
	for _, sh := range eff.Shapes {
		for _, szK := range sizeKeys {
			for _, spK := range speedKeys {
				for _, seg := range segs {
					for _, rat := range ratios {
						for _, pm := range pModes {
							for _, pa := range pAmps {
								for _, fm := range fModes {
									v := cloneEffectSingle(eff)
									v.Shapes = []string{sh}
									v.Sizes = map[string][2]int{szK: eff.Sizes[szK]}
									v.Speeds = map[string]float64{spK: eff.Speeds[spK]}
									v.SegmentCounts = []int{seg}
									v.LineGapRatios = []string{rat}
									v.PulseModes = []string{pm}
									v.PulseAmplitudes = []float64{pa}
									v.FlashModes = []string{fm}
									parts := []string{sh, szK, spK}
									if eff.Family == "perimeter-tracer" {
										parts = append(parts, fmt.Sprintf("s%d", seg), "r"+strings.ReplaceAll(rat, ":", "x"))
									} else if eff.Family == "pulse" {
										parts = append(parts, pm, fmt.Sprintf("a%.2f", pa))
									} else if eff.Family == "flash" {
										parts = append(parts, fm)
									}
									out = append(out, effectVariant{tag: strings.Join(parts, "_"), eff: v})
								}
							}
						}
					}
				}
			}
		}
	}
	return out
}

// ExpandScenarios expands a validated Config into concrete TestRunSpecs.
func ExpandScenarios(cfg *Config) []TestRunSpec {
	reg := DefaultRegistry()
	var specs []TestRunSpec

	for _, count := range cfg.Instances.Counts {
		for _, wl := range cfg.Workloads {
			// 0. Complete Stage 16C exported widget package workload (M4)
			if cfg.Instances.Isolation == "stage16c-widgets" && cfg.WidgetRuntime != nil {
				pkgDir := resolveAssetPath(cfg.ConfigDir, cfg.WidgetRuntime.PackageDir)
				pkg, err := LoadStage16CPackage(pkgDir)
				wName := "stage16c-widget"
				var lims []string
				if err == nil && pkg != nil {
					wName = pkg.Manifest.WidgetName
					if cfg.WidgetRuntime.Updates != nil && len(cfg.WidgetRuntime.Updates.Properties) > 0 {
						_, skipped, _ := ValidateWidgetProperties(pkg, cfg.WidgetRuntime.Updates.Properties)
						lims = append(lims, skipped...)
					}
				}
				visCount := wl.VisibleCount
				if wl.Visibility == "offscreen" {
					visCount = 0
				}
				specs = append(specs, TestRunSpec{
					RunID:              fmt.Sprintf("%s__stage16c-runtime__%s__n%d", wName, wl.Name, count),
					Family:             wName,
					Method:             "stage16c-runtime",
					SchedulingStrategy: "stage16c-widget-runtime",
					VariationStrategy:  "stage16c-package",
					WorkloadName:       wl.Name,
					WorkloadVisibility: wl.Visibility,
					Count:              count,
					Isolation:          "stage16c-widgets",
					VisibleCount:       visCount,
					SuspendOffscreen:   wl.SuspendOffscreen,
					Seed:               cfg.Execution.RandomSeed,
					WidgetRuntime:      cfg.WidgetRuntime,
					ConfigDir:          cfg.ConfigDir,
					Limitations:        lims,
				})
				continue
			}

			// 1. Per-effect scenarios
			for _, eff := range cfg.Effects {
				strat := eff.VariationStrategy
				if strat == "" {
					strat = "representative"
				}
				variants := expandEffectVariations(eff)
				for _, v := range variants {
					for _, method := range v.eff.Methods {
						scenSeed := cfg.Execution.RandomSeed + int64(len(specs))*104729
						rng := rand.New(rand.NewSource(scenSeed))
						gen := reg[v.eff.Family]
						instances := buildEffectInstances(v.eff, method, count, rng)
						var lims []string
						limSeen := map[string]bool{}
						nonEq, unsup := false, false
						for _, inst := range instances {
							re := gen.Render(inst, 240, 160)
							if re.Limitation != "" && !limSeen[re.Limitation] {
								limSeen[re.Limitation] = true
								lims = append(lims, re.Limitation)
							}
							if re.NonEquivalent {
								nonEq = true
							}
							if re.Unsupported {
								unsup = true
							}
						}
						isShared := cfg.Instances.Isolation == "shared-scene" || cfg.Instances.Isolation == "shared-svg-scene" || cfg.Instances.Isolation == "shared-dt-scene"
						if isShared && method == "smil" && wl.SuspendOffscreen {
							msg := "SMIL animations inside a single shared-scene <svg> share one SVG timeline and cannot be suspended per-<g> without globally pausing sibling elements"
							if !limSeen[msg] {
								limSeen[msg] = true
								lims = append(lims, msg)
							}
							unsup = true
						}
						runID := fmt.Sprintf("%s__%s__%s__n%d", v.eff.Family, method, wl.Name, count)
						if v.tag != "" {
							runID = fmt.Sprintf("%s__%s__%s__%s__n%d", v.eff.Family, v.tag, method, wl.Name, count)
						}
						visCount := wl.VisibleCount
						if wl.Visibility == "offscreen" {
							visCount = 0
						}
						specs = append(specs, TestRunSpec{
							RunID:              runID,
							Family:             v.eff.Family,
							Method:             method,
							SchedulingStrategy: resolveSchedulingStrategy(v.eff, method),
							VariationStrategy:  strat,
							VariationTag:       v.tag,
							WorkloadName:       wl.Name,
							WorkloadVisibility: wl.Visibility,
							Count:              count,
							Isolation:          cfg.Instances.Isolation,
							VisibleCount:       visCount,
							SuspendOffscreen:   wl.SuspendOffscreen,
							NonEquivalent:      nonEq,
							Unsupported:        unsup,
							Seed:               scenSeed,
							Scene:              cfg.Scene,
							ConfigDir:          cfg.ConfigDir,
							Instances:          instances,
							Limitations:        lims,
						})
					}
				}
			}

			// 2. Mixed workload scenarios (if configured)
			if cfg.Mixture != nil {
				for _, method := range cfg.Mixture.Methods {
					scenSeed := cfg.Execution.RandomSeed + int64(len(specs))*104729
					rng := rand.New(rand.NewSource(scenSeed))
					instances, alloc, lims, nonEq := buildMixtureInstances(cfg, method, count, rng)
					mixName := cfg.Mixture.Name
					if mixName == "" {
						mixName = "mixed"
					}
					visCount := wl.VisibleCount
					if wl.Visibility == "offscreen" {
						visCount = 0
					}
					specs = append(specs, TestRunSpec{
						RunID:              fmt.Sprintf("%s__%s__%s__n%d", mixName, method, wl.Name, count),
						Family:             mixName,
						Method:             method,
						SchedulingStrategy: resolveSchedulingStrategy(EffectSpec{}, method),
						VariationStrategy:  "mixture",
						WorkloadName:       wl.Name,
						WorkloadVisibility: wl.Visibility,
						Count:              count,
						Isolation:          cfg.Instances.Isolation,
						VisibleCount:       visCount,
						SuspendOffscreen:   wl.SuspendOffscreen,
						IsMixture:          true,
						MixtureAllocation:  alloc,
						NonEquivalent:      nonEq,
						Seed:               scenSeed,
						Scene:              cfg.Scene,
						ConfigDir:          cfg.ConfigDir,
						Instances:          instances,
						Limitations:        lims,
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

// buildDecorrelatedPermutation returns a balanced, deterministically shuffled index mapping of length count over nValues,
// avoiding modulo correlation across multiple factors (F-D04).
func buildDecorrelatedOrder(count, nValues int, rng *rand.Rand) []int {
	out := make([]int, count)
	if nValues <= 1 {
		return out
	}
	for i := 0; i < count; i++ {
		out[i] = i % nValues
	}
	rng.Shuffle(count, func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func buildEffectInstances(eff EffectSpec, method string, count int, rng *rand.Rand) []InstanceParams {
	sizeKeys := sortedMapKeys(eff.Sizes)
	speedKeys := sortedMapKeys(eff.Speeds)

	dirs := eff.Directions
	if len(dirs) == 0 {
		dirs = []string{"cw", "ccw"}
	}
	segs := eff.SegmentCounts
	if len(segs) == 0 {
		segs = []int{2}
	}
	ratios := eff.LineGapRatios
	if len(ratios) == 0 {
		ratios = []string{"1:3"}
	}
	pModes := eff.PulseModes
	if len(pModes) == 0 {
		pModes = []string{"scale-opacity"}
	}
	pAmps := eff.PulseAmplitudes
	if len(pAmps) == 0 {
		pAmps = []float64{0.2}
	}
	fModes := eff.FlashModes
	if len(fModes) == 0 {
		fModes = []string{"repeating"}
	}

	shapeIdx := buildDecorrelatedOrder(count, len(eff.Shapes), rng)
	sizeIdx := buildDecorrelatedOrder(count, len(sizeKeys), rng)
	speedIdx := buildDecorrelatedOrder(count, len(speedKeys), rng)
	segIdx := buildDecorrelatedOrder(count, len(segs), rng)
	ratioIdx := buildDecorrelatedOrder(count, len(ratios), rng)
	dirIdx := buildDecorrelatedOrder(count, len(dirs), rng)
	pmIdx := buildDecorrelatedOrder(count, len(pModes), rng)
	paIdx := buildDecorrelatedOrder(count, len(pAmps), rng)
	fmIdx := buildDecorrelatedOrder(count, len(fModes), rng)

	out := make([]InstanceParams, 0, count)
	for i := 0; i < count; i++ {
		shape := eff.Shapes[shapeIdx[i]]
		sz := eff.Sizes[sizeKeys[sizeIdx[i]]]
		spd := eff.Speeds[speedKeys[speedIdx[i]]]
		seg := segs[segIdx[i]]
		ratio := ratios[ratioIdx[i]]
		dir := dirs[dirIdx[i]]
		pm := pModes[pmIdx[i]]
		pAmp := pAmps[paIdx[i]]
		fm := fModes[fmIdx[i]]
		stagger := 0.0
		if fm == "staggered" {
			stagger = float64(i%8) * 0.15
		}
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
			RAFScheduling:  eff.RAFScheduling,
			Color:          palette[i%len(palette)],
		})
	}
	return out
}

// allocateMixtureCounts uses Largest-Remainder (Hamilton) rounding so allocated counts sum cleanly to count (F-D07).
func allocateMixtureCounts(weights map[string]float64, count int) map[string]int {
	fams := sortedMapKeys(weights)
	totalW := 0.0
	for _, f := range fams {
		totalW += weights[f]
	}
	type remEntry struct {
		fam string
		rem float64
	}
	alloc := make(map[string]int, len(fams))
	rems := make([]remEntry, 0, len(fams))
	used := 0
	for _, f := range fams {
		exact := (weights[f] / totalW) * float64(count)
		fl := int(math.Floor(exact))
		alloc[f] = fl
		used += fl
		rems = append(rems, remEntry{fam: f, rem: exact - float64(fl)})
	}
	sort.Slice(rems, func(i, j int) bool {
		if math.Abs(rems[i].rem-rems[j].rem) > 1e-9 {
			return rems[i].rem > rems[j].rem
		}
		return rems[i].fam < rems[j].fam
	})
	for i := 0; i < count-used && i < len(rems); i++ {
		alloc[rems[i].fam]++
	}
	return alloc
}

func buildMixtureInstances(cfg *Config, method string, count int, rng *rand.Rand) ([]InstanceParams, map[string]int, []string, bool) {
	fams := sortedMapKeys(cfg.Mixture.Weights)
	alloc := allocateMixtureCounts(cfg.Mixture.Weights, count)

	effMap := map[string]EffectSpec{}
	for _, e := range cfg.Effects {
		effMap[e.Family] = e
	}
	reg := DefaultRegistry()

	assigned := make([]string, 0, count)
	for _, f := range fams {
		for j := 0; j < alloc[f]; j++ {
			assigned = append(assigned, f)
		}
	}
	rng.Shuffle(len(assigned), func(i, j int) { assigned[i], assigned[j] = assigned[j], assigned[i] })

	var lims []string
	limSeen := map[string]bool{}
	nonEq := false

	// Pre-build per-family de-correlated instance pools so shapes/sizes/speeds vary across the mixture (F-D07)
	famPools := map[string][]InstanceParams{}
	famCursor := map[string]int{}
	for _, fam := range fams {
		nFam := alloc[fam]
		if nFam == 0 {
			continue
		}
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
			if fam == "static-gauge" {
				msg := fmt.Sprintf("mixture family static-gauge uses declared static baseline alongside %s dynamic widgets", method)
				if !limSeen[msg] {
					limSeen[msg] = true
					lims = append(lims, msg)
				}
				m = "static"
			} else {
				fallback := gen.SupportedMethods()[0]
				msg := fmt.Sprintf("mixture family %s does not support %s; substituted %s (non-equivalent)", fam, method, fallback)
				if !limSeen[msg] {
					limSeen[msg] = true
					lims = append(lims, msg)
				}
				nonEq = true
				m = fallback
			}
		}
		famPools[fam] = buildEffectInstances(eff, m, nFam, rng)
	}

	out := make([]InstanceParams, 0, count)
	for i, fam := range assigned {
		idx := famCursor[fam]
		famCursor[fam] = idx + 1
		one := famPools[fam][idx]
		one.ID = fmt.Sprintf("mix_%d", i+1)
		out = append(out, one)
	}
	return out, alloc, lims, nonEq
}

func computeCellDimensions(instances []InstanceParams) (int, int) {
	cellW, cellH := 180, 120
	for _, inst := range instances {
		if inst.Width+24 > cellW {
			cellW = inst.Width + 24
		}
		if inst.Height+24 > cellH {
			cellH = inst.Height + 24
		}
	}
	return cellW, cellH
}

const labSharedRuntimeJS = `
(function(rootScope) {
  const drivers = window.__labRafDrivers || [];
  const dataDrivers = window.__labDataDrivers || [];
  const suspendedSet = window.__labSuspendedSet || new Set();
  const findPayload = (id) => {
    const root = (rootScope && rootScope.getElementById) ? rootScope.getElementById(id) : document.getElementById(id);
    if (!root) return { root: null, el: null };
    return { root, el: root.querySelector('.lab-payload') || root };
  };
  const applyDriver = (d, elapsed) => {
    if (!d.el) return;
    if (d.kind === 'tracer') {
      const p = (elapsed % d.dur) / d.dur;
      const off = (d.dir === 'ccw' ? 1 : -1) * (p * 1000);
      d.el.style.strokeDashoffset = off.toFixed(2) + 'px';
    } else if (d.kind === 'pulse') {
      const phase = (elapsed % d.dur) / d.dur;
      const wave = 0.5 - 0.5 * Math.cos(phase * Math.PI * 2);
      const sc = d.minS + (d.maxS - d.minS) * wave;
      const op = d.minOp + (1 - d.minOp) * wave;
      if (d.mode === 'opacity-only') {
        d.el.setAttribute('opacity', op.toFixed(3));
      } else if (d.mode === 'scale-only') {
        d.el.setAttribute('transform', 'scale(' + sc.toFixed(4) + ')');
      } else {
        d.el.setAttribute('transform', 'scale(' + sc.toFixed(4) + ')');
        d.el.setAttribute('opacity', op.toFixed(3));
      }
    } else if (d.kind === 'flash') {
      const t = elapsed - (d.stagger || 0);
      if (t < 0) {
        d.el.setAttribute('opacity', '0');
        return;
      }
      const cyc = d.single ? t : (t % d.interval);
      if ((!d.single || t <= d.dur) && cyc <= d.dur) {
        const u = cyc / d.dur;
        const op = 0.95 * Math.sin(u * Math.PI);
        const sc = u < 0.25 ? (0.85 + (1.08 - 0.85) * (u / 0.25)) : (1.08 - (1.08 - 1.0) * ((u - 0.25) / 0.75));
        d.el.setAttribute('opacity', Math.max(0, op).toFixed(3));
        d.el.setAttribute('transform', 'scale(' + sc.toFixed(4) + ')');
      } else {
        d.el.setAttribute('opacity', '0');
        d.el.setAttribute('transform', 'scale(1)');
      }
    } else if (d.kind === 'particle') {
      if (!d.pathEl) {
        const pRoot = (rootScope && rootScope.getElementById) ? rootScope.getElementById(d.pathId) : document.getElementById(d.pathId);
        d.pathEl = pRoot;
        if (pRoot && typeof pRoot.getTotalLength === 'function') {
          d.totalLen = pRoot.getTotalLength();
        }
      }
      if (d.pathEl && d.totalLen > 0) {
        const p = (elapsed % d.dur) / d.dur;
        const pt = d.pathEl.getPointAtLength(p * d.totalLen);
        d.el.setAttribute('cx', pt.x.toFixed(2));
        d.el.setAttribute('cy', pt.y.toFixed(2));
      }
    }
  };

  const sharedList = [];
  const perInstList = [];
  drivers.forEach(d => {
    const resolved = findPayload(d.id);
    if (!resolved.el) return;
    const item = Object.assign({ el: resolved.el }, d);
    if (d.perInstance) perInstList.push(item);
    else sharedList.push(item);
  });

  if (sharedList.length > 0) {
    const startT = performance.now();
    function stepShared(now) {
      const elapsed = (now - startT) / 1000;
      for (let i = 0; i < sharedList.length; i++) {
        const d = sharedList[i];
        if (!suspendedSet.has(d.id)) applyDriver(d, elapsed);
      }
      requestAnimationFrame(stepShared);
    }
    requestAnimationFrame(stepShared);
  }

  perInstList.forEach(d => {
    const startT = performance.now();
    function stepOne(now) {
      if (!suspendedSet.has(d.id)) {
        applyDriver(d, (now - startT) / 1000);
      }
      requestAnimationFrame(stepOne);
    }
    requestAnimationFrame(stepOne);
  });

  if (dataDrivers.length > 0) {
    let stepIdx = 0;
    setInterval(() => {
      stepIdx++;
      for (let i = 0; i < dataDrivers.length; i++) {
        const dd = dataDrivers[i];
        // Data/telemetry updates continue even when decorative animations are suspended off-screen
        const r = findPayload(dd.id);
        if (r.el) {
          const v = 25 + ((dd.baseVal + stepIdx * 11 + i * 7) % 70);
          r.el.setAttribute('stroke-dasharray', v + ' 100');
        }
      }
    }, 300);
  }
})(typeof document !== 'undefined' ? document : null);
`

// BuildTestPageHTML generates the self-contained HTML harness page for a TestRunSpec.
func BuildTestPageHTML(spec TestRunSpec) string {
	if spec.Isolation == "stage16c-widgets" {
		return buildStage16CWidgetsHTML(spec)
	}
	if spec.Isolation == "shared-dt-scene" {
		return buildDTSceneHTML(spec)
	}
	reg := DefaultRegistry()
	var cssBuf, bodyBuf, jsBuf strings.Builder
	cellW, cellH := computeCellDimensions(spec.Instances)

	offscreenSpacer := ""
	if spec.WorkloadVisibility == "offscreen" {
		offscreenSpacer = `<div class="lab-offscreen-spacer" style="height:2200px;grid-column:1/-1;"></div>`
	}

	isShared := spec.Isolation == "shared-scene" || spec.Isolation == "shared-svg-scene"
	if isShared {
		cols := 3
		if spec.VisibleCount == 0 && spec.WorkloadVisibility != "offscreen" {
			cols = 6
		}
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
		bodyBuf.WriteString(fmt.Sprintf(`%s<div class="lab-shared-wrap" style="grid-column:1/-1;"><svg class="lab-shared-svg" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg></div>`,
			offscreenSpacer, totalW, totalH, totalW, totalH, inner.String()))
	} else {
		if offscreenSpacer != "" {
			bodyBuf.WriteString(offscreenSpacer)
		}
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
	if spec.WorkloadVisibility == "offscreen" {
		visHeightCSS = "#labGrid { max-height: 480px; overflow-y: auto; }"
	} else if spec.VisibleCount > 0 {
		rowsVis := (spec.VisibleCount + 2) / 3
		if rowsVis < 1 {
			rowsVis = 1
		}
		rowPx := cellH + 44
		if isShared {
			rowPx = cellH
		}
		visHeightCSS = fmt.Sprintf("#labGrid { max-height: %dpx; overflow-y: auto; }", rowsVis*rowPx)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab — %s</title>
<style>
  body { margin: 0; padding: 12px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  #labGrid { display: grid; grid-template-columns: repeat(3, %dpx); gap: 10px; }
  .lab-card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 6px; }
  .lab-meta { font-size: 11px; color: #8b949e; margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .lab-card.lab-suspended, .lab-card.lab-suspended *,
  .lab-instance.lab-suspended, .lab-instance.lab-suspended * { animation-play-state: paused !important; }
  %s
  %s
</style>
</head>
<body>
<div id="labHeader" style="font-size:12px;margin-bottom:8px;color:#8b949e;">Run: %s | Instances: %d | Isolation: %s | Scheduling: %s | Workload: %s</div>
<div id="labGrid">%s</div>
<script>
window.__labRafDrivers = [];
window.__labDataDrivers = [];
window.__labLongTasks = [];
window.__labSuspendedSet = new Set();
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
  const isSharedScene = %v;
  const totalCount = %d;
  const suspendedSet = window.__labSuspendedSet;
  const rootEl = document.getElementById('labGrid');

  if (!suspendOffscreen) return;

  if (isSharedScene) {
    // Shared-scene <g class="lab-instance"> visibility via bounding rect vs scroll container
    const updateSharedVisibility = () => {
      const rr = rootEl ? rootEl.getBoundingClientRect() : { top: 0, left: 0, bottom: window.innerHeight, right: window.innerWidth };
      const groups = document.querySelectorAll('.lab-shared-svg .lab-instance');
      groups.forEach(g => {
        const r = g.getBoundingClientRect();
        const visible = (r.bottom > rr.top + 1) && (r.top < rr.bottom - 1) && (r.right > rr.left + 1) && (r.left < rr.right - 1);
        const id = g.id || '';
        if (visible) {
          g.classList.remove('lab-suspended');
          suspendedSet.delete(id);
        } else {
          g.classList.add('lab-suspended');
          suspendedSet.add(id);
        }
      });
      window.__labSuspendedCount = suspendedSet.size;
      window.__labVisibleCount = totalCount - suspendedSet.size;
    };
    updateSharedVisibility();
    if (rootEl) rootEl.addEventListener('scroll', updateSharedVisibility, { passive: true });
    window.addEventListener('scroll', updateSharedVisibility, { passive: true });
    if (typeof ResizeObserver !== 'undefined' && rootEl) {
      new ResizeObserver(updateSharedVisibility).observe(rootEl);
    }
    window.__labRefreshVisibility = updateSharedVisibility;
  } else if (typeof IntersectionObserver !== 'undefined') {
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
      window.__labVisibleCount = totalCount - suspendedSet.size;
    }, { root: rootEl, threshold: 0.01 });
    document.querySelectorAll('.lab-card').forEach(c => io.observe(c));
  }
})();
%s
</script>
</body>
</html>`, spec.RunID, cellW+16, visHeightCSS, cssBuf.String(), spec.RunID, spec.Count, spec.Isolation, spec.SchedulingStrategy, spec.WorkloadName, bodyBuf.String(), spec.Count, jsBuf.String(), spec.SuspendOffscreen, isShared, spec.Count, labSharedRuntimeJS)
}

// BuildStandaloneSampleSVG generates a clean standalone SVG sample for a single InstanceParams,
// embedding a self-contained script driver for javascript-raf / data-driven samples (F-A01).
func BuildStandaloneSampleSVG(p InstanceParams) string {
	reg := DefaultRegistry()
	gen := reg[p.Family]
	if gen == nil {
		return ""
	}
	w, h := computeCellDimensions([]InstanceParams{p})
	if w < 220 {
		w = 220
	}
	if h < 140 {
		h = 140
	}
	re := gen.Render(p, w, h)
	style := ""
	if re.CSS != "" {
		style = fmt.Sprintf("\n  <style>\n%s\n  </style>", re.CSS)
	}
	script := ""
	if re.JSDriver != "" {
		script = fmt.Sprintf(`
  <script type="application/ecmascript"><![CDATA[
(function(){
  var cs = document.currentScript;
  var svg = (cs && (cs.ownerSVGElement || (cs.closest && cs.closest('svg')))) || document.documentElement;
  if (!svg || svg.__labRafStarted) return;
  svg.__labRafStarted = true;
  window.__labRafDrivers = window.__labRafDrivers || [];
  window.__labDataDrivers = window.__labDataDrivers || [];
  window.__labSuspendedSet = window.__labSuspendedSet || new Set();
  %s
  %s
})();
]]></script>`, re.JSDriver, labSharedRuntimeJS)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg id="%s" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s
  <rect width="%d" height="%d" fill="#0d1117" rx="8" />
  %s%s
</svg>`, p.ID, w, h, w, h, style, w, h, re.SVGInner, script)
}
