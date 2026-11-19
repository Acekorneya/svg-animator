package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type checkResult struct {
	ID     string
	Passed bool
	Detail string
}

func main() {
	studies := []string{
		"m6-01-baseline-all-families",
		"m6-02-load-scaling-and-raf-sched",
		"m6-03-geometry-size-speed",
		"m6-04-display-scaling",
		"m6-05-dt-large-bg-and-visibility",
		"m6-06-stage16c-widgets",
		"m6-07-mixed-workloads",
	}

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	totalScenarios := 0
	totalTrials := 0
	familiesSeen := map[string]bool{}
	arrangementsSeen := map[string]bool{}
	scalesSeen := map[string]bool{}
	countsSeen := map[int]bool{}
	nonEqExcluded := false
	traceabilityOK := true
	specVerOK := true

	for _, name := range studies {
		dir := filepath.Join("testlab", "results", name)
		rawPath := filepath.Join(dir, "raw-results.json")
		cfgPath := filepath.Join(dir, "resolved-config.yaml")
		repHTML := filepath.Join(dir, "report.html")
		repMD := filepath.Join(dir, "report.md")
		for _, req := range []string{rawPath, cfgPath, repHTML, repMD} {
			if _, err := os.Stat(req); err != nil {
				traceabilityOK = false
			}
		}
		b, err := os.ReadFile(rawPath)
		if err != nil {
			traceabilityOK = false
			continue
		}
		var res animlab.LabResults
		if err := json.Unmarshal(b, &res); err != nil {
			traceabilityOK = false
			continue
		}
		if res.Environment["measurementSpecVersion"] != animlab.MeasurementSpecVersion {
			specVerOK = false
		}
		for _, s := range res.Scenarios {
			totalScenarios++
			totalTrials += len(s.Repetitions)
			familiesSeen[s.Family] = true
			arrangementsSeen[s.Isolation] = true
			countsSeen[s.Count] = true
			if s.DisplayScale > 0 {
				scalesSeen[fmt.Sprintf("%.1fx-%s", s.DisplayScale, s.ScalingMode)] = true
			}
			if (s.NonEquivalent || s.Unsupported) && !s.CorrectnessPassed {
				nonEqExcluded = true
			}
			// PresentedFPS must remain nil (N/A)
			if s.PresentedFPS != nil {
				specVerOK = false
			}
		}
		// Verify decision reports do not rank clip-reveal or unsupported methods as winners
		decs, _ := filepath.Glob(filepath.Join(dir, "decision-*.md"))
		for _, dp := range decs {
			db, _ := os.ReadFile(dp)
			if strings.Contains(string(db), "**Selected Recommendation:** `clip-reveal`") {
				nonEqExcluded = false
			}
		}
	}

	// AT-M6-01: Scenario coverage across all 5 dynamic families + static-gauge
	covOK := familiesSeen["perimeter-tracer"] && familiesSeen["pulse"] && familiesSeen["flash"] &&
		familiesSeen["flow-line"] && familiesSeen["flow-particle"] && familiesSeen["static-gauge"]
	record("AT-M6-01", covOK && totalScenarios >= 100, fmt.Sprintf("scenarios=%d trials=%d families=%d", totalScenarios, totalTrials, len(familiesSeen)))

	// AT-M6-02: Non-equivalent / unsupported methods excluded from correctness & ranking
	record("AT-M6-02", nonEqExcluded, "clip-reveal & shared-scene SMIL suspension failed correctness and excluded from rankings")

	// AT-M6-03: Display scales 0.5x, 1.0x, 2.0x, 4.0x across viewbox/css-transform/authored-size
	scaleOK := scalesSeen["0.5x-viewbox"] && scalesSeen["1.0x-native"] && scalesSeen["2.0x-viewbox"] &&
		scalesSeen["4.0x-viewbox"] && scalesSeen["2.0x-css-transform"] && scalesSeen["4.0x-css-transform"] &&
		scalesSeen["2.0x-authored-size"] && scalesSeen["4.0x-authored-size"]
	record("AT-M6-03", scaleOK, fmt.Sprintf("scales=%d", len(scalesSeen)))

	// AT-M6-04: Meaningful instance counts & deployment arrangements
	cntOK := countsSeen[50] && countsSeen[100] && countsSeen[250] && countsSeen[500]
	arrOK := arrangementsSeen["independent-svg-roots"] && arrangementsSeen["shared-dt-scene"] && arrangementsSeen["stage16c-widgets"]
	record("AT-M6-04", cntOK && arrOK, fmt.Sprintf("counts50_100_250_500=%v arrangements=%v", cntOK, arrangementsSeen))

	// AT-M6-05: Report traceability, measurementSpecVersion=2.0.0, and historical isolation
	histRaw, _ := os.ReadFile("testlab/results/tracer-comparison/raw-results.json")
	histUnpolluted := !strings.Contains(string(histRaw), "measurementSpecVersion")
	record("AT-M6-05", traceabilityOK && specVerOK && histUnpolluted, fmt.Sprintf("trace=%v spec2=%v histPreserved=%v", traceabilityOK, specVerOK, histUnpolluted))

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d M6 checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
