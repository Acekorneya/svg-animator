package animlab

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

type RepetitionResult struct {
	Repetition           int               `json:"repetition"`
	OrderIndex           int               `json:"orderIndex"`
	Status               TrialStatus       `json:"status"`
	StatusReason         string            `json:"statusReason,omitempty"`
	ProcessCPUSec        *float64          `json:"processCpuSec"`
	OwnedPIDCount        int               `json:"ownedPidCount,omitempty"`
	Frames               *FrameStats       `json:"frames"`
	PresentedFPS         *float64          `json:"presentedFps"`
	PresentedFPSNote     string            `json:"presentedFpsNote,omitempty"`
	LongTaskCount        *int              `json:"longTaskCount"`
	LongTaskMs           *float64          `json:"longTaskMs"`
	TaskDeltaMs          *float64          `json:"taskDeltaMs"`
	ScriptDeltaMs        *float64          `json:"scriptDeltaMs"`
	LayoutDeltaMs        *float64          `json:"layoutDeltaMs"`
	RecalcDeltaMs        *float64          `json:"recalcDeltaMs"`
	LayoutCount          *int64            `json:"layoutCountDelta"`
	JSHeapMB             *float64          `json:"jsHeapMB"`
	DOMNodes             *int64            `json:"domNodes"`
	ActualViewportWidth  int               `json:"actualViewportWidth"`
	ActualViewportHeight int               `json:"actualViewportHeight"`
	DevicePixelRatio     float64           `json:"devicePixelRatio"`
	Correctness          CorrectnessResult `json:"correctness"`
}

type ScenarioReport struct {
	RunID                string             `json:"runId"`
	Family               string             `json:"family"`
	Method               string             `json:"method"`
	SchedulingStrategy   string             `json:"schedulingStrategy,omitempty"`
	WorkloadName         string             `json:"workloadName"`
	DisplayScale         float64            `json:"displayScale,omitempty"`
	ScalingMode          string             `json:"scalingMode,omitempty"`
	Count                int                `json:"count"`
	Isolation            string             `json:"isolation"`
	NonEquivalent        bool               `json:"nonEquivalent,omitempty"`
	Unsupported          bool               `json:"unsupported,omitempty"`
	Limitations          []string           `json:"limitations,omitempty"`
	CorrectnessPassed    bool               `json:"correctnessPassed"`
	FaultCode            string             `json:"faultCode,omitempty"`
	CorrectnessNote      string             `json:"correctnessNote"`
	ValidRepetitions     int                `json:"validRepetitions"`
	TotalRepetitions     int                `json:"totalRepetitions"`
	AvgRafHz             *float64           `json:"avgRafHz"`
	AvgRafIntervalMs     *float64           `json:"avgRafIntervalMs"`
	P95RafIntervalMs     *float64           `json:"p95RafIntervalMs"`
	PresentedFPS         *float64           `json:"presentedFps"`
	AvgProcessCPUSec     *float64           `json:"avgProcessCpuSec"`
	AvgTaskMs            *float64           `json:"avgTaskMs"`
	AvgScriptMs          *float64           `json:"avgScriptMs"`
	AvgLayoutMs          *float64           `json:"avgLayoutMs"`
	AvgRecalcMs          *float64           `json:"avgRecalcMs"`
	AvgHeapMB            *float64           `json:"avgHeapMB"`
	DOMNodes             *int64             `json:"domNodes"`
	LongTasks            *int               `json:"longTasks"`
	RafHzStats           *DistributionStats `json:"rafHzStats,omitempty"`
	RafIntervalStats     *DistributionStats `json:"rafIntervalStats,omitempty"`
	ProcessCPUStats      *DistributionStats `json:"processCpuStats,omitempty"`
	TaskMsStats          *DistributionStats `json:"taskMsStats,omitempty"`
	ActualViewportWidth  int                `json:"actualViewportWidth,omitempty"`
	ActualViewportHeight int                `json:"actualViewportHeight,omitempty"`
	DevicePixelRatio     float64            `json:"devicePixelRatio,omitempty"`
	ScreenshotPath       string             `json:"screenshotPath,omitempty"`
	Repetitions          []RepetitionResult `json:"repetitions"`
}

type LabResults struct {
	Timestamp         string                      `json:"timestamp"`
	TestName          string                      `json:"testName"`
	Description       string                      `json:"description"`
	Environment       map[string]any              `json:"environment"`
	MetricDefinitions map[string]MetricDefinition `json:"metricDefinitions"`
	ExecutionOrders   [][]string                  `json:"executionOrders"`
	Scenarios         []ScenarioReport            `json:"scenarios"`
}

// RunSuite executes all (or filtered) scenarios in cfg against Chromium CDP and returns LabResults.
func RunSuite(cfg *Config, outDir, filterTest string) (*LabResults, error) {
	specs := ExpandScenarios(cfg)
	if filterTest != "" {
		var filtered []TestRunSpec
		for _, s := range specs {
			if strings.Contains(s.RunID, filterTest) || s.Family == filterTest || s.Method == filterTest || s.WorkloadName == filterTest {
				filtered = append(filtered, s)
			}
		}
		specs = filtered
		if len(specs) == 0 {
			return nil, fmt.Errorf("no scenarios matched filter %q", filterTest)
		}
	}

	var (
		mu    sync.RWMutex
		pages = map[string]string{}
	)
	bindHost := cfg.Browser.HostBindIP
	if bindHost == "" {
		bindHost = detectBindIP(cfg.Browser.CDPEndpoint)
	}
	ln, err := net.Listen("tcp", bindHost+":0")
	if err != nil {
		return nil, fmt.Errorf("failed to start ephemeral test server on %s: %w", bindHost, err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/test/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/test/")
		mu.RLock()
		html, ok := pages[key]
		mu.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	baseURL := fmt.Sprintf("http://%s/test/", ln.Addr().String())

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cfg.Browser.CDPEndpoint)
	defer aCancel()
	probeCtx, pCancel := chromedp.NewContext(allocCtx)
	if _, err := chromedp.Call(probeCtx, performance.Enable, performance.EnableParams{}); err != nil {
		pCancel()
		return nil, fmt.Errorf("connecting to CDP %s: %w", cfg.Browser.CDPEndpoint, err)
	}

	// Collect browser ownership & rendering environment on an isolated probe tab, then close it
	ownership := EstablishBrowserOwnership(probeCtx, cfg.Browser.CDPEndpoint, cfg.Browser.ProcessCPUHost)
	envMeta := collectEnvironmentMetadata(probeCtx, cfg, ownership)
	pCancel()

	res := &LabResults{
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		TestName:          cfg.Test.Name,
		Description:       cfg.Test.Description,
		Environment:       envMeta,
		MetricDefinitions: DefaultMetricDefinitions(),
	}

	repMap := map[string]*ScenarioReport{}
	for _, s := range specs {
		repMap[s.RunID] = &ScenarioReport{
			RunID:              s.RunID,
			Family:             s.Family,
			Method:             s.Method,
			SchedulingStrategy: s.SchedulingStrategy,
			WorkloadName:       s.WorkloadName,
			DisplayScale:       s.DisplayScale,
			ScalingMode:        s.ScalingMode,
			Count:              s.Count,
			Isolation:          s.Isolation,
			NonEquivalent:      s.NonEquivalent,
			Unsupported:        s.Unsupported,
			Limitations:        s.Limitations,
		}
		mu.Lock()
		pages[s.RunID+".html"] = BuildTestPageHTML(s)
		mu.Unlock()
	}

	ssDir := filepath.Join(outDir, "screenshots")
	if cfg.Output.Screenshots {
		_ = os.MkdirAll(ssDir, 0755)
	}

	for rep := 0; rep < cfg.Execution.Repetitions; rep++ {
		ordered := orderTrialsForRepetition(specs, cfg.Execution, rep)
		orderIDs := make([]string, len(ordered))
		for i, s := range ordered {
			orderIDs[i] = s.RunID
		}
		res.ExecutionOrders = append(res.ExecutionOrders, orderIDs)

		for ordIdx, s := range ordered {
			fmt.Printf("[Lab] Rep %d/%d (%d/%d): %s\n", rep+1, cfg.Execution.Repetitions, ordIdx+1, len(ordered), s.RunID)
			rr, ssBuf := runSingleTrial(allocCtx, cfg, ownership, baseURL+s.RunID+".html", s, rep+1, ordIdx+1, cfg.Output.Screenshots && rep == 0)
			sr := repMap[s.RunID]
			sr.Repetitions = append(sr.Repetitions, rr)
			if len(ssBuf) > 0 && sr.ScreenshotPath == "" {
				ssPath := filepath.Join(ssDir, s.RunID+".png")
				if err := os.WriteFile(ssPath, ssBuf, 0644); err == nil {
					sr.ScreenshotPath = filepath.Join("screenshots", s.RunID+".png")
				}
			}
		}
	}

	for _, s := range specs {
		sr := repMap[s.RunID]
		aggregateScenarioReport(sr)
		res.Scenarios = append(res.Scenarios, *sr)
	}
	return res, nil
}

func orderTrialsForRepetition(specs []TestRunSpec, execCfg ExecutionConfig, rep int) []TestRunSpec {
	ordered := make([]TestRunSpec, len(specs))
	copy(ordered, specs)
	n := len(ordered)
	if n <= 1 {
		return ordered
	}
	strategy := execCfg.OrderStrategy
	if strategy == "" && execCfg.AlternateMethodOrder {
		strategy = "alternating"
	}
	switch strategy {
	case "randomized":
		r := rand.New(rand.NewSource(execCfg.RandomSeed + int64(rep)*7919))
		r.Shuffle(n, func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	case "alternating":
		// Balanced cyclic rotation so every scenario rotates through first/middle/last positions
		shift := rep % n
		rotated := make([]TestRunSpec, n)
		for i := 0; i < n; i++ {
			rotated[i] = ordered[(i+shift)%n]
		}
		return rotated
	}
	return ordered
}

func aggregateScenarioReport(sr *ScenarioReport) {
	sr.TotalRepetitions = len(sr.Repetitions)
	if sr.TotalRepetitions == 0 {
		return
	}

	allPassed := true
	var failReasons []string
	var rafHzVals, rafIntVals, p95IntVals, cpuVals, taskVals, scriptVals, layoutVals, recalcVals, heapVals []float64
	var longTaskSum int
	hasLongTask := false
	var lastDOMNodes *int64

	for _, r := range sr.Repetitions {
		if r.ActualViewportWidth > 0 {
			sr.ActualViewportWidth = r.ActualViewportWidth
			sr.ActualViewportHeight = r.ActualViewportHeight
			sr.DevicePixelRatio = r.DevicePixelRatio
		}
		if !r.Correctness.Passed {
			allPassed = false
			failReasons = append(failReasons, r.Correctness.Reason)
			if sr.FaultCode == "" && r.Correctness.FaultCode != "" {
				sr.FaultCode = r.Correctness.FaultCode
			}
		}
		if r.Status != TrialValid {
			continue
		}
		sr.ValidRepetitions++
		if r.Frames != nil && r.Frames.Frames > 0 {
			rafHzVals = append(rafHzVals, r.Frames.RafHz)
			rafIntVals = append(rafIntVals, r.Frames.AvgRafIntervalMs)
			p95IntVals = append(p95IntVals, r.Frames.P95RafIntervalMs)
		}
		if r.ProcessCPUSec != nil {
			cpuVals = append(cpuVals, *r.ProcessCPUSec)
		}
		if r.TaskDeltaMs != nil {
			taskVals = append(taskVals, *r.TaskDeltaMs)
		}
		if r.ScriptDeltaMs != nil {
			scriptVals = append(scriptVals, *r.ScriptDeltaMs)
		}
		if r.LayoutDeltaMs != nil {
			layoutVals = append(layoutVals, *r.LayoutDeltaMs)
		}
		if r.RecalcDeltaMs != nil {
			recalcVals = append(recalcVals, *r.RecalcDeltaMs)
		}
		if r.JSHeapMB != nil {
			heapVals = append(heapVals, *r.JSHeapMB)
		}
		if r.DOMNodes != nil {
			lastDOMNodes = r.DOMNodes
		}
		if r.LongTaskCount != nil {
			longTaskSum += *r.LongTaskCount
			hasLongTask = true
		}
	}

	sr.CorrectnessPassed = allPassed && sr.ValidRepetitions == sr.TotalRepetitions
	if sr.CorrectnessPassed {
		sr.CorrectnessNote = sr.Repetitions[0].Correctness.Reason
	} else {
		if sr.ValidRepetitions < sr.TotalRepetitions {
			failReasons = append(failReasons, fmt.Sprintf("%d/%d trials invalid/failed", sr.TotalRepetitions-sr.ValidRepetitions, sr.TotalRepetitions))
		}
		sr.CorrectnessNote = strings.Join(failReasons, "; ")
	}

	if d := computeDistribution(rafHzVals); d != nil {
		sr.RafHzStats = d
		sr.AvgRafHz = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(rafIntVals); d != nil {
		sr.RafIntervalStats = d
		sr.AvgRafIntervalMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(p95IntVals); d != nil {
		sr.P95RafIntervalMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(cpuVals); d != nil {
		sr.ProcessCPUStats = d
		sr.AvgProcessCPUSec = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(taskVals); d != nil {
		sr.TaskMsStats = d
		sr.AvgTaskMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(scriptVals); d != nil {
		sr.AvgScriptMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(layoutVals); d != nil {
		sr.AvgLayoutMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(recalcVals); d != nil {
		sr.AvgRecalcMs = ptrFloat64(d.Mean)
	}
	if d := computeDistribution(heapVals); d != nil {
		sr.AvgHeapMB = ptrFloat64(d.Mean)
	}
	sr.DOMNodes = lastDOMNodes
	if hasLongTask {
		sr.LongTasks = &longTaskSum
	}
	// PresentedFPS remains nil (N/A) unless real compositor frame telemetry is available.
}

func runSingleTrial(allocCtx context.Context, cfg *Config, own BrowserOwnership, url string, s TestRunSpec, rep, ord int, takeShot bool) (RepetitionResult, []byte) {
	// Isolated per-trial tab/target; guaranteed cleanup on normal exit, error, or disconnect.
	trialCtx, cancelTrial := chromedp.NewContext(allocCtx)
	defer cancelTrial()

	res := RepetitionResult{
		Repetition:       rep,
		OrderIndex:       ord,
		Status:           TrialValid,
		PresentedFPS:     nil,
		PresentedFPSNote: "N/A: compositor presented-frame counter unavailable in standard headless/SwiftShader CDP",
	}

	if _, err := chromedp.Call(trialCtx, performance.Enable, performance.EnableParams{}); err != nil {
		res.Status = TrialFailed
		res.StatusReason = fmt.Sprintf("CDP performance.Enable failed: %v", err)
		return res, nil
	}

	dpr := cfg.Browser.DeviceScaleFactor
	if dpr <= 0 {
		dpr = 1.0
	}
	if cfg.Browser.ViewportWidth > 0 && cfg.Browser.ViewportHeight > 0 {
		if _, err := chromedp.Call(trialCtx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             int64(cfg.Browser.ViewportWidth),
			Height:            int64(cfg.Browser.ViewportHeight),
			DeviceScaleFactor: dpr,
			Mobile:            false,
		}); err != nil {
			res.Status = TrialFailed
			res.StatusReason = fmt.Sprintf("SetDeviceMetricsOverride failed: %v", err)
			return res, nil
		}
	}

	if _, err := chromedp.Run(trialCtx, chromedp.Navigate(url)); err != nil {
		res.Status = TrialFailed
		res.StatusReason = fmt.Sprintf("navigate failed: %v", err)
		return res, nil
	}

	// Verify actual viewport dimensions & DPR in the browser
	vpJSON, err := chromedp.Run(trialCtx, chromedp.Evaluate[string](`JSON.stringify({
		w: window.innerWidth,
		h: window.innerHeight,
		dpr: window.devicePixelRatio || 1
	})`))
	if err == nil {
		var vp struct {
			W   int     `json:"w"`
			H   int     `json:"h"`
			DPR float64 `json:"dpr"`
		}
		if json.Unmarshal([]byte(vpJSON), &vp) == nil {
			res.ActualViewportWidth = vp.W
			res.ActualViewportHeight = vp.H
			res.DevicePixelRatio = vp.DPR
		}
	}
	if cfg.Browser.ViewportWidth > 0 && (res.ActualViewportWidth != cfg.Browser.ViewportWidth || res.ActualViewportHeight != cfg.Browser.ViewportHeight) {
		res.Status = TrialContaminated
		res.StatusReason = fmt.Sprintf("viewport mismatch: configured %dx%d, actual %dx%d",
			cfg.Browser.ViewportWidth, cfg.Browser.ViewportHeight, res.ActualViewportWidth, res.ActualViewportHeight)
	}

	if cfg.Execution.WarmupSeconds > 0 {
		_, _ = chromedp.Run(trialCtx, chromedp.Sleep(time.Duration(cfg.Execution.WarmupSeconds*float64(time.Second))))
	}

	// Correctness verification (separated from timed measurement window)
	if hasMeasurement(cfg.Measurements, "animation_correctness") {
		res.Correctness = EvaluateCorrectness(trialCtx, s)
		// Brief settle after correctness probes before timing begins
		_, _ = chromedp.Run(trialCtx, chromedp.Sleep(100*time.Millisecond))
	} else {
		res.Correctness = CorrectnessResult{Passed: true, Reason: "animation_correctness disabled in config"}
	}

	if hasMeasurement(cfg.Measurements, "frame_timing") || hasMeasurement(cfg.Measurements, "long_tasks") {
		_, _ = chromedp.Run(trialCtx, chromedp.Evaluate[bool](`(() => {
			window.__labFrames = [];
			window.__labSampling = true;
			window.__labLongTaskBase = (window.__labLongTasks || []).length;
			let last = performance.now();
			function tick(now) {
				if (!window.__labSampling) return;
				window.__labFrames.push(now - last);
				last = now;
				requestAnimationFrame(tick);
			}
			requestAnimationFrame(tick);
			return true;
		})()`))
	}

	var cpu0 *float64
	if hasMeasurement(cfg.Measurements, "process_cpu") {
		cpu0, _, _ = SampleOwnedProcessTreeCPU(own, cfg.Browser.ProcessCPUHost)
	}
	m0, m0OK := getCDPMetrics(trialCtx)

	_, _ = chromedp.Run(trialCtx, chromedp.Sleep(time.Duration(cfg.Execution.DurationSeconds*float64(time.Second))))

	m1, m1OK := getCDPMetrics(trialCtx)
	if hasMeasurement(cfg.Measurements, "process_cpu") {
		cpu1, pidCnt, cpuErr := SampleOwnedProcessTreeCPU(own, cfg.Browser.ProcessCPUHost)
		res.OwnedPIDCount = pidCnt
		if cpuErr == nil && cpu0 != nil && cpu1 != nil && *cpu1 >= *cpu0 {
			res.ProcessCPUSec = ptrFloat64(*cpu1 - *cpu0)
		} else if cpuErr != nil && res.Status == TrialValid {
			res.Status = TrialPartial
			res.StatusReason = fmt.Sprintf("CPU attribution unavailable: %v", cpuErr)
		}
	}

	if hasMeasurement(cfg.Measurements, "frame_timing") || hasMeasurement(cfg.Measurements, "long_tasks") {
		rawFrameJSON, evalErr := chromedp.Run(trialCtx, chromedp.Evaluate[string](`(() => {
			window.__labSampling = false;
			const lt = (window.__labLongTasks || []).slice(window.__labLongTaskBase || 0);
			let ltDur = 0;
			for (const x of lt) ltDur += x.dur || 0;
			return JSON.stringify({
				frames: (window.__labFrames || []).slice(2),
				longTaskCount: lt.length,
				longTaskMs: ltDur
			});
		})()`))
		if evalErr == nil {
			var fData struct {
				Frames        []float64 `json:"frames"`
				LongTaskCount int       `json:"longTaskCount"`
				LongTaskMs    float64   `json:"longTaskMs"`
			}
			if json.Unmarshal([]byte(rawFrameJSON), &fData) == nil {
				if hasMeasurement(cfg.Measurements, "frame_timing") && len(fData.Frames) >= 2 {
					fs := computeFrameStats(fData.Frames)
					res.Frames = &fs
				}
				if hasMeasurement(cfg.Measurements, "long_tasks") {
					res.LongTaskCount = &fData.LongTaskCount
					res.LongTaskMs = ptrFloat64(fData.LongTaskMs)
				}
			}
		}
	}

	if m0OK && m1OK {
		res.TaskDeltaMs = ptrFloat64((m1["TaskDuration"] - m0["TaskDuration"]) * 1000)
		res.ScriptDeltaMs = ptrFloat64((m1["ScriptDuration"] - m0["ScriptDuration"]) * 1000)
		res.LayoutDeltaMs = ptrFloat64((m1["LayoutDuration"] - m0["LayoutDuration"]) * 1000)
		res.RecalcDeltaMs = ptrFloat64((m1["RecalcStyleDuration"] - m0["RecalcStyleDuration"]) * 1000)
		res.LayoutCount = ptrInt64(int64(m1["LayoutCount"] - m0["LayoutCount"]))
		if hasMeasurement(cfg.Measurements, "memory") {
			res.JSHeapMB = ptrFloat64(m1["JSHeapUsedSize"] / (1024 * 1024))
		}
		res.DOMNodes = ptrInt64(int64(m1["Nodes"]))
	} else if res.Status == TrialValid {
		res.Status = TrialFailed
		res.StatusReason = "CDP Performance.getMetrics failed"
	}

	var ssBuf []byte
	if takeShot {
		ssBuf, _ = chromedp.Run(trialCtx, chromedp.CaptureScreenshot())
	}

	return res, ssBuf
}

func collectEnvironmentMetadata(ctx context.Context, cfg *Config, own BrowserOwnership) map[string]any {
	if cfg.Browser.ViewportWidth > 0 && cfg.Browser.ViewportHeight > 0 {
		dpr := cfg.Browser.DeviceScaleFactor
		if dpr <= 0 {
			dpr = 1.0
		}
		_, _ = chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             int64(cfg.Browser.ViewportWidth),
			Height:            int64(cfg.Browser.ViewportHeight),
			DeviceScaleFactor: dpr,
			Mobile:            false,
		})
	}
	_, _ = chromedp.Run(ctx, chromedp.Navigate("about:blank"))

	probeJSON, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		let vendor = '', renderer = '';
		try {
			const c = document.createElement('canvas');
			const gl = c.getContext('webgl') || c.getContext('experimental-webgl');
			if (gl) {
				const dbg = gl.getExtension('WEBGL_debug_renderer_info');
				if (dbg) {
					vendor = gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) || '';
					renderer = gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) || '';
				} else {
					renderer = gl.getParameter(gl.RENDERER) || '';
				}
			}
		} catch (_) {}
		return JSON.stringify({
			ua: navigator.userAgent,
			hwConc: navigator.hardwareConcurrency || 0,
			vw: window.innerWidth,
			vh: window.innerHeight,
			dpr: window.devicePixelRatio || 1,
			vendor,
			renderer
		});
	})()`))
	var bp struct {
		UA       string  `json:"ua"`
		HWConc   int     `json:"hwConc"`
		VW       int     `json:"vw"`
		VH       int     `json:"vh"`
		DPR      float64 `json:"dpr"`
		Vendor   string  `json:"vendor"`
		Renderer string  `json:"renderer"`
	}
	_ = json.Unmarshal([]byte(probeJSON), &bp)

	isSW := strings.Contains(strings.ToLower(bp.Renderer), "swiftshader") ||
		strings.Contains(strings.ToLower(bp.Renderer), "llvmpipe") ||
		strings.Contains(strings.ToLower(bp.UA), "headless")

	env := map[string]any{
		"measurementSpecVersion": MeasurementSpecVersion,
		"userAgent":              bp.UA,
		"headless":               cfg.Browser.Headless || strings.Contains(bp.UA, "HeadlessChrome"),
		"hardwareConcurrency":    bp.HWConc,
		"webglVendor":            bp.Vendor,
		"webglRenderer":          bp.Renderer,
		"isSoftwareRenderer":     isSW,
		"configuredViewport": map[string]any{
			"width":             cfg.Browser.ViewportWidth,
			"height":            cfg.Browser.ViewportHeight,
			"deviceScaleFactor": cfg.Browser.DeviceScaleFactor,
		},
		"actualViewport": map[string]any{
			"width":            bp.VW,
			"height":           bp.VH,
			"devicePixelRatio": bp.DPR,
		},
		"browserOwnership":  own,
		"runnerOS":          runtime.GOOS + "/" + runtime.GOARCH,
		"cdpEndpoint":       cfg.Browser.CDPEndpoint,
		"processCpuHost":    cfg.Browser.ProcessCPUHost,
		"executionStrategy": cfg.Execution.OrderStrategy,
	}
	for k, v := range CollectHostHardwareInfo(cfg.Browser.ProcessCPUHost) {
		env[k] = v
	}
	return env
}

func hasMeasurement(list []string, target string) bool {
	for _, m := range list {
		if m == target {
			return true
		}
	}
	return false
}

func getCDPMetrics(ctx context.Context) (map[string]float64, bool) {
	res := map[string]float64{}
	out, err := chromedp.Call(ctx, performance.GetMetrics, cdp.Empty{})
	if err != nil {
		return res, false
	}
	for _, m := range out.Metrics {
		res[m.Name] = m.Value
	}
	return res, true
}

func detectBindIP(cdpEndpoint string) string {
	if strings.Contains(cdpEndpoint, "127.0.0.1") || strings.Contains(cdpEndpoint, "localhost") {
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
		return "127.0.0.1"
	}
	return "0.0.0.0"
}
