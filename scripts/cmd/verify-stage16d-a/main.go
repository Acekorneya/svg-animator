package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	studioURL   = "http://192.168.100.40:8088/"
	benchURL    = "http://192.168.100.40:8092/stage16d-benchmark/index.html"
	cdpEndpoint = "http://127.0.0.1:9222"
	remoteDir   = "/home/nigel/acceptance-demo/stage16d-a-widget"
)

type CheckResult struct {
	Number  int
	Name    string
	Passed  bool
	Message string
}

func runVoid(ctx context.Context, actions ...chromedp.Action[chromedp.Void]) error {
	for _, a := range actions {
		if _, err := chromedp.Run(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16D-A: Real-Browser Chrome CDP Verification")
	fmt.Println("==================================================================")

	var (
		results []CheckResult
		mu      sync.Mutex
	)
	record := func(num int, name string, passed bool, msg string) {
		mu.Lock()
		defer mu.Unlock()
		results = append(results, CheckResult{Number: num, Name: name, Passed: passed, Message: msg})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] Criteria %2d: %-36s - %s\n", status, num, name, msg)
	}

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cancel := context.WithTimeout(allocCtx, 90*time.Second)
	defer cancel()
	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	// 1. Open Studio and verify Low-Load Effects UI + Go/WASM creation
	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1366, 900),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		log.Fatalf("Failed to open Studio: %v", err)
	}

	// Wait for WASM engine readiness (up to 15s with cache disabled)
	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(500*time.Millisecond))
	}

	studioReport, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);

		document.getElementById('railBtn-lite-effects').click();
		document.getElementById('btnAddLiteTracer').click();
		document.getElementById('btnAddLitePulse').click();
		document.getElementById('btnAddLiteFlash').click();

		const list = window.LiteEffects.getAllLiteEffects();
		const svg = document.querySelector('#svgContainer svg');
		const tracerEl = svg ? svg.querySelector('[data-semantic-target="lite-1"] .lite-payload') : null;
		const pulseEl = svg ? svg.querySelector('[data-semantic-target="lite-2"] .lite-payload') : null;
		const flashEl = svg ? svg.querySelector('[data-semantic-target="lite-3"] .lite-payload') : null;
		const hasFilter = svg ? Boolean(svg.querySelector('[data-lite-id] filter')) : false;

		return JSON.stringify({
			count: list.length,
			pathLen: tracerEl ? tracerEl.getAttribute('pathLength') : '',
			dashArr: tracerEl ? tracerEl.getAttribute('stroke-dasharray') : '',
			hasPulse: Boolean(pulseEl),
			hasFlash: Boolean(flashEl),
			hasFilter
		});
	})()`))
	if err != nil {
		log.Fatalf("Studio evaluation failed: %v", err)
	}

	var stData struct {
		Count    int    `json:"count"`
		PathLen  string `json:"pathLen"`
		DashArr  string `json:"dashArr"`
		HasPulse bool   `json:"hasPulse"`
		HasFlash bool   `json:"hasFlash"`
		HasFilt  bool   `json:"hasFilter"`
	}
	_ = json.Unmarshal([]byte(studioReport), &stData)
	record(1, "Studio Low-Load Effects Creation",
		stData.Count == 3 && stData.PathLen == "1000" && stData.DashArr == "125 375" && stData.HasPulse && stData.HasFlash && !stData.HasFilt,
		fmt.Sprintf("count=%d, pathLength=%s, dasharray=%s, pulse=%v, flash=%v, blurFilter=%v",
			stData.Count, stData.PathLen, stData.DashArr, stData.HasPulse, stData.HasFlash, stData.HasFilt))

	// 2. Rename, bind parameters, and export standalone widget package
	exportJson, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', { name: 'border' });
		window.LiteEffects.updateLiteEffect('2', { name: 'heartbeat' });
		window.LiteEffects.updateLiteEffect('3', { name: 'alert' });

		window.EngineBridge.addWidgetParameter({ name: 'border-lite-color', type: 'color', target: 'lite-1', property: 'color', default: '#38bdf8' });
		window.EngineBridge.addWidgetParameter({ name: 'border-lite-speed', type: 'speed', target: 'lite-1', property: 'speed', default: 1.0, min: 0.1, max: 5.0 });
		window.EngineBridge.addWidgetParameter({ name: 'heartbeat-lite-opacity', type: 'number', target: 'lite-2', property: 'opacity', default: 0.9, min: 0, max: 1 });
		window.EngineBridge.addWidgetParameter({ name: 'alert-lite-trigger', type: 'number', target: 'lite-3', property: 'trigger', default: 0, min: 0, max: 1000000 });

		const rawSvg = window.SceneState.buildFullSVG(window.currentRenderState);
		const res = window.EngineBridge.buildWidgetPackage('lite-effects-demo', rawSvg, '');
		return JSON.stringify({ success: Boolean(res && res.success), zipB64: res ? res.zipB64 : '', err: res ? res.error : '' });
	})()`))
	if err != nil {
		log.Fatalf("Export failed: %v", err)
	}
	var expData struct {
		Success bool   `json:"success"`
		ZipB64  string `json:"zipB64"`
		Err     string `json:"err"`
	}
	_ = json.Unmarshal([]byte(exportJson), &expData)
	record(2, "Lite Effects Widget Package Export", expData.Success && len(expData.ZipB64) > 1000,
		fmt.Sprintf("success=%v, err=%q", expData.Success, expData.Err))

	// Unpack ZIP on Kiwiberry and test via file://
	zipBytes, _ := base64.StdEncoding.DecodeString(expData.ZipB64)
	_ = os.WriteFile("/tmp/lite-effects-demo-package.zip", zipBytes, 0644)
	if out, err := exec.Command("scp", "/tmp/lite-effects-demo-package.zip", "nigel@192.168.100.40:/tmp/lite-effects-demo-package.zip").CombinedOutput(); err != nil {
		log.Fatalf("SCP failed: %v, %s", err, string(out))
	}
	// Inject the latest svg-widget.js into the unpacked package as well so demo.html uses the latest runtime
	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && unzip -q -o /tmp/lite-effects-demo-package.zip -d %s", remoteDir, remoteDir, remoteDir)
	if out, err := exec.Command("ssh", "nigel@192.168.100.40", unzipCmd).CombinedOutput(); err != nil {
		log.Fatalf("Unzip failed: %v, %s", err, string(out))
	}
	_, _ = exec.Command("scp", "internal/web/static/js/svg-widget.js", fmt.Sprintf("nigel@192.168.100.40:%s/lite-effects-demo/js/svg-widget.js", remoteDir)).CombinedOutput()

	demoURL := fmt.Sprintf("file://%s/lite-effects-demo/demo.html", remoteDir)
	if err := runVoid(taskCtx, chromedp.Navigate(demoURL), chromedp.WaitReady("#container-unit-a svg"), chromedp.Sleep(500*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open exported demo.html: %v", err)
	}

	demoCheck, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const wA = instances['unit-a'];
		const wB = instances['unit-b'];
		wA.setParameter('border-lite-color', '#ef4444');
		wA.setParameter('border-lite-speed', 2.5);
		wA.setParameter('heartbeat-lite-opacity', 0.45);
		wA.setParameter('alert-lite-trigger', 1);

		const strokeA = wA.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')?.getAttribute('stroke') || '';
		const strokeB = wB.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')?.getAttribute('stroke') || '';
		const trigA = wA.svgElement.querySelector('[data-semantic-target="lite-3"] .lite-payload')?.getAttribute('data-trigger-nonce') || '';
		return JSON.stringify({ strokeA, strokeB, trigA });
	})()`))
	var dData struct {
		StrokeA string `json:"strokeA"`
		StrokeB string `json:"strokeB"`
		TrigA   string `json:"trigA"`
	}
	_ = json.Unmarshal([]byte(demoCheck), &dData)
	record(3, "Exported file:// Runtime Controls", dData.StrokeA == "#ef4444" && dData.StrokeB == "#38bdf8" && dData.TrigA == "1",
		fmt.Sprintf("unit-a stroke=%s, unit-b stroke=%s, flash trigger=%s", dData.StrokeA, dData.StrokeB, dData.TrigA))

	// 3. Verify real-browser IntersectionObserver suspension & scroll recovery
	if err := runVoid(taskCtx, chromedp.Navigate(benchURL), chromedp.WaitReady("#grid"), chromedp.Sleep(300*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open benchmark URL: %v", err)
	}
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[string](`(async () => JSON.stringify(await window.BenchmarkHarness.setup({count:30,layout:'dashboard',mode:'animated',visibilityAware:true})))()`,
		func(p *cdpruntime.EvaluateParams) { b := true; p.AwaitPromise = &b }))
	// Use AwaitPromise via simple polling / sleep
	_ = runVoid(taskCtx, chromedp.Sleep(800*time.Millisecond))

	visStatsBefore, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.BenchmarkHarness.getStats())`))
	var sb struct {
		Mounted   int `json:"mounted"`
		Visible   int `json:"visible"`
		Active    int `json:"active"`
		Suspended int `json:"suspended"`
	}
	_ = json.Unmarshal([]byte(visStatsBefore), &sb)

	// Scroll to bottom and verify hidden widgets resume and retain updates
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.BenchmarkHarness.stepUpdates(1.0, 3);
		const vp = document.getElementById('scrollViewport');
		vp.scrollTop = vp.scrollHeight;
		return true;
	})()`))
	_ = runVoid(taskCtx, chromedp.Sleep(400*time.Millisecond))

	visStatsAfter, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const cards = document.querySelectorAll('.bench-card');
		const lastWrapper = cards[cards.length - 1]?.querySelector('.sva-widget-wrapper');
		const lastText = cards[cards.length - 1]?.querySelector('[data-semantic-target="node-1"] text')?.textContent || '';
		const lastSuspended = lastWrapper ? lastWrapper.classList.contains('sva-suspended') : true;
		const firstWrapper = cards[0]?.querySelector('.sva-widget-wrapper');
		const firstSuspended = firstWrapper ? firstWrapper.classList.contains('sva-suspended') : false;
		return JSON.stringify({ lastText, lastSuspended, firstSuspended });
	})()`))
	var sa struct {
		LastText       string `json:"lastText"`
		LastSuspended  bool   `json:"lastSuspended"`
		FirstSuspended bool   `json:"firstSuspended"`
	}
	_ = json.Unmarshal([]byte(visStatsAfter), &sa)
	record(4, "Scroll Visibility Suspend & Resume",
		sb.Suspended == 21 && sb.Active == 9 && !sa.LastSuspended && sa.FirstSuspended && strings.Contains(sa.LastText, "node-30"),
		fmt.Sprintf("Before scroll: vis=%d/susp=%d; After scroll: firstSusp=%v, lastSusp=%v, lastText=%q",
			sb.Visible, sb.Suspended, sa.FirstSuspended, sa.LastSuspended, sa.LastText))

	fmt.Println("\n==================================================================")
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
		}
	}
	if !allPassed {
		os.Exit(1)
	}
	fmt.Println(">>> ALL 4 STAGE 16D-A REAL-BROWSER CDP CRITERIA PASS 100% <<<")
}
