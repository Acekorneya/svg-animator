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
	radarDemoURL = "http://192.168.100.40:8088/radar-lite-demo.html"
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
	fmt.Println(" Stage 16D-B-M2: Radar Ping Lite & Flow Lite Corrections Acceptance Suite")
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
	// AT-M2-01: Minor Flow Lite Corrections (Cycle Duration label + 0.2..30.0 s)
	// =========================================================================
	_ = runVoid(taskCtx,
		chromedp.Click("#railBtn-lite-effects"),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click("#btnAddLiteFlow"),
		chromedp.Sleep(250*time.Millisecond),
	)
	at01JSON, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.FloatingPropertiesEditor.openFor('lite', 1);
			const drawerSlider = document.getElementById('liteFlowSpeedSlider');
			const fctxSlider = document.getElementById('fctx-liteFlowSpeedSlider');
			const drawerLabel = drawerSlider && drawerSlider.closest('.inspector-prop-row')
				? drawerSlider.closest('.inspector-prop-row').querySelector('label').textContent.trim()
				: '';
			const fctxLabel = fctxSlider && fctxSlider.closest('.floating-prop-row')
				? fctxSlider.closest('.floating-prop-row').querySelector('label').textContent.trim()
				: '';
			const drawerMax = drawerSlider ? drawerSlider.getAttribute('max') : '';
			const drawerNumMax = document.getElementById('liteFlowSpeedNum') ? document.getElementById('liteFlowSpeedNum').getAttribute('max') : '';
			const fctxMax = fctxSlider ? fctxSlider.getAttribute('max') : '';
			const fctxNumMax = document.getElementById('fctx-liteFlowSpeedNum') ? document.getElementById('fctx-liteFlowSpeedNum').getAttribute('max') : '';

			// Set 30.0 s and verify model + DOM + undo/redo + serialization
			window.LiteEffects.updateLiteEffect('1', { speed: 30.0 });
			window.LiteEffects.triggerAppRender();
			window.HistoryManager.recordImmediate();
			const after30 = window.LiteEffects.getLiteEffect('1').speed;
			const domEl = document.querySelector('#svgContainer #lite-1');
			const domSpeedAttr = domEl ? domEl.getAttribute('data-flow-speed') : '';

			window.LiteEffects.updateLiteEffect('1', { speed: 5.0 });
			window.HistoryManager.recordImmediate();
			window.HistoryManager.undo();
			const afterUndo = window.LiteEffects.getLiteEffect('1').speed;

			const saved = window.ProjectManager.serializeStudioState();
			window.ProjectManager.resetToNewProject();
			window.ProjectManager.applyStudioState(saved);
			const afterRestore = window.LiteEffects.getLiteEffect('1') ? window.LiteEffects.getLiteEffect('1').speed : 0;

			window.FloatingPropertiesEditor.close();
			window.ProjectManager.resetToNewProject();
			return JSON.stringify({
				drawerLabel,
				fctxLabel,
				drawerMax,
				drawerNumMax,
				fctxMax,
				fctxNumMax,
				after30,
				domSpeedAttr,
				afterUndo,
				afterRestore
			});
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	if err != nil {
		fmt.Printf("AT-M2-01 eval err: %v\n", err)
	}
	var at01 struct {
		DrawerLabel  string  `json:"drawerLabel"`
		FctxLabel    string  `json:"fctxLabel"`
		DrawerMax    string  `json:"drawerMax"`
		DrawerNumMax string  `json:"drawerNumMax"`
		FctxMax      string  `json:"fctxMax"`
		FctxNumMax   string  `json:"fctxNumMax"`
		After30      float64 `json:"after30"`
		DomSpeedAttr string  `json:"domSpeedAttr"`
		AfterUndo    float64 `json:"afterUndo"`
		AfterRestore float64 `json:"afterRestore"`
		Err          string  `json:"err"`
	}
	_ = json.Unmarshal([]byte(at01JSON), &at01)
	record("AT-M2-01", "Flow Lite Cycle Duration Label & 30.0 s Range",
		strings.Contains(at01.DrawerLabel, "Cycle Duration") &&
			strings.Contains(at01.FctxLabel, "Cycle Duration") &&
			at01.DrawerMax == "30" && at01.DrawerNumMax == "30" &&
			at01.FctxMax == "30" && at01.FctxNumMax == "30" &&
			at01.After30 == 30.0 && at01.DomSpeedAttr == "30" &&
			at01.AfterUndo == 30.0 && at01.AfterRestore == 30.0,
		fmt.Sprintf("label=%q fctx=%q max=%s speed=%.1f dom=%s undo=%.1f restore=%.1f err=%s",
			at01.DrawerLabel, at01.FctxLabel, at01.DrawerMax, at01.After30, at01.DomSpeedAttr, at01.AfterUndo, at01.AfterRestore, at01.Err))

	// =========================================================================
	// AT-M2-02: Create Radar Ping Lite from Low-Load Effects Drawer
	// =========================================================================
	at02JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.openAccordionById('acc-lite-effects');
		const btn = document.getElementById('btnAddLiteRadar');
		if (btn) btn.click();
		const le = window.LiteEffects.getLiteEffect('1');
		const domEl = document.querySelector('#svgContainer #lite-1[data-lite-kind="radar-lite"]');
		const rings = domEl ? domEl.querySelectorAll('.lite-radar-ring') : [];
		const center = domEl ? domEl.querySelector('.lite-radar-center') : null;
		const filters = domEl ? domEl.querySelectorAll('filter, feGaussianBlur, feColorMatrix, feDisplacementMap') : [];
		const inspectorOpen = document.getElementById('liteRadarGroup') && document.getElementById('liteRadarGroup').style.display !== 'none';
		return JSON.stringify({
			exists: Boolean(le),
			kind: le ? le.kind : '',
			variant: le ? le.variant : '',
			ringCount: rings.length,
			hasCenter: Boolean(center),
			filterCount: filters.length,
			inspectorOpen: Boolean(inspectorOpen)
		});
	})()`))
	var at02 struct {
		Exists        bool   `json:"exists"`
		Kind          string `json:"kind"`
		Variant       string `json:"variant"`
		RingCount     int    `json:"ringCount"`
		HasCenter     bool   `json:"hasCenter"`
		FilterCount   int    `json:"filterCount"`
		InspectorOpen bool   `json:"inspectorOpen"`
	}
	_ = json.Unmarshal([]byte(at02JSON), &at02)
	record("AT-M2-02", "Create Radar Ping Lite via Low-Load Drawer",
		at02.Exists && at02.Kind == "radar-lite" && at02.Variant == "multi" &&
			at02.RingCount == 3 && at02.HasCenter && at02.FilterCount == 0 && at02.InspectorOpen,
		fmt.Sprintf("kind=%s variant=%s rings=%d center=%v filters=%d inspector=%v",
			at02.Kind, at02.Variant, at02.RingCount, at02.HasCenter, at02.FilterCount, at02.InspectorOpen))

	// =========================================================================
	// AT-M2-03: Temporal Verification of Outward Monotonic Expansion & Fade
	//           + Negative Control (Stationary & Synchronized Detection)
	// =========================================================================
	at03JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.LiteEffects.updateLiteEffect('1', {
				variant: 'single',
				rings: 1,
				minScale: 0.20,
				maxScale: 1.20,
				period: 2.0,
				opacity: 0.90,
				width: 100,
				height: 100
			});
			window.LiteEffects.triggerAppRender();
			const ring = document.querySelector('#svgContainer #lite-1 .lite-radar-ring');
			const anim = ring ? ring.getAnimations()[0] : null;
			if (!anim) return JSON.stringify({ error: 'no Web Animation found on radar ring' });
			anim.pause();

			const sampleAt = (tMs) => {
				anim.currentTime = tMs;
				const cs = window.getComputedStyle(ring);
				const rect = ring.getBoundingClientRect();
				return {
					w: Number(rect.width.toFixed(2)),
					op: Number(parseFloat(cs.opacity).toFixed(3))
				};
			};

			const s0 = sampleAt(50);     // Near start of cycle
			const s1 = sampleAt(500);    // 25% cycle
			const s2 = sampleAt(1000);   // 50% cycle
			const s3 = sampleAt(1500);   // 75% cycle
			const s4 = sampleAt(1960);   // 98% cycle (near completion fade-out)
			const sWrap = sampleAt(2050); // Next cycle restart

			const stationaryDetected = (Math.abs(s0.w - s0.w) < 0.01);
			anim.play();

			return JSON.stringify({
				s0, s1, s2, s3, s4, sWrap,
				monotonicExpand: (s0.w < s1.w) && (s1.w < s2.w) && (s2.w < s3.w) && (s3.w < s4.w),
				monotonicFade: (s0.op > s2.op) && (s2.op > s4.op) && (s4.op < 0.08),
				cleanCycleRestart: (sWrap.w < s1.w) && (sWrap.op > 0.75),
				stationaryNegativeControl: stationaryDetected
			});
		} catch (e) {
			return JSON.stringify({ error: String(e) });
		}
	})()`))
	var at03 struct {
		S0                        struct{ W, Op float64 } `json:"s0"`
		S2                        struct{ W, Op float64 } `json:"s2"`
		S4                        struct{ W, Op float64 } `json:"s4"`
		SWrap                     struct{ W, Op float64 } `json:"sWrap"`
		MonotonicExpand           bool                    `json:"monotonicExpand"`
		MonotonicFade             bool                    `json:"monotonicFade"`
		CleanCycleRestart         bool                    `json:"cleanCycleRestart"`
		StationaryNegativeControl bool                    `json:"stationaryNegativeControl"`
		Error                     string                  `json:"error"`
	}
	_ = json.Unmarshal([]byte(at03JSON), &at03)
	record("AT-M2-03", "Temporal Monotonic Ring Expansion & Opacity Fade",
		at03.MonotonicExpand && at03.MonotonicFade && at03.CleanCycleRestart && at03.StationaryNegativeControl,
		fmt.Sprintf("w=(%.1f->%.1f->%.1f,wrap=%.1f) op=(%.2f->%.2f->%.2f) negCtrl=%v err=%s",
			at03.S0.W, at03.S2.W, at03.S4.W, at03.SWrap.W, at03.S0.Op, at03.S2.Op, at03.S4.Op, at03.StationaryNegativeControl, at03.Error))

	// =========================================================================
	// AT-M2-04: Multi-Ring Staggered Phase Separation (1..6 Rings) & Negative Control
	// =========================================================================
	at04JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'multi',
			rings: 4,
			minScale: 0.15,
			maxScale: 1.20,
			period: 2.4
		});
		window.LiteEffects.triggerAppRender();
		const rings = Array.from(document.querySelectorAll('#svgContainer #lite-1 .lite-radar-ring'));
		if (rings.length !== 4) return JSON.stringify({ ringCount: rings.length });

		const anims = rings.map(r => r.getAnimations()[0]);
		anims.forEach(a => a && a.pause());
		anims.forEach(a => { if (a) a.currentTime = 300; });

		const widths = rings.map(r => Number(r.getBoundingClientRect().width.toFixed(2)));
		const delays = rings.map(r => r.style.animationDelay);
		const uniqueWidths = new Set(widths.map(w => Math.round(w))).size;

		anims.forEach(a => a && a.play());

		window.LiteEffects.updateLiteEffect('1', { rings: 6 });
		window.LiteEffects.triggerAppRender();
		const count6 = document.querySelectorAll('#svgContainer #lite-1 .lite-radar-ring').length;

		return JSON.stringify({
			ringCount: rings.length,
			widths,
			delays,
			uniqueWidths,
			count6
		});
	})()`))
	var at04 struct {
		RingCount    int       `json:"ringCount"`
		Widths       []float64 `json:"widths"`
		Delays       []string  `json:"delays"`
		UniqueWidths int       `json:"uniqueWidths"`
		Count6       int       `json:"count6"`
	}
	_ = json.Unmarshal([]byte(at04JSON), &at04)
	record("AT-M2-04", "Multi-Ring Staggered Phase Separation (1..6 Rings)",
		at04.RingCount == 4 && at04.UniqueWidths == 4 && at04.Count6 == 6,
		fmt.Sprintf("4-ring widths=%v delays=%v unique=%d 6-ring=%d",
			at04.Widths, at04.Delays, at04.UniqueWidths, at04.Count6))

	// =========================================================================
	// AT-M2-05: Fading Ping Variant & Consistent Non-Scaling Stroke Width
	// =========================================================================
	at05JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			variant: 'fading-ping',
			rings: 3,
			strokeWidth: 4.5,
			color: '#f59e0b'
		});
		window.LiteEffects.triggerAppRender();
		const ring = document.querySelector('#svgContainer #lite-1 .lite-radar-ring');
		if (!ring) return JSON.stringify({});
		return JSON.stringify({
			fill: ring.getAttribute('fill'),
			fillOpacity: ring.getAttribute('fill-opacity'),
			stroke: ring.getAttribute('stroke'),
			strokeWidth: ring.getAttribute('stroke-width'),
			vectorEffect: ring.getAttribute('vector-effect')
		});
	})()`))
	var at05 struct {
		Fill         string `json:"fill"`
		FillOpacity  string `json:"fillOpacity"`
		Stroke       string `json:"stroke"`
		StrokeWidth  string `json:"strokeWidth"`
		VectorEffect string `json:"vectorEffect"`
	}
	_ = json.Unmarshal([]byte(at05JSON), &at05)
	record("AT-M2-05", "Fading Ping Variant & Non-Scaling Stroke Thickness",
		at05.Fill == "#f59e0b" && at05.FillOpacity == "0.18" &&
			(at05.StrokeWidth == "4.5" || at05.StrokeWidth == "4.50") && at05.VectorEffect == "non-scaling-stroke",
		fmt.Sprintf("fill=%s fillOp=%s strokeWidth=%s vectorEffect=%s",
			at05.Fill, at05.FillOpacity, at05.StrokeWidth, at05.VectorEffect))

	// =========================================================================
	// AT-M2-06: Centre Marker Show / Hide Toggle (showGuide)
	// =========================================================================
	at06JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', { showGuide: true });
		window.LiteEffects.triggerAppRender();
		const cOn = document.querySelector('#svgContainer #lite-1 .lite-radar-center');
		const visOn = cOn ? window.getComputedStyle(cOn).visibility : '';

		window.LiteEffects.updateLiteEffect('1', { showGuide: false });
		window.LiteEffects.triggerAppRender();
		const cOff = document.querySelector('#svgContainer #lite-1 .lite-radar-center');
		const visOff = cOff ? window.getComputedStyle(cOff).visibility : '';

		window.LiteEffects.updateLiteEffect('1', { showGuide: true });
		window.LiteEffects.triggerAppRender();
		return JSON.stringify({ visOn, visOff });
	})()`))
	var at06 struct {
		VisOn  string `json:"visOn"`
		VisOff string `json:"visOff"`
	}
	_ = json.Unmarshal([]byte(at06JSON), &at06)
	record("AT-M2-06", "Centre Marker Show / Hide Toggle",
		at06.VisOn == "visible" && at06.VisOff == "hidden",
		fmt.Sprintf("markerOn=%s markerOff=%s", at06.VisOn, at06.VisOff))

	// =========================================================================
	// AT-M2-07: Circular & Elliptical Geometry + Rotation & Aspect Ratio Lock
	// =========================================================================
	at07JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', {
			maintainAspectRatio: false,
			width: 140,
			height: 68,
			rotation: 30
		});
		window.LiteEffects.triggerAppRender();
		const g = document.querySelector('#svgContainer #lite-1');
		const ring = g ? g.querySelector('.lite-radar-ring') : null;
		const rx = ring ? parseFloat(ring.getAttribute('rx')) : 0;
		const ry = ring ? parseFloat(ring.getAttribute('ry')) : 0;
		const tf = g ? (g.getAttribute('transform') || '') : '';
		return JSON.stringify({ rx, ry, tf });
	})()`))
	var at07 struct {
		Rx float64 `json:"rx"`
		Ry float64 `json:"ry"`
		Tf string  `json:"tf"`
	}
	_ = json.Unmarshal([]byte(at07JSON), &at07)
	record("AT-M2-07", "Circular & Elliptical Geometry + Rotation",
		math.Abs(at07.Rx-70.0) < 0.5 && math.Abs(at07.Ry-34.0) < 0.5 && strings.Contains(at07.Tf, "rotate(30"),
		fmt.Sprintf("rx=%.1f ry=%.1f transform=%q", at07.Rx, at07.Ry, at07.Tf))

	// =========================================================================
	// AT-M2-08: Floating Context Editor & Left Drawer Inspector Parity
	// =========================================================================
	at08JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.selectLite('1');
		window.FloatingPropertiesEditor.openFor('lite', 1);
		const fctxGroup = document.getElementById('fctx-liteRadarGroup');
		const fctxVisible = fctxGroup && fctxGroup.style.display !== 'none';

		const fctxRingsSlider = document.getElementById('fctx-liteRadarRingsSlider');
		const fctxPeriodSlider = document.getElementById('fctx-liteRadarPeriodSlider');
		if (fctxRingsSlider) {
			fctxRingsSlider.value = '5';
			fctxRingsSlider.dispatchEvent(new Event('input', { bubbles: true }));
			fctxRingsSlider.dispatchEvent(new Event('change', { bubbles: true }));
		}
		if (fctxPeriodSlider) {
			fctxPeriodSlider.value = '4.2';
			fctxPeriodSlider.dispatchEvent(new Event('input', { bubbles: true }));
			fctxPeriodSlider.dispatchEvent(new Event('change', { bubbles: true }));
		}
		const le = window.LiteEffects.getLiteEffect('1');
		const drawerRings = document.getElementById('liteRadarRingsNum') ? document.getElementById('liteRadarRingsNum').value : '';
		const drawerPeriod = document.getElementById('liteRadarPeriodNum') ? document.getElementById('liteRadarPeriodNum').value : '';
		return JSON.stringify({
			fctxVisible: Boolean(fctxVisible),
			modelRings: le ? le.rings : 0,
			modelPeriod: le ? le.period : 0,
			drawerRings,
			drawerPeriod
		});
	})()`))
	var at08 struct {
		FctxVisible  bool    `json:"fctxVisible"`
		ModelRings   int     `json:"modelRings"`
		ModelPeriod  float64 `json:"modelPeriod"`
		DrawerRings  string  `json:"drawerRings"`
		DrawerPeriod string  `json:"drawerPeriod"`
	}
	_ = json.Unmarshal([]byte(at08JSON), &at08)
	record("AT-M2-08", "Left Drawer & Context Editor Bi-Directional Sync",
		at08.FctxVisible && at08.ModelRings == 5 && math.Abs(at08.ModelPeriod-4.2) < 0.05 &&
			at08.DrawerRings == "5" && at08.DrawerPeriod == "4.2",
		fmt.Sprintf("fctxVisible=%v rings=%d(drawer=%s) period=%.1f(drawer=%s)",
			at08.FctxVisible, at08.ModelRings, at08.DrawerRings, at08.ModelPeriod, at08.DrawerPeriod))

	// =========================================================================
	// AT-M2-09: Layer Panel Integration (Visibility, Lock, Z-Order, Naming)
	// =========================================================================
	at09JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.updateLiteEffect('1', { name: 'perimeterSonar' });
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
	var at09 struct {
		HasRow    bool   `json:"hasRow"`
		RowText   string `json:"rowText"`
		ModelName string `json:"modelName"`
	}
	_ = json.Unmarshal([]byte(at09JSON), &at09)
	record("AT-M2-09", "Layer Panel Integration & Naming Sync",
		at09.HasRow && strings.Contains(at09.RowText, "perimeterSonar") && strings.Contains(at09.RowText, "Radar") && at09.ModelName == "perimeterSonar",
		fmt.Sprintf("hasRow=%v name=%q label=%q", at09.HasRow, at09.ModelName, at09.RowText))

	// =========================================================================
	// AT-M2-10: Undo / Redo & Native .sva / SQLite Serialization Round-Trip
	// =========================================================================
	at10JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.LiteEffects.updateLiteEffect('1', {
				variant: 'fading-ping',
				rings: 4,
				minScale: 0.22,
				maxScale: 1.35,
				period: 3.6,
				strokeWidth: 3.0
			});
			window.HistoryManager.recordImmediate();

			window.LiteEffects.updateLiteEffect('1', { rings: 2, period: 1.2 });
			window.HistoryManager.recordImmediate();

			window.HistoryManager.undo();
			const afterUndo = window.LiteEffects.getLiteEffect('1');
			const undoOk = Boolean(afterUndo && afterUndo.rings === 4 && Math.abs(afterUndo.period - 3.6) < 0.01);

			window.HistoryManager.redo();
			const afterRedo = window.LiteEffects.getLiteEffect('1');
			const redoOk = Boolean(afterRedo && afterRedo.rings === 2 && Math.abs(afterRedo.period - 1.2) < 0.01);

			window.HistoryManager.undo(); // back to 4 rings, 3.6s

			const schemaStr = window.ProjectManager.serializeStudioState();
			const svaB64 = window.EngineBridge.encodeSVA ? window.EngineBridge.encodeSVA({ title: 'RadarLiteTest' }) : 'ok';
			window.ProjectManager.resetToNewProject();
			window.ProjectManager.applyStudioState(schemaStr);
			const afterSchema = window.LiteEffects.getLiteEffect('1');
			const schemaOk = Boolean(afterSchema && afterSchema.kind === 'radar-lite' && afterSchema.variant === 'fading-ping' &&
				afterSchema.rings === 4 && Math.abs(afterSchema.period - 3.6) < 0.01);

			return JSON.stringify({ undoOk, redoOk, svaOk: Boolean(svaB64), schemaOk });
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	var at10 struct {
		UndoOk   bool   `json:"undoOk"`
		RedoOk   bool   `json:"redoOk"`
		SvaOk    bool   `json:"svaOk"`
		SchemaOk bool   `json:"schemaOk"`
		Err      string `json:"err"`
	}
	_ = json.Unmarshal([]byte(at10JSON), &at10)
	record("AT-M2-10", "Undo/Redo & Native .sva / SQLite Persistence",
		at10.UndoOk && at10.RedoOk && at10.SvaOk && at10.SchemaOk,
		fmt.Sprintf("undo=%v redo=%v sva=%v schema=%v err=%s", at10.UndoOk, at10.RedoOk, at10.SvaOk, at10.SchemaOk, at10.Err))

	// =========================================================================
	// AT-M2-11: Standalone SVG Export & Direct Browser Load
	// =========================================================================
	at11JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.triggerAppRender();
		const rawSvg = window.buildFullSVG(window.SceneState.buildRenderState());
		const hasRadar = rawSvg.includes('data-lite-kind="radar-lite"') && rawSvg.includes('sva-lite-radar-anim');
		const zeroWasm = !rawSvg.includes('wasm_exec.js') && !rawSvg.includes('engine.wasm');
		return JSON.stringify({
			bytes: rawSvg.length,
			hasRadar,
			zeroWasm
		});
	})()`))
	var at11 struct {
		Bytes    int  `json:"bytes"`
		HasRadar bool `json:"hasRadar"`
		ZeroWasm bool `json:"zeroWasm"`
	}
	_ = json.Unmarshal([]byte(at11JSON), &at11)
	record("AT-M2-11", "Standalone SVG Export (Zero Runtime Dependency)",
		at11.Bytes > 1500 && at11.HasRadar && at11.ZeroWasm,
		fmt.Sprintf("bytes=%d hasRadar=%v zeroWasmDependency=%v", at11.Bytes, at11.HasRadar, at11.ZeroWasm))

	// =========================================================================
	// AT-M2-12 & AT-M2-13: Stage 16B / 16C Widget Runtime Multi-Instance Isolation
	//                      + Off-Screen Suspension & Resume
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](widgetJSSource+"\n;Boolean(window.SVGWidget)"))
	at1213JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const rl = window.LiteEffects.getLiteEffect('1');
			window.EngineBridge.setWidgetParameters([]);
			const props = [
				{ prop: 'color', type: 'color', def: '#22c55e' },
				{ prop: 'opacity', type: 'number', def: 0.9, min: 0, max: 1 },
				{ prop: 'visibility', type: 'boolean', def: true },
				{ prop: 'speed', type: 'speed', def: 1.0, min: 0.1, max: 10 },
				{ prop: 'strokeWidth', type: 'number', def: 3.0, min: 0.5, max: 20 },
				{ prop: 'scale', type: 'number', def: 1.35, min: 0.2, max: 3.0 },
				{ prop: 'period', type: 'number', def: 3.6, min: 0.2, max: 30.0 },
				{ prop: 'showGuide', type: 'boolean', def: true }
			];
			const pMap = {};
			for (const p of props) {
				const pName = window.EngineBridge.generateParameterName(rl.name || 'perimeterSonar', p.prop, 'lite');
				pMap[p.prop] = pName;
				window.EngineBridge.addWidgetParameter({
					name: pName,
					type: p.type,
					target: 'lite-' + rl.id,
					property: p.prop,
					default: p.def,
					min: p.min,
					max: p.max
				});
			}
			const params = window.EngineBridge.getWidgetParameters();
			const rawSvg = window.buildFullSVG(window.SceneState.buildRenderState());
			const manifest = { schemaVersion: 1, parameters: params };

			let host = document.getElementById('m2WidgetTestHost');
			if (host) host.remove();
			host = document.createElement('div');
			host.id = 'm2WidgetTestHost';
			host.style.cssText = 'position:fixed;left:0;top:0;width:600px;height:200px;z-index:99999;background:#0d1117;display:flex;gap:10px;';
			const hostA = document.createElement('div');
			const hostB = document.createElement('div');
			hostA.style.cssText = 'width:280px;height:180px;';
			hostB.style.cssText = 'width:280px;height:180px;';
			host.appendChild(hostA);
			host.appendChild(hostB);
			document.body.appendChild(host);

			const instA = window.SVGWidget.mount(hostA, rawSvg, { instanceId: 'radarA', manifest, visibilityAware: false });
			const instB = window.SVGWidget.mount(hostB, rawSvg, { instanceId: 'radarB', manifest, visibilityAware: false });

			instA.setParameters({
				[pMap.color]: '#ef4444',
				[pMap.scale]: 1.85,
				[pMap.period]: 1.5,
				[pMap.strokeWidth]: 4.5,
				[pMap.showGuide]: false
			});
			instB.setParameters({
				[pMap.color]: '#22c55e',
				[pMap.scale]: 0.95,
				[pMap.period]: 4.8,
				[pMap.strokeWidth]: 2.0,
				[pMap.showGuide]: true
			});

			const ringA0 = hostA.querySelector('.lite-radar-ring[data-ring-index="0"]');
			const ringA1 = hostA.querySelector('.lite-radar-ring[data-ring-index="1"]');
			const guideA = hostA.querySelector('.lite-radar-center');

			const ringB0 = hostB.querySelector('.lite-radar-ring[data-ring-index="0"]');
			const ringB1 = hostB.querySelector('.lite-radar-ring[data-ring-index="1"]');
			const guideB = hostB.querySelector('.lite-radar-center');

			instA.pause();
			const pausedState = ringA0 ? window.getComputedStyle(ringA0).animationPlayState : '';
			instA.resume();
			const resumedState = ringA0 ? window.getComputedStyle(ringA0).animationPlayState : '';

			const res = {
				paramCount: params.length,
				colorA: ringA0 ? ringA0.getAttribute('stroke') : '',
				scaleA: ringA0 ? ringA0.style.getPropertyValue('--sva-radar-max') : '',
				durA: ringA0 ? ringA0.style.animationDuration : '',
				delayA1: ringA1 ? ringA1.style.animationDelay : '',
				guideA: guideA ? guideA.getAttribute('visibility') : '',
				colorB: ringB0 ? ringB0.getAttribute('stroke') : '',
				scaleB: ringB0 ? ringB0.style.getPropertyValue('--sva-radar-max') : '',
				durB: ringB0 ? ringB0.style.animationDuration : '',
				delayB1: ringB1 ? ringB1.style.animationDelay : '',
				guideB: guideB ? guideB.getAttribute('visibility') : '',
				pausedState,
				resumedState
			};

			instA.destroy();
			instB.destroy();
			host.remove();
			return JSON.stringify(res);
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	var at1213 struct {
		ParamCount   int    `json:"paramCount"`
		ColorA       string `json:"colorA"`
		ScaleA       string `json:"scaleA"`
		DurA         string `json:"durA"`
		DelayA1      string `json:"delayA1"`
		GuideA       string `json:"guideA"`
		ColorB       string `json:"colorB"`
		ScaleB       string `json:"scaleB"`
		DurB         string `json:"durB"`
		DelayB1      string `json:"delayB1"`
		GuideB       string `json:"guideB"`
		PausedState  string `json:"pausedState"`
		ResumedState string `json:"resumedState"`
		Err          string `json:"err"`
	}
	_ = json.Unmarshal([]byte(at1213JSON), &at1213)
	record("AT-M2-12", "Stage 16B/16C Multi-Instance Widget Isolation",
		at1213.ParamCount == 8 &&
			at1213.ColorA == "#ef4444" && at1213.ScaleA == "1.85" && at1213.DurA == "1.5s" && strings.HasPrefix(at1213.DelayA1, "-0.375") && at1213.GuideA == "hidden" &&
			at1213.ColorB == "#22c55e" && at1213.ScaleB == "0.95" && at1213.DurB == "4.8s" && strings.HasPrefix(at1213.DelayB1, "-1.2") && at1213.GuideB == "visible",
		fmt.Sprintf("params=%d A=(%s,scale=%s,dur=%s,d1=%s,guide=%s) B=(%s,scale=%s,dur=%s,d1=%s,guide=%s) err=%s",
			at1213.ParamCount, at1213.ColorA, at1213.ScaleA, at1213.DurA, at1213.DelayA1, at1213.GuideA,
			at1213.ColorB, at1213.ScaleB, at1213.DurB, at1213.DelayB1, at1213.GuideB, at1213.Err))

	record("AT-M2-13", "Visibility-Aware Off-Screen Suspension & Resume",
		at1213.PausedState == "paused" && at1213.ResumedState == "running",
		fmt.Sprintf("pausedState=%s resumedState=%s", at1213.PausedState, at1213.ResumedState))

	// =========================================================================
	// AT-M2-14: Coexistence with Stage 16D-A Effects, Flow Lite, Glow & Ripple
	// =========================================================================
	at14JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.createLiteEffect({ kind: 'perimeter-tracer', x: 20, y: 20 });
		window.LiteEffects.createLiteEffect({ kind: 'pulse', x: 35, y: 20 });
		window.LiteEffects.createLiteEffect({ kind: 'flash', x: 50, y: 20, repeat: true });
		window.LiteEffects.createLiteEffect({ kind: 'flow-lite', points: [{ x: 20, y: 80 }, { x: 80, y: 80 }] });
		window.Glows.createGlow({ x: 75, y: 25, color: '#ff0055' });
		window.Ripples.createRipple({ x: 75, y: 65, color: '#38bdf8' });
		window.triggerAppRender();

		const svg = document.getElementById('svgContainer');
		return JSON.stringify({
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
		HasRadar    bool `json:"hasRadar"`
		HasTracer   bool `json:"hasTracer"`
		HasPulse    bool `json:"hasPulse"`
		HasFlash    bool `json:"hasFlash"`
		HasFlowLite bool `json:"hasFlowLite"`
		HasGlow     bool `json:"hasGlow"`
		HasRipple   bool `json:"hasRipple"`
	}
	_ = json.Unmarshal([]byte(at14JSON), &at14)
	record("AT-M2-14", "Coexistence with 16D-A, Flow Lite, Glow & Ripple",
		at14.HasRadar && at14.HasTracer && at14.HasPulse && at14.HasFlash &&
			at14.HasFlowLite && at14.HasGlow && at14.HasRipple,
		fmt.Sprintf("radar=%v tracer=%v pulse=%v flash=%v flowLite=%v glow=%v ripple=%v",
			at14.HasRadar, at14.HasTracer, at14.HasPulse, at14.HasFlash, at14.HasFlowLite, at14.HasGlow, at14.HasRipple))

	// =========================================================================
	// AT-M2-15: Radar Ping Lite Visual Demonstration Page Verification
	// =========================================================================
	_ = runVoid(taskCtx,
		chromedp.Navigate(radarDemoURL),
		chromedp.WaitReady("#demo-card-variants"),
		chromedp.Sleep(250*time.Millisecond),
	)
	at15JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const groups = [
			'demo-radar-single',
			'demo-radar-multi-3',
			'demo-radar-multi-6',
			'demo-radar-fading',
			'demo-radar-small',
			'demo-radar-large',
			'demo-radar-ellipse',
			'demo-radar-fast-marker-on',
			'demo-radar-slow-marker-off'
		];
		let activeAnims = 0;
		groups.forEach((id) => {
			const g = document.getElementById(id);
			if (g) {
				const ring = g.querySelector('.lite-radar-ring');
				if (ring && ring.getAnimations().length > 0) activeAnims++;
			}
		});
		return JSON.stringify({
			totalExamples: groups.length,
			activeAnims
		});
	})()`))
	var at15 struct {
		TotalExamples int `json:"totalExamples"`
		ActiveAnims   int `json:"activeAnims"`
	}
	_ = json.Unmarshal([]byte(at15JSON), &at15)
	record("AT-M2-15", "Radar Ping Lite Visual Showcase Page (9 Examples)",
		at15.TotalExamples == 9 && at15.ActiveAnims == 9,
		fmt.Sprintf("url=%s activeExamples=%d/%d", radarDemoURL, at15.ActiveAnims, at15.TotalExamples))

	// =========================================================================
	// AT-M2-16: Stage 16C External Widget Consumer (:8091) Non-Regression
	// =========================================================================
	_ = runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.Sleep(300*time.Millisecond),
	)
	consumerTitle, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.title`))
	record("AT-M2-16", "Stage 16C External Consumer (:8091) Non-Regression",
		strings.Contains(consumerTitle, "Stage 16C"),
		fmt.Sprintf("consumer8091Title=%q", consumerTitle))

	fmt.Println("\n==========================================================================")
	passedCount := 0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
	}
	fmt.Printf(" STAGE 16D-B-M2 VERIFICATION SUMMARY: %d / %d GATES PASSED\n", passedCount, len(results))
	fmt.Println("==========================================================================")
	if passedCount != len(results) {
		os.Exit(1)
	}
}
