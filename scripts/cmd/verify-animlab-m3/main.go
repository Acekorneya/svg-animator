package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	bindIP := os.Getenv("ANIMLAB_BIND_IP")
	if bindIP == "" {
		bindIP = "192.168.100.47"
	}
	evidenceDir := filepath.Join("testlab", "m3-evidence")
	_ = os.MkdirAll(evidenceDir, 0755)

	var (
		mu    sync.RWMutex
		pages = map[string]string{}
	)
	ln, err := net.Listen("tcp", bindIP+":0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(r.URL.Path, "/")
			mu.RLock()
			body, ok := pages[name]
			mu.RUnlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(body))
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	baseURL := fmt.Sprintf("http://%s/", ln.Addr().String())

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	// =========================================================================
	// AT-M3-01 & AT-M3-02: Validate all testlab/configs/*.yaml
	// =========================================================================
	{
		cfgs, _ := filepath.Glob("testlab/configs/*.yaml")
		allOK := len(cfgs) >= 8
		var notes []string
		for _, cp := range cfgs {
			if filepath.Base(cp) == "base-kiwiberry.yaml" {
				continue
			}
			if _, err := animlab.LoadConfig(cp); err != nil {
				allOK = false
				notes = append(notes, fmt.Sprintf("%s:%v", filepath.Base(cp), err))
			}
		}
		record("AT-M3-01", allOK, fmt.Sprintf("validated %d YAML configs failures=%v", len(cfgs), notes))
	}

	// =========================================================================
	// AT-M3-03: Isolated & factorial variation + de-correlated instances
	// =========================================================================
	{
		cfgSize, err1 := animlab.LoadConfig("testlab/configs/isolated-size-study.yaml")
		cfgSpeed, err2 := animlab.LoadConfig("testlab/configs/isolated-speed-study.yaml")
		cfgPreset, err3 := animlab.LoadConfig("testlab/configs/preset-override-example.yaml")
		spSize := animlab.ExpandScenarios(cfgSize)
		spSpeed := animlab.ExpandScenarios(cfgSpeed)
		spPreset := animlab.ExpandScenarios(cfgPreset)
		// 3 sizes * 3 methods = 9; 3 speeds * 3 methods = 9
		ok := err1 == nil && err2 == nil && err3 == nil &&
			len(spSize) == 9 && len(spSpeed) == 9 &&
			cfgPreset.Execution.WarmupSeconds == 0 && len(spPreset) == 2
		record("AT-M3-02", err3 == nil && cfgPreset.Execution.WarmupSeconds == 0 && len(spPreset) == 2,
			fmt.Sprintf("presetWarmup=%.1f scenarios=%d", cfgPreset.Execution.WarmupSeconds, len(spPreset)))
		record("AT-M3-03", ok, fmt.Sprintf("isolatedSize=%d isolatedSpeed=%d", len(spSize), len(spSpeed)))
	}

	// =========================================================================
	// AT-M3-04: Mixed workload exact allocation & seed reproducibility
	// =========================================================================
	{
		cfgMix, err := animlab.LoadConfig("testlab/configs/mixed-dashboard.yaml")
		sp1 := animlab.ExpandScenarios(cfgMix)
		sp2 := animlab.ExpandScenarios(cfgMix)
		var mixSpec *animlab.TestRunSpec
		for i := range sp1 {
			if sp1[i].IsMixture && sp1[i].Method == "css" {
				mixSpec = &sp1[i]
				break
			}
		}
		sumAlloc := 0
		if mixSpec != nil {
			for _, v := range mixSpec.MixtureAllocation {
				sumAlloc += v
			}
		}
		repro := err == nil && mixSpec != nil && sumAlloc == mixSpec.Count &&
			len(sp1) == len(sp2) && sp1[0].Instances[0].Shape == sp2[0].Instances[0].Shape
		record("AT-M3-04", repro, fmt.Sprintf("mixCount=%d alloc=%v", mixSpec.Count, mixSpec.MixtureAllocation))
	}

	// =========================================================================
	// AT-M3-04 & AT-M3-05: Shared-scene visibility, suspension, resumption & data-driven telemetry
	// =========================================================================
	{
		cfgSS, err := animlab.LoadConfig("testlab/configs/shared-scene-visibility.yaml")
		if err != nil {
			record("AT-M3-05", false, err.Error())
		} else {
			specs := animlab.ExpandScenarios(cfgSS)
			allOK := true
			var notes []string
			for _, s := range specs {
				mu.Lock()
				pages[s.RunID+".html"] = animlab.BuildTestPageHTML(s)
				mu.Unlock()

				tCtx, tCancel := chromedp.NewContext(allocCtx)
				_, _ = chromedp.Run(tCtx, chromedp.Navigate(baseURL+s.RunID+".html"))
				_, _ = chromedp.Run(tCtx, chromedp.Sleep(250*time.Millisecond))
				corr := animlab.EvaluateCorrectness(tCtx, s)
				// Also verify actual DOM bounding-rect / class visibility counts match configured VisibleCount (9)
				domVisCnt, _ := chromedp.Run(tCtx, chromedp.Evaluate[int](`(() => {
					const rr = document.getElementById('labGrid').getBoundingClientRect();
					return Array.from(document.querySelectorAll('.lab-shared-svg .lab-instance')).filter(g => {
						const r = g.getBoundingClientRect();
						return !g.classList.contains('lab-suspended') && r.bottom > rr.top + 1 && r.top < rr.bottom - 1;
					}).length;
				})()`))
				domSuspCnt, _ := chromedp.Run(tCtx, chromedp.Evaluate[int](`document.querySelectorAll('.lab-shared-svg .lab-instance.lab-suspended').length`))
				tCancel()
				if !corr.Passed || domVisCnt != 9 || domSuspCnt != 27 || corr.VisibleCount != 9 || corr.SuspendedCount != 27 {
					allOK = false
					notes = append(notes, fmt.Sprintf("%s:passed=%v,domVis=%d,domSusp=%d(%s)",
						s.Method, corr.Passed, domVisCnt, domSuspCnt, corr.Reason))
				} else {
					notes = append(notes, fmt.Sprintf("%s:domVis=%d,domSusp=%d", s.Method, domVisCnt, domSuspCnt))
				}
			}

			// Verify shared-scene SMIL with suspend_offscreen=true is marked Unsupported and fails correctness
			smilCfg := *cfgSS
			smilCfg.Effects = append([]animlab.EffectSpec(nil), cfgSS.Effects...)
			smilCfg.Effects[0].Methods = []string{"smil"}
			smilSpecs := animlab.ExpandScenarios(&smilCfg)
			smilCorr := animlab.EvaluateCorrectness(context.Background(), smilSpecs[0])
			if !smilSpecs[0].Unsupported || smilCorr.Passed || smilCorr.FaultCode != string(animlab.FaultUnsupportedOrNonEquivalent) {
				allOK = false
				notes = append(notes, fmt.Sprintf("smil-shared:unsup=%v,fault=%s", smilSpecs[0].Unsupported, smilCorr.FaultCode))
			} else {
				notes = append(notes, "smil-shared:UNSUPPORTED_OR_NON_EQUIVALENT")
			}

			// Verify data-driven telemetry continues updating even when off-screen (suspended)
			ddSpec := animlab.TestRunSpec{
				RunID: "dd-offscreen", Family: "static-gauge", Method: "data-driven",
				WorkloadName: "offscreen", WorkloadVisibility: "offscreen",
				Count: 3, Isolation: "shared-scene", SuspendOffscreen: true,
				Instances: []animlab.InstanceParams{
					{ID: "dd_1", Family: "static-gauge", Method: "data-driven", Shape: "circle", Width: 60, Height: 60, Color: "#38bdf8"},
					{ID: "dd_2", Family: "static-gauge", Method: "data-driven", Shape: "circle", Width: 60, Height: 60, Color: "#22c55e"},
					{ID: "dd_3", Family: "static-gauge", Method: "data-driven", Shape: "circle", Width: 60, Height: 60, Color: "#f59e0b"},
				},
			}
			mu.Lock()
			pages["dd-offscreen.html"] = animlab.BuildTestPageHTML(ddSpec)
			mu.Unlock()
			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(baseURL+"dd-offscreen.html"))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(150*time.Millisecond))
			d0, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`document.querySelector('#dd_1 .lab-payload').getAttribute('stroke-dasharray')`))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(450*time.Millisecond))
			d1, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`document.querySelector('#dd_1 .lab-payload').getAttribute('stroke-dasharray')`))
			suspCnt, _ := chromedp.Run(tCtx, chromedp.Evaluate[int](`window.__labSuspendedCount`))
			tCancel()
			ddOK := suspCnt == 3 && d0 != "" && d1 != "" && d0 != d1
			if !ddOK {
				allOK = false
			}
			record("AT-M3-05", allOK, fmt.Sprintf("sharedScene=%v dataDrivenOffscreen=(susp=%d,%s->%s)", notes, suspCnt, d0, d1))
		}
	}

	// =========================================================================
	// AT-M3-06: YAML-only change demonstration (NO Go edits, NO recompilation!)
	// Build binary once, then mutate YAML from 100 -> 250 tracers & speed/size/shape
	// =========================================================================
	{
		binPath := "/tmp/anim-test-lab-m3-demo"
		buildOut, err := exec.Command("go", "build", "-o", binPath, "./cmd/anim-test-lab").CombinedOutput()
		if err != nil {
			record("AT-M3-06", false, fmt.Sprintf("build failed: %v (%s)", err, string(buildOut)))
		} else {
			yamlPath := filepath.Join(evidenceDir, "yaml-only-demo.yaml")
			out100 := filepath.Join(evidenceDir, "demo-100")
			out250 := filepath.Join(evidenceDir, "demo-250")
			_ = os.RemoveAll(out100)
			_ = os.RemoveAll(out250)

			// Step 1: 100 rectangle tracers, speed 2.0s, size 60x40, ratio 25:75
			yaml100 := `version: 1
test:
  name: yaml-only-demo
browser:
  engine: chromium
  headless: true
execution:
  warmup_seconds: 0.2
  duration_seconds: 1.0
  repetitions: 1
instances:
  counts: [100]
  isolation: independent-svg-roots
effects:
  - family: perimeter-tracer
    methods: [css]
    shapes: [rectangle]
    sizes:
      base: [60, 40]
    speeds:
      base: 2.0
    segment_counts: [2]
    line_gap_ratios: ["25:75"]
    directions: [cw]
workloads:
  - name: all-active
    visibility: all
output:
  samples: true
`
			_ = os.WriteFile(yamlPath, []byte(yaml100), 0644)
			val100, errV1 := exec.Command(binPath, "validate", "-c", yamlPath).CombinedOutput()
			samp100, errS1 := exec.Command(binPath, "samples", "-c", yamlPath, "-out", out100).CombinedOutput()

			// Step 2: Edit YAML ONLY -> 250 circle tracers, speed 0.9s, size 120x120, ratio 40:60
			yaml250 := strings.ReplaceAll(yaml100, "counts: [100]", "counts: [250]")
			yaml250 = strings.ReplaceAll(yaml250, "shapes: [rectangle]", "shapes: [circle]")
			yaml250 = strings.ReplaceAll(yaml250, "base: [60, 40]", "base: [120, 120]")
			yaml250 = strings.ReplaceAll(yaml250, "base: 2.0", "base: 0.9")
			yaml250 = strings.ReplaceAll(yaml250, `"25:75"`, `"40:60"`)
			_ = os.WriteFile(yamlPath, []byte(yaml250), 0644)
			val250, errV2 := exec.Command(binPath, "validate", "-c", yamlPath).CombinedOutput()
			samp250, errS2 := exec.Command(binPath, "samples", "-c", yamlPath, "-out", out250).CombinedOutput()

			// Verify actual browser DOM count on generated 250-instance workload & inspect generated SVG
			cfg100, _ := animlab.LoadConfig(yamlPath) // now 250
			_ = cfg100
			specs250 := animlab.ExpandScenarios(cfg100)
			mu.Lock()
			pages["demo250.html"] = animlab.BuildTestPageHTML(specs250[0])
			mu.Unlock()
			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(baseURL+"demo250.html"))
			domCount250, _ := chromedp.Run(tCtx, chromedp.Evaluate[int](`document.querySelectorAll('.lab-payload').length`))
			dur250, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`document.querySelector('.lab-payload').getAttribute('data-lab-dur')`))
			tag250, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`document.querySelector('.lab-payload').tagName.toLowerCase()`))
			tCancel()

			// Keep a clean YAML-only demo log
			demoLog := fmt.Sprintf("=== Step 1 (100 tracers) ===\n%s\n%s\n=== Step 2 (250 circle tracers, 0.9s, 140x90, 40:60) ===\n%s\n%s\nBrowser DOM .lab-payload count=%d tag=%s data-lab-dur=%s\n",
				string(val100), string(samp100), string(val250), string(samp250), domCount250, tag250, dur250)
			_ = os.WriteFile(filepath.Join(evidenceDir, "yaml-only-workflow.log"), []byte(demoLog), 0644)

			// Preserve one sample SVG from 100 and 250, clean bulk folders
			svgs100, _ := filepath.Glob(filepath.Join(out100, "samples", "*.svg"))
			svgs250, _ := filepath.Glob(filepath.Join(out250, "samples", "*.svg"))
			if len(svgs100) > 0 {
				if b, e := os.ReadFile(svgs100[0]); e == nil {
					_ = os.WriteFile(filepath.Join(evidenceDir, "demo-100-sample.svg"), b, 0644)
				}
			}
			if len(svgs250) > 0 {
				if b, e := os.ReadFile(svgs250[0]); e == nil {
					_ = os.WriteFile(filepath.Join(evidenceDir, "demo-250-circle-sample.svg"), b, 0644)
				}
			}
			_ = os.RemoveAll(out100)
			_ = os.RemoveAll(out250)
			_ = os.Remove(binPath)

			ok := errV1 == nil && errS1 == nil && errV2 == nil && errS2 == nil &&
				strings.Contains(string(val100), "count=100") &&
				strings.Contains(string(val250), "count=250") &&
				domCount250 == 250 && tag250 == "ellipse" && dur250 == "0.900"
			record("AT-M3-06", ok, fmt.Sprintf("domCount250=%d tag=%s dur=%s", domCount250, tag250, dur250))
		}
	}

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d M3 checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
