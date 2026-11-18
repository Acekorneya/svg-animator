package animlab

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WriteAllOutputs writes resolved-config.yaml, raw-results.json, Markdown/HTML reports,
// per-effect Decision Reports, and the Visual Demonstration Catalogue.
func WriteAllOutputs(cfg *Config, res *LabResults, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// 1. Always preserve exact resolved YAML configuration (self-contained, inheritance already resolved)
	resolvedCfg := *cfg
	resolvedCfg.Inherits = ""
	resolvedBytes, err := yaml.Marshal(&resolvedCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "resolved-config.yaml"), resolvedBytes, 0644); err != nil {
		return err
	}

	// 2. Raw machine-readable JSON results
	if cfg.Output.RawResults && res != nil {
		raw, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, "raw-results.json"), raw, 0644); err != nil {
			return err
		}
	}

	// 3. Visual Samples & Browsable Catalogue
	var sampleFiles []SampleEntry
	if cfg.Output.Samples {
		var err error
		sampleFiles, err = GenerateVisualCatalogue(cfg, outDir, res != nil)
		if err != nil {
			return err
		}
	}

	if res == nil {
		return nil
	}

	// 4. Markdown Report
	if cfg.Output.MarkdownReport {
		md := BuildMarkdownReport(cfg, res)
		if err := os.WriteFile(filepath.Join(outDir, "report.md"), []byte(md), 0644); err != nil {
			return err
		}
	}

	// 5. HTML Report
	if cfg.Output.HTMLReport {
		h := BuildHTMLReport(cfg, res, sampleFiles)
		if err := os.WriteFile(filepath.Join(outDir, "report.html"), []byte(h), 0644); err != nil {
			return err
		}
	}

	// 6. Per-Effect Decision Reports
	if cfg.Output.DecisionReport {
		if err := WriteDecisionReports(cfg, res, outDir); err != nil {
			return err
		}
	}

	return nil
}

type SampleEntry struct {
	Filename   string
	Family     string
	Method     string
	Shape      string
	Size       string
	Speed      string
	Limitation string
}

// GenerateVisualCatalogue writes standalone SVG samples and a browsable HTML catalogue.
func GenerateVisualCatalogue(cfg *Config, outDir string, hasReport bool) ([]SampleEntry, error) {
	sampDir := filepath.Join(outDir, "samples")
	catDir := filepath.Join(outDir, "catalogue")
	if err := os.MkdirAll(sampDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(catDir, 0755); err != nil {
		return nil, err
	}

	reg := DefaultRegistry()
	var entries []SampleEntry
	idx := 0
	for _, eff := range cfg.Effects {
		gen := reg[eff.Family]
		if gen == nil {
			continue
		}
		sizeKeys := sortedMapKeys(eff.Sizes)
		speedKeys := sortedMapKeys(eff.Speeds)
		for _, m := range eff.Methods {
			for _, sh := range eff.Shapes {
				for _, szK := range sizeKeys {
					sz := eff.Sizes[szK]
					for _, spK := range speedKeys {
						idx++
						sp := eff.Speeds[spK]
						seg := 2
						if len(eff.SegmentCounts) > 0 {
							seg = eff.SegmentCounts[0]
						}
						ratio := "1:3"
						if len(eff.LineGapRatios) > 0 {
							ratio = eff.LineGapRatios[0]
						}
						pMode := "scale-opacity"
						if len(eff.PulseModes) > 0 {
							pMode = eff.PulseModes[0]
						}
						pAmp := 0.2
						if len(eff.PulseAmplitudes) > 0 {
							pAmp = eff.PulseAmplitudes[0]
						}
						fMode := "repeating"
						if len(eff.FlashModes) > 0 {
							fMode = eff.FlashModes[0]
						}
						ip := InstanceParams{
							ID:             fmt.Sprintf("samp_%d", idx),
							Family:         eff.Family,
							Method:         m,
							Shape:          sh,
							Width:          sz[0],
							Height:         sz[1],
							Speed:          sp,
							Segments:       seg,
							LineGapRatio:   ratio,
							Direction:      "cw",
							PulseMode:      pMode,
							PulseAmplitude: pAmp,
							FlashMode:      fMode,
							FlashDurMs:     eff.FlashDurationMs,
							FlashIntervalS: eff.FlashIntervalS,
							Color:          palette[(idx-1)%len(palette)],
						}
						re := gen.Render(ip, 220, 140)
						svg := BuildStandaloneSampleSVG(ip)
						fname := fmt.Sprintf("%s__%s__%s__%s__%s.svg", eff.Family, m, sh, szK, spK)
						if err := os.WriteFile(filepath.Join(sampDir, fname), []byte(svg), 0644); err != nil {
							return nil, err
						}
						entries = append(entries, SampleEntry{
							Filename:   fname,
							Family:     eff.Family,
							Method:     m,
							Shape:      sh,
							Size:       fmt.Sprintf("%s (%dx%d)", szK, sz[0], sz[1]),
							Speed:      fmt.Sprintf("%s (%.2fs)", spK, sp),
							Limitation: re.Limitation,
						})
					}
				}
			}
		}
	}

	var cards strings.Builder
	for _, e := range entries {
		limBadge := ""
		if e.Limitation != "" {
			limBadge = fmt.Sprintf(`<div class="lim">%s</div>`, html.EscapeString(e.Limitation))
		}
		cards.WriteString(fmt.Sprintf(`<div class="card" data-family="%s" data-method="%s" data-shape="%s">
  <div class="title">%s &middot; <strong>%s</strong></div>
  <div class="meta">%s | %s | %s</div>
  <object type="image/svg+xml" data="../samples/%s" width="220" height="140"></object>
  <div><a href="../samples/%s" target="_blank">Open Standalone SVG</a></div>
  %s
</div>`, html.EscapeString(e.Family), html.EscapeString(e.Method), html.EscapeString(e.Shape),
			html.EscapeString(e.Family), html.EscapeString(e.Method),
			html.EscapeString(e.Shape), html.EscapeString(e.Size), html.EscapeString(e.Speed),
			html.EscapeString(e.Filename), html.EscapeString(e.Filename), limBadge))
	}

	reportLink := ""
	if hasReport {
		reportLink = `<a href="../report.html" style="color:#58a6ff;margin-left:16px;">View Comparison Report &rarr;</a>`
	}

	catHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8" /><title>Animation Test Lab — Visual Catalogue</title>
<style>
  body { margin: 0; padding: 20px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  h1 { margin: 0 0 8px; font-size: 20px; }
  .bar { margin: 12px 0 18px; display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
  select { background: #161b22; color: #c9d1d9; border: 1px solid #30363d; padding: 6px 10px; border-radius: 6px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(245px, 1fr)); gap: 14px; }
  .card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 10px; }
  .title { font-size: 13px; margin-bottom: 2px; }
  .meta { font-size: 11px; color: #8b949e; margin-bottom: 8px; }
  .lim { margin-top: 6px; font-size: 11px; color: #f59e0b; }
  a { color: #58a6ff; font-size: 11px; text-decoration: none; }
</style></head><body>
<h1>SVG Animation Test Lab — Visual Sample Catalogue (%s)</h1>
<div class="bar">
  <label>Family: <select id="fFam"><option value="">All</option></select></label>
  <label>Method: <select id="fMet"><option value="">All</option></select></label>
  <label>Shape: <select id="fShp"><option value="">All</option></select></label>
  <a href="../resolved-config.yaml">resolved-config.yaml</a>%s
</div>
<div class="grid" id="grid">%s</div>
<script>
const cards = Array.from(document.querySelectorAll('.card'));
const fill = (id, attr) => {
  const sel = document.getElementById(id);
  [...new Set(cards.map(c => c.getAttribute(attr)))].sort().forEach(v => {
    const o = document.createElement('option'); o.value = v; o.textContent = v; sel.appendChild(o);
  });
};
fill('fFam', 'data-family'); fill('fMet', 'data-method'); fill('fShp', 'data-shape');
function apply() {
  const f = fFam.value, m = fMet.value, s = fShp.value;
  cards.forEach(c => {
    const ok = (!f || c.dataset.family === f) && (!m || c.dataset.method === m) && (!s || c.dataset.shape === s);
    c.style.display = ok ? '' : 'none';
  });
}
fFam.onchange = fMet.onchange = fShp.onchange = apply;
</script></body></html>`, html.EscapeString(cfg.Test.Name), reportLink, cards.String())

	if err := os.WriteFile(filepath.Join(catDir, "index.html"), []byte(catHTML), 0644); err != nil {
		return nil, err
	}
	return entries, nil
}

// BuildMarkdownReport builds a comprehensive Markdown comparison report.
func BuildMarkdownReport(cfg *Config, res *LabResults) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SVG Animation Test Lab Report — `%s`\n\n", res.TestName))
	if res.Description != "" {
		sb.WriteString(res.Description + "\n\n")
	}
	sb.WriteString(fmt.Sprintf("- **Timestamp:** `%s`\n", res.Timestamp))
	sb.WriteString(fmt.Sprintf("- **Browser:** `%v` (`hardwareConcurrency: %v`)\n", res.Environment["userAgent"], res.Environment["hardwareConcurrency"]))
	sb.WriteString(fmt.Sprintf("- **Execution:** warmup=`%.1fs`, duration=`%.1fs`, repetitions=`%d`, alternating=`%v`, seed=`%d`\n",
		cfg.Execution.WarmupSeconds, cfg.Execution.DurationSeconds, cfg.Execution.Repetitions, cfg.Execution.AlternateMethodOrder, cfg.Execution.RandomSeed))
	sb.WriteString(fmt.Sprintf("- **Note:** %v\n\n", res.Environment["gpuNote"]))

	sb.WriteString("## Results Matrix\n\n")
	sb.WriteString("| Run ID | Family | Method | Workload | Count | Correctness | FPS | Avg (ms) | P95 (ms) | Chrome CPU (s) | Task (ms) | Heap (MB) | Long Tasks |\n")
	sb.WriteString("|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, s := range res.Scenarios {
		corr := "PASS"
		if !s.CorrectnessPassed {
			corr = "FAIL (" + s.CorrectnessNote + ")"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | %s | %d | %s | %.1f | %.2f | %.2f | %.3f | %.2f | %.1f | %d |\n",
			s.RunID, s.Family, s.Method, s.WorkloadName, s.Count, corr, s.AvgFPS, s.AvgFrameMs, s.P95FrameMs, s.AvgProcessCPUSec, s.AvgTaskMs, s.AvgHeapMB, s.LongTasks))
	}
	sb.WriteString("\n")

	var limNotes []string
	for _, s := range res.Scenarios {
		for _, l := range s.Limitations {
			limNotes = append(limNotes, fmt.Sprintf("- **%s (%s):** %s", s.Family, s.Method, l))
		}
	}
	if len(limNotes) > 0 {
		sb.WriteString("## Method Limitations Observed\n\n")
		sb.WriteString(strings.Join(limNotes, "\n") + "\n\n")
	}
	return sb.String()
}

// BuildHTMLReport builds a clean HTML comparison report.
func BuildHTMLReport(cfg *Config, res *LabResults, samples []SampleEntry) string {
	var rows strings.Builder
	for _, s := range res.Scenarios {
		badge := `<span class="pass">PASS</span>`
		if !s.CorrectnessPassed {
			badge = fmt.Sprintf(`<span class="fail">FAIL: %s</span>`, html.EscapeString(s.CorrectnessNote))
		}
		shot := "-"
		if s.ScreenshotPath != "" {
			shot = fmt.Sprintf(`<a href="%s" target="_blank">screenshot</a>`, html.EscapeString(s.ScreenshotPath))
		}
		rows.WriteString(fmt.Sprintf(`<tr>
  <td><code>%s</code></td><td>%s</td><td><code>%s</code></td><td>%s</td><td>%d</td>
  <td>%s</td><td>%.1f</td><td>%.2f</td><td>%.2f</td><td>%.3f</td><td>%.2f</td><td>%.1f</td><td>%d</td><td>%s</td>
</tr>`, html.EscapeString(s.RunID), html.EscapeString(s.Family), html.EscapeString(s.Method),
			html.EscapeString(s.WorkloadName), s.Count, badge, s.AvgFPS, s.AvgFrameMs, s.P95FrameMs,
			s.AvgProcessCPUSec, s.AvgTaskMs, s.AvgHeapMB, s.LongTasks, shot))
	}
	catLink := ""
	if len(samples) > 0 {
		catLink = `<a href="catalogue/index.html">Open Visual Sample Catalogue &rarr;</a>`
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8" /><title>Animation Test Lab Report — %s</title>
<style>
  body { margin: 0; padding: 20px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  h1 { margin: 0 0 6px; font-size: 20px; }
  .meta { font-size: 12px; color: #8b949e; margin-bottom: 14px; }
  table { width: 100%%; border-collapse: collapse; font-size: 12px; }
  th, td { border: 1px solid #30363d; padding: 6px 8px; text-align: left; }
  th { background: #161b22; }
  .pass { color: #3fb950; font-weight: 600; }
  .fail { color: #f85149; font-weight: 600; }
  a { color: #58a6ff; text-decoration: none; }
</style></head><body>
<h1>SVG Animation Test Lab Report — %s</h1>
<div class="meta">%s | %s | %s</div>
<table>
  <thead><tr><th>Run ID</th><th>Family</th><th>Method</th><th>Workload</th><th>Count</th><th>Correctness</th><th>FPS</th><th>Avg (ms)</th><th>P95 (ms)</th><th>CPU (s)</th><th>Task (ms)</th><th>Heap (MB)</th><th>Long Tasks</th><th>Sample</th></tr></thead>
  <tbody>%s</tbody>
</table>
</body></html>`, html.EscapeString(res.TestName), html.EscapeString(res.TestName),
		html.EscapeString(res.Timestamp), html.EscapeString(fmt.Sprint(res.Environment["userAgent"])), catLink, rows.String())
}

// WriteDecisionReports generates one evidence-based decision report per tested effect family.
func WriteDecisionReports(cfg *Config, res *LabResults, outDir string) error {
	byFamily := map[string][]ScenarioReport{}
	for _, s := range res.Scenarios {
		byFamily[s.Family] = append(byFamily[s.Family], s)
	}
	fams := sortedMapKeys(byFamily)
	for _, fam := range fams {
		list := byFamily[fam]
		// Group by method
		type agg struct {
			method   string
			runs     int
			passed   int
			fpsSum   float64
			taskSum  float64
			cpuSum   float64
			lims     []string
		}
		mAgg := map[string]*agg{}
		for _, s := range list {
			a, ok := mAgg[s.Method]
			if !ok {
				a = &agg{method: s.Method}
				mAgg[s.Method] = a
			}
			a.runs++
			if s.CorrectnessPassed {
				a.passed++
			}
			a.fpsSum += s.AvgFPS
			a.taskSum += s.AvgTaskMs
			a.cpuSum += s.AvgProcessCPUSec
			a.lims = append(a.lims, s.Limitations...)
		}

		// Rank only methods that passed 100% of correctness checks
		var candidates []*agg
		for _, a := range mAgg {
			if a.passed == a.runs && a.method != "static" {
				candidates = append(candidates, a)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			// Prefer higher avg FPS, then lower main-thread task duration, then lower CPU
			if math.Abs(candidates[i].fpsSum-candidates[j].fpsSum) > 1.0 {
				return candidates[i].fpsSum > candidates[j].fpsSum
			}
			if math.Abs(candidates[i].taskSum-candidates[j].taskSum) > 5.0 {
				return candidates[i].taskSum < candidates[j].taskSum
			}
			return candidates[i].cpuSum < candidates[j].cpuSum
		})

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Decision Report — `%s`\n\n", fam))
		sb.WriteString("## 1. Tested Methods & Correctness\n\n")
		sb.WriteString("| Method | Trials | Correctness Passed | Avg FPS | Avg Main-Thread Task (ms) | Avg Chrome CPU (s) | Limitations |\n")
		sb.WriteString("|---|---:|---:|---:|---:|---:|---|\n")
		for _, m := range sortedMapKeys(mAgg) {
			a := mAgg[m]
			lim := "-"
			if len(a.lims) > 0 {
				lim = a.lims[0]
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %d | %d/%d | %.1f | %.2f | %.3f | %s |\n",
				a.method, a.runs, a.passed, a.runs, a.fpsSum/float64(a.runs), a.taskSum/float64(a.runs), a.cpuSum/float64(a.runs), lim))
		}
		sb.WriteString("\n## 2. Assessment & Recommendation\n\n")
		if len(candidates) == 0 {
			sb.WriteString("- **Recommendation:** Inconclusive — no dynamic rendering method passed 100% of correctness checks.\n")
		} else {
			best := candidates[0]
			sb.WriteString(fmt.Sprintf("- **Selected Recommendation:** `%s` (passed %d/%d correctness checks; %.1f avg FPS; %.2f ms main-thread task work; %.3f s Chrome process-tree CPU).\n",
				best.method, best.passed, best.runs, best.fpsSum/float64(best.runs), best.taskSum/float64(best.runs), best.cpuSum/float64(best.runs)))
			if len(candidates) > 1 {
				sb.WriteString("- **Alternatives Considered:**\n")
				for _, alt := range candidates[1:] {
					sb.WriteString(fmt.Sprintf("  - `%s`: %.1f avg FPS, %.2f ms main-thread task, %.3f s CPU.\n",
						alt.method, alt.fpsSum/float64(alt.runs), alt.taskSum/float64(alt.runs), alt.cpuSum/float64(alt.runs)))
				}
			}
		}
		sb.WriteString("\n- **Export & Browser Compatibility:** CSS `@keyframes` and SMIL `<animate>` work in standalone SVG documents without JavaScript; `javascript-raf` requires a host script runtime. Notice this report does not automatically modify Studio.\n")

		fname := fmt.Sprintf("decision-%s.md", strings.ReplaceAll(fam, " ", "-"))
		if err := os.WriteFile(filepath.Join(outDir, fname), []byte(sb.String()), 0644); err != nil {
			return err
		}
	}
	return nil
}
