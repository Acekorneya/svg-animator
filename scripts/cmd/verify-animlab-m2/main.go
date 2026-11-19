package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type checkResult struct {
	ID     string
	Passed bool
	Detail string
}

func main() {
	cdpEndpoint := os.Getenv("ANIMLAB_CDP")
	if cdpEndpoint == "" {
		cdpEndpoint = "http://127.0.0.1:9222"
	}
	cpuHost := os.Getenv("ANIMLAB_CPU_HOST")
	if cpuHost == "" {
		cpuHost = "nigel@192.168.100.40"
	}
	bindIP := os.Getenv("ANIMLAB_BIND_IP")
	if bindIP == "" {
		bindIP = "192.168.100.47"
	}
	evidenceDir := filepath.Join("testlab", "m2-evidence")
	_ = os.MkdirAll(evidenceDir, 0755)

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	baseCfg := func(name string, vw, vh int, dpr float64, reps int, order string) *animlab.Config {
		return &animlab.Config{
			Version: 1,
			Test:    animlab.TestMeta{Name: name, Description: "M2 Measurement Integrity verification"},
			Browser: animlab.BrowserConfig{
				Engine:            "chromium",
				Headless:          true,
				CDPEndpoint:       cdpEndpoint,
				HostBindIP:        bindIP,
				ProcessCPUHost:    cpuHost,
				ViewportWidth:     vw,
				ViewportHeight:    vh,
				DeviceScaleFactor: dpr,
			},
			Execution: animlab.ExecutionConfig{
				WarmupSeconds:   0.3,
				DurationSeconds: 1.2,
				Repetitions:     reps,
				RandomSeed:      2026,
				OrderStrategy:   order,
			},
			Instances: animlab.InstancesConfig{
				Counts:    []int{20},
				Isolation: "independent-svg-roots",
			},
			Effects: []animlab.EffectSpec{
				{
					Family:        "perimeter-tracer",
					Methods:       []string{"css", "smil", "javascript-raf"},
					Shapes:        []string{"rect"},
					Sizes:         map[string][2]int{"small": {50, 50}},
					Speeds:        map[string]float64{"medium": 1.5},
					SegmentCounts: []int{2},
					LineGapRatios: []string{"25:75"},
				},
			},
			Workloads:    []animlab.WorkloadSpec{{Name: "all-active", Visibility: "all"}},
			Measurements: []string{"process_cpu", "frame_timing", "long_tasks", "rendering_activity", "memory", "animation_correctness"},
			Output:       animlab.OutputConfig{Dir: evidenceDir, RawResults: true, HTMLReport: true, MarkdownReport: true, DecisionReport: true},
		}
	}

	// =========================================================================
	// AT-M2-01: Browser process-tree ownership & isolation against unrelated CPU load
	// =========================================================================
	{
		cfg := baseCfg("m2-cpu-isolation", 1024, 768, 1.0, 1, "sequential")
		cfg.Effects = []animlab.EffectSpec{{
			Family: "static-gauge", Methods: []string{"static"},
			Shapes: []string{"circle"}, Sizes: map[string][2]int{"small": {40, 40}}, Speeds: map[string]float64{"medium": 1.0},
		}}
		cfg.Instances.Counts = []int{10}

		resClean, err := animlab.RunSuite(cfg, evidenceDir, "")
		if err != nil {
			record("AT-M2-01", false, fmt.Sprintf("clean RunSuite err: %v", err))
		} else {
			// Start an unrelated CPU-burning chrome-named python/shell process on Kiwiberry
			burnCmd := exec.Command("ssh", "-o", "BatchMode=yes", cpuHost,
				"python3 -c 'import time; t=time.time()+4.0; x=0\nwhile time.time()<t: x+=1' --unrelated-chrome-load >/dev/null 2>&1 & echo $!")
			burnOut, _ := burnCmd.Output()
			burnPID := strings.TrimSpace(string(burnOut))

			resNoisy, err2 := animlab.RunSuite(cfg, evidenceDir, "")
			if burnPID != "" {
				_ = exec.Command("ssh", "-o", "BatchMode=yes", cpuHost, "kill -9 "+burnPID+" 2>/dev/null || true").Run()
			}
			if err2 != nil {
				record("AT-M2-01", false, fmt.Sprintf("noisy RunSuite err: %v", err2))
			} else {
				ownMap, _ := resClean.Environment["browserOwnership"].(animlab.BrowserOwnership)
				cClean := resClean.Scenarios[0].AvgProcessCPUSec
				cNoisy := resNoisy.Scenarios[0].AvgProcessCPUSec
				pids := resClean.Scenarios[0].Repetitions[0].OwnedPIDCount
				ok := ownMap.Mode == "external-cdp-isolated-tree" && ownMap.RootPID > 0 && pids >= 2 &&
					cClean != nil && cNoisy != nil && *cNoisy < 0.35
				c1, c2 := -1.0, -1.0
				if cClean != nil {
					c1 = *cClean
				}
				if cNoisy != nil {
					c2 = *cNoisy
				}
				record("AT-M2-01", ok, fmt.Sprintf("mode=%s rootPID=%d ownedPIDs=%d cleanCPU=%.4fs noisyCPU=%.4fs",
					ownMap.Mode, ownMap.RootPID, pids, c1, c2))
			}
		}
	}

	// =========================================================================
	// AT-M2-02: Distinct viewports & DPR applied and verified in Chromium
	// =========================================================================
	{
		cfgA := baseCfg("m2-vp-a", 1024, 768, 1.0, 1, "sequential")
		cfgA.Effects = cfgA.Effects[:1]
		cfgA.Effects[0].Methods = []string{"css"}
		cfgA.Instances.Counts = []int{5}
		cfgA.Measurements = []string{"frame_timing", "memory"}

		cfgB := baseCfg("m2-vp-b", 1440, 900, 2.0, 1, "sequential")
		cfgB.Effects = cfgB.Effects[:1]
		cfgB.Effects[0].Methods = []string{"css"}
		cfgB.Instances.Counts = []int{5}
		cfgB.Measurements = []string{"frame_timing", "memory"}

		resA, errA := animlab.RunSuite(cfgA, evidenceDir, "")
		resB, errB := animlab.RunSuite(cfgB, evidenceDir, "")
		ok := errA == nil && errB == nil &&
			resA.Scenarios[0].ActualViewportWidth == 1024 && resA.Scenarios[0].ActualViewportHeight == 768 && resA.Scenarios[0].DevicePixelRatio == 1.0 &&
			resB.Scenarios[0].ActualViewportWidth == 1440 && resB.Scenarios[0].ActualViewportHeight == 900 && resB.Scenarios[0].DevicePixelRatio == 2.0
		detail := fmt.Sprintf("A=%dx%d@%.1f B=%dx%d@%.1f",
			resA.Scenarios[0].ActualViewportWidth, resA.Scenarios[0].ActualViewportHeight, resA.Scenarios[0].DevicePixelRatio,
			resB.Scenarios[0].ActualViewportWidth, resB.Scenarios[0].ActualViewportHeight, resB.Scenarios[0].DevicePixelRatio)
		record("AT-M2-02", ok, detail)
	}

	// =========================================================================
	// AT-M2-03, AT-M2-04, AT-M2-05, AT-M2-06, AT-M2-07:
	// Repeated-trial DOM stability, alternating order, metric definitions/naming,
	// environment metadata, and mean-based decision reports
	// =========================================================================
	{
		cfg := baseCfg("m2-full-suite", 1280, 720, 1.0, 3, "alternating")
		cfg.Instances.Counts = []int{40}
		res, err := animlab.RunSuite(cfg, evidenceDir, "")
		if err != nil {
			record("AT-M2-03", false, err.Error())
		} else {
			_ = animlab.WriteAllOutputs(cfg, res, evidenceDir)
			if b, err := json.MarshalIndent(res, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(evidenceDir, "raw-results-sample.json"), b, 0644)
			}

			// AT-M2-03: Check DOM stability across 3 repetitions of 40-instance scenarios
			maxDomSpread := int64(0)
			for _, sc := range res.Scenarios {
				var minD, maxD int64 = math.MaxInt64, 0
				for _, r := range sc.Repetitions {
					if r.DOMNodes != nil {
						if *r.DOMNodes < minD {
							minD = *r.DOMNodes
						}
						if *r.DOMNodes > maxD {
							maxD = *r.DOMNodes
						}
					}
				}
				if maxD >= minD && (maxD-minD) > maxDomSpread {
					maxDomSpread = maxD - minD
				}
			}
			record("AT-M2-03", maxDomSpread <= 10, fmt.Sprintf("maxDomNodeSpreadAcrossReps=%d", maxDomSpread))

			// AT-M2-04: Verify alternating execution order recorded across 3 reps
			ordOK := len(res.ExecutionOrders) == 3 &&
				res.ExecutionOrders[0][0] != res.ExecutionOrders[1][0] &&
				res.ExecutionOrders[1][0] != res.ExecutionOrders[2][0]
			record("AT-M2-04", ordOK, fmt.Sprintf("orders=%v", res.ExecutionOrders))

			// AT-M2-05: Verify rAF Hz vs PresentedFPS=nil and MetricDefinitions
			_, hasRafDef := res.MetricDefinitions["rafHz"]
			_, hasPresDef := res.MetricDefinitions["presentedFps"]
			_, has16Def := res.MetricDefinitions["longFramesOver16ms"]
			_, has20Def := res.MetricDefinitions["longFramesOver20ms"]
			sc0 := res.Scenarios[0]
			metricsOK := hasRafDef && hasPresDef && has16Def && has20Def &&
				sc0.AvgRafHz != nil && *sc0.AvgRafHz > 0 && sc0.PresentedFPS == nil && sc0.RafHzStats != nil
			record("AT-M2-05", metricsOK, fmt.Sprintf("rafHz=%.1f presentedFps=%v defs=%d", *sc0.AvgRafHz, sc0.PresentedFPS, len(res.MetricDefinitions)))

			// AT-M2-06: Environment metadata completeness
			env := res.Environment
			envOK := env["measurementSpecVersion"] == animlab.MeasurementSpecVersion &&
				env["userAgent"] != "" && env["webglRenderer"] != nil &&
				env["isSoftwareRenderer"] != nil && env["targetOS"] != nil && env["clkTck"] != nil
			record("AT-M2-06", envOK, fmt.Sprintf("renderer=%v sw=%v targetOS=%v clkTck=%v",
				env["webglRenderer"], env["isSoftwareRenderer"], env["targetOS"], env["clkTck"]))

			// AT-M2-07: Decision report & markdown report formatting
			decBytes, _ := os.ReadFile(filepath.Join(evidenceDir, "decision-perimeter-tracer.md"))
			repBytes, _ := os.ReadFile(filepath.Join(evidenceDir, "report.md"))
			decOK := strings.Contains(string(decBytes), "Mean rAF (Hz)") &&
				strings.Contains(string(repBytes), "rAF (Hz)") &&
				strings.Contains(string(repBytes), "| N/A |")
			record("AT-M2-07", decOK, "mean-based decision report & N/A Presented FPS verified")
		}
	}

	// =========================================================================
	// NC-09: Unavailable CPU host & dead PID must report N/A (nil), never 0.0
	// =========================================================================
	{
		cfgBad := baseCfg("m2-nc09-unavail", 800, 600, 1.0, 1, "sequential")
		cfgBad.Browser.ProcessCPUHost = "nonexistent-host-invalid.local"
		cfgBad.Effects = cfgBad.Effects[:1]
		cfgBad.Effects[0].Methods = []string{"css"}
		cfgBad.Instances.Counts = []int{5}

		resBad, err := animlab.RunSuite(cfgBad, evidenceDir, "")
		if err != nil {
			record("NC-09", false, fmt.Sprintf("RunSuite err: %v", err))
		} else {
			sc := resBad.Scenarios[0]
			rep0 := sc.Repetitions[0]
			md := animlab.BuildMarkdownReport(cfgBad, resBad)
			// Dead PID direct probe check
			allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
			ctx, cCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Run(ctx, chromedp.Navigate("about:blank"))
			deadOwn := animlab.BrowserOwnership{Mode: "external-cdp-isolated-tree", RootPID: 99999999, ClkTck: 100}
			deadCPU, _, deadErr := animlab.SampleOwnedProcessTreeCPU(deadOwn, cpuHost)
			cCancel()
			aCancel()
			_ = time.Millisecond // keep time used

			ok := rep0.ProcessCPUSec == nil && sc.AvgProcessCPUSec == nil &&
				rep0.Status == animlab.TrialPartial &&
				strings.Contains(md, "| N/A |") &&
				deadCPU == nil && deadErr != nil
			record("NC-09", ok, fmt.Sprintf("unavailHostCPU=%v status=%s deadPIDerr=%v", rep0.ProcessCPUSec, rep0.Status, deadErr))
		}
	}

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d M2 checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
