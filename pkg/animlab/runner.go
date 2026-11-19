package animlab

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

type FrameStats struct {
	Frames           int     `json:"frames"`
	FPS              float64 `json:"fps"`
	AvgFrameMs       float64 `json:"avgFrameMs"`
	P95FrameMs       float64 `json:"p95FrameMs"`
	MaxFrameMs       float64 `json:"maxFrameMs"`
	LongFramesOver16 int     `json:"longFramesOver16ms"`
	LongFramesOver50 int     `json:"longFramesOver50ms"`
}

type RepetitionResult struct {
	Repetition     int               `json:"repetition"`
	OrderIndex     int               `json:"orderIndex"`
	ProcessCPUTicks float64          `json:"processCpuSec"`
	Frames         FrameStats        `json:"frames"`
	LongTaskCount  int               `json:"longTaskCount"`
	LongTaskMs     float64           `json:"longTaskMs"`
	TaskDeltaMs    float64           `json:"taskDeltaMs"`
	ScriptDeltaMs  float64           `json:"scriptDeltaMs"`
	LayoutDeltaMs  float64           `json:"layoutDeltaMs"`
	RecalcDeltaMs  float64           `json:"recalcDeltaMs"`
	LayoutCount    int64             `json:"layoutCountDelta"`
	JSHeapMB       float64           `json:"jsHeapMB"`
	DOMNodes       int64             `json:"domNodes"`
	Correctness    CorrectnessResult `json:"correctness"`
}

type ScenarioReport struct {
	RunID              string             `json:"runId"`
	Family             string             `json:"family"`
	Method             string             `json:"method"`
	SchedulingStrategy string             `json:"schedulingStrategy,omitempty"`
	WorkloadName       string             `json:"workloadName"`
	Count              int                `json:"count"`
	Isolation          string             `json:"isolation"`
	NonEquivalent      bool               `json:"nonEquivalent,omitempty"`
	Unsupported        bool               `json:"unsupported,omitempty"`
	Limitations        []string           `json:"limitations,omitempty"`
	CorrectnessPassed  bool               `json:"correctnessPassed"`
	FaultCode          string             `json:"faultCode,omitempty"`
	CorrectnessNote    string             `json:"correctnessNote"`
	AvgFPS            float64            `json:"avgFps"`
	AvgFrameMs        float64            `json:"avgFrameMs"`
	P95FrameMs        float64            `json:"p95FrameMs"`
	AvgProcessCPUSec  float64            `json:"avgProcessCpuSec"`
	AvgTaskMs         float64            `json:"avgTaskMs"`
	AvgScriptMs       float64            `json:"avgScriptMs"`
	AvgLayoutMs       float64            `json:"avgLayoutMs"`
	AvgRecalcMs       float64            `json:"avgRecalcMs"`
	AvgHeapMB         float64            `json:"avgHeapMB"`
	DOMNodes          int64              `json:"domNodes"`
	LongTasks         int                `json:"longTasks"`
	ScreenshotPath    string             `json:"screenshotPath,omitempty"`
	Repetitions       []RepetitionResult `json:"repetitions"`
}

type LabResults struct {
	Timestamp   string           `json:"timestamp"`
	TestName    string           `json:"testName"`
	Description string           `json:"description"`
	Environment map[string]any   `json:"environment"`
	Scenarios   []ScenarioReport `json:"scenarios"`
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

	// Start ephemeral HTTP server serving generated test pages
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
		return nil, fmt.Errorf("binding HTTP server on %s: %w", bindHost, err)
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
	ctx, cCancel := chromedp.NewContext(allocCtx)
	defer cCancel()
	if _, err := chromedp.Call(ctx, performance.Enable, performance.EnableParams{}); err != nil {
		return nil, fmt.Errorf("connecting to CDP %s: %w", cfg.Browser.CDPEndpoint, err)
	}

	// Collect browser & host environment
	ua, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`navigator.userAgent`))
	hwConc, _ := chromedp.Run(ctx, chromedp.Evaluate[int](`navigator.hardwareConcurrency || 0`))
	res := &LabResults{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		TestName:    cfg.Test.Name,
		Description: cfg.Test.Description,
		Environment: map[string]any{
			"userAgent":           ua,
			"hardwareConcurrency": hwConc,
			"runnerOS":            runtime.GOOS + "/" + runtime.GOARCH,
			"cdpEndpoint":         cfg.Browser.CDPEndpoint,
			"processCpuHost":      cfg.Browser.ProcessCPUHost,
			"gpuNote":             "Headless/virtualised environments may use SwiftShader software rendering; raw GPU hardware counters are omitted when unavailable.",
		},
	}

	// Organize execution across repetitions with optional alternating method order
	repMap := map[string]*ScenarioReport{}
	for _, s := range specs {
		repMap[s.RunID] = &ScenarioReport{
			RunID:              s.RunID,
			Family:             s.Family,
			Method:             s.Method,
			SchedulingStrategy: s.SchedulingStrategy,
			WorkloadName:       s.WorkloadName,
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
		ordered := make([]TestRunSpec, len(specs))
		copy(ordered, specs)
		if cfg.Execution.AlternateMethodOrder && rep%2 == 1 {
			for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}

		for ordIdx, s := range ordered {
			fmt.Printf("[Lab] Rep %d/%d (%d/%d): %s\n", rep+1, cfg.Execution.Repetitions, ordIdx+1, len(ordered), s.RunID)
			rr, ssBuf, err := runSingleTrial(ctx, cfg, baseURL+s.RunID+".html", s, rep+1, ordIdx+1, cfg.Output.Screenshots && rep == 0)
			if err != nil {
				return nil, fmt.Errorf("running %s rep %d: %w", s.RunID, rep+1, err)
			}
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

	// Aggregate across repetitions in deterministic spec order
	for _, s := range specs {
		sr := repMap[s.RunID]
		n := float64(len(sr.Repetitions))
		if n > 0 {
			allPassed := true
			var failReasons []string
			for _, r := range sr.Repetitions {
				sr.AvgFPS += r.Frames.FPS
				sr.AvgFrameMs += r.Frames.AvgFrameMs
				sr.P95FrameMs += r.Frames.P95FrameMs
				sr.AvgProcessCPUSec += r.ProcessCPUTicks
				sr.AvgTaskMs += r.TaskDeltaMs
				sr.AvgScriptMs += r.ScriptDeltaMs
				sr.AvgLayoutMs += r.LayoutDeltaMs
				sr.AvgRecalcMs += r.RecalcDeltaMs
				sr.AvgHeapMB += r.JSHeapMB
				sr.DOMNodes = r.DOMNodes
				sr.LongTasks += r.LongTaskCount
				if !r.Correctness.Passed {
					allPassed = false
					failReasons = append(failReasons, r.Correctness.Reason)
					if sr.FaultCode == "" && r.Correctness.FaultCode != "" {
						sr.FaultCode = r.Correctness.FaultCode
					}
				}
			}
			sr.AvgFPS /= n
			sr.AvgFrameMs /= n
			sr.P95FrameMs /= n
			sr.AvgProcessCPUSec /= n
			sr.AvgTaskMs /= n
			sr.AvgScriptMs /= n
			sr.AvgLayoutMs /= n
			sr.AvgRecalcMs /= n
			sr.AvgHeapMB /= n
			sr.CorrectnessPassed = allPassed
			if allPassed {
				sr.CorrectnessNote = sr.Repetitions[0].Correctness.Reason
			} else {
				sr.CorrectnessNote = strings.Join(failReasons, "; ")
			}
		}
		res.Scenarios = append(res.Scenarios, *sr)
	}
	return res, nil
}

func runSingleTrial(ctx context.Context, cfg *Config, url string, s TestRunSpec, rep, ord int, takeShot bool) (RepetitionResult, []byte, error) {
	if _, err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
		return RepetitionResult{}, nil, err
	}
	if cfg.Execution.WarmupSeconds > 0 {
		_, _ = chromedp.Run(ctx, chromedp.Sleep(time.Duration(cfg.Execution.WarmupSeconds*float64(time.Second))))
	}

	// Verify animation correctness before/during measurement
	corr := EvaluateCorrectness(ctx, s)

	// Start rAF collector in page
	_, _ = chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
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

	cpu0 := sampleChromeCPUSec(cfg.Browser.ProcessCPUHost)
	m0 := getCDPMetrics(ctx)

	_, _ = chromedp.Run(ctx, chromedp.Sleep(time.Duration(cfg.Execution.DurationSeconds*float64(time.Second))))

	m1 := getCDPMetrics(ctx)
	cpu1 := sampleChromeCPUSec(cfg.Browser.ProcessCPUHost)

	rawFrameJSON, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
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
	var fData struct {
		Frames        []float64 `json:"frames"`
		LongTaskCount int       `json:"longTaskCount"`
		LongTaskMs    float64   `json:"longTaskMs"`
	}
	_ = json.Unmarshal([]byte(rawFrameJSON), &fData)

	var ssBuf []byte
	if takeShot {
		ssBuf, _ = chromedp.Run(ctx, chromedp.CaptureScreenshot())
	}

	cpuDelta := 0.0
	if cpu0 >= 0 && cpu1 >= cpu0 {
		cpuDelta = cpu1 - cpu0
	}

	return RepetitionResult{
		Repetition:      rep,
		OrderIndex:      ord,
		ProcessCPUTicks: cpuDelta,
		Frames:          computeFrameStats(fData.Frames),
		LongTaskCount:   fData.LongTaskCount,
		LongTaskMs:      fData.LongTaskMs,
		TaskDeltaMs:     (m1["TaskDuration"] - m0["TaskDuration"]) * 1000,
		ScriptDeltaMs:   (m1["ScriptDuration"] - m0["ScriptDuration"]) * 1000,
		LayoutDeltaMs:   (m1["LayoutDuration"] - m0["LayoutDuration"]) * 1000,
		RecalcDeltaMs:   (m1["RecalcStyleDuration"] - m0["RecalcStyleDuration"]) * 1000,
		LayoutCount:     int64(m1["LayoutCount"] - m0["LayoutCount"]),
		JSHeapMB:        m1["JSHeapUsedSize"] / (1024 * 1024),
		DOMNodes:        int64(m1["Nodes"]),
		Correctness:     corr,
	}, ssBuf, nil
}

func computeFrameStats(frames []float64) FrameStats {
	if len(frames) == 0 {
		return FrameStats{}
	}
	sorted := append([]float64(nil), frames...)
	sort.Float64s(sorted)
	sum, maxF := 0.0, 0.0
	over16, over50 := 0, 0
	for _, f := range frames {
		sum += f
		if f > maxF {
			maxF = f
		}
		if f > 20.0 {
			over16++
		}
		if f > 50.0 {
			over50++
		}
	}
	mean := sum / float64(len(frames))
	p95 := sorted[int(math.Floor(0.95*float64(len(sorted)-1)))]
	return FrameStats{
		Frames:           len(frames),
		FPS:              1000.0 / mean,
		AvgFrameMs:       mean,
		P95FrameMs:       p95,
		MaxFrameMs:       maxF,
		LongFramesOver16: over16,
		LongFramesOver50: over50,
	}
}

func getCDPMetrics(ctx context.Context) map[string]float64 {
	res := map[string]float64{}
	out, err := chromedp.Call(ctx, performance.GetMetrics, cdp.Empty{})
	if err != nil {
		return res
	}
	for _, m := range out.Metrics {
		res[m.Name] = m.Value
	}
	return res
}

// sampleChromeCPUSec sums utime+stime (in seconds, assuming 100 Hz USER_HZ) across all chrome processes.
func sampleChromeCPUSec(sshHost string) float64 {
	cmdStr := `awk '{u+=$14; s+=$15} END {printf "%.4f", (u+s)/100}' /proc/$(pgrep -d, -f chrome | tr ',' ' ' 2>/dev/null)/stat 2>/dev/null`
	// Safer portable loop over pgrep
	sh := `tot=0; for p in $(pgrep -f chrome 2>/dev/null); do [ -r /proc/$p/stat ] && awk '{print $14+$15}' /proc/$p/stat 2>/dev/null; done | awk '{s+=$1} END {printf "%.4f", s/100}'`
	var out []byte
	var err error
	if sshHost != "" {
		out, err = exec.Command("ssh", sshHost, sh).Output()
	} else {
		_ = cmdStr
		out, err = exec.Command("bash", "-c", sh).Output()
	}
	if err != nil {
		return -1
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return -1
	}
	return v
}

func detectBindIP(cdpEndpoint string) string {
	if strings.Contains(cdpEndpoint, "127.0.0.1") || strings.Contains(cdpEndpoint, "localhost") {
		// Check if we have a LAN IP on eth0 (for remote Kiwiberry Chrome over tunneled 9222)
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
