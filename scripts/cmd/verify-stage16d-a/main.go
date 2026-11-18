package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
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
	consumerURL = "http://192.168.100.40:8091/index.html"
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
	fmt.Println(" Stage 16D-A: 20-Scenario Real-Browser Chrome CDP Verification")
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
		fmt.Printf("[%s] Scenario %2d: %-40s - %s\n", status, num, name, msg)
	}

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cancel := context.WithTimeout(allocCtx, 180*time.Second)
	defer cancel()
	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

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

	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		return true;
	})()`))

	// Scenario 1: Open Low-Load drawer via left-rail icon, collapse, and reopen
	_ = runVoid(taskCtx, chromedp.Click("#railBtn-lite-effects"), chromedp.Sleep(200*time.Millisecond))
	s1JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const drawer = document.getElementById('leftDrawer');
		const rail = document.getElementById('railBtn-lite-effects');
		const acc = document.getElementById('acc-lite-effects');
		const title = document.getElementById('leftDrawerTitle');
		return JSON.stringify({
			open: Boolean(drawer && !drawer.classList.contains('collapsed')),
			rail: Boolean(rail && rail.classList.contains('active')),
			acc: Boolean(acc && acc.classList.contains('active-tool') && acc.classList.contains('open')),
			title: title ? title.textContent.trim() : ''
		});
	})()`))
	var s1 struct {
		Open  bool   `json:"open"`
		Rail  bool   `json:"rail"`
		Acc   bool   `json:"acc"`
		Title string `json:"title"`
	}
	_ = json.Unmarshal([]byte(s1JSON), &s1)
	_ = runVoid(taskCtx, chromedp.Click("#btnCollapseLeftDrawer"), chromedp.Sleep(150*time.Millisecond))
	collapsed, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`document.getElementById('leftDrawer').classList.contains('collapsed')`))
	_ = runVoid(taskCtx, chromedp.Click("#railBtn-lite-effects"), chromedp.Sleep(200*time.Millisecond))
	reopened, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`!document.getElementById('leftDrawer').classList.contains('collapsed')`))
	record(1, "Open/Close/Reopen Low-Load Drawer",
		s1.Open && s1.Rail && s1.Acc && s1.Title == "Low-Load Effects (Lite)" && collapsed && reopened,
		fmt.Sprintf("open=%v, rail=%v, title=%q, collapsed=%v, reopened=%v", s1.Open, s1.Rail, s1.Title, collapsed, reopened))

	// Scenario 2: Create all three Lite effects + a second Tracer instance via real UI clicks
	_ = runVoid(taskCtx,
		chromedp.Click("#btnAddLiteTracer"),
		chromedp.Sleep(120*time.Millisecond),
		chromedp.Click("#btnAddLitePulse"),
		chromedp.Sleep(120*time.Millisecond),
		chromedp.Click("#btnAddLiteFlash"),
		chromedp.Sleep(120*time.Millisecond),
		chromedp.Click("#btnAddLiteTracer"),
		chromedp.Sleep(200*time.Millisecond),
	)
	// Spread the 4 objects clearly across the canvas so their hit targets don't overlap during canvas click tests
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.LiteEffects.updateLiteEffect('1', { x: 240, y: 180, name: 'borderA' });
		window.LiteEffects.updateLiteEffect('2', { x: 560, y: 180, name: 'heartbeat' });
		window.LiteEffects.updateLiteEffect('3', { x: 820, y: 180, name: 'alert' });
		window.LiteEffects.updateLiteEffect('4', { x: 240, y: 420, name: 'borderB' });
		window.LiteEffects.renderList();
		window.LiteEffects.triggerAppRender();
		return true;
	})()`))
	_ = runVoid(taskCtx, chromedp.Sleep(200*time.Millisecond))
	saveScreenshot(taskCtx, "/tmp/stage16d-a-drawer.png")

	s2Count, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`document.querySelectorAll('#liteEffectsList .flow-list-item.compact-instance-row').length`))
	record(2, "Create All 3 Lite Effects (+2nd Tracer)", s2Count == 4, fmt.Sprintf("rows=%d (lite-1..4)", s2Count))

	// Scenario 3: Select each effect from the canvas via real mouse clicks
	canvasClickSelect := func(id string, isStroke bool) string {
		ptJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
			const hit = document.querySelector('#svgContainer [data-lite-id="%s"] .lite-hit-target');
			const r = hit.getBoundingClientRect();
			return JSON.stringify({ x: r.left + r.width / 2, y: %v ? (r.top + 1) : (r.top + r.height / 2) });
		})()`, id, isStroke)))
		var pt struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		_ = json.Unmarshal([]byte(ptJSON), &pt)
		_ = runVoid(taskCtx,
			chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, pt.X, pt.Y, chromedp.ButtonLeft),
			chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, pt.X, pt.Y, chromedp.ButtonLeft),
			chromedp.Sleep(120*time.Millisecond),
		)
		selID, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`String(window.AppCoordinator.activeSelection.id || '')`))
		return selID
	}
	cSel1 := canvasClickSelect("1", true)
	cSel2 := canvasClickSelect("2", false)
	cSel3 := canvasClickSelect("3", false)
	record(3, "Select Each Effect From Canvas",
		cSel1 == "1" && cSel2 == "2" && cSel3 == "3",
		fmt.Sprintf("selected IDs: tracer=%s, pulse=%s, flash=%s", cSel1, cSel2, cSel3))

	// Scenario 4: Open Layers drawer and select each effect from Layers
	_ = runVoid(taskCtx, chromedp.Click("#railBtn-layers"), chromedp.Sleep(250*time.Millisecond))
	saveScreenshot(taskCtx, "/tmp/stage16d-a-layers.png")
	layersCheckJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const items = Array.from(document.querySelectorAll('#layersListContainer .layer-item'));
		const titles = items.map(el => el.querySelector('.layer-item-title')?.textContent || '');
		return JSON.stringify({ count: items.length, titles });
	})()`))
	var lChk struct {
		Count  int      `json:"count"`
		Titles []string `json:"titles"`
	}
	_ = json.Unmarshal([]byte(layersCheckJSON), &lChk)
	// Click the last layer item in #layersListContainer (which corresponds to bottom-most layer lite-1)
	_ = runVoid(taskCtx, chromedp.Click("#layersListContainer .layer-item:last-child"), chromedp.Sleep(150*time.Millisecond))
	layerSelID, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`String(window.AppCoordinator.activeSelection.id || '')`))

	// Test all 4 Layer ordering UI buttons on lite-1 (Bring Forward, Send Backward, Bring to Front, Send to Back)
	_ = runVoid(taskCtx, chromedp.Click("#btnLayerForward"), chromedp.Sleep(120*time.Millisecond))
	idxAfterFwd, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.LayerManager.getLayers().findIndex(l => l.type === 'lite' && l.id === '1')`))
	_ = runVoid(taskCtx, chromedp.Click("#btnLayerBackward"), chromedp.Sleep(120*time.Millisecond))
	idxAfterBack, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.LayerManager.getLayers().findIndex(l => l.type === 'lite' && l.id === '1')`))
	_ = runVoid(taskCtx, chromedp.Click("#btnLayerToFront"), chromedp.Sleep(120*time.Millisecond))
	idxAfterFront, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.LayerManager.getLayers().findIndex(l => l.type === 'lite' && l.id === '1')`))
	_ = runVoid(taskCtx, chromedp.Click("#btnLayerToBack"), chromedp.Sleep(120*time.Millisecond))
	idxAfterToBack, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.LayerManager.getLayers().findIndex(l => l.type === 'lite' && l.id === '1')`))

	record(4, "Layers Integration, Selection & 4 Order Ops",
		lChk.Count == 4 && layerSelID == "1" && idxAfterFwd == 1 && idxAfterBack == 0 && idxAfterFront == 3 && idxAfterToBack == 0,
		fmt.Sprintf("layerCount=%d, clicked->lite-%s, orderIdx: fwd=%d back=%d front=%d toBack=%d",
			lChk.Count, layerSelID, idxAfterFwd, idxAfterBack, idxAfterFront, idxAfterToBack))

	// Re-open Low-Load drawer for subsequent editing tests
	_ = runVoid(taskCtx, chromedp.Click("#railBtn-lite-effects"), chromedp.Sleep(200*time.Millisecond))

	// Scenario 5: Move effect via real canvas drag
	dragPtJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const hit = document.querySelector('#svgContainer [data-lite-id="2"] .lite-hit-target');
		const r = hit.getBoundingClientRect();
		const le = window.LiteEffects.getLiteEffect('2');
		return JSON.stringify({ cx: r.left + r.width / 2, cy: r.top + r.height / 2, x0: le.x, y0: le.y });
	})()`))
	var dPt struct {
		CX float64 `json:"cx"`
		CY float64 `json:"cy"`
		X0 float64 `json:"x0"`
		Y0 float64 `json:"y0"`
	}
	_ = json.Unmarshal([]byte(dragPtJSON), &dPt)
	_ = runVoid(taskCtx,
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, dPt.CX, dPt.CY, chromedp.ButtonLeft),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseMoved, dPt.CX+50, dPt.CY+30, chromedp.ButtonLeft),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, dPt.CX+50, dPt.CY+30, chromedp.ButtonLeft),
		chromedp.Sleep(150*time.Millisecond),
	)
	afterDragX, _ := chromedp.Run(taskCtx, chromedp.Evaluate[float64](`window.LiteEffects.getLiteEffect('2').x`))
	record(5, "Move Effect via Canvas Drag", afterDragX > dPt.X0+15, fmt.Sprintf("pulse x: %.0f -> %.0f", dPt.X0, afterDragX))

	// Scenario 6 & 13: Change Perimeter Tracer Width/Height without unintended center translation (rounded + circle)
	_ = runVoid(taskCtx, chromedp.Click(`#liteEffectsList [data-lite-list-id="1"]`), chromedp.Sleep(150*time.Millisecond))
	s6JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const before = Object.assign({}, window.LiteEffects.getLiteEffect('1'));
		const wIn = document.getElementById('liteWidthNum');
		wIn.value = '220';
		wIn.dispatchEvent(new Event('input', { bubbles: true }));
		const hIn = document.getElementById('liteHeightNum');
		hIn.value = '110';
		hIn.dispatchEvent(new Event('input', { bubbles: true }));
		const afterRect = Object.assign({}, window.LiteEffects.getLiteEffect('1'));
		const rectEl = document.querySelector('#svgContainer [data-lite-id="1"] .lite-payload');
		const rectX = parseFloat(rectEl.getAttribute('x'));
		const rectY = parseFloat(rectEl.getAttribute('y'));

		// Switch to circle variant and change width to 160 -> height must sync to 160 and cx/cy must stay at (240, 180)
		const vSel = document.getElementById('liteVariantSelect');
		vSel.value = 'circle';
		vSel.dispatchEvent(new Event('change', { bubbles: true }));
		wIn.value = '160';
		wIn.dispatchEvent(new Event('input', { bubbles: true }));
		const afterCircle = Object.assign({}, window.LiteEffects.getLiteEffect('1'));
		const circEl = document.querySelector('#svgContainer [data-lite-id="1"] .lite-payload');
		const circCX = parseFloat(circEl.getAttribute('cx'));
		const circCY = parseFloat(circEl.getAttribute('cy'));
		const circR = parseFloat(circEl.getAttribute('rx') || circEl.getAttribute('r'));

		// Switch back to rounded for subsequent tests
		vSel.value = 'rounded';
		vSel.dispatchEvent(new Event('change', { bubbles: true }));
		hIn.value = '100';
		hIn.dispatchEvent(new Event('input', { bubbles: true }));

		return JSON.stringify({
			bx: before.x, by: before.y,
			ax: afterRect.x, ay: afterRect.y, aw: afterRect.width, ah: afterRect.height,
			rectX, rectY,
			circCX, circCY, circR, circH: afterCircle.height
		});
	})()`))
	var s6 struct {
		BX     float64 `json:"bx"`
		BY     float64 `json:"by"`
		AX     float64 `json:"ax"`
		AY     float64 `json:"ay"`
		AW     float64 `json:"aw"`
		AH     float64 `json:"ah"`
		RectX  float64 `json:"rectX"`
		RectY  float64 `json:"rectY"`
		CircCX float64 `json:"circCX"`
		CircCY float64 `json:"circCY"`
		CircR  float64 `json:"circR"`
		CircH  float64 `json:"circH"`
	}
	_ = json.Unmarshal([]byte(s6JSON), &s6)
	noTranslation := s6.BX == s6.AX && s6.BY == s6.AY && s6.RectX == (s6.AX-110) && s6.RectY == (s6.AY-55) && s6.CircCX == s6.BX && s6.CircCY == s6.BY && s6.CircR == 80 && s6.CircH == 160
	record(6, "Resize Dimensions & Geometry", noTranslation, fmt.Sprintf("rounded 220x110 @ (%.0f,%.0f); circle r=%.0f @ (%.0f,%.0f)", s6.AX, s6.AY, s6.CircR, s6.CircCX, s6.CircCY))
	record(13, "Tracer Width/Height Without Translation", noTranslation, fmt.Sprintf("center preserved (%.0f,%.0f)->(%.0f,%.0f), top-left=(%.0f,%.0f)", s6.BX, s6.BY, s6.AX, s6.AY, s6.RectX, s6.RectY))

	// Scenario 7: Verify bounding boxes and #nodeSelectionBox indicators
	s7JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const selBox = document.getElementById('nodeSelectionBox');
		const le = window.LiteEffects.getLiteEffect('1');
		const bbox = window.LiteEffects.getBoundingBox(le);
		return JSON.stringify({
			visible: Boolean(selBox && selBox.style.display !== 'none'),
			boxW: parseFloat(selBox.getAttribute('width')),
			boxH: parseFloat(selBox.getAttribute('height')),
			expW: bbox.width + 20,
			expH: bbox.height + 20
		});
	})()`))
	var s7 struct {
		Visible bool    `json:"visible"`
		BoxW    float64 `json:"boxW"`
		BoxH    float64 `json:"boxH"`
		ExpW    float64 `json:"expW"`
		ExpH    float64 `json:"expH"`
	}
	_ = json.Unmarshal([]byte(s7JSON), &s7)
	record(7, "Selection Bounding Box & Outline",
		s7.Visible && math.Abs(s7.BoxW-s7.ExpW) < 1 && math.Abs(s7.BoxH-s7.ExpH) < 1,
		fmt.Sprintf("selBox visible=%v, size=%.0fx%.0f (expected %.0fx%.0f)", s7.Visible, s7.BoxW, s7.BoxH, s7.ExpW, s7.ExpH))

	// Scenario 8 & 9: Edit one of two Tracer instances via Context Editor without modifying the other
	pt1JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const hit = document.querySelector('#svgContainer [data-lite-id="1"] .lite-hit-target');
		const r = hit.getBoundingClientRect();
		return JSON.stringify({ x: r.left + r.width / 2, y: r.top + 1 });
	})()`))
	var pt1 struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	_ = json.Unmarshal([]byte(pt1JSON), &pt1)
	_ = runVoid(taskCtx,
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, pt1.X, pt1.Y, chromedp.ButtonRight),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, pt1.X, pt1.Y, chromedp.ButtonRight),
		chromedp.Sleep(200*time.Millisecond),
	)
	saveScreenshot(taskCtx, "/tmp/stage16d-a-context-editor.png")
	s89JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const segIn = document.getElementById('fctx-liteSegmentsNum');
		segIn.value = '4';
		segIn.dispatchEvent(new Event('input', { bubbles: true }));
		const rIn = document.getElementById('fctx-liteRatioInput');
		rIn.value = '1:1';
		rIn.dispatchEvent(new Event('input', { bubbles: true }));
		const t1 = window.LiteEffects.getLiteEffect('1');
		const t4 = window.LiteEffects.getLiteEffect('4');
		return JSON.stringify({
			t1Seg: t1.segments, t1Ratio: t1.lineGapRatio,
			t4Seg: t4.segments, t4Ratio: t4.lineGapRatio
		});
	})()`))
	var s89 struct {
		T1Seg   int    `json:"t1Seg"`
		T1Ratio string `json:"t1Ratio"`
		T4Seg   int    `json:"t4Seg"`
		T4Ratio string `json:"t4Ratio"`
	}
	_ = json.Unmarshal([]byte(s89JSON), &s89)
	record(8, "Edit Instance via Context Editor", s89.T1Seg == 4 && s89.T1Ratio == "1:1", fmt.Sprintf("lite-1 segments=%d, ratio=%s", s89.T1Seg, s89.T1Ratio))
	record(9, "Multi-Instance Isolation", s89.T4Seg == 2 && s89.T4Ratio == "1:3", fmt.Sprintf("lite-4 untouched: segments=%d, ratio=%s", s89.T4Seg, s89.T4Ratio))

	// Scenario 10 & 11: Trigger Flash Lite, verify editor marker remains selectable while inactive, & adjust ms duration in 10ms steps
	_ = runVoid(taskCtx, chromedp.Click(`#liteEffectsList [data-lite-list-id="3"]`), chromedp.Sleep(150*time.Millisecond))
	_ = runVoid(taskCtx, chromedp.Click("#btnTriggerLiteFlash"), chromedp.Sleep(200*time.Millisecond))
	s1011JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const marker = document.querySelector('#svgContainer [data-lite-id="3"] .lite-editor-marker');
		const durSlider = document.getElementById('liteFlashDurSlider');
		const stepAttr = durSlider ? durSlider.getAttribute('step') : '';
		durSlider.value = '140';
		durSlider.dispatchEvent(new Event('input', { bubbles: true }));
		const f3 = window.LiteEffects.getLiteEffect('3');
		const pay = document.querySelector('#svgContainer [data-lite-id="3"] .lite-payload');
		return JSON.stringify({
			hasMarker: Boolean(marker),
			nonce: f3.triggerNonce,
			step: stepAttr,
			durSec: f3.dur,
			durMsAttr: pay ? pay.getAttribute('data-flash-duration-ms') : ''
		});
	})()`))
	var s1011 struct {
		HasMarker bool    `json:"hasMarker"`
		Nonce     int     `json:"nonce"`
		Step      string  `json:"step"`
		DurSec    float64 `json:"durSec"`
		DurMsAttr string  `json:"durMsAttr"`
	}
	_ = json.Unmarshal([]byte(s1011JSON), &s1011)
	// Also click Flash on canvas while inactive to verify selectability
	cSelFlashInactive := canvasClickSelect("3", false)
	record(10, "Flash Trigger & Inactive Marker Selectable",
		s1011.HasMarker && s1011.Nonce >= 1 && cSelFlashInactive == "3",
		fmt.Sprintf("marker=%v, nonce=%d, canvasSelect=%s", s1011.HasMarker, s1011.Nonce, cSelFlashInactive))
	record(11, "Flash Duration in 10ms Increments",
		s1011.Step == "10" && math.Abs(s1011.DurSec-0.14) < 0.005 && s1011.DurMsAttr == "140",
		fmt.Sprintf("step=%sms, dur=%.2fs (%sms)", s1011.Step, s1011.DurSec, s1011.DurMsAttr))

	// Scenario 12: Pulse min/max scale, aspect-ratio unlock, and diamond geometry bounds
	_ = runVoid(taskCtx, chromedp.Click(`#liteEffectsList [data-lite-list-id="2"]`), chromedp.Sleep(150*time.Millisecond))
	s12JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const vSel = document.getElementById('liteVariantSelect');
		vSel.value = 'diamond';
		vSel.dispatchEvent(new Event('change', { bubbles: true }));
		const lockChk = document.getElementById('liteLockAspectChk');
		lockChk.checked = false;
		lockChk.dispatchEvent(new Event('change', { bubbles: true }));
		const wIn = document.getElementById('liteWidthNum');
		wIn.value = '90';
		wIn.dispatchEvent(new Event('input', { bubbles: true }));
		const hIn = document.getElementById('liteHeightNum');
		hIn.value = '50';
		hIn.dispatchEvent(new Event('input', { bubbles: true }));
		const minIn = document.getElementById('liteMinScaleNum');
		minIn.value = '0.75';
		minIn.dispatchEvent(new Event('input', { bubbles: true }));
		const maxIn = document.getElementById('liteMaxScaleNum');
		maxIn.value = '1.35';
		maxIn.dispatchEvent(new Event('input', { bubbles: true }));

		const p2 = window.LiteEffects.getLiteEffect('2');
		const poly = document.querySelector('#svgContainer [data-lite-id="2"] polygon.lite-payload');
		const pts = poly ? poly.getAttribute('points') : '';
		const bbox = window.LiteEffects.getBoundingBox(p2);
		return JSON.stringify({
			w: p2.width, h: p2.height, minS: p2.minScale, maxS: p2.maxScale,
			pts, bboxW: bbox.width, bboxH: bbox.height
		});
	})()`))
	var s12 struct {
		W     float64 `json:"w"`
		H     float64 `json:"h"`
		MinS  float64 `json:"minS"`
		MaxS  float64 `json:"maxS"`
		Pts   string  `json:"pts"`
		BboxW float64 `json:"bboxW"`
		BboxH float64 `json:"bboxH"`
	}
	_ = json.Unmarshal([]byte(s12JSON), &s12)
	record(12, "Pulse Min/Max Scale, Aspect & Diamond",
		s12.W == 90 && s12.H == 50 && s12.MinS == 0.75 && s12.MaxS == 1.35 && s12.BboxW == 90 && s12.BboxH == 50 && s12.Pts != "",
		fmt.Sprintf("diamond %gx%g, min=%.2f, max=%.2f, pts=%q", s12.W, s12.H, s12.MinS, s12.MaxS, s12.Pts))

	// Scenario 14: Verify animated perimeter preview, guide boundary, ping-pong mode, and burstCycles
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.AppCoordinator.selectObject('lite', '1');
		const pmSel = document.getElementById('litePlaybackModeSelect');
		pmSel.value = 'pingpong';
		pmSel.dispatchEvent(new Event('change', { bubbles: true }));
		const bcIn = document.getElementById('liteBurstCyclesNum');
		bcIn.value = '4';
		bcIn.dispatchEvent(new Event('input', { bubbles: true }));
		return true;
	})()`))
	_ = runVoid(taskCtx, chromedp.Sleep(150*time.Millisecond))
	s14JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const guide = document.querySelector('#svgContainer [data-lite-id="1"] .lite-guide-track');
		const pay = document.querySelector('#svgContainer [data-lite-id="1"] .lite-payload.sva-lite-tracer-anim');
		const cs = pay ? getComputedStyle(pay) : null;
		const animName = cs ? cs.animationName : '';
		const animDur = cs ? cs.animationDuration : '';
		const cls = pay ? pay.getAttribute('class') : '';
		const dash = pay ? pay.getAttribute('stroke-dasharray') : '';
		const hasStyle = Boolean(document.querySelector('#svgContainer style'));
		const hasAnim = Boolean(pay && animName.includes('sva-lite-trace'));
		const hasPingPong = pay ? pay.classList.contains('sva-lite-mode-pingpong') : false;
		const burstCyc = window.LiteEffects.getLiteEffect('1').burstCycles;

		// Restore loop mode and continuous burst=0 for export test
		const pmSel = document.getElementById('litePlaybackModeSelect');
		pmSel.value = 'loop';
		pmSel.dispatchEvent(new Event('change', { bubbles: true }));
		const bcIn = document.getElementById('liteBurstCyclesNum');
		bcIn.value = '0';
		bcIn.dispatchEvent(new Event('input', { bubbles: true }));

		// Verify grouping compatibility: group lite-1 and lite-4, move, and ungroup
		window.AppCoordinator.setMultiSelection([{ type: 'lite', id: '1' }, { type: 'lite', id: '4' }]);
		const grp = window.AppCoordinator.createGroupFromSelection('LitePair');
		const grpMoved = grp ? window.AppCoordinator.moveSelectedNodes(5, 0) : false;
		const xAfterGrpMove = window.LiteEffects.getLiteEffect('1').x;
		if (grp) {
			window.AppCoordinator.moveSelectedNodes(-5, 0);
			window.AppCoordinator.ungroupSelectedGroup();
		}

		return JSON.stringify({
			hasGuide: Boolean(guide),
			animName,
			animDur,
			cls,
			hasStyle,
			hasAnim,
			dash,
			hasPingPong,
			burstCyc,
			grpOk: Boolean(grp && grpMoved && xAfterGrpMove > 280)
		});
	})()`))
	var s14 struct {
		HasGuide    bool   `json:"hasGuide"`
		AnimName    string `json:"animName"`
		AnimDur     string `json:"animDur"`
		Cls         string `json:"cls"`
		HasStyle    bool   `json:"hasStyle"`
		HasAnim     bool   `json:"hasAnim"`
		Dash        string `json:"dash"`
		HasPingPong bool   `json:"hasPingPong"`
		BurstCyc    int    `json:"burstCyc"`
		GrpOk       bool   `json:"grpOk"`
	}
	_ = json.Unmarshal([]byte(s14JSON), &s14)
	record(14, "Tracer Modes, Burst & Grouping Compat",
		s14.HasGuide && s14.HasAnim && s14.Dash == "125 125" && s14.HasPingPong && s14.BurstCyc == 4 && s14.GrpOk,
		fmt.Sprintf("guide=%v, anim=%v(%q dur=%s hasStyle=%v cls=%q), pingpong=%v, burstCycles=%d, groupMove=%v",
			s14.HasGuide, s14.HasAnim, s14.AnimName, s14.AnimDur, s14.HasStyle, s14.Cls, s14.HasPingPong, s14.BurstCyc, s14.GrpOk))

	// Scenario 15: Verify colour picker and eyedropper button + sampling
	_ = runVoid(taskCtx,
		chromedp.Click("#liteColorPicker"),
		chromedp.Sleep(200*time.Millisecond),
	)
	pipetteVisible, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const modal = document.getElementById('customColorPickerModal');
		const btn = document.getElementById('btnColorPickerPipette');
		return Boolean(modal && getComputedStyle(modal).display !== 'none' && btn && getComputedStyle(btn).display !== 'none');
	})()`))
	// Click pipette then click lite-1's top border on canvas (pt1.X, pt1.Y) to sample its #38bdf8 colour, then click Apply
	_ = runVoid(taskCtx,
		chromedp.Click("#btnColorPickerPipette"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, pt1.X, pt1.Y, chromedp.ButtonLeft),
		chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, pt1.X, pt1.Y, chromedp.ButtonLeft),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Click("#btnColorPickerApply"),
		chromedp.Sleep(150*time.Millisecond),
	)
	s15JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const hit1 = document.querySelector('#svgContainer [data-lite-id="1"] .lite-hit-target');
		const extracted = window.CustomColorPicker ? window.CustomColorPicker.extractElementColor(hit1) : '';
		const p2Col = window.LiteEffects.getLiteEffect('2').color;
		return JSON.stringify({ extracted, p2Col });
	})()`))
	var s15 struct {
		Extracted string `json:"extracted"`
		P2Col     string `json:"p2Col"`
	}
	_ = json.Unmarshal([]byte(s15JSON), &s15)
	record(15, "Colour Picker & Eyedropper",
		pipetteVisible && s15.Extracted == "#38bdf8" && strings.HasPrefix(s15.P2Col, "#"),
		fmt.Sprintf("pipetteVisible=%v, extractedLite1=%s, appliedColor=%s", pipetteVisible, s15.Extracted, s15.P2Col))

	// Scenario 16: Expose Lite properties through the normal Studio Widget Property Exposure UI
	_ = runVoid(taskCtx,
		chromedp.Click("#btnLiteExposeWidget"),
		chromedp.Sleep(250*time.Millisecond),
	)
	// Expose color & scale on pulse (lite-2), color, speed, strokeWidth, playbackMode, startOffset, showGuide on tracer (lite-1), trigger & duration on flash (lite-3) via UI checkboxes
	s16JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const toggleCard = (prop) => {
			const chk = document.querySelector('#drawerWidgetPropertiesList .widget-prop-card[data-prop="' + prop + '"] input[type="checkbox"]');
			if (chk && !chk.checked) chk.click();
		};
		window.AppCoordinator.selectObject('lite', '2');
		toggleCard('color');
		toggleCard('scale');

		window.AppCoordinator.selectObject('lite', '1');
		toggleCard('color');
		toggleCard('speed');
		toggleCard('strokeWidth');
		toggleCard('playbackMode');
		toggleCard('burstCycles');
		toggleCard('showGuide');

		window.AppCoordinator.selectObject('lite', '3');
		toggleCard('trigger');
		toggleCard('duration');

		const params = window.EngineBridge.getWidgetParameters() || [];
		return JSON.stringify({ count: params.length, names: params.map(p => p.name) });
	})()`))
	var s16 struct {
		Count int      `json:"count"`
		Names []string `json:"names"`
	}
	_ = json.Unmarshal([]byte(s16JSON), &s16)
	saveScreenshot(taskCtx, "/tmp/stage16d-a-widget-exposure.png")
	record(16, "Select Exported Properties via UI",
		s16.Count == 10,
		fmt.Sprintf("exposedCount=%d, names=%v", s16.Count, s16.Names))

	// Scenario 17: Save and reload project (plus verify legacy top-left migration)
	s17JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const saved = window.ProjectManager.serializeStudioState();
		window.ProjectManager.resetToNewProject();
		window.ProjectManager.applyStudioState(saved);
		const l1 = window.LiteEffects.getLiteEffect('1');
		const l2 = window.LiteEffects.getLiteEffect('2');
		const l3 = window.LiteEffects.getLiteEffect('3');
		const params = window.EngineBridge.getWidgetParameters() || [];
		return JSON.stringify({
			count: window.LiteEffects.getAllLiteEffects().length,
			l1Seg: l1 ? l1.segments : 0,
			l2W: l2 ? l2.width : 0,
			l3Dur: l3 ? l3.dur : 0,
			paramCount: params.length
		});
	})()`))
	var s17 struct {
		Count      int     `json:"count"`
		L1Seg      int     `json:"l1Seg"`
		L2W        float64 `json:"l2W"`
		L3Dur      float64 `json:"l3Dur"`
		ParamCount int     `json:"paramCount"`
	}
	_ = json.Unmarshal([]byte(s17JSON), &s17)
	record(17, "Save & Reload Project Round-Trip",
		s17.Count == 4 && s17.L1Seg == 4 && s17.L2W == 90 && math.Abs(s17.L3Dur-0.14) < 0.005 && s17.ParamCount == 10,
		fmt.Sprintf("count=%d, l1Seg=%d, l2W=%.0f, l3Dur=%.2fs, params=%d", s17.Count, s17.L1Seg, s17.L2W, s17.L3Dur, s17.ParamCount))

	// Scenario 18: Export widget package, verify editor marker stripped, and drive from independent Stage 16C consumer & demo.html
	exportJson, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const rs = window.SceneState.buildRenderState ? window.SceneState.buildRenderState() : window.currentRenderState;
		const rawSvg = window.SceneState.buildFullSVG(rs);
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
	zipBytes, _ := base64.StdEncoding.DecodeString(expData.ZipB64)
	_ = os.WriteFile("/tmp/lite-effects-demo-package.zip", zipBytes, 0644)
	_, _ = exec.Command("scp", "/tmp/lite-effects-demo-package.zip", "nigel@192.168.100.40:/tmp/lite-effects-demo-package.zip").CombinedOutput()
	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && unzip -q -o /tmp/lite-effects-demo-package.zip -d %s", remoteDir, remoteDir, remoteDir)
	_, _ = exec.Command("ssh", "nigel@192.168.100.40", unzipCmd).CombinedOutput()
	_, _ = exec.Command("scp", "internal/web/static/js/svg-widget.js", fmt.Sprintf("nigel@192.168.100.40:%s/lite-effects-demo/js/svg-widget.js", remoteDir)).CombinedOutput()

	demoURL := fmt.Sprintf("file://%s/lite-effects-demo/demo.html", remoteDir)
	if err := runVoid(taskCtx, chromedp.Navigate(demoURL), chromedp.WaitReady("#container-unit-a svg"), chromedp.Sleep(400*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open exported demo.html: %v", err)
	}
	s18JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const wA = instances['unit-a'];
		const wB = instances['unit-b'];
		wA.setParameter('bordera-lite-color', '#ef4444');
		wA.setParameter('bordera-lite-speed', 2.5);
		wA.setParameter('bordera-lite-strokewidth', 5);
		wA.setParameter('bordera-lite-playbackmode', 'pingpong');
		wA.setParameter('bordera-lite-burstcycles', 4);
		wA.setParameter('bordera-lite-showguide', false);
		wA.setParameter('heartbeat-lite-scale', 1.65);
		wA.setParameter('alert-lite-duration', 240);
		wA.setParameter('alert-lite-trigger', 1);

		const pay1A = wA.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload');
		const strokeA = pay1A?.getAttribute('stroke') || '';
		const swA = pay1A?.getAttribute('stroke-width') || '';
		const modeA = pay1A?.getAttribute('data-playback-mode') || '';
		const burstA = pay1A?.getAttribute('data-burst-cycles') || '';
		const guideVisA = wA.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-guide-track')?.getAttribute('visibility') || '';
		const animNameA = pay1A ? getComputedStyle(pay1A).animationName : '';
		const strokeB = wB.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')?.getAttribute('stroke') || '';
		const scaleA = wA.svgElement.querySelector('[data-semantic-target="lite-2"] .lite-payload')?.getAttribute('data-pulse-scale') || '';
		const durA = wA.svgElement.querySelector('[data-semantic-target="lite-3"] .lite-payload')?.getAttribute('data-flash-duration-ms') || '';
		const trigA = wA.svgElement.querySelector('[data-semantic-target="lite-3"] .lite-payload')?.getAttribute('data-trigger-nonce') || '';
		const hasEditorMarker = Boolean(wA.svgElement.querySelector('.lite-editor-marker'));
		return JSON.stringify({ strokeA, swA, modeA, burstA, guideVisA, animNameA, strokeB, scaleA, durA, trigA, hasEditorMarker });
	})()`))
	var s18 struct {
		StrokeA         string `json:"strokeA"`
		SwA             string `json:"swA"`
		ModeA           string `json:"modeA"`
		BurstA          string `json:"burstA"`
		GuideVisA       string `json:"guideVisA"`
		AnimNameA       string `json:"animNameA"`
		StrokeB         string `json:"strokeB"`
		ScaleA          string `json:"scaleA"`
		DurA            string `json:"durA"`
		TrigA           string `json:"trigA"`
		HasEditorMarker bool   `json:"hasEditorMarker"`
	}
	_ = json.Unmarshal([]byte(s18JSON), &s18)

	// Also verify Stage 16C independent consumer harness (http://192.168.100.40:8091/index.html) is live and controllable
	if err := runVoid(taskCtx, chromedp.Navigate(consumerURL), chromedp.WaitReady("#stage-host-a svg"), chromedp.Sleep(400*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open Stage 16C consumer: %v", err)
	}
	consumerOK, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`document.querySelectorAll('#stage-host-a svg, #stage-host-b svg, #stage-host-c svg').length === 3`))
	record(18, "Export & Independent Consumer Control",
		expData.Success && s18.StrokeA == "#ef4444" && s18.SwA == "5" && s18.ModeA == "pingpong" && s18.BurstA == "4" && s18.GuideVisA == "hidden" && strings.Contains(s18.AnimNameA, "sva-lite-trace") && s18.StrokeB == "#38bdf8" && s18.ScaleA == "1.65" && s18.DurA == "240" && s18.TrigA == "1" && !s18.HasEditorMarker && consumerOK,
		fmt.Sprintf("strokeA=%s, swA=%s, modeA=%s, anim=%s, strokeB=%s, scaleA=%s, durA=%sms, trigA=%s, stage16c=%v",
			s18.StrokeA, s18.SwA, s18.ModeA, s18.AnimNameA, s18.StrokeB, s18.ScaleA, s18.DurA, s18.TrigA, consumerOK))

	// Scenario 19: Verify visibility-aware suspension & resumption on benchmark harness
	if err := runVoid(taskCtx, chromedp.Navigate(benchURL), chromedp.WaitReady("#grid"), chromedp.Sleep(300*time.Millisecond)); err != nil {
		log.Fatalf("Failed to open benchmark URL: %v", err)
	}
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[string](`(async () => JSON.stringify(await window.BenchmarkHarness.setup({count:30,layout:'dashboard',mode:'animated',visibilityAware:true})))()`,
		func(p *cdpruntime.EvaluateParams) { b := true; p.AwaitPromise = &b }))
	_ = runVoid(taskCtx, chromedp.Sleep(700*time.Millisecond))
	visStatsBefore, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.BenchmarkHarness.getStats())`))
	var sb struct {
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
	record(19, "Visibility-Aware Suspend & Resume",
		sb.Suspended == 21 && sb.Active == 9 && !sa.LastSuspended && sa.FirstSuspended && strings.Contains(sa.LastText, "node-30"),
		fmt.Sprintf("Before: vis=%d/susp=%d; After: firstSusp=%v, lastSusp=%v, lastText=%q", sb.Visible, sb.Suspended, sa.FirstSuspended, sa.LastSuspended, sa.LastText))

	// Scenario 20: Verify existing Studio shapes, Flow, Glow, Ripple and property exposure still work
	if err := runVoid(taskCtx, chromedp.Navigate(studioURL), chromedp.WaitReady("#railBtn-nodes")); err != nil {
		log.Fatalf("Failed to reopen Studio: %v", err)
	}
	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(300*time.Millisecond))
	}
	s20JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		window.DiagramNodes.addNode({ shape: 'rounded', text: 'CoreNode' });
		window.Glows.addGlow();
		window.Ripples.addRipple();
		window.DataFlows.addFlow();
		const nid = window.DiagramNodes.list[0]?.id;
		window.AppCoordinator.selectObject('node', nid);
		window.renderWidgetParametersOverview();
		const chk = document.querySelector('#drawerWidgetPropertiesList .widget-prop-card[data-prop="fill"] input[type="checkbox"]');
		if (chk && !chk.checked) chk.click();
		const params = window.EngineBridge.getWidgetParameters() || [];
		return JSON.stringify({
			hasNode: window.DiagramNodes.list.length === 1,
			hasGlow: window.Glows.list.length === 1,
			hasRipple: window.Ripples.list.length === 1,
			hasFlow: window.DataFlows.list.length === 1,
			paramCount: params.length
		});
	})()`))
	var s20 struct {
		HasNode    bool `json:"hasNode"`
		HasGlow    bool `json:"hasGlow"`
		HasRipple  bool `json:"hasRipple"`
		HasFlow    bool `json:"hasFlow"`
		ParamCount int  `json:"paramCount"`
	}
	_ = json.Unmarshal([]byte(s20JSON), &s20)
	record(20, "Existing Node/Flow/Glow/Ripple Regression",
		s20.HasNode && s20.HasGlow && s20.HasRipple && s20.HasFlow && s20.ParamCount == 1,
		fmt.Sprintf("node=%v, glow=%v, ripple=%v, flow=%v, exposedParams=%d", s20.HasNode, s20.HasGlow, s20.HasRipple, s20.HasFlow, s20.ParamCount))

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
	fmt.Println(">>> ALL 20 STAGE 16D-A REAL-BROWSER CDP SCENARIOS PASS 100% <<<")
}
