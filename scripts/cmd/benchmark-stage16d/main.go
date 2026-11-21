package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/performance"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type ScenarioSpec struct {
	Name            string
	Count           int
	Layout          string
	Mode            string
	Scroll          bool
	HiddenUpdate    bool
	VisibilityAware bool
}

type FrameMetrics struct {
	Frames             int     `json:"frames"`
	FPS                float64 `json:"fps"`
	AvgFrameMs         float64 `json:"avgFrameMs"`
	P95FrameMs         float64 `json:"p95FrameMs"`
	MaxFrameMs         float64 `json:"maxFrameMs"`
	LongFramesOver16ms int     `json:"longFramesOver16ms"`
	LongFramesOver50ms int     `json:"longFramesOver50ms"`
}

type InstanceStats struct {
	Mounted   int `json:"mounted"`
	Visible   int `json:"visible"`
	Active    int `json:"active"`
	Suspended int `json:"suspended"`
}

type ScenarioResult struct {
	Name           string        `json:"name"`
	Count          int           `json:"count"`
	Layout         string        `json:"layout"`
	Mode           string        `json:"mode"`
	VisibilityMode bool          `json:"visibilityAware"`
	MountMs        float64       `json:"mountMs"`
	UpdateMs       float64       `json:"updateMs,omitempty"`
	Frames         FrameMetrics  `json:"frames"`
	Stats          InstanceStats `json:"stats"`
	ScriptDeltaMs  float64       `json:"scriptDeltaMs"`
	LayoutDeltaMs  float64       `json:"layoutDeltaMs"`
	StyleDeltaMs   float64       `json:"styleDeltaMs"`
	TaskDeltaMs    float64       `json:"taskDeltaMs"`
	LayoutCount    int64         `json:"layoutCountDelta"`
	JSHeapMB       float64       `json:"jsHeapUsedMB"`
	DOMNodes       int64         `json:"domNodes"`
	Note           string        `json:"note,omitempty"`
}

type BenchmarkReport struct {
	Timestamp       string           `json:"timestamp"`
	URL             string           `json:"url"`
	Environment     map[string]any   `json:"environment"`
	UnavailableNote string           `json:"unavailableMetricsNote"`
	Scenarios       []ScenarioResult `json:"scenarios"`
}

func runVoid(ctx context.Context, actions ...chromedp.Action[chromedp.Void]) error {
	for _, a := range actions {
		if _, err := chromedp.Run(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func getPerfMap(ctx context.Context) map[string]float64 {
	res := make(map[string]float64)
	out, err := chromedp.Call(ctx, performance.GetMetrics, cdp.Empty{})
	if err != nil {
		return res
	}
	for _, m := range out.Metrics {
		res[m.Name] = m.Value
	}
	return res
}

func main() {
	os.Exit(run())
}

func run() int {
	benchURL := flag.String("url", "http://192.168.100.40:8091/stage16d-benchmark/index.html", "Benchmark page URL")
	cdpURL := flag.String("cdp", "http://127.0.0.1:9222", "Chrome CDP endpoint")
	cpuHost := flag.String("cpu-host", "nigel@192.168.100.40", "Target browser SSH host")
	visAware := flag.Bool("visibility-aware", false, "Enable visibility-aware suspension in scenarios")
	outPath := flag.String("out", "docs/benchmarks/stage16d-a-baseline.json", "Output JSON report path")
	flag.Parse()

	sess, err := animlab.AcquireBrowserSession(context.Background(), *cdpURL, *cpuHost)
	if err != nil {
		log.Printf("Failed to acquire browser session: %v", err)
		return 1
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(sess.AllocCtx, 180*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	if _, err := chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true}); err != nil {
		log.Printf("Warning: failed to disable cache: %v", err)
	}
	if _, err := chromedp.Call(taskCtx, performance.Enable, performance.EnableParams{}); err != nil {
		log.Printf("Failed to enable performance metrics: %v", err)
		return 1
	}
	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1366, 900),
		chromedp.Navigate(*benchURL),
		chromedp.WaitReady("#grid"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		log.Printf("Failed to open benchmark page: %v", err)
		return 1
	}

	// Gather environment metadata
	envJson, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		let renderer = 'unavailable';
		try {
			const c = document.createElement('canvas');
			const gl = c.getContext('webgl') || c.getContext('experimental-webgl');
			if (gl) {
				const dbg = gl.getExtension('WEBGL_debug_renderer_info');
				renderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER);
			}
		} catch (e) {}
		return JSON.stringify({
			userAgent: navigator.userAgent,
			devicePixelRatio: window.devicePixelRatio,
			hardwareConcurrency: navigator.hardwareConcurrency || null,
			viewport: { width: window.innerWidth, height: window.innerHeight },
			webglRenderer: renderer
		});
	})()`))

	envMap := make(map[string]any)
	_ = json.Unmarshal([]byte(envJson), &envMap)
	if out, err := exec.Command("uname", "-sr").Output(); err == nil {
		envMap["hostOS"] = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("nproc").Output(); err == nil {
		envMap["hostCPUs"] = strings.TrimSpace(string(out))
	}

	scenarios := []ScenarioSpec{
		{Name: "30-static-dashboard", Count: 30, Layout: "dashboard", Mode: "static", VisibilityAware: *visAware},
		{Name: "30-animated-dashboard", Count: 30, Layout: "dashboard", Mode: "animated", VisibilityAware: *visAware},
		{Name: "30-animated-all-visible", Count: 30, Layout: "all-visible", Mode: "animated", VisibilityAware: *visAware},
		{Name: "30-animated-all-hidden", Count: 30, Layout: "all-hidden", Mode: "animated", VisibilityAware: *visAware},
		{Name: "100-static-dashboard", Count: 100, Layout: "dashboard", Mode: "static", VisibilityAware: *visAware},
		{Name: "100-animated-dashboard", Count: 100, Layout: "dashboard", Mode: "animated", VisibilityAware: *visAware},
		{Name: "100-mixed-dashboard", Count: 100, Layout: "dashboard", Mode: "mixed", VisibilityAware: *visAware},
		{Name: "100-animated-scroll", Count: 100, Layout: "dashboard", Mode: "animated", Scroll: true, VisibilityAware: *visAware},
		{Name: "100-animated-hidden-updates", Count: 100, Layout: "all-hidden", Mode: "animated", HiddenUpdate: true, VisibilityAware: *visAware},
		{Name: "250-animated-dashboard", Count: 250, Layout: "dashboard", Mode: "animated", VisibilityAware: *visAware},
	}

	report := BenchmarkReport{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		URL:             *benchURL,
		Environment:     envMap,
		UnavailableNote: "Direct per-process OS CPU % and raw GPU utilization % are not exposed by HeadlessChrome CDP; main-thread Script/Layout/RecalcStyle/Task durations, frame timing, DOM node count, and JS heap are directly measured.",
	}

	fmt.Println("==========================================================================================")
	fmt.Printf(" Stage 16D Browser Scalability Benchmark (visibilityAware=%v)\n", *visAware)
	fmt.Println("==========================================================================================")
	fmt.Printf("%-28s | %5s | %6s | %6s | %7s | %6s | %7s | %7s\n",
		"Scenario", "Vis", "Susp", "FPS", "Avg(ms)", "P95", "TaskMs", "HeapMB")
	fmt.Println(strings.Repeat("-", 90))

	for _, sc := range scenarios {
		setupExpr := fmt.Sprintf(`window.BenchmarkHarness.setup({count:%d,layout:%q,mode:%q,visibilityAware:%v})`,
			sc.Count, sc.Layout, sc.Mode, sc.VisibilityAware)

		setupJson, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](
			fmt.Sprintf(`(async () => JSON.stringify(await %s))()`, setupExpr),
			func(p *cdpruntime.EvaluateParams) { b := true; p.AwaitPromise = &b },
		))
		if err != nil {
			log.Printf("Setup %s failed: %v", sc.Name, err)
			return 1
		}

		// Wait 200ms warm-up
		_ = runVoid(taskCtx, chromedp.Sleep(200*time.Millisecond))

		var setupParsed struct {
			Count   int           `json:"count"`
			MountMs float64       `json:"mountMs"`
			Stats   InstanceStats `json:"stats"`
		}
		_ = json.Unmarshal([]byte(setupJson), &setupParsed)

		mBefore := getPerfMap(taskCtx)

		var updateMs float64
		if sc.HiddenUpdate {
			uVal, _ := chromedp.Run(taskCtx, chromedp.Evaluate[float64](`(() => {
				let total = 0;
				for (let t = 1; t <= 5; t++) total += window.BenchmarkHarness.stepUpdates(1.0, t);
				return total;
			})()`))
			updateMs = uVal
		}

		framesJson, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](
			fmt.Sprintf(`(async () => JSON.stringify(await window.BenchmarkHarness.measureFrames(1000, %v)))()`, sc.Scroll),
			func(p *cdpruntime.EvaluateParams) { b := true; p.AwaitPromise = &b },
		))
		if err != nil {
			log.Printf("MeasureFrames %s failed: %v", sc.Name, err)
			return 1
		}

		mAfter := getPerfMap(taskCtx)

		statsAfterJson, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.BenchmarkHarness.getStats())`))
		var statsAfter InstanceStats
		_ = json.Unmarshal([]byte(statsAfterJson), &statsAfter)

		var fm FrameMetrics
		_ = json.Unmarshal([]byte(framesJson), &fm)

		res := ScenarioResult{
			Name:           sc.Name,
			Count:          sc.Count,
			Layout:         sc.Layout,
			Mode:           sc.Mode,
			VisibilityMode: sc.VisibilityAware,
			MountMs:        setupParsed.MountMs,
			UpdateMs:       updateMs,
			Frames:         fm,
			Stats:          statsAfter,
			ScriptDeltaMs:  round2((mAfter["ScriptDuration"] - mBefore["ScriptDuration"]) * 1000),
			LayoutDeltaMs:  round2((mAfter["LayoutDuration"] - mBefore["LayoutDuration"]) * 1000),
			StyleDeltaMs:   round2((mAfter["RecalcStyleDuration"] - mBefore["RecalcStyleDuration"]) * 1000),
			TaskDeltaMs:    round2((mAfter["TaskDuration"] - mBefore["TaskDuration"]) * 1000),
			LayoutCount:    int64(mAfter["LayoutCount"] - mBefore["LayoutCount"]),
			JSHeapMB:       round2(mAfter["JSHeapUsedSize"] / (1024 * 1024)),
			DOMNodes:       int64(mAfter["Nodes"]),
		}
		report.Scenarios = append(report.Scenarios, res)

		fmt.Printf("%-28s | %5d | %6d | %6.1f | %7.2f | %6.2f | %7.2f | %7.2f\n",
			res.Name, res.Stats.Visible, res.Stats.Suspended, res.Frames.FPS, res.Frames.AvgFrameMs, res.Frames.P95FrameMs, res.TaskDeltaMs, res.JSHeapMB)
	}

	_ = os.MkdirAll(filepath.Dir(*outPath), 0755)
	outBytes, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(*outPath, outBytes, 0644); err != nil {
		log.Printf("Failed to write report: %v", err)
		return 1
	}
	fmt.Printf("\nSaved benchmark report to %s\n", *outPath)
	return 0
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
