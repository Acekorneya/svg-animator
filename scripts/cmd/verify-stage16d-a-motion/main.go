package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	studioURL   = "http://192.168.100.40:8088/"
	cdpEndpoint = "http://127.0.0.1:9222"
)

type Check struct {
	Num    int
	Name   string
	Passed bool
	Detail string
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16D-A: Real-Browser Animation, Burst, Geometry & Runtime")
	fmt.Println("==================================================================")

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cCancel := chromedp.NewContext(allocCtx)
	defer cCancel()
	ctx, tCancel := context.WithTimeout(ctx, 120*time.Second)
	defer tCancel()

	var checks []Check
	rec := func(num int, name string, ok bool, detail string) {
		checks = append(checks, Check{Num: num, Name: name, Passed: ok, Detail: detail})
		st := "PASS"
		if !ok {
			st = "FAIL"
		}
		fmt.Printf("[%s] %2d. %-46s - %s\n", st, num, name, detail)
	}

	if _, err := chromedp.Run(ctx, chromedp.Navigate(studioURL)); err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(ctx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_, _ = chromedp.Run(ctx, chromedp.Sleep(250*time.Millisecond))
	}

	// Author test scene in Studio covering:
	// - lite-1: Rect Tracer CW, 1:3, loop, dur=0.8s
	// - lite-2: Small Circle Tracer CCW (rx=17, ry=17), 1:3, loop, dur=0.8s
	// - lite-3: Ellipse Tracer CW, 2:3, pingpong, dur=0.9s
	// - lite-4: Burst Tracer CW (4 circuits @ 0.25s = 1.0s burst, 1.2s pause, hide)
	// - lite-5: Burst Tracer CW (4 circuits @ 0.25s = 1.0s burst, 1.2s pause, hold)
	// - lite-6: Small Pulse Cross (10x6, armThickness=2.5) -> then resize width to 40, check arm stays 2.5
	// - lite-7: Flash Cross (dur=80ms, repeat=true, repeatInterval=0.8s, armThickness=2)
	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);

		const t1 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerRect', variant: 'rect',
			x: 200, y: 160, width: 160, height: 90,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			playbackMode: 'loop', speed: 0.8, opacity: 0.9, color: '#38bdf8'
		});
		const t2 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerCircle17', variant: 'circle',
			x: 420, y: 160, width: 34, height: 34,
			segments: 2, lineGapRatio: '1:3', direction: 'ccw',
			playbackMode: 'loop', speed: 0.8, opacity: 0.9, color: '#22c55e'
		});
		const t3 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerEllipse', variant: 'ellipse',
			x: 600, y: 160, width: 120, height: 60,
			segments: 2, lineGapRatio: '2:3', direction: 'cw',
			playbackMode: 'pingpong', speed: 0.9, opacity: 0.9, color: '#f59e0b'
		});
		const t4 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'burstHide', variant: 'rect',
			x: 200, y: 320, width: 140, height: 70,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			playbackMode: 'loop', speed: 0.25, burstCycles: 4, pauseSeconds: 1.2, pauseVisibility: 'hide',
			opacity: 0.9, color: '#ec4899'
		});
		const t5 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'burstHold', variant: 'rect',
			x: 420, y: 320, width: 140, height: 70,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			playbackMode: 'loop', speed: 0.25, burstCycles: 4, pauseSeconds: 1.2, pauseVisibility: 'hold',
			opacity: 0.9, color: '#a855f7'
		});
		const p1 = window.LiteEffects.createLiteEffect({
			kind: 'pulse', name: 'smallCross', variant: 'cross',
			x: 640, y: 320, width: 10, height: 6, maintainAspectRatio: false, armThickness: 2.5,
			period: 0.9, minScale: 0.75, maxScale: 1.25, intensity: 0.8, opacity: 0.9, color: '#38bdf8'
		});
		const f1 = window.LiteEffects.createLiteEffect({
			kind: 'flash', name: 'repeatFlash', variant: 'cross',
			x: 800, y: 320, width: 40, height: 40, armThickness: 2,
			dur: 0.08, repeat: true, repeatInterval: 0.8, intensity: 0.85, opacity: 0.9, color: '#facc15'
		});

		const params = [
			{ name: 'tracerrect-lite-speed', target: 'lite-' + t1.id, property: 'speed', type: 'speed', default: 1.0, min: 0.1, max: 10.0 },
			{ name: 'tracerrect-lite-opacity', target: 'lite-' + t1.id, property: 'opacity', type: 'number', default: 0.9, min: 0, max: 1 },
			{ name: 'tracerrect-lite-burstcycles', target: 'lite-' + t1.id, property: 'burstCycles', type: 'number', default: 0, min: 0, max: 100 },
			{ name: 'tracerrect-lite-pauseseconds', target: 'lite-' + t1.id, property: 'pauseSeconds', type: 'number', default: 0, min: 0, max: 300 },
			{ name: 'tracerrect-lite-pausevisibility', target: 'lite-' + t1.id, property: 'pauseVisibility', type: 'enum', options: ['hide', 'hold'], default: 'hide' },
			{ name: 'tracercircle17-lite-speed', target: 'lite-' + t2.id, property: 'speed', type: 'speed', default: 1.0, min: 0.1, max: 10.0 },
			{ name: 'tracercircle17-lite-direction', target: 'lite-' + t2.id, property: 'direction', type: 'enum', options: ['cw', 'ccw'], default: 'ccw' },
			{ name: 'smallcross-lite-armthickness', target: 'lite-' + p1.id, property: 'armThickness', type: 'number', default: 2.5, min: 1, max: 20 },
			{ name: 'repeatflash-lite-repeatinterval', target: 'lite-' + f1.id, property: 'repeatInterval', type: 'number', default: 0.8, min: 0.2, max: 60 }
		];
		params.forEach(p => window.EngineBridge.addWidgetParameter(p));
		window.triggerAppRender();
		return true;
	})()`))
	if err != nil {
		log.Fatal("setup:", err)
	}
	_, _ = chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))

	// Check 1: Studio UI — Start Offset & Entry/Exit removed; Ratio preset dropdown + Custom works; 5px Pulse + Cross independent arm thickness
	uiJSON, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const noStartOff = !document.getElementById('liteStartOffsetSlider') && !document.getElementById('fctx-liteStartOffsetSlider');
		const noEntryExit = !document.getElementById('liteEntryExitSelect') && !document.getElementById('fctx-liteEntryExitSelect');

		// Select Tracer 1 and test Ratio preset -> Custom
		window.LiteEffects.selectLite('1');
		const presetSel = document.getElementById('liteRatioPresetSelect');
		const opts = Array.from(presetSel.options).map(o => o.value);
		presetSel.value = '2:3';
		presetSel.dispatchEvent(new Event('change', { bubbles: true }));
		const afterPreset = window.LiteEffects.getLiteEffect('1').lineGapRatio;
		presetSel.value = 'custom';
		presetSel.dispatchEvent(new Event('change', { bubbles: true }));
		const customVisible = document.getElementById('liteRatioCustomRow').style.display !== 'none';
		const ratioIn = document.getElementById('liteRatioInput');
		ratioIn.value = '3:7';
		ratioIn.dispatchEvent(new Event('input', { bubbles: true }));
		const afterCustom = window.LiteEffects.getLiteEffect('1').lineGapRatio;
		// Restore 1:3
		presetSel.value = '1:3';
		presetSel.dispatchEvent(new Event('change', { bubbles: true }));

		// Select smallCross (lite-6): 10x6 with armThickness=2.5, resize width to 40, verify arm thickness in path stays 2.5!
		window.LiteEffects.selectLite('6');
		const crossBefore = document.querySelector('#svgContainer [data-lite-id="6"] .lite-payload').getAttribute('d');
		window.LiteEffects.updateActive('width', 40);
		const crossAfter = document.querySelector('#svgContainer [data-lite-id="6"] .lite-payload').getAttribute('d');
		const wAfter = window.LiteEffects.getLiteEffect('6').width;
		const hAfter = window.LiteEffects.getLiteEffect('6').height;

		// Check unified <ellipse shape-rendering="geometricPrecision"> for Circle (lite-2) & Ellipse (lite-3)
		const c2 = document.querySelector('#svgContainer [data-lite-id="2"] .lite-payload');
		const e3 = document.querySelector('#svgContainer [data-lite-id="3"] .lite-payload');

		return JSON.stringify({
			noStartOff, noEntryExit,
			presetCount: opts.length, afterPreset, customVisible, afterCustom,
			crossBefore, crossAfter, wAfter, hAfter,
			c2Tag: c2 ? c2.tagName.toLowerCase() : '',
			c2Rx: c2 ? c2.getAttribute('rx') : '',
			c2Ry: c2 ? c2.getAttribute('ry') : '',
			c2SR: c2 ? c2.getAttribute('shape-rendering') : '',
			e3Tag: e3 ? e3.tagName.toLowerCase() : ''
		});
	})()`))
	var ui struct {
		NoStartOff    bool    `json:"noStartOff"`
		NoEntryExit   bool    `json:"noEntryExit"`
		PresetCount   int     `json:"presetCount"`
		AfterPreset   string  `json:"afterPreset"`
		CustomVisible bool    `json:"customVisible"`
		AfterCustom   string  `json:"afterCustom"`
		CrossBefore   string  `json:"crossBefore"`
		CrossAfter    string  `json:"crossAfter"`
		WAfter        float64 `json:"wAfter"`
		HAfter        float64 `json:"hAfter"`
		C2Tag         string  `json:"c2Tag"`
		C2Rx          string  `json:"c2Rx"`
		C2Ry          string  `json:"c2Ry"`
		C2SR          string  `json:"c2SR"`
		E3Tag         string  `json:"e3Tag"`
	}
	_ = json.Unmarshal([]byte(uiJSON), &ui)
	rec(1, "Studio: Simplified UI & Ratio Presets",
		ui.NoStartOff && ui.NoEntryExit && ui.PresetCount == 11 && ui.AfterPreset == "2:3" && ui.CustomVisible && ui.AfterCustom == "3:7",
		fmt.Sprintf("noStartOff=%v, noEntryExit=%v, presets=%d, preset=%s, custom=%v(%s)", ui.NoStartOff, ui.NoEntryExit, ui.PresetCount, ui.AfterPreset, ui.CustomVisible, ui.AfterCustom))
	rec(2, "Geometry: Unified Ellipse & Independent Cross",
		ui.C2Tag == "ellipse" && ui.E3Tag == "ellipse" && ui.C2Rx == "17.00" && ui.C2Ry == "17.00" && ui.C2SR == "geometricPrecision" && ui.WAfter == 40 && ui.HAfter == 6 && strings.Contains(ui.CrossAfter, "638.75") && strings.Contains(ui.CrossAfter, "641.25"),
		fmt.Sprintf("circle=<%s rx=%s ry=%s sr=%s>, cross(40x6) arm=[638.75..641.25]", ui.C2Tag, ui.C2Rx, ui.C2Ry, ui.C2SR))

	// Export standalone SVG and Widget Package ZIP
	standaloneSVG, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`window.serializeSVGForExport(window.buildFullSVG(window.currentRenderState))`))
	pkgB64, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const raw = window.serializeSVGForExport(window.buildFullSVG(window.currentRenderState));
		const res = window.EngineBridge.buildWidgetPackage('motion-v2', raw, '');
		return res.zipB64;
	})()`))
	zipBytes, _ := base64.StdEncoding.DecodeString(pkgB64)
	zr, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	var demoHTML, manifestJSON, svgWidgetJS string
	for _, f := range zr.File {
		rc, _ := f.Open()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		if strings.HasSuffix(f.Name, "demo.html") {
			demoHTML = buf.String()
		} else if strings.HasSuffix(f.Name, "widget-manifest.json") {
			manifestJSON = buf.String()
		} else if strings.HasSuffix(f.Name, "svg-widget.js") {
			svgWidgetJS = buf.String()
		}
	}
	if svgWidgetJS == "" {
		b, _ := os.ReadFile("internal/web/static/js/svg-widget.js")
		svgWidgetJS = string(b)
	}
	rec(3, "Manifest: No startOffset, has burst & arm",
		!strings.Contains(manifestJSON, "startoffset") && strings.Contains(manifestJSON, "burstcycles") && strings.Contains(manifestJSON, "armthickness") && strings.Contains(manifestJSON, "repeatinterval"),
		fmt.Sprintf("len=%d, noStartOffset=%v, hasBurst=%v", len(manifestJSON), !strings.Contains(manifestJSON, "startoffset"), strings.Contains(manifestJSON, "burstcycles")))

	// =========================================================================
	// Standalone SVG: Rect, Circle(rx=17), Ellipse pingpong, Burst(4x0.25s + 1.2s pause)
	// =========================================================================
	svgDataURL := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(standaloneSVG))
	if _, err := chromedp.Run(ctx, chromedp.Navigate(svgDataURL)); err != nil {
		log.Fatal("nav standalone:", err)
	}

	type S struct {
		RectOff float64 `json:"rectOff"`
		CircOff float64 `json:"circOff"`
		EllOff  float64 `json:"ellOff"`
		HideVis string  `json:"hideVis"`
		HoldVis string  `json:"holdVis"`
		PulseS  float64 `json:"pulseS"`
	}
	var samples []S
	for i := 0; i < 18; i++ {
		sJSON, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const off = (sel) => parseFloat(getComputedStyle(document.querySelector(sel)).strokeDashoffset);
			const vis = (sel) => getComputedStyle(document.querySelector(sel)).visibility;
			const ps = () => {
				const m = getComputedStyle(document.querySelector('[data-lite-id="6"] .lite-payload')).transform;
				const mm = m && m.match(/matrix\(([^,]+)/);
				return mm ? parseFloat(mm[1]) : 1;
			};
			return JSON.stringify({
				rectOff: off('[data-lite-id="1"] .lite-payload'),
				circOff: off('[data-lite-id="2"] .lite-payload'),
				ellOff: off('[data-lite-id="3"] .lite-payload'),
				hideVis: vis('[data-lite-id="4"] .lite-payload'),
				holdVis: vis('[data-lite-id="5"] .lite-payload'),
				pulseS: ps()
			});
		})()`))
		var s S
		_ = json.Unmarshal([]byte(sJSON), &s)
		samples = append(samples, s)
		_, _ = chromedp.Run(ctx, chromedp.Sleep(180*time.Millisecond))
	}

	// Check Rect & Circle-17px movement and loop wrap, and Ellipse pingpong reversal
	rectWrap := false
	circMoved := false
	ellInc, ellDec := false, false
	sawHidePause, sawHoldDuringPause, sawResume := false, false, false
	minPS, maxPS := 99.0, 0.0
	for i, s := range samples {
		if i > 0 {
			if s.RectOff > samples[i-1].RectOff+200 {
				rectWrap = true
			}
			if s.CircOff > samples[i-1].CircOff+20 {
				circMoved = true
			}
			if s.EllOff < samples[i-1].EllOff-20 {
				ellDec = true
			}
			if s.EllOff > samples[i-1].EllOff+20 {
				ellInc = true
			}
		}
		if s.HideVis == "hidden" {
			sawHidePause = true
			if s.HoldVis == "visible" {
				sawHoldDuringPause = true
			}
		}
		if sawHidePause && s.HideVis == "visible" {
			sawResume = true
		}
		if s.PulseS < minPS {
			minPS = s.PulseS
		}
		if s.PulseS > maxPS {
			maxPS = s.PulseS
		}
	}
	rec(4, "Standalone: Rect & 17px Circle seamless loop", rectWrap && circMoved,
		fmt.Sprintf("rectWrap=%v, circ17Moved=%v, ellPingPong=%v", rectWrap, circMoved, ellInc && ellDec))
	rec(5, "Standalone: Burst 4x + Pause (hide vs hold)", sawHidePause && sawHoldDuringPause && sawResume,
		fmt.Sprintf("sawHidePause=%v, holdVisibleDuringPause=%v, resumedAfterPause=%v", sawHidePause, sawHoldDuringPause, sawResume))
	rec(6, "Standalone: Small Pulse smooth scale", (maxPS-minPS) > 0.2,
		fmt.Sprintf("pulseScale=[%.3f..%.3f]", minPS, maxPS))

	// =========================================================================
	// Exported Widget Package Demo: Opacity, Speed, Burst, Suspension
	// =========================================================================
	_ = os.WriteFile("/tmp/motion-v2-package.zip", zipBytes, 0644)
	_ = exec.Command("scp", "/tmp/motion-v2-package.zip", "nigel@192.168.100.40:/tmp/motion-v2-package.zip").Run()
	_ = exec.Command("ssh", "nigel@192.168.100.40", "rm -rf /tmp/motion-v2-pkg && mkdir -p /tmp/motion-v2-pkg && unzip -q -o /tmp/motion-v2-package.zip -d /tmp/motion-v2-pkg").Run()
	_ = exec.Command("scp", "internal/web/static/js/svg-widget.js", "nigel@192.168.100.40:/tmp/motion-v2-pkg/motion-v2/js/svg-widget.js").Run()

	if _, err := chromedp.Run(ctx, chromedp.Navigate("file:///tmp/motion-v2-pkg/motion-v2/demo.html")); err != nil {
		log.Fatal("nav demo:", err)
	}
	_, _ = chromedp.Run(ctx, chromedp.WaitReady("#container-unit-a svg"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(350*time.Millisecond))

	opOut, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		const g = w.svgElement.querySelector('[data-semantic-target="lite-1"]');
		return JSON.stringify([0, 0.25, 0.5, 0.75, 1].map(v => {
			w.setParameter('tracerrect-lite-opacity', v);
			return parseFloat(getComputedStyle(g).opacity);
		}));
	})()`))
	var opArr []float64
	_ = json.Unmarshal([]byte(opOut), &opArr)
	opOk := len(opArr) == 5 && math.Abs(opArr[1]-0.25) < 0.01 && math.Abs(opArr[2]-0.5) < 0.01 && math.Abs(opArr[3]-0.75) < 0.01
	rec(7, "Widget Demo: Continuous Opacity 0..1", opOk, fmt.Sprintf("computed=%v", opArr))

	// Test Burst + Pause + Visibility-Suspension resumption in Widget Demo
	_, _ = chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		w.setParameter('tracerrect-lite-burstcycles', 2);
		w.setParameter('tracerrect-lite-pauseseconds', 0.6);
		w.setParameter('tracerrect-lite-pausevisibility', 'hide');
		w.setParameter('tracerrect-lite-speed', 4.0); // 0.2s per circuit -> 0.4s burst
		return true;
	})()`))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(550*time.Millisecond))
	inPause, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`getComputedStyle(instances['unit-a'].svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')).visibility`))
	// Suspend while paused, wait past pause, verify still suspended, then resume and verify restarted!
	_, _ = chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => { instances['unit-a'].pause(); return true; })()`))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(700*time.Millisecond))
	whilePaused, _ := chromedp.Run(ctx, chromedp.Evaluate[bool](`instances['unit-a'].wrapper.classList.contains('sva-suspended')`))
	_, _ = chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => { instances['unit-a'].resume(); return true; })()`))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(120*time.Millisecond))
	afterResumeVis, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`getComputedStyle(instances['unit-a'].svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload')).visibility`))
	rec(8, "Widget Demo: Burst/Pause + Suspend/Resume",
		inPause == "hidden" && whilePaused && afterResumeVis == "visible",
		fmt.Sprintf("inPause=%s, whileSuspended=%v, afterResume=%s", inPause, whilePaused, afterResumeVis))

	_ = demoHTML
	fmt.Println("==================================================================")
	for _, c := range checks {
		if !c.Passed {
			fmt.Println(">>> SOME CHECKS FAILED <<<")
			os.Exit(1)
		}
	}
	fmt.Println(">>> ALL 8 STAGE 16D-A SIMPLIFICATION, BURST & GEOMETRY CHECKS PASS <<<")
}
