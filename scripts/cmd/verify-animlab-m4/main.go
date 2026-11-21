package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type checkResult struct {
	ID     string
	Passed bool
	Detail string
}

func main() {
	os.Exit(run())
}

func run() int {
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
	evidenceDir := filepath.Join("testlab", "m4-evidence")
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

	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire browser session: %v\n", err)
		return 1
	}
	defer sess.Close()
	allocCtx := sess.AllocCtx

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	setPage := func(name, html string) string {
		mu.Lock()
		pages[name] = html
		mu.Unlock()
		return baseURL + name
	}

	// =========================================================================
	// AT-M4-01 & AT-M4-02: Flattened-background DT scene + 100 window tracers (Demo A)
	// =========================================================================
	{
		cfg, err := animlab.LoadConfig("testlab/configs/dt-building-window-tracers.yaml")
		if err != nil {
			record("AT-M4-01", false, err.Error())
			record("AT-M4-02", false, err.Error())
		} else {
			specs := animlab.ExpandScenarios(cfg)
			spec := specs[0] // css 100 window tracers
			html := animlab.BuildTestPageHTML(spec)
			_ = os.WriteFile(filepath.Join(evidenceDir, "demo-a-digital-twin.html"), []byte(html), 0644)
			u := setPage("dt100.html", html)

			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Call(tCtx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
				Width: 1320, Height: 800, DeviceScaleFactor: 1, Mobile: false,
			})
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(u))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(300*time.Millisecond))

			bgCheck, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`(() => {
				const img = document.getElementById('labDtBg');
				const svgs = document.querySelectorAll('svg').length;
				const insts = document.querySelectorAll('.lab-shared-svg .lab-instance').length;
				const directChildren = document.querySelector('.lab-shared-svg').children.length;
				return JSON.stringify({
					bgLoaded: !!(img && img.complete && img.naturalWidth === 1280 && img.naturalHeight === 720),
					svgs, insts, directChildren
				});
			})()`))
			corr := animlab.EvaluateCorrectness(tCtx, spec)
			shot, _ := chromedp.Run(tCtx, chromedp.CaptureScreenshot())
			_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m4-01-dt-scene.png"), shot, 0644)
			tCancel()

			m401OK := strings.Contains(bgCheck, `"bgLoaded":true`) &&
				strings.Contains(bgCheck, `"svgs":1`) &&
				strings.Contains(bgCheck, `"directChildren":100`)
			record("AT-M4-01", m401OK, bgCheck)
			record("AT-M4-02", corr.Passed && strings.Contains(corr.Reason, "100 instances"), corr.Reason)
		}
	}

	// =========================================================================
	// AT-M4-03: Configurable DT load 50, 100, 250 via YAML without recompilation
	// =========================================================================
	{
		binPath := "/tmp/anim-test-lab-m4"
		if out, err := exec.Command("go", "build", "-o", binPath, "./cmd/anim-test-lab").CombinedOutput(); err != nil {
			record("AT-M4-03", false, fmt.Sprintf("build failed: %v (%s)", err, string(out)))
		} else {
			baseBytes, _ := os.ReadFile("testlab/configs/dt-building-window-tracers.yaml")
			tmpYAML := filepath.Join(evidenceDir, "dt-scale-temp.yaml")
			allOK := true
			var countsOut []string
			for _, cnt := range []int{50, 100, 250} {
				mut := strings.Replace(string(baseBytes), "counts: [100]", fmt.Sprintf("counts: [%d]", cnt), 1)
				mut = strings.Replace(mut, "inherits: base-kiwiberry.yaml", "inherits: ../configs/base-kiwiberry.yaml", 1)
				_ = os.WriteFile(tmpYAML, []byte(mut), 0644)
				vOut, vErr := exec.Command(binPath, "validate", "-c", tmpYAML).CombinedOutput()
				cfgC, lErr := animlab.LoadConfig(tmpYAML)
				if vErr != nil || lErr != nil {
					allOK = false
					countsOut = append(countsOut, fmt.Sprintf("%d:err(%v/%s)", cnt, vErr, string(vOut)))
					continue
				}
				sp := animlab.ExpandScenarios(cfgC)[0]
				u := setPage(fmt.Sprintf("dt-%d.html", cnt), animlab.BuildTestPageHTML(sp))
				tCtx, tCancel := chromedp.NewContext(allocCtx)
				_, _ = chromedp.Run(tCtx, chromedp.Navigate(u))
				domCnt, _ := chromedp.Run(tCtx, chromedp.Evaluate[int](`document.querySelectorAll('.lab-instance').length`))
				tCancel()
				if domCnt != cnt {
					allOK = false
				}
				countsOut = append(countsOut, fmt.Sprintf("%d->dom:%d", cnt, domCnt))
			}
			_ = os.Remove(tmpYAML)
			_ = os.Remove(binPath)
			record("AT-M4-03", allOK, strings.Join(countsOut, ", "))
		}
	}

	// =========================================================================
	// AT-M4-04: Equivalent layout comparison across all 4 arrangements
	// =========================================================================
	{
		arrs := []string{"shared-svg-scene", "shared-dt-scene", "independent-svg-roots", "stage16c-widgets"}
		allOK := true
		var notes []string
		for _, arr := range arrs {
			var spec animlab.TestRunSpec
			if arr == "stage16c-widgets" {
				cfgW, _ := animlab.LoadConfig("testlab/configs/stage16c-widget-dashboard.yaml")
				cfgW.Instances.Counts = []int{12}
				spec = animlab.ExpandScenarios(cfgW)[0]
			} else {
				cfgD, _ := animlab.LoadConfig("testlab/configs/dt-building-window-tracers.yaml")
				cfgD.Instances.Counts = []int{12}
				cfgD.Instances.Isolation = arr
				spec = animlab.ExpandScenarios(cfgD)[0]
			}
			u := setPage("arr-"+arr+".html", animlab.BuildTestPageHTML(spec))
			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(u))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(200*time.Millisecond))
			info, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`JSON.stringify({
				svgs: document.querySelectorAll('svg').length,
				bg: document.querySelectorAll('#labDtBg').length,
				widgets: (window.__labWidgetInstances || []).length
			})`))
			tCancel()
			notes = append(notes, arr+"="+info)
			if arr == "shared-svg-scene" && !strings.Contains(info, `"svgs":1,"bg":0`) {
				allOK = false
			}
			if arr == "shared-dt-scene" && !strings.Contains(info, `"svgs":1,"bg":1`) {
				allOK = false
			}
			if arr == "independent-svg-roots" && !strings.Contains(info, `"svgs":12,"bg":0`) {
				allOK = false
			}
			if arr == "stage16c-widgets" && !strings.Contains(info, `"svgs":12,"bg":0,"widgets":12`) {
				allOK = false
			}
		}
		record("AT-M4-04", allOK, strings.Join(notes, "; "))
	}

	// =========================================================================
	// AT-M4-05 & AT-M4-06: Complete Stage 16C runtime + 100-widget independence (Demo B)
	// =========================================================================
	{
		cfgW, err := animlab.LoadConfig("testlab/configs/stage16c-widget-dashboard.yaml")
		if err != nil {
			record("AT-M4-05", false, err.Error())
			record("AT-M4-06", false, err.Error())
		} else {
			spec := animlab.ExpandScenarios(cfgW)[0]
			html := animlab.BuildTestPageHTML(spec)
			_ = os.WriteFile(filepath.Join(evidenceDir, "demo-b-stage16c-dashboard.html"), []byte(html), 0644)
			u := setPage("s16c-100.html", html)

			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Call(tCtx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
				Width: 1060, Height: 760, DeviceScaleFactor: 1, Mobile: false,
			})
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(u))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(350*time.Millisecond))

			corr := animlab.EvaluateCorrectness(tCtx, spec)
			shot, _ := chromedp.Run(tCtx, chromedp.CaptureScreenshot())
			_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m4-02-stage16c-widgets.png"), shot, 0644)

			// Pause background ticker so we can assert deterministic surgical independence on #42 and #77
			indepJSON, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`(() => {
				if (window.__labWidgetUpdateTimer) { clearInterval(window.__labWidgetUpdateTimer); window.__labWidgetUpdateTimer = null; }
				const before0 = window.__labGetWidgetParam(0, 'hostname-node-text');
				const before41 = window.__labGetWidgetParam(41, 'hostname-node-text');
				const before99 = window.__labGetWidgetParam(99, 'hostname-node-text');
				window.__labSetWidgetParam(42, 'hostname-node-text', 'sentinel-pve-42');
				window.__labSetWidgetParam(77, 'hostname-node-text', 'sentinel-pve-77');
				const after0 = window.__labGetWidgetParam(0, 'hostname-node-text');
				const after41 = window.__labGetWidgetParam(41, 'hostname-node-text');
				const after42 = window.__labGetWidgetParam(42, 'hostname-node-text');
				const after77 = window.__labGetWidgetParam(77, 'hostname-node-text');
				const after99 = window.__labGetWidgetParam(99, 'hostname-node-text');
				return JSON.stringify({
					pkgId: window.__labWidgetManifest.packageId,
					unchangedOthers: before0 === after0 && before41 === after41 && before99 === after99,
					after42, after77
				});
			})()`))
			tCancel()

			record("AT-M4-05", corr.Passed && strings.Contains(indepJSON, "host-monitor-v1"), corr.Reason+" "+indepJSON)
			indepOK := strings.Contains(indepJSON, `"unchangedOthers":true`) &&
				strings.Contains(indepJSON, `"after42":"sentinel-pve-42"`) &&
				strings.Contains(indepJSON, `"after77":"sentinel-pve-77"`)
			record("AT-M4-06", indepOK, indepJSON)
		}
	}

	// =========================================================================
	// AT-M4-07: Visibility suspension on DT overlays & Stage 16C widgets (with off-screen setParameter)
	// =========================================================================
	{
		cfgW, _ := animlab.LoadConfig("testlab/configs/stage16c-widget-dashboard.yaml")
		cfgW.Instances.Counts = []int{24}
		cfgW.Workloads = []animlab.WorkloadSpec{{Name: "vis-8", Visibility: "dashboard", VisibleCount: 8, SuspendOffscreen: true}}
		cfgW.WidgetRuntime.Updates.Enabled = false
		spW := animlab.ExpandScenarios(cfgW)[0]
		uW := setPage("s16c-susp.html", animlab.BuildTestPageHTML(spW))

		tCtx, tCancel := chromedp.NewContext(allocCtx)
		_, _ = chromedp.Run(tCtx, chromedp.Navigate(uW))
		_, _ = chromedp.Run(tCtx, chromedp.Sleep(300*time.Millisecond))
		corrW := animlab.EvaluateCorrectness(tCtx, spW)
		// Update parameter on off-screen widget #20, then scroll into view & verify persisted
		offParamRes, _ := chromedp.Run(tCtx, chromedp.Evaluate[string](`(() => {
			window.__labSetWidgetParam(20, 'hostname-node-text', 'offscreen-updated-20');
			const card = document.getElementById('wcard_20');
			if (card) card.scrollIntoView({ behavior: 'instant', block: 'center' });
			return window.__labGetWidgetParam(20, 'hostname-node-text');
		})()`))
		tCancel()

		// DT overlay suspension check
		cfgDT, _ := animlab.LoadConfig("testlab/configs/dt-building-window-tracers.yaml")
		cfgDT.Instances.Counts = []int{40}
		cfgDT.Workloads = []animlab.WorkloadSpec{{Name: "vis-10", Visibility: "dashboard", VisibleCount: 10, SuspendOffscreen: true}}
		spDT := animlab.ExpandScenarios(cfgDT)[0]
		uDT := setPage("dt-susp.html", animlab.BuildTestPageHTML(spDT))
		tCtx2, tCancel2 := chromedp.NewContext(allocCtx)
		_, _ = chromedp.Run(tCtx2, chromedp.Navigate(uDT))
		_, _ = chromedp.Run(tCtx2, chromedp.Sleep(250*time.Millisecond))
		corrDT := animlab.EvaluateCorrectness(tCtx2, spDT)
		tCancel2()

		ok := corrW.Passed && corrW.SuspendedCount > 0 && offParamRes == "offscreen-updated-20" &&
			corrDT.Passed && corrDT.SuspendedCount > 0
		record("AT-M4-07", ok, fmt.Sprintf("widgetSusp=%d offParam=%s dtSusp=%d", corrW.SuspendedCount, offParamRes, corrDT.SuspendedCount))
	}

	// =========================================================================
	// AT-M4-08: Mixed DT workload over flattened background
	// =========================================================================
	{
		cfgM, err := animlab.LoadConfig("testlab/configs/dt-mixed-workload.yaml")
		if err != nil {
			record("AT-M4-08", false, err.Error())
		} else {
			var mixSpec *animlab.TestRunSpec
			for _, s := range animlab.ExpandScenarios(cfgM) {
				if s.IsMixture {
					cp := s
					mixSpec = &cp
					break
				}
			}
			u := setPage("dt-mix.html", animlab.BuildTestPageHTML(*mixSpec))
			tCtx, tCancel := chromedp.NewContext(allocCtx)
			_, _ = chromedp.Run(tCtx, chromedp.Navigate(u))
			_, _ = chromedp.Run(tCtx, chromedp.Sleep(250*time.Millisecond))
			corr := animlab.EvaluateCorrectness(tCtx, *mixSpec)
			tCancel()
			record("AT-M4-08", corr.Passed, fmt.Sprintf("alloc=%v reason=%s", mixSpec.MixtureAllocation, corr.Reason))
		}
	}

	// =========================================================================
	// AT-M4-09: Lifecycle isolation across repeated complete Stage 16C widget trials
	// =========================================================================
	{
		cfgL, _ := animlab.LoadConfig("testlab/configs/stage16c-widget-dashboard.yaml")
		cfgL.Instances.Counts = []int{30}
		cfgL.Execution.WarmupSeconds = 0.2
		cfgL.Execution.DurationSeconds = 0.8
		cfgL.Execution.Repetitions = 3
		cfgL.Browser.HostBindIP = bindIP
		cfgL.Measurements = []string{"frame_timing", "memory", "animation_correctness"}
		res, err := animlab.RunSuite(cfgL, evidenceDir, "")
		if err != nil {
			record("AT-M4-09", false, err.Error())
		} else {
			var minD, maxD int64 = math.MaxInt64, 0
			for _, r := range res.Scenarios[0].Repetitions {
				if r.DOMNodes != nil {
					if *r.DOMNodes < minD {
						minD = *r.DOMNodes
					}
					if *r.DOMNodes > maxD {
						maxD = *r.DOMNodes
					}
				}
			}
			spread := maxD - minD
			record("AT-M4-09", res.Scenarios[0].CorrectnessPassed && spread <= 20,
				fmt.Sprintf("reps=3 domMin=%d domMax=%d spread=%d", minD, maxD, spread))
		}
	}

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d M4 checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		return 1
	}
	return 0
}
