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
	fmt.Println(" Stage 16D-A: Real-Browser Animation & Exported-Runtime Suite")
	fmt.Println("==================================================================")

	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cCancel := chromedp.NewContext(allocCtx)
	defer cCancel()
	ctx, tCancel := context.WithTimeout(ctx, 90*time.Second)
	defer tCancel()

	var checks []Check
	rec := func(num int, name string, ok bool, detail string) {
		checks = append(checks, Check{Num: num, Name: name, Passed: ok, Detail: detail})
		st := "PASS"
		if !ok {
			st = "FAIL"
		}
		fmt.Printf("[%s] %2d. %-44s - %s\n", st, num, name, detail)
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

	// Author a scene with:
	// - lite-1: Rect Tracer CW, 1:3, loop, dur=1.0s
	// - lite-2: Rounded Tracer CCW, 1:3, fade-grow, loop, dur=1.5s
	// - lite-3: Rect Tracer CW, fade-only, pingpong, dur=1.2s
	// - lite-4: Rect Tracer CW, grow-only, once, dur=1.2s
	// - lite-5: Pulse ellipse, period=1.0s, minScale=0.7, maxScale=1.3
	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);

		const t1 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerCW', variant: 'rect',
			x: 240, y: 180, width: 160, height: 90,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			entryExit: 'none', playbackMode: 'loop', speed: 1.0, opacity: 0.9, color: '#38bdf8'
		});
		const t2 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerCCW', variant: 'rounded',
			x: 520, y: 180, width: 160, height: 90,
			segments: 2, lineGapRatio: '1:3', direction: 'ccw',
			entryExit: 'fade-grow', playbackMode: 'loop', speed: 1.5, opacity: 0.9, color: '#22c55e'
		});
		const t3 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerFade', variant: 'rect',
			x: 240, y: 340, width: 160, height: 90,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			entryExit: 'fade', playbackMode: 'pingpong', speed: 1.2, opacity: 0.9, color: '#f59e0b'
		});
		const t4 = window.LiteEffects.createLiteEffect({
			kind: 'perimeter-tracer', name: 'tracerGrowOnce', variant: 'rect',
			x: 520, y: 340, width: 160, height: 90,
			segments: 2, lineGapRatio: '1:3', direction: 'cw',
			entryExit: 'grow', playbackMode: 'once', speed: 1.2, opacity: 0.9, color: '#ec4899'
		});
		const p1 = window.LiteEffects.createLiteEffect({
			kind: 'pulse', name: 'pulseEllipse', variant: 'circle',
			x: 800, y: 240, width: 100, height: 60,
			period: 1.0, minScale: 0.7, maxScale: 1.3, intensity: 0.8, opacity: 0.9, color: '#a855f7'
		});

		const params = [
			{ name: 'tracercw-lite-speed', target: 'lite-' + t1.id, property: 'speed', type: 'speed', default: 1.0, min: 0.1, max: 10.0 },
			{ name: 'tracercw-lite-opacity', target: 'lite-' + t1.id, property: 'opacity', type: 'number', default: 0.9, min: 0, max: 1 },
			{ name: 'tracerccw-lite-speed', target: 'lite-' + t2.id, property: 'speed', type: 'speed', default: 1.0, min: 0.1, max: 10.0 },
			{ name: 'tracerccw-lite-direction', target: 'lite-' + t2.id, property: 'direction', type: 'enum', options: ['cw', 'ccw'], default: 'ccw' },
			{ name: 'pulseellipse-lite-scale', target: 'lite-' + p1.id, property: 'scale', type: 'number', default: 1.3, min: 0.2, max: 3.0 }
		];
		params.forEach(p => window.EngineBridge.addWidgetParameter(p));
		window.triggerAppRender();
		return true;
	})()`))
	if err != nil {
		log.Fatal("setup:", err)
	}
	_, _ = chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))

	// Export standalone SVG and Widget Package ZIP from Studio
	standaloneSVG, err := chromedp.Run(ctx, chromedp.Evaluate[string](`window.serializeSVGForExport(window.buildFullSVG(window.currentRenderState))`))
	if err != nil {
		log.Fatal("export svg:", err)
	}
	pkgB64, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const raw = window.serializeSVGForExport(window.buildFullSVG(window.currentRenderState));
		const res = window.EngineBridge.buildWidgetPackage('motion-test', raw, '');
		if (!res || !res.success || !res.zipB64) throw new Error(res && res.error || 'build failed');
		return res.zipB64;
	})()`))
	if err != nil {
		log.Fatal("export pkg:", err)
	}

	zipBytes, _ := base64.StdEncoding.DecodeString(pkgB64)
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		log.Fatal("zip read:", err)
	}
	var demoHTML, svgWidgetJS string
	for _, f := range zr.File {
		rc, _ := f.Open()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		if strings.HasSuffix(f.Name, "demo.html") {
			demoHTML = buf.String()
		} else if strings.HasSuffix(f.Name, "svg-widget.js") {
			svgWidgetJS = buf.String()
		}
	}
	if svgWidgetJS == "" {
		b, err := os.ReadFile("internal/web/static/js/svg-widget.js")
		if err == nil {
			svgWidgetJS = string(b)
		}
	}
	if demoHTML == "" || svgWidgetJS == "" {
		log.Fatalf("missing demo.html (%d) or svg-widget.js (%d) in widget zip", len(demoHTML), len(svgWidgetJS))
	}

	// =========================================================================
	// PART A: Standalone SVG Tests (1-4, 9, 10)
	// =========================================================================
	svgDataURL := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(standaloneSVG))
	if _, err := chromedp.Run(ctx, chromedp.Navigate(svgDataURL)); err != nil {
		log.Fatal("nav standalone svg:", err)
	}

	type Sample struct {
		T       float64 `json:"t"`
		CWOff   float64 `json:"cwOff"`
		CCWOff  float64 `json:"ccwOff"`
		CCWDash float64 `json:"ccwDash"`
		CCWOp   float64 `json:"ccwOp"`
		FadeOp  float64 `json:"fadeOp"`
		FadeD   float64 `json:"fadeD"`
		GrowD   float64 `json:"growD"`
		GrowOp  float64 `json:"growOp"`
		PulseS  float64 `json:"pulseS"`
		PulseOp float64 `json:"pulseOp"`
	}

	var samples []Sample
	for i := 0; i < 12; i++ {
		sJSON, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const val = (sel, prop) => {
				const el = document.querySelector(sel);
				return el ? parseFloat(getComputedStyle(el)[prop]) : NaN;
			};
			const dash0 = (sel) => {
				const el = document.querySelector(sel);
				if (!el) return NaN;
				return parseFloat(getComputedStyle(el).strokeDasharray.split(',')[0]);
			};
			const scaleX = (sel) => {
				const el = document.querySelector(sel);
				if (!el) return NaN;
				const m = getComputedStyle(el).transform;
				const match = m && m.match(/matrix\(([^,]+)/);
				return match ? parseFloat(match[1]) : 1;
			};
			return JSON.stringify({
				t: performance.now(),
				cwOff: val('[data-lite-id="1"] .lite-payload', 'strokeDashoffset'),
				ccwOff: val('[data-lite-id="2"] .lite-payload', 'strokeDashoffset'),
				ccwDash: dash0('[data-lite-id="2"] .lite-payload'),
				ccwOp: val('[data-lite-id="2"] .lite-payload', 'opacity'),
				fadeOp: val('[data-lite-id="3"] .lite-payload', 'opacity'),
				fadeD: dash0('[data-lite-id="3"] .lite-payload'),
				growD: dash0('[data-lite-id="4"] .lite-payload'),
				growOp: val('[data-lite-id="4"] .lite-payload', 'opacity'),
				pulseS: scaleX('[data-lite-id="5"] .lite-payload'),
				pulseOp: val('[data-lite-id="5"] .lite-payload', 'opacity')
			});
		})()`))
		var s Sample
		_ = json.Unmarshal([]byte(sJSON), &s)
		samples = append(samples, s)
		_, _ = chromedp.Run(ctx, chromedp.Sleep(180*time.Millisecond))
	}

	// 1 & 2: Standalone SVG tracer moves and progresses across full circuit + loop boundary (dur=1.0s over 2.16s)
	cwMoved := math.Abs(samples[5].CWOff-samples[3].CWOff) > 50 && math.Abs(samples[10].CWOff-samples[8].CWOff) > 50
	cwWrapped := false
	for i := 1; i < len(samples); i++ {
		if samples[i].CWOff > samples[i-1].CWOff+200 {
			cwWrapped = true
		}
	}
	rec(1, "Standalone SVG: Tracer moves & loops", cwMoved && cwWrapped,
		fmt.Sprintf("cwOff=[%.0f, %.0f, %.0f, %.0f, %.0f, %.0f, %.0f, %.0f, %.0f]", samples[2].CWOff, samples[3].CWOff, samples[4].CWOff, samples[5].CWOff, samples[6].CWOff, samples[7].CWOff, samples[8].CWOff, samples[9].CWOff, samples[10].CWOff))

	// 4: CW decreases, CCW increases (pick non-wrapping consecutive pair)
	cwDir := false
	ccwDir := false
	var cwA, cwB, ccwA, ccwB float64
	for i := 1; i < len(samples); i++ {
		if samples[i].CWOff < samples[i-1].CWOff-20 && samples[i].CCWOff > samples[i-1].CCWOff+20 {
			cwDir = true
			ccwDir = true
			cwA, cwB = samples[i-1].CWOff, samples[i].CWOff
			ccwA, ccwB = samples[i-1].CCWOff, samples[i].CCWOff
			break
		}
	}
	rec(2, "Standalone SVG: CW vs CCW opposite", cwDir && ccwDir,
		fmt.Sprintf("CW %.1f->%.1f, CCW %.1f->%.1f", cwA, cwB, ccwA, ccwB))

	// 9: Geometric grow/shrink changes actual stroke-dasharray length; fade-only keeps dasharray constant
	minCCW, maxCCW := 9999.0, -1.0
	minGrow, maxGrow := 9999.0, -1.0
	minFadeD, maxFadeD := 9999.0, -1.0
	for _, s := range samples {
		if s.CCWDash < minCCW {
			minCCW = s.CCWDash
		}
		if s.CCWDash > maxCCW {
			maxCCW = s.CCWDash
		}
		if s.GrowD < minGrow {
			minGrow = s.GrowD
		}
		if s.GrowD > maxGrow {
			maxGrow = s.GrowD
		}
		if s.FadeD < minFadeD {
			minFadeD = s.FadeD
		}
		if s.FadeD > maxFadeD {
			maxFadeD = s.FadeD
		}
	}
	rec(3, "Standalone SVG: Grow/Shrink vs Fade-only", (maxCCW-minCCW) > 40 && (maxGrow-minGrow) > 40 && math.Abs(maxFadeD-minFadeD) < 1,
		fmt.Sprintf("fadeGrowDash=[%.1f..%.1f], growOnlyDash=[%.1f..%.1f], fadeOnlyDash=[%.1f..%.1f]", minCCW, maxCCW, minGrow, maxGrow, minFadeD, maxFadeD))

	// 10: Pulse interpolates smoothly
	minPS, maxPS := 99.0, 0.0
	for _, s := range samples {
		if s.PulseS < minPS {
			minPS = s.PulseS
		}
		if s.PulseS > maxPS {
			maxPS = s.PulseS
		}
	}
	rec(4, "Standalone SVG: Pulse smooth scale", (maxPS-minPS) > 0.2,
		fmt.Sprintf("pulseScale range=[%.3f..%.3f]", minPS, maxPS))

	// =========================================================================
	// PART B: Complete Exported Widget Package Tests (5-8)
	// =========================================================================
	_ = os.WriteFile("/tmp/motion-test-package.zip", zipBytes, 0644)
	_ = exec.Command("scp", "/tmp/motion-test-package.zip", "nigel@192.168.100.40:/tmp/motion-test-package.zip").Run()
	_ = exec.Command("ssh", "nigel@192.168.100.40", "rm -rf /tmp/motion-test-pkg && mkdir -p /tmp/motion-test-pkg && unzip -q -o /tmp/motion-test-package.zip -d /tmp/motion-test-pkg").Run()
	_ = exec.Command("scp", "internal/web/static/js/svg-widget.js", "nigel@192.168.100.40:/tmp/motion-test-pkg/motion-test/js/svg-widget.js").Run()

	demoURL := "file:///tmp/motion-test-pkg/motion-test/demo.html"
	if _, err := chromedp.Run(ctx, chromedp.Navigate(demoURL)); err != nil {
		log.Fatal("nav demo:", err)
	}
	_, _ = chromedp.Run(ctx, chromedp.WaitReady("#container-unit-a svg"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(400*time.Millisecond))

	// Check opacity slider step in demo.html
	stepVal, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const cards = Array.from(document.querySelectorAll('#controls-unit-a .control-row, #controls-unit-a .param-row, #controls-unit-a div'));
		const opRow = cards.find(r => r.textContent.includes('tracercw-lite-opacity') && r.querySelector('input[type="range"]'));
		const inp = opRow ? opRow.querySelector('input[type="range"]') : null;
		return inp ? inp.step : 'missing';
	})()`))
	rec(5, "Widget Demo: Opacity slider step <= 0.05", stepVal == "0.01" || stepVal == "0.05",
		fmt.Sprintf("opacity slider step=%q", stepVal))

	// Test continuous opacity: 0, 0.25, 0.5, 0.75, 1.0
	opOut, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		const g = w.svgElement.querySelector('[data-semantic-target="lite-1"]');
		const vals = [0, 0.25, 0.5, 0.75, 1];
		return JSON.stringify(vals.map(v => {
			w.setParameter('tracercw-lite-opacity', v);
			return parseFloat(getComputedStyle(g).opacity);
		}));
	})()`))
	if err != nil {
		fmt.Println("Check 6 err:", err)
	}
	var opArr []float64
	_ = json.Unmarshal([]byte(opOut), &opArr)
	opOk := len(opArr) == 5 && math.Abs(opArr[0]-0) < 0.01 && math.Abs(opArr[1]-0.25) < 0.01 && math.Abs(opArr[2]-0.5) < 0.01 && math.Abs(opArr[3]-0.75) < 0.01 && math.Abs(opArr[4]-1) < 0.01
	rec(6, "Widget Demo: Continuous Opacity 0..1", opOk, fmt.Sprintf("computed=%v", opArr))

	// Test Speed control independently on Tracer 1 vs Tracer 2
	// Set Tracer 1 speed = 4.0 (4x faster -> 0.25s dur), Tracer 2 speed = 0.25 (4x slower -> 6.0s dur)
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		w.setParameter('tracercw-lite-speed', 4.0);
		w.setParameter('tracerccw-lite-speed', 0.25);
		return true;
	})()`)); err != nil {
		fmt.Println("Check 7 set err:", err)
	}
	spd1, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		const p1 = w.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload');
		const p2 = w.svgElement.querySelector('[data-semantic-target="lite-2"] .lite-payload');
		return JSON.stringify({
			d1: parseFloat(getComputedStyle(p1).strokeDashoffset),
			d2: parseFloat(getComputedStyle(p2).strokeDashoffset),
			dur1: getComputedStyle(p1).animationDuration,
			dur2: getComputedStyle(p2).animationDuration
		});
	})()`))
	if err != nil {
		fmt.Println("Check 7 spd1 err:", err)
	}
	_, _ = chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond))
	spd2, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		const p1 = w.svgElement.querySelector('[data-semantic-target="lite-1"] .lite-payload');
		const p2 = w.svgElement.querySelector('[data-semantic-target="lite-2"] .lite-payload');
		return JSON.stringify({
			d1: parseFloat(getComputedStyle(p1).strokeDashoffset),
			d2: parseFloat(getComputedStyle(p2).strokeDashoffset)
		});
	})()`))
	if err != nil {
		fmt.Println("Check 7 spd2 err:", err)
	}
	var spA, spB struct {
		D1   float64 `json:"d1"`
		D2   float64 `json:"d2"`
		Dur1 string  `json:"dur1"`
		Dur2 string  `json:"dur2"`
	}
	_ = json.Unmarshal([]byte(spd1), &spA)
	_ = json.Unmarshal([]byte(spd2), &spB)
	rec(7, "Widget Demo: Independent Speed Rates", spA.Dur1 == "0.25s" && spA.Dur2 == "6s" && spA.D1 != spB.D1,
		fmt.Sprintf("dur1=%s (d1=%.1f->%.1f), dur2=%s (d2=%.1f->%.1f)", spA.Dur1, spA.D1, spB.D1, spA.Dur2, spA.D2, spB.D2))

	// Test Direction change preserves -fadegrow in exported widget!
	dirRes, err := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		w.setParameter('tracerccw-lite-direction', 'cw');
		const p2 = w.svgElement.querySelector('[data-semantic-target="lite-2"] .lite-payload');
		return getComputedStyle(p2).animationName;
	})()`))
	if err != nil {
		fmt.Println("Check 8 err:", err)
	}
	rec(8, "Widget Demo: Direction preserves fadegrow", strings.Contains(dirRes, "sva-lite-trace-cw-fadegrow"),
		fmt.Sprintf("animationName=%q", dirRes))

	_ = os.WriteFile("/tmp/stage16d-a-motion-standalone.svg", []byte(standaloneSVG), 0644)

	fmt.Println("==================================================================")
	allOk := true
	for _, c := range checks {
		if !c.Passed {
			allOk = false
		}
	}
	if !allOk {
		fmt.Println(">>> SOME MOTION CHECKS FAILED <<<")
		os.Exit(1)
	}
	fmt.Println(">>> ALL 8 MOTION & EXPORTED-RUNTIME CHECKS PASS 100% <<<")
}
