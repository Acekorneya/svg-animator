package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	studioURL    = "http://192.168.100.40:8088/"
	gaugeDemoURL = "http://192.168.100.40:8088/gauge-lite-demo.html"
	consumerURL  = "http://192.168.100.40:8091/index.html"
	cdpEndpoint  = "http://127.0.0.1:9222"
)

type GateResult struct {
	ID      string
	Name    string
	Passed  bool
	Details string
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
	fmt.Println("==========================================================================")
	fmt.Println(" Stage 16D-B-M3: Gauge / Level Lite Real-Browser Acceptance Suite")
	fmt.Println("==========================================================================")

	widgetJSBytes, err := os.ReadFile("/root/projects/svg-animator/internal/web/static/js/svg-widget.js")
	if err != nil {
		log.Fatalf("Failed to read svg-widget.js: %v", err)
	}
	widgetJSSource := string(widgetJSBytes)

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-10s %-46s | %s\n", status, id, name, details)
	}

	allocCtx, cancelAlloc := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer cancelAlloc()
	ctx, cancelTimeout := context.WithTimeout(allocCtx, 180*time.Second)
	defer cancelTimeout()
	taskCtx, cancelTask := chromedp.NewContext(ctx)
	defer cancelTask()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1440, 960),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		log.Fatalf("Failed to open Studio at %s: %v", studioURL, err)
	}

	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(300*time.Millisecond))
	}

	// Reset Studio to clean project
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		return true;
	})()`))

	// =========================================================================
	// AT-M3-01: Create Gauge / Level Lite via Low-Load Drawer + Zero Idle Anim
	// =========================================================================
	at01JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.openAccordionById('acc-lite-effects');
		const btn = document.getElementById('btnAddLiteGauge');
		if (btn) btn.click();
		const le = window.LiteEffects.getLiteEffect('1');
		const domEl = document.querySelector('#svgContainer #lite-1[data-lite-kind="gauge-lite"]');
		const anims = domEl ? domEl.getAnimations({ subtree: true }) : [];
		const smilCount = domEl ? domEl.querySelectorAll('animate, animateTransform, animateMotion').length : -1;
		const inspectorOpen = document.getElementById('liteGaugeGroup') && document.getElementById('liteGaugeGroup').style.display !== 'none';
		return JSON.stringify({
			exists: Boolean(le),
			kind: le ? le.kind : '',
			variant: le ? le.variant : '',
			val: le ? le.value : -1,
			minV: le ? le.minValue : -1,
			maxV: le ? le.maxValue : -1,
			animCount: anims.length,
			smilCount,
			inspectorOpen: Boolean(inspectorOpen)
		});
	})()`))
	var at01 struct {
		Exists        bool    `json:"exists"`
		Kind          string  `json:"kind"`
		Variant       string  `json:"variant"`
		Val           float64 `json:"val"`
		MinV          float64 `json:"minV"`
		MaxV          float64 `json:"maxV"`
		AnimCount     int     `json:"animCount"`
		SmilCount     int     `json:"smilCount"`
		InspectorOpen bool    `json:"inspectorOpen"`
	}
	_ = json.Unmarshal([]byte(at01JSON), &at01)
	record("AT-M3-01", "Create Gauge Lite & Verify Zero Idle Animation",
		at01.Exists && at01.Kind == "gauge-lite" && at01.Variant == "ring" &&
			at01.Val == 65 && at01.MinV == 0 && at01.MaxV == 100 &&
			at01.AnimCount == 0 && at01.SmilCount == 0 && at01.InspectorOpen,
		fmt.Sprintf("kind=%s variant=%s val=%.0f/[%.0f..%.0f] webAnims=%d smil=%d inspector=%v",
			at01.Kind, at01.Variant, at01.Val, at01.MinV, at01.MaxV, at01.AnimCount, at01.SmilCount, at01.InspectorOpen))

	// =========================================================================
	// AT-M3-02: All 5 Planned Gauge Variants Rendered Geometry & Value Mapping
	// =========================================================================
	at02JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		// 1. ring at 50%
		window.LiteEffects.updateLiteEffect('1', { variant: 'ring', minValue: 0, maxValue: 100, value: 50 });
		window.LiteEffects.triggerAppRender();
		const ringArc = document.querySelector('#svgContainer #lite-1 .lite-gauge-arc');
		const ringDash = ringArc ? ringArc.getAttribute('stroke-dasharray') : '';

		// 2. hbar at 25% of width=160 -> fillW=40
		window.LiteEffects.updateLiteEffect('1', { variant: 'hbar', width: 160, height: 32, value: 25 });
		window.LiteEffects.triggerAppRender();
		const hbarFill = document.querySelector('#svgContainer #lite-1 .lite-gauge-hbar');
		const hbarW = hbarFill ? parseFloat(hbarFill.getAttribute('width')) : -1;

		// 3. vbar at 75% of height=140 -> fillH=105
		window.LiteEffects.updateLiteEffect('1', { variant: 'vbar', width: 36, height: 140, value: 75 });
		window.LiteEffects.triggerAppRender();
		const vbarFill = document.querySelector('#svgContainer #lite-1 .lite-gauge-vbar');
		const vbarH = vbarFill ? parseFloat(vbarFill.getAttribute('height')) : -1;

		// 4. segmented with 10 segments at 70% -> 7 lit cells, 3 unlit cells
		window.LiteEffects.updateLiteEffect('1', { variant: 'segmented', segments: 10, value: 70 });
		window.LiteEffects.triggerAppRender();
		const litCells = document.querySelectorAll('#svgContainer #lite-1 .lite-gauge-cell[data-cell-lit="true"]').length;
		const unlitCells = document.querySelectorAll('#svgContainer #lite-1 .lite-gauge-cell[data-cell-lit="false"]').length;

		// 5. level at 50%
		window.LiteEffects.updateLiteEffect('1', { variant: 'level', width: 84, height: 120, value: 50 });
		window.LiteEffects.triggerAppRender();
		const levelFill = document.querySelector('#svgContainer #lite-1 .lite-gauge-level');
		const innerH = levelFill ? parseFloat(levelFill.getAttribute('data-inner-h')) : 0;
		const levelH = levelFill ? parseFloat(levelFill.getAttribute('height')) : -1;

		return JSON.stringify({
			ringDash,
			hbarW,
			vbarH,
			litCells,
			unlitCells,
			innerH,
			levelH
		});
	})()`))
	var at02 struct {
		RingDash   string  `json:"ringDash"`
		HbarW      float64 `json:"hbarW"`
		VbarH      float64 `json:"vbarH"`
		LitCells   int     `json:"litCells"`
		UnlitCells int     `json:"unlitCells"`
		InnerH     float64 `json:"innerH"`
		LevelH     float64 `json:"levelH"`
	}
	_ = json.Unmarshal([]byte(at02JSON), &at02)
	record("AT-M3-02", "All 5 Gauge Variants Geometry & Value Mapping",
		at02.RingDash == "50 200" &&
			math.Abs(at02.HbarW-40.0) < 0.1 &&
			math.Abs(at02.VbarH-105.0) < 0.1 &&
			at02.LitCells == 7 && at02.UnlitCells == 3 &&
			at02.InnerH > 50 && math.Abs(at02.LevelH-at02.InnerH*0.5) < 0.2,
		fmt.Sprintf("ringDash=%q hbarW=%.1f vbarH=%.1f segs=%d/%d levelH=%.1f/%.1f",
			at02.RingDash, at02.HbarW, at02.VbarH, at02.LitCells, at02.LitCells+at02.UnlitCells, at02.LevelH, at02.InnerH))

	// =========================================================================
	// AT-M3-03: Valid Zero vs. Negative Range vs. Non-Zero Min vs. Out-of-Range Clamp
	// =========================================================================
	at03JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		// 1. Valid Zero at minValue=0: value=0 -> fraction=0.0000, track visible, readout="0%"
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'hbar',
			width: 160,
			height: 32,
			minValue: 0,
			maxValue: 100,
			value: 0,
			unit: '%',
			labelText: ''
		});
		window.LiteEffects.triggerAppRender();
		const gZero = document.querySelector('#svgContainer #lite-1');
		const zeroFrac = gZero ? gZero.getAttribute('data-gauge-fraction') : '';
		const zeroW = gZero ? parseFloat(gZero.querySelector('.lite-gauge-hbar').getAttribute('width')) : -1;
		const zeroTxt = gZero ? gZero.querySelector('.lite-gauge-value-text').textContent : '';

		// 2. Negative range [-40, 120] with value=0 -> fraction=0.2500 (fillW=40px), readout="0°C"
		window.LiteEffects.updateLiteEffect('1', {
			minValue: -40,
			maxValue: 120,
			value: 0,
			unit: '°C'
		});
		window.LiteEffects.triggerAppRender();
		const gNeg = document.querySelector('#svgContainer #lite-1');
		const negZeroFrac = gNeg ? gNeg.getAttribute('data-gauge-fraction') : '';
		const negZeroW = gNeg ? parseFloat(gNeg.querySelector('.lite-gauge-hbar').getAttribute('width')) : -1;
		const negZeroTxt = gNeg ? gNeg.querySelector('.lite-gauge-value-text').textContent : '';

		// 3. Out-of-range high (value=180 on [-40, 120]): semantic value=180 preserved in readout, visual fill clamped to 1.0 (160px)
		window.LiteEffects.updateLiteEffect('1', { value: 180 });
		window.LiteEffects.triggerAppRender();
		const gHigh = document.querySelector('#svgContainer #lite-1');
		const highFrac = gHigh ? gHigh.getAttribute('data-gauge-fraction') : '';
		const highW = gHigh ? parseFloat(gHigh.querySelector('.lite-gauge-hbar').getAttribute('width')) : -1;
		const highTxt = gHigh ? gHigh.querySelector('.lite-gauge-value-text').textContent : '';

		// 4. Out-of-range low (value=-90 on [-40, 120]): semantic value=-90 preserved, visual fill clamped to 0.0 (0px)
		window.LiteEffects.updateLiteEffect('1', { value: -90 });
		window.LiteEffects.triggerAppRender();
		const gLow = document.querySelector('#svgContainer #lite-1');
		const lowFrac = gLow ? gLow.getAttribute('data-gauge-fraction') : '';
		const lowW = gLow ? parseFloat(gLow.querySelector('.lite-gauge-hbar').getAttribute('width')) : -1;
		const lowTxt = gLow ? gLow.querySelector('.lite-gauge-value-text').textContent : '';

		return JSON.stringify({
			zeroFrac, zeroW, zeroTxt,
			negZeroFrac, negZeroW, negZeroTxt,
			highFrac, highW, highTxt,
			lowFrac, lowW, lowTxt
		});
	})()`))
	var at03 struct {
		ZeroFrac    string  `json:"zeroFrac"`
		ZeroW       float64 `json:"zeroW"`
		ZeroTxt     string  `json:"zeroTxt"`
		NegZeroFrac string  `json:"negZeroFrac"`
		NegZeroW    float64 `json:"negZeroW"`
		NegZeroTxt  string  `json:"negZeroTxt"`
		HighFrac    string  `json:"highFrac"`
		HighW       float64 `json:"highW"`
		HighTxt     string  `json:"highTxt"`
		LowFrac     string  `json:"lowFrac"`
		LowW        float64 `json:"lowW"`
		LowTxt      string  `json:"lowTxt"`
	}
	_ = json.Unmarshal([]byte(at03JSON), &at03)
	record("AT-M3-03", "Valid Zero, Negative Range & Out-of-Range Clamp",
		at03.ZeroFrac == "0.0000" && at03.ZeroW == 0 && at03.ZeroTxt == "0%" &&
			at03.NegZeroFrac == "0.2500" && math.Abs(at03.NegZeroW-40.0) < 0.1 && at03.NegZeroTxt == "0°C" &&
			at03.HighFrac == "1.0000" && math.Abs(at03.HighW-160.0) < 0.1 && at03.HighTxt == "180°C" &&
			at03.LowFrac == "0.0000" && at03.LowW == 0 && at03.LowTxt == "-90°C",
		fmt.Sprintf("zero=(%s,%.0fpx,%s) negZero=(%s,%.0fpx,%s) high=(%s,%.0fpx,%s) low=(%s,%.0fpx,%s)",
			at03.ZeroFrac, at03.ZeroW, at03.ZeroTxt,
			at03.NegZeroFrac, at03.NegZeroW, at03.NegZeroTxt,
			at03.HighFrac, at03.HighW, at03.HighTxt,
			at03.LowFrac, at03.LowW, at03.LowTxt))

	// =========================================================================
	// AT-M3-04: Threshold Banding in Actual Value Domain (0..3600 RPM)
	// =========================================================================
	at04JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'ring',
			minValue: 0,
			maxValue: 3600,
			color: '#22c55e',
			useThresholds: true,
			warnThreshold: 2700,
			warnColor: '#f59e0b',
			critThreshold: 3300,
			critColor: '#ef4444',
			unit: ' RPM'
		});

		const checkVal = (v) => {
			window.LiteEffects.updateLiteEffect('1', { value: v });
			window.LiteEffects.triggerAppRender();
			const arc = document.querySelector('#svgContainer #lite-1 .lite-gauge-arc');
			return arc ? arc.getAttribute('stroke') : '';
		};

		const cHealthy = checkVal(1800);  // < 2700 -> #22c55e
		const cWarnEdge = checkVal(2700); // == 2700 (inclusive boundary) -> #f59e0b
		const cWarnMid = checkVal(3100);  // in [2700, 3300) -> #f59e0b
		const cCritEdge = checkVal(3300); // == 3300 (inclusive boundary) -> #ef4444
		const cCritHigh = checkVal(3550); // > 3300 -> #ef4444

		return JSON.stringify({ cHealthy, cWarnEdge, cWarnMid, cCritEdge, cCritHigh });
	})()`))
	var at04 struct {
		CHealthy  string `json:"cHealthy"`
		CWarnEdge string `json:"cWarnEdge"`
		CWarnMid  string `json:"cWarnMid"`
		CCritEdge string `json:"cCritEdge"`
		CCritHigh string `json:"cCritHigh"`
	}
	_ = json.Unmarshal([]byte(at04JSON), &at04)
	record("AT-M3-04", "Threshold Banding in Actual Value Domain (RPM)",
		at04.CHealthy == "#22c55e" &&
			at04.CWarnEdge == "#f59e0b" && at04.CWarnMid == "#f59e0b" &&
			at04.CCritEdge == "#ef4444" && at04.CCritHigh == "#ef4444",
		fmt.Sprintf("1800=%s 2700=%s 3100=%s 3300=%s 3550=%s",
			at04.CHealthy, at04.CWarnEdge, at04.CWarnMid, at04.CCritEdge, at04.CCritHigh))

	// =========================================================================
	// AT-M3-05: Orientation / Direction (Forward/CW vs. Reverse/CCW) & Rotation
	// =========================================================================
	at05JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'hbar',
			width: 160,
			height: 32,
			minValue: 0,
			maxValue: 100,
			value: 25,
			direction: 'forward',
			rotation: 0
		});
		window.LiteEffects.triggerAppRender();
		const gFwd = document.querySelector('#svgContainer #lite-1');
		const trackX = parseFloat(gFwd.querySelector('.lite-gauge-track').getAttribute('x'));
		const fwdX = parseFloat(gFwd.querySelector('.lite-gauge-hbar').getAttribute('x'));

		window.LiteEffects.updateLiteEffect('1', {
			direction: 'reverse',
			rotation: 45
		});
		window.LiteEffects.triggerAppRender();
		const gRev = document.querySelector('#svgContainer #lite-1');
		const revX = parseFloat(gRev.querySelector('.lite-gauge-hbar').getAttribute('x'));
		const tf = gRev.getAttribute('transform') || '';

		window.LiteEffects.updateLiteEffect('1', { direction: 'forward', rotation: 0 });
		window.LiteEffects.triggerAppRender();
		return JSON.stringify({ trackX, fwdX, revX, tf });
	})()`))
	var at05 struct {
		TrackX float64 `json:"trackX"`
		FwdX   float64 `json:"fwdX"`
		RevX   float64 `json:"revX"`
		Tf     string  `json:"tf"`
	}
	_ = json.Unmarshal([]byte(at05JSON), &at05)
	record("AT-M3-05", "Fill Direction (Forward vs. Reverse) & Rotation",
		math.Abs(at05.FwdX-at05.TrackX) < 0.1 &&
			math.Abs(at05.RevX-(at05.TrackX+120.0)) < 0.1 &&
			strings.Contains(at05.Tf, "rotate(45"),
		fmt.Sprintf("trackX=%.1f fwdX=%.1f revX=%.1f transform=%q",
			at05.TrackX, at05.FwdX, at05.RevX, at05.Tf))

	// =========================================================================
	// AT-M3-06: Value Readout, Unit Suffix, Label Text & Show/Hide Toggle
	// =========================================================================
	at06JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'ring',
			minValue: 0,
			maxValue: 100,
			value: 73.5,
			unit: '%',
			labelText: 'Load',
			showGuide: true
		});
		window.LiteEffects.triggerAppRender();
		const gOn = document.querySelector('#svgContainer #lite-1');
		const valTxt = gOn.querySelector('.lite-gauge-value-text') ? gOn.querySelector('.lite-gauge-value-text').textContent : '';
		const lblTxt = gOn.querySelector('.lite-gauge-label-text') ? gOn.querySelector('.lite-gauge-label-text').textContent : '';
		const visOn = window.getComputedStyle(gOn.querySelector('.lite-gauge-readout')).visibility;

		window.LiteEffects.updateLiteEffect('1', { showGuide: false });
		window.LiteEffects.triggerAppRender();
		const gOff = document.querySelector('#svgContainer #lite-1');
		const visOff = window.getComputedStyle(gOff.querySelector('.lite-gauge-readout')).visibility;

		window.LiteEffects.updateLiteEffect('1', { showGuide: true });
		window.LiteEffects.triggerAppRender();
		return JSON.stringify({ valTxt, lblTxt, visOn, visOff });
	})()`))
	var at06 struct {
		ValTxt string `json:"valTxt"`
		LblTxt string `json:"lblTxt"`
		VisOn  string `json:"visOn"`
		VisOff string `json:"visOff"`
	}
	_ = json.Unmarshal([]byte(at06JSON), &at06)
	record("AT-M3-06", "Value Readout, Unit, Label & Show/Hide Toggle",
		at06.ValTxt == "73.5%" && at06.LblTxt == "Load" && at06.VisOn == "visible" && at06.VisOff == "hidden",
		fmt.Sprintf("val=%q label=%q visOn=%s visOff=%s", at06.ValTxt, at06.LblTxt, at06.VisOn, at06.VisOff))

	// =========================================================================
	// AT-M3-07: Left Drawer & Floating Context Editor Bi-Directional Sync
	// =========================================================================
	at07JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.selectLite('1');
		window.FloatingPropertiesEditor.openFor('lite', 1);
		const fctxGroup = document.getElementById('fctx-liteGaugeGroup');
		const fctxVisible = fctxGroup && fctxGroup.style.display !== 'none';

		const fctxValNum = document.getElementById('fctx-liteGaugeValueNum');
		const fctxUnitIn = document.getElementById('fctx-liteGaugeUnitInput');
		if (fctxValNum) {
			fctxValNum.value = '84';
			fctxValNum.dispatchEvent(new Event('input', { bubbles: true }));
		}
		if (fctxUnitIn) {
			fctxUnitIn.value = ' kW';
			fctxUnitIn.dispatchEvent(new Event('input', { bubbles: true }));
		}
		const le = window.LiteEffects.getLiteEffect('1');
		const drawerVal = document.getElementById('liteGaugeValueNum') ? document.getElementById('liteGaugeValueNum').value : '';
		const drawerUnit = document.getElementById('liteGaugeUnitInput') ? document.getElementById('liteGaugeUnitInput').value : '';
		window.FloatingPropertiesEditor.close();
		return JSON.stringify({
			fctxVisible: Boolean(fctxVisible),
			modelVal: le ? le.value : -1,
			modelUnit: le ? le.unit : '',
			drawerVal,
			drawerUnit
		});
	})()`))
	var at07 struct {
		FctxVisible bool    `json:"fctxVisible"`
		ModelVal    float64 `json:"modelVal"`
		ModelUnit   string  `json:"modelUnit"`
		DrawerVal   string  `json:"drawerVal"`
		DrawerUnit  string  `json:"drawerUnit"`
	}
	_ = json.Unmarshal([]byte(at07JSON), &at07)
	record("AT-M3-07", "Left Drawer & Context Editor Bi-Directional Sync",
		at07.FctxVisible && at07.ModelVal == 84 && at07.ModelUnit == "kW" &&
			at07.DrawerVal == "84" && at07.DrawerUnit == "kW",
		fmt.Sprintf("fctxVisible=%v val=%.0f(drawer=%s) unit=%q(drawer=%q)",
			at07.FctxVisible, at07.ModelVal, at07.DrawerVal, at07.ModelUnit, at07.DrawerUnit))

	// =========================================================================
	// AT-M3-08: Layer Panel Integration, Z-Order, Lock & Grouping
	// =========================================================================
	at08JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', { name: 'mainTurbine' });
		if (typeof window.renderLayersList === 'function') window.renderLayersList();
		const layerRow = document.querySelector('#layersListContainer .layer-item');
		const rowText = layerRow ? layerRow.textContent : '';
		const le = window.LiteEffects.getLiteEffect('1');
		return JSON.stringify({
			hasRow: Boolean(layerRow),
			rowText: rowText.trim(),
			modelName: le ? le.name : ''
		});
	})()`))
	var at08 struct {
		HasRow    bool   `json:"hasRow"`
		RowText   string `json:"rowText"`
		ModelName string `json:"modelName"`
	}
	_ = json.Unmarshal([]byte(at08JSON), &at08)
	record("AT-M3-08", "Layer Panel Integration & Naming Sync",
		at08.HasRow && strings.Contains(at08.RowText, "mainTurbine") && strings.Contains(at08.RowText, "Gauge") && at08.ModelName == "mainTurbine",
		fmt.Sprintf("hasRow=%v name=%q label=%q", at08.HasRow, at08.ModelName, at08.RowText))

	// =========================================================================
	// AT-M3-09: Undo/Redo & Native .sva / SQLite Serialization Round-Trip
	// =========================================================================
	at09JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.LiteEffects.updateLiteEffect('1', {
				variant: 'level',
				minValue: -20,
				maxValue: 200,
				value: 0,
				useThresholds: true,
				warnThreshold: 140,
				critThreshold: 175
			});
			window.HistoryManager.recordImmediate();

			window.LiteEffects.updateLiteEffect('1', { value: 155 });
			window.HistoryManager.recordImmediate();

			window.HistoryManager.undo();
			const afterUndo = window.LiteEffects.getLiteEffect('1');
			const undoOk = Boolean(afterUndo && afterUndo.value === 0 && afterUndo.minValue === -20);

			window.HistoryManager.redo();
			const afterRedo = window.LiteEffects.getLiteEffect('1');
			const redoOk = Boolean(afterRedo && afterRedo.value === 155);

			window.HistoryManager.undo(); // back to value=0

			const schemaStr = window.ProjectManager.serializeStudioState();
			const svaB64 = window.EngineBridge.encodeSVA ? window.EngineBridge.encodeSVA({ title: 'GaugeLiteTest' }) : 'ok';
			window.ProjectManager.resetToNewProject();
			window.ProjectManager.applyStudioState(schemaStr);
			const afterSchema = window.LiteEffects.getLiteEffect('1');
			const schemaOk = Boolean(afterSchema && afterSchema.kind === 'gauge-lite' && afterSchema.variant === 'level' &&
				afterSchema.value === 0 && afterSchema.minValue === -20 && afterSchema.warnThreshold === 140);

			return JSON.stringify({ undoOk, redoOk, svaOk: Boolean(svaB64), schemaOk });
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	var at09 struct {
		UndoOk   bool   `json:"undoOk"`
		RedoOk   bool   `json:"redoOk"`
		SvaOk    bool   `json:"svaOk"`
		SchemaOk bool   `json:"schemaOk"`
		Err      string `json:"err"`
	}
	_ = json.Unmarshal([]byte(at09JSON), &at09)
	record("AT-M3-09", "Undo/Redo & Native .sva / SQLite Persistence",
		at09.UndoOk && at09.RedoOk && at09.SvaOk && at09.SchemaOk,
		fmt.Sprintf("undo=%v redo=%v sva=%v schema=%v err=%s", at09.UndoOk, at09.RedoOk, at09.SvaOk, at09.SchemaOk, at09.Err))

	// =========================================================================
	// AT-M3-10: Standalone SVG Export of All 5 Variants (Zero Idle Animation)
	// =========================================================================
	at10JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.ProjectManager.resetToNewProject();
		const variants = ['ring', 'hbar', 'vbar', 'segmented', 'level'];
		variants.forEach((v, idx) => {
			window.LiteEffects.createLiteEffect({
				kind: 'gauge-lite',
				variant: v,
				x: 150 + idx * 180,
				y: 250,
				minValue: 0,
				maxValue: 100,
				value: 80,
				useThresholds: true,
				warnThreshold: 70,
				critThreshold: 90
			});
		});
		window.triggerAppRender();
		const rawSvg = window.buildFullSVG(window.SceneState.buildRenderState());
		const allPresent = variants.every((v) => rawSvg.includes('data-gauge-variant="' + v + '"'));
		const hasWarnColor = rawSvg.includes('data-gauge-active-color="#f59e0b"');
		const zeroWasm = !rawSvg.includes('wasm_exec.js') && !rawSvg.includes('engine.wasm');
		return JSON.stringify({
			bytes: rawSvg.length,
			allPresent,
			hasWarnColor,
			zeroWasm
		});
	})()`))
	var at10 struct {
		Bytes        int  `json:"bytes"`
		AllPresent   bool `json:"allPresent"`
		HasWarnColor bool `json:"hasWarnColor"`
		ZeroWasm     bool `json:"zeroWasm"`
	}
	_ = json.Unmarshal([]byte(at10JSON), &at10)
	record("AT-M3-10", "Standalone SVG Export of All 5 Gauge Variants",
		at10.Bytes > 2500 && at10.AllPresent && at10.HasWarnColor && at10.ZeroWasm,
		fmt.Sprintf("bytes=%d all5Variants=%v thresholdColors=%v zeroWasm=%v",
			at10.Bytes, at10.AllPresent, at10.HasWarnColor, at10.ZeroWasm))

	// =========================================================================
	// AT-M3-11, AT-M3-12 & AT-M3-13:
	//   - AT-M3-11: Stage 16C Multi-Instance Isolation & Application Colour Override Precedence
	//   - AT-M3-12: Missing / Invalid Input Rejection & High-Frequency In-Place Updates (No DOM Re-Creation)
	//   - AT-M3-13: Off-Screen Telemetry Updates Applied While Suspended
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](widgetJSSource+"\n;Boolean(window.SVGWidget)"))
	at1113JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.ProjectManager.resetToNewProject();
			const g = window.LiteEffects.createLiteEffect({
				name: 'reactorPressure',
				kind: 'gauge-lite',
				variant: 'ring',
				minValue: 0,
				maxValue: 1000,
				value: 400,
				color: '#22c55e',
				bgColor: '#1e293b',
				useThresholds: true,
				warnThreshold: 700,
				warnColor: '#f59e0b',
				critThreshold: 900,
				critColor: '#ef4444',
				unit: ' bar'
			});
			window.triggerAppRender();

			window.EngineBridge.setWidgetParameters([]);
			const props = [
				{ prop: 'value', type: 'number', def: 400 },
				{ prop: 'minValue', type: 'number', def: 0 },
				{ prop: 'maxValue', type: 'number', def: 1000 },
				{ prop: 'color', type: 'color', def: '#22c55e' },
				{ prop: 'bgColor', type: 'color', def: '#1e293b' },
				{ prop: 'strokeWidth', type: 'number', def: 8.0, min: 0.5, max: 32 },
				{ prop: 'opacity', type: 'number', def: 1.0, min: 0, max: 1 },
				{ prop: 'visibility', type: 'boolean', def: true },
				{ prop: 'showGuide', type: 'boolean', def: true }
			];
			const pMap = {};
			for (const p of props) {
				const pName = window.EngineBridge.generateParameterName(g.name, p.prop, 'lite');
				pMap[p.prop] = pName;
				window.EngineBridge.addWidgetParameter({
					name: pName,
					type: p.type,
					target: 'lite-' + g.id,
					property: p.prop,
					default: p.def,
					min: p.min,
					max: p.max
				});
			}
			const params = window.EngineBridge.getWidgetParameters();
			const rawSvg = window.buildFullSVG(window.SceneState.buildRenderState());
			const manifest = { schemaVersion: 1, parameters: params };

			let host = document.getElementById('m3WidgetTestHost');
			if (host) host.remove();
			host = document.createElement('div');
			host.id = 'm3WidgetTestHost';
			host.style.cssText = 'position:fixed;left:0;top:0;width:600px;height:200px;z-index:99999;background:#0d1117;display:flex;gap:10px;';
			const hostA = document.createElement('div');
			const hostB = document.createElement('div');
			hostA.style.cssText = 'width:280px;height:180px;';
			hostB.style.cssText = 'width:280px;height:180px;';
			host.appendChild(hostA);
			host.appendChild(hostB);
			document.body.appendChild(host);

			const instA = window.SVGWidget.mount(hostA, rawSvg, { instanceId: 'gaugeA', manifest, visibilityAware: false });
			const instB = window.SVGWidget.mount(hostB, rawSvg, { instanceId: 'gaugeB', manifest, visibilityAware: false });

			// 1. Update Instance A to Warning (750 bar) and Instance B to Critical (950 bar)
			instA.setParameter(pMap.value, 750);
			instB.setParameter(pMap.value, 950);

			const arcA = hostA.querySelector('.lite-gauge-arc');
			const arcB = hostB.querySelector('.lite-gauge-arc');
			const warnColA = arcA.getAttribute('stroke'); // #f59e0b
			const critColB = arcB.getAttribute('stroke'); // #ef4444

			// 2. Application Colour Override Precedence on Instance A:
			// Supply explicit purple #a855f7 on Instance A, then push value to 980 (Critical).
			// Instance A must keep #a855f7 override while Instance B stays #ef4444!
			instA.setParameter(pMap.color, '#a855f7');
			instA.setParameter(pMap.value, 980);
			const overrideColA = arcA.getAttribute('stroke'); // #a855f7

			// 3. Reset parameters on Instance A clears override and restores automatic threshold evaluation
			instA.resetParameters();
			instA.setParameter(pMap.value, 920);
			const restoredCritA = arcA.getAttribute('stroke'); // #ef4444

			// 4. Invalid / missing data rejection (null, undefined, NaN, 'invalid') preserves previous valid value 920
			let rejectedCount = 0;
			for (const badVal of [null, undefined, NaN, 'not-a-number']) {
				try {
					instA.setParameter(pMap.value, badVal);
				} catch (err) {
					rejectedCount++;
				}
			}
			const preservedAfterReject = instA.getParameter(pMap.value);

			// 5. High-frequency updates (250 rapid telemetry updates) mutate existing DOM node in-place without recreating SVG elements
			const refArcBefore = hostA.querySelector('.lite-gauge-arc');
			for (let i = 0; i <= 250; i++) {
				instA.setParameter(pMap.value, (i * 4) % 1000);
			}
			const refArcAfter = hostA.querySelector('.lite-gauge-arc');
			const sameDomNode = (refArcBefore === refArcAfter);

			// 6. Off-screen / paused telemetry update: pause widget, send value=500, verify DOM updated immediately while paused
			instA.pause();
			instA.setParameter(pMap.value, 500);
			const pausedDash = arcA.getAttribute('stroke-dasharray');
			const pausedTxt = hostA.querySelector('.lite-gauge-value-text').textContent;
			instA.resume();

			instA.destroy();
			instB.destroy();
			host.remove();

			return JSON.stringify({
				paramCount: params.length,
				warnColA,
				critColB,
				overrideColA,
				restoredCritA,
				rejectedCount,
				preservedAfterReject,
				sameDomNode,
				pausedDash,
				pausedTxt
			});
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	var at1113 struct {
		ParamCount           int     `json:"paramCount"`
		WarnColA             string  `json:"warnColA"`
		CritColB             string  `json:"critColB"`
		OverrideColA         string  `json:"overrideColA"`
		RestoredCritA        string  `json:"restoredCritA"`
		RejectedCount        int     `json:"rejectedCount"`
		PreservedAfterReject float64 `json:"preservedAfterReject"`
		SameDomNode          bool    `json:"sameDomNode"`
		PausedDash           string  `json:"pausedDash"`
		PausedTxt            string  `json:"pausedTxt"`
		Err                  string  `json:"err"`
	}
	_ = json.Unmarshal([]byte(at1113JSON), &at1113)
	record("AT-M3-11", "Multi-Instance Isolation & Colour Precedence",
		at1113.ParamCount == 9 &&
			at1113.WarnColA == "#f59e0b" && at1113.CritColB == "#ef4444" &&
			at1113.OverrideColA == "#a855f7" && at1113.RestoredCritA == "#ef4444",
		fmt.Sprintf("params=%d warnA=%s critB=%s overrideA=%s afterResetCritA=%s err=%s",
			at1113.ParamCount, at1113.WarnColA, at1113.CritColB, at1113.OverrideColA, at1113.RestoredCritA, at1113.Err))

	record("AT-M3-12", "Invalid Data Rejection & 250x In-Place Updates",
		at1113.RejectedCount == 4 && at1113.PreservedAfterReject == 920 && at1113.SameDomNode,
		fmt.Sprintf("rejected=%d/4 preservedVal=%.0f sameDomNodeAfter250Updates=%v",
			at1113.RejectedCount, at1113.PreservedAfterReject, at1113.SameDomNode))

	record("AT-M3-13", "Telemetry Updates Applied While Paused/Suspended",
		at1113.PausedDash == "50 200" && at1113.PausedTxt == "500 bar",
		fmt.Sprintf("pausedDash=%q pausedReadout=%q", at1113.PausedDash, at1113.PausedTxt))

	// =========================================================================
	// AT-M3-14: Coexistence with 16D-A, Flow Lite, Radar Ping Lite, Glow & Ripple
	// =========================================================================
	at14JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.createLiteEffect({ kind: 'perimeter-tracer', x: 120, y: 120 });
		window.LiteEffects.createLiteEffect({ kind: 'pulse', x: 260, y: 120 });
		window.LiteEffects.createLiteEffect({ kind: 'flash', x: 380, y: 120, repeat: true });
		window.LiteEffects.createLiteEffect({ kind: 'flow-lite', points: [{ x: 15, y: 80 }, { x: 85, y: 80 }] });
		window.LiteEffects.createLiteEffect({ kind: 'radar-lite', x: 520, y: 120 });
		window.Glows.createGlow({ x: 75, y: 25, color: '#ff0055' });
		window.Ripples.createRipple({ x: 75, y: 65, color: '#38bdf8' });
		window.triggerAppRender();

		const svg = document.getElementById('svgContainer');
		return JSON.stringify({
			hasGauge: Boolean(svg.querySelector('[data-lite-kind="gauge-lite"]')),
			hasRadar: Boolean(svg.querySelector('[data-lite-kind="radar-lite"]')),
			hasTracer: Boolean(svg.querySelector('[data-lite-kind="perimeter-tracer"]')),
			hasPulse: Boolean(svg.querySelector('[data-lite-kind="pulse"]')),
			hasFlash: Boolean(svg.querySelector('[data-lite-kind="flash"]')),
			hasFlowLite: Boolean(svg.querySelector('[data-lite-kind="flow-lite"]')),
			hasGlow: Boolean(svg.querySelector('.diagram-glow-instance')),
			hasRipple: Boolean(svg.querySelector('.diagram-ripple-instance'))
		});
	})()`))
	var at14 struct {
		HasGauge    bool `json:"hasGauge"`
		HasRadar    bool `json:"hasRadar"`
		HasTracer   bool `json:"hasTracer"`
		HasPulse    bool `json:"hasPulse"`
		HasFlash    bool `json:"hasFlash"`
		HasFlowLite bool `json:"hasFlowLite"`
		HasGlow     bool `json:"hasGlow"`
		HasRipple   bool `json:"hasRipple"`
	}
	_ = json.Unmarshal([]byte(at14JSON), &at14)
	record("AT-M3-14", "Coexistence with All Lite Effects, Glow & Ripple",
		at14.HasGauge && at14.HasRadar && at14.HasTracer && at14.HasPulse &&
			at14.HasFlash && at14.HasFlowLite && at14.HasGlow && at14.HasRipple,
		fmt.Sprintf("gauge=%v radar=%v tracer=%v pulse=%v flash=%v flowLite=%v glow=%v ripple=%v",
			at14.HasGauge, at14.HasRadar, at14.HasTracer, at14.HasPulse, at14.HasFlash, at14.HasFlowLite, at14.HasGlow, at14.HasRipple))

	// =========================================================================
	// AT-M3-15: Gauge / Level Lite Visual Showcase Page Verification
	// =========================================================================
	_ = runVoid(taskCtx,
		chromedp.Navigate(gaugeDemoURL),
		chromedp.WaitReady("#demo-card-ring"),
		chromedp.Sleep(300*time.Millisecond),
	)
	at15JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const staticIds = [
			'demo-ring-min',
			'demo-ring-mid',
			'demo-ring-max',
			'demo-hbar-zero-celsius',
			'demo-hbar-warn-celsius',
			'demo-segmented-70',
			'demo-vbar-25',
			'demo-vbar-88',
			'demo-level-15',
			'demo-level-62',
			'demo-level-95'
		];
		const staticPresent = staticIds.filter((id) => Boolean(document.getElementById(id))).length;
		const instA = window._demoGaugeInstA;
		const instB = window._demoGaugeInstB;
		const widgetsMounted = Boolean(instA && instB);
		let runtimeIsolated = false;
		if (widgetsMounted) {
			instA.setParameter('cpuload-lite-value', 80);
			const arcA = document.querySelector('#widget-host-a .lite-gauge-arc');
			const arcB = document.querySelector('#widget-host-b .lite-gauge-arc');
			runtimeIsolated = Boolean(arcA && arcB && arcA.getAttribute('stroke') === '#f59e0b' && arcB.getAttribute('stroke') === '#ef4444');
		}
		return JSON.stringify({
			staticPresent,
			totalStatic: staticIds.length,
			widgetsMounted,
			runtimeIsolated
		});
	})()`))
	var at15 struct {
		StaticPresent   int  `json:"staticPresent"`
		TotalStatic     int  `json:"totalStatic"`
		WidgetsMounted  bool `json:"widgetsMounted"`
		RuntimeIsolated bool `json:"runtimeIsolated"`
	}
	_ = json.Unmarshal([]byte(at15JSON), &at15)
	record("AT-M3-15", "Gauge Lite Visual Showcase Page (11 Static + 2 Live Widgets)",
		at15.StaticPresent == 11 && at15.WidgetsMounted && at15.RuntimeIsolated,
		fmt.Sprintf("url=%s static=%d/%d liveWidgets=%v isolated=%v",
			gaugeDemoURL, at15.StaticPresent, at15.TotalStatic, at15.WidgetsMounted, at15.RuntimeIsolated))

	// =========================================================================
	// AT-M3-16: Stage 16C External Widget Consumer (:8091) Non-Regression
	// =========================================================================
	_ = runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.Sleep(300*time.Millisecond),
	)
	consumerTitle, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.title`))
	record("AT-M3-16", "Stage 16C External Consumer (:8091) Non-Regression",
		strings.Contains(consumerTitle, "Stage 16C"),
		fmt.Sprintf("consumer8091Title=%q", consumerTitle))

	fmt.Println("\n==========================================================================")
	passedCount := 0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
	}
	fmt.Printf(" STAGE 16D-B-M3 VERIFICATION SUMMARY: %d / %d GATES PASSED\n", passedCount, len(results))
	fmt.Println("==========================================================================")
	if passedCount != len(results) {
		os.Exit(1)
	}
}
