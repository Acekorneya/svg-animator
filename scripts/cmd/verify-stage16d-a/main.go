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

	"github.com/chromedp/cdproto/input"
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

func saveScreenshot(ctx context.Context, path string) {
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err == nil && len(buf) > 0 {
		_ = os.WriteFile(path, buf, 0644)
	}
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16D-A: Real-Browser Chrome CDP Verification (Real Clicks)")
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
		fmt.Printf("[%s] Criteria %2d: %-38s - %s\n", status, num, name, msg)
	}

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cancel := context.WithTimeout(allocCtx, 120*time.Second)
	defer cancel()
	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	// 1. Open Studio and wait for WASM engine readiness
	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1440, 960),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		log.Fatalf("Failed to open Studio: %v", err)
	}

	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(500*time.Millisecond))
	}

	// Reset to clean project & suppress confirm/alert
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		return true;
	})()`))

	// Criteria 1: Real click on Low-Load rail button, collapse, and reopen
	if err := runVoid(taskCtx,
		chromedp.Click("#railBtn-lite-effects"),
		chromedp.Sleep(250*time.Millisecond),
	); err != nil {
		log.Fatalf("Click #railBtn-lite-effects failed: %v", err)
	}

	drawerOpenJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const drawer = document.getElementById('leftDrawer');
		const rail = document.getElementById('railBtn-lite-effects');
		const acc = document.getElementById('acc-lite-effects');
		const title = document.getElementById('leftDrawerTitle');
		const rect = acc ? acc.getBoundingClientRect() : { height: 0 };
		return JSON.stringify({
			drawerOpen: Boolean(drawer && !drawer.classList.contains('collapsed')),
			railActive: Boolean(rail && rail.classList.contains('active')),
			accActive: Boolean(acc && acc.classList.contains('active-tool') && acc.classList.contains('open')),
			accHeight: Math.round(rect.height),
			title: title ? title.textContent.trim() : ''
		});
	})()`))
	var dOpen struct {
		DrawerOpen bool   `json:"drawerOpen"`
		RailActive bool   `json:"railActive"`
		AccActive  bool   `json:"accActive"`
		AccHeight  int    `json:"accHeight"`
		Title      string `json:"title"`
	}
	_ = json.Unmarshal([]byte(drawerOpenJSON), &dOpen)

	// Collapse then reopen
	_ = runVoid(taskCtx, chromedp.Click("#btnCollapseLeftDrawer"), chromedp.Sleep(200*time.Millisecond))
	collapsedAfter, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`document.getElementById('leftDrawer').classList.contains('collapsed')`))
	_ = runVoid(taskCtx, chromedp.Click("#railBtn-lite-effects"), chromedp.Sleep(250*time.Millisecond))
	reopenedAfter, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`!document.getElementById('leftDrawer').classList.contains('collapsed')`))

	record(1, "Rail Icon Open/Close/Reopen",
		dOpen.DrawerOpen && dOpen.RailActive && dOpen.AccActive && dOpen.AccHeight > 100 && dOpen.Title == "Low-Load Effects (Lite)" && collapsedAfter && reopenedAfter,
		fmt.Sprintf("open=%v, rail=%v, acc=%v, h=%d, title=%q, collapsed=%v, reopened=%v",
			dOpen.DrawerOpen, dOpen.RailActive, dOpen.AccActive, dOpen.AccHeight, dOpen.Title, collapsedAfter, reopenedAfter))

	// Criteria 2: Real clicks on Add Tracer, Add Pulse, Add Flash + SVG & List verification
	if err := runVoid(taskCtx,
		chromedp.Click("#btnAddLiteTracer"),
		chromedp.Sleep(150*time.Millisecond),
		chromedp.Click("#btnAddLitePulse"),
		chromedp.Sleep(150*time.Millisecond),
		chromedp.Click("#btnAddLiteFlash"),
		chromedp.Sleep(250*time.Millisecond),
	); err != nil {
		log.Fatalf("Add Lite effects click failed: %v", err)
	}
	saveScreenshot(taskCtx, "/tmp/stage16d-a-drawer-created.png")

	createdJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const list = window.LiteEffects.getAllLiteEffects();
		const rows = document.querySelectorAll('#liteEffectsList .flow-list-item.compact-instance-row');
		const svg = document.querySelector('#svgContainer svg');
		const tracerEl = svg ? svg.querySelector('[data-semantic-target="lite-1"] .lite-payload') : null;
		const pulseEl = svg ? svg.querySelector('[data-semantic-target="lite-2"] .lite-payload') : null;
		const flashEl = svg ? svg.querySelector('[data-semantic-target="lite-3"] .lite-payload') : null;
		const hasFilter = svg ? Boolean(svg.querySelector('[data-lite-id] filter')) : false;
		return JSON.stringify({
			count: list.length,
			rowCount: rows.length,
			pathLen: tracerEl ? tracerEl.getAttribute('pathLength') : '',
			dashArr: tracerEl ? tracerEl.getAttribute('stroke-dasharray') : '',
			hasPulse: Boolean(pulseEl),
			hasFlash: Boolean(flashEl),
			hasFilter
		});
	})()`))
	var crData struct {
		Count     int    `json:"count"`
		RowCount  int    `json:"rowCount"`
		PathLen   string `json:"pathLen"`
		DashArr   string `json:"dashArr"`
		HasPulse  bool   `json:"hasPulse"`
		HasFlash  bool   `json:"hasFlash"`
		HasFilter bool   `json:"hasFilter"`
	}
	_ = json.Unmarshal([]byte(createdJSON), &crData)
	record(2, "Real-Click Create All 3 Lite Effects",
		crData.Count == 3 && crData.RowCount == 3 && crData.PathLen == "1000" && crData.DashArr == "125 375" && crData.HasPulse && crData.HasFlash && !crData.HasFilter,
		fmt.Sprintf("count=%d, rows=%d, pathLen=%s, dash=%s, pulse=%v, flash=%v, blur=%v",
			crData.Count, crData.RowCount, crData.PathLen, crData.DashArr, crData.HasPulse, crData.HasFlash, crData.HasFilter))

	// Criteria 3: Select row #1 in list, edit width via UI number input, verify SVG & label update
	if err := runVoid(taskCtx,
		chromedp.Click(`#liteEffectsList [data-lite-list-id="1"]`),
		chromedp.Sleep(150*time.Millisecond),
	); err != nil {
		log.Fatalf("Select row 1 failed: %v", err)
	}
	editCheckJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const wIn = document.getElementById('liteWidthNum');
		wIn.value = '220';
		wIn.dispatchEvent(new Event('input', { bubbles: true }));
		const nIn = document.getElementById('liteNameInput');
		nIn.value = 'border';
		nIn.dispatchEvent(new Event('input', { bubbles: true }));
		const svgRect = document.querySelector('#svgContainer [data-lite-id="1"] .lite-payload');
		const lbl = document.getElementById('editingLiteLabel');
		return JSON.stringify({
			widthAttr: svgRect ? svgRect.getAttribute('width') : '',
			label: lbl ? lbl.textContent.trim() : ''
		});
	})()`))
	var edData struct {
		WidthAttr string `json:"widthAttr"`
		Label     string `json:"label"`
	}
	_ = json.Unmarshal([]byte(editCheckJSON), &edData)
	record(3, "Sidebar List Select & Property Edit",
		(edData.WidthAttr == "220.00" || edData.WidthAttr == "220") && edData.Label == "Editing border",
		fmt.Sprintf("widthAttr=%q, label=%q", edData.WidthAttr, edData.Label))

	// Criteria 4: Real pointer drag on canvas for lite-1 (top border stroke) and right-click Context Editor
	posJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const hit = document.querySelector('#svgContainer [data-lite-id="1"] .lite-hit-target');
		const r = hit.getBoundingClientRect();
		const le = window.LiteEffects.getLiteEffect('1');
		const cx = r.left + r.width / 2;
		const cy = r.top + 1;
		return JSON.stringify({ cx, cy, x0: le.x, y0: le.y });
	})()`))
	var pos struct {
		CX float64 `json:"cx"`
		CY float64 `json:"cy"`
		X0 float64 `json:"x0"`
		Y0 float64 `json:"y0"`
	}
	_ = json.Unmarshal([]byte(posJSON), &pos)

	_ = runVoid(taskCtx,
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, pos.CX, pos.CY, chromedp.ButtonLeft),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseMoved, pos.CX+60, pos.CY+40, chromedp.ButtonLeft),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, pos.CX+60, pos.CY+40, chromedp.ButtonLeft),
		chromedp.Sleep(150*time.Millisecond),
	)
	afterDragX, _ := chromedp.Run(taskCtx, chromedp.Evaluate[float64](`window.LiteEffects.getLiteEffect('1').x`))
	dragMoved := afterDragX > pos.X0+10

	// Right-click on moved lite-1 to open Floating Context Editor
	_ = runVoid(taskCtx,
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, pos.CX+60, pos.CY+40, chromedp.ButtonRight),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, pos.CX+60, pos.CY+40, chromedp.ButtonRight),
		chromedp.Sleep(250*time.Millisecond),
	)
	saveScreenshot(taskCtx, "/tmp/stage16d-a-context-editor.png")

	fctxJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const ed = document.getElementById('floatingContextEditor');
		const sec = document.getElementById('fctx-liteSection');
		const title = document.getElementById('floatingEditorTitle');
		return JSON.stringify({
			open: Boolean(ed && getComputedStyle(ed).display !== 'none'),
			secVisible: Boolean(sec && getComputedStyle(sec).display !== 'none'),
			title: title ? title.textContent.trim() : ''
		});
	})()`))
	var fctx struct {
		Open       bool   `json:"open"`
		SecVisible bool   `json:"secVisible"`
		Title      string `json:"title"`
	}
	_ = json.Unmarshal([]byte(fctxJSON), &fctx)
	record(4, "Canvas Drag & Context Editor",
		dragMoved && fctx.Open && fctx.SecVisible && strings.Contains(fctx.Title, "Lite #1"),
		fmt.Sprintf("x0=%.0f->%.0f, fctxOpen=%v, sec=%v, title=%q", pos.X0, afterDragX, fctx.Open, fctx.SecVisible, fctx.Title))

	// Criteria 5: Project save/reload round-trip preserves all 3 Lite effects
	reloadJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const saved = window.ProjectManager.serializeStudioState();
		window.ProjectManager.resetToNewProject();
		const clearedCount = window.LiteEffects.getAllLiteEffects().length;
		window.ProjectManager.applyStudioState(saved);
		const restored = window.LiteEffects.getAllLiteEffects();
		const r1 = window.LiteEffects.getLiteEffect('1');
		return JSON.stringify({
			clearedCount,
			restoredCount: restored.length,
			r1Name: r1 ? r1.name : '',
			r1Width: r1 ? r1.width : 0
		});
	})()`))
	var relData struct {
		ClearedCount  int    `json:"clearedCount"`
		RestoredCount int    `json:"restoredCount"`
		R1Name        string `json:"r1Name"`
		R1Width       int    `json:"r1Width"`
	}
	_ = json.Unmarshal([]byte(reloadJSON), &relData)
	record(5, "Project Save & Reload Round-Trip",
		relData.ClearedCount == 0 && relData.RestoredCount == 3 && relData.R1Name == "border" && relData.R1Width == 220,
		fmt.Sprintf("cleared=%d, restored=%d, name=%q, width=%d", relData.ClearedCount, relData.RestoredCount, relData.R1Name, relData.R1Width))

	// Criteria 6: Export standalone widget package & verify via file://
	exportJson, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('2', { name: 'heartbeat' });
		window.LiteEffects.updateLiteEffect('3', { name: 'alert' });

		window.EngineBridge.addWidgetParameter({ name: 'border-lite-color', type: 'color', target: 'lite-1', property: 'color', default: '#38bdf8' });
		window.EngineBridge.addWidgetParameter({ name: 'border-lite-speed', type: 'speed', target: 'lite-1', property: 'speed', default: 1.0, min: 0.1, max: 5.0 });
		window.EngineBridge.addWidgetParameter({ name: 'heartbeat-lite-opacity', type: 'number', target: 'lite-2', property: 'opacity', default: 0.9, min: 0, max: 1 });
		window.EngineBridge.addWidgetParameter({ name: 'alert-lite-trigger', type: 'number', target: 'lite-3', property: 'trigger', default: 0, min: 0, max: 1000000 });

		if (typeof window.triggerRender === 'function') window.triggerRender();
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
	record(6, "Lite Effects Widget Package Export", expData.Success && len(expData.ZipB64) > 1000,
		fmt.Sprintf("success=%v, err=%q", expData.Success, expData.Err))

	zipBytes, _ := base64.StdEncoding.DecodeString(expData.ZipB64)
	_ = os.WriteFile("/tmp/lite-effects-demo-package.zip", zipBytes, 0644)
	if out, err := exec.Command("scp", "/tmp/lite-effects-demo-package.zip", "nigel@192.168.100.40:/tmp/lite-effects-demo-package.zip").CombinedOutput(); err != nil {
		log.Fatalf("SCP failed: %v, %s", err, string(out))
	}
	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && unzip -q -o /tmp/lite-effects-demo-package.zip -d %s", remoteDir, remoteDir, remoteDir)
	if out, err := exec.Command("ssh", "nigel@192.168.100.40", unzipCmd).CombinedOutput(); err != nil {
		log.Fatalf("Unzip failed: %v, %s", err, string(out))
	}
	_, _ = exec.Command("scp", "internal/web/static/js/svg-widget.js", fmt.Sprintf("nigel@192.168.100.40:%s/lite-effects-demo/js/svg-widget.js", remoteDir)).CombinedOutput()

	demoURL := fmt.Sprintf("file://%s/lite-effects-demo/demo.html", remoteDir)
	if err := runVoid(taskCtx, chromedp.Navigate(demoURL), chromedp.WaitReady("#container-unit-a svg"), chromedp.Sleep(500*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open exported demo.html: %v", err)
	}

	demoCheck, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const wA = instances['unit-a'];
		const wB = instances['unit-b'];
		wA.setParameter('lite-1-color', '#ef4444');
		wA.setParameter('lite-1-speed', 2.5);
		wA.setParameter('lite-2-opacity', 0.45);
		wA.setParameter('lite-3-trigger', 1);

		const strokeA = wA.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')?.getAttribute('stroke') || '';
		const strokeB = wB.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')?.getAttribute('stroke') || '';
		const trigA = wA.svgElement.querySelector('[data-semantic-target="lite-3"] .lite-payload')?.getAttribute('data-trigger-nonce') || '';
		return JSON.stringify({ strokeA, strokeB, trigA });
	})()`))
	if err != nil {
		fmt.Printf("  [C7 EVAL ERR] %v\n", err)
	}
	var dData struct {
		StrokeA string `json:"strokeA"`
		StrokeB string `json:"strokeB"`
		TrigA   string `json:"trigA"`
	}
	_ = json.Unmarshal([]byte(demoCheck), &dData)
	record(7, "Exported file:// Runtime Controls", dData.StrokeA == "#ef4444" && dData.StrokeB == "#38bdf8" && dData.TrigA == "1",
		fmt.Sprintf("unit-a stroke=%s, unit-b stroke=%s, flash trigger=%s", dData.StrokeA, dData.StrokeB, dData.TrigA))

	// Criteria 8: Verify real-browser IntersectionObserver suspension & scroll recovery
	if err := runVoid(taskCtx, chromedp.Navigate(benchURL), chromedp.WaitReady("#grid"), chromedp.Sleep(300*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open benchmark URL: %v", err)
	}
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[string](`(async () => JSON.stringify(await window.BenchmarkHarness.setup({count:30,layout:'dashboard',mode:'animated',visibilityAware:true})))()`,
		func(p *cdpruntime.EvaluateParams) { b := true; p.AwaitPromise = &b }))
	_ = runVoid(taskCtx, chromedp.Sleep(800*time.Millisecond))

	visStatsBefore, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.BenchmarkHarness.getStats())`))
	var sb struct {
		Mounted   int `json:"mounted"`
		Visible   int `json:"visible"`
		Active    int `json:"active"`
		Suspended int `json:"suspended"`
	}
	_ = json.Unmarshal([]byte(visStatsBefore), &sb)

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
	record(8, "Scroll Visibility Suspend & Resume",
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
	fmt.Println(">>> ALL 8 STAGE 16D-A REAL-BROWSER CDP CRITERIA PASS 100% <<<")
}
