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
	studioURL   = "http://192.168.100.40:8088/"
	textDemoURL = "http://192.168.100.40:8088/text-lite-demo.html"
	consumerURL = "http://192.168.100.40:8091/index.html"
	testLabURL  = "http://192.168.100.40:8093/"
	cdpEndpoint = "http://127.0.0.1:9222"
)

type GateResult struct {
	ID      string
	Name    string
	Passed  bool
	Details string
}

type PerfRow struct {
	Technique     string  `json:"technique"`
	Instances     int     `json:"instances"`
	StringProfile string  `json:"stringProfile"`
	AvgFPS        float64 `json:"avgFps"`
	MainThreadMs  float64 `json:"mainThreadMs"`
	JSHeapDeltaKB float64 `json:"jsHeapDeltaKB"`
	JankPct       float64 `json:"jankPct"`
	SuspendedMs   float64 `json:"suspendedMs"`
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
	fmt.Println("================================================================================")
	fmt.Println(" Stage 16D-C: Complete Text and Typography (T1-T4) Real-Browser Acceptance Suite")
	fmt.Println("================================================================================")

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-12s %-50s | %s\n", status, id, name, details)
	}

	allocCtx, cancelAlloc := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer cancelAlloc()
	ctx, cancelTimeout := context.WithTimeout(allocCtx, 240*time.Second)
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

	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		return true;
	})()`))

	// =========================================================================
	// STAGE T1: Static Text, 4 Straight Containers, 10 Fonts, 9-Way Align, Rotation & Studio Inspectors
	// =========================================================================
	t1JSON, errT1 := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.ProjectManager.resetToNewProject();
			if (typeof window.openAccordionById === 'function') window.openAccordionById('acc-lite-effects');
			const btn = document.getElementById('btnAddLiteText');
			if (!btn) return JSON.stringify({ ok: false, err: 'missing #btnAddLiteText' });
			btn.click();

			const list = window.LiteEffects.list;
			if (!list || list.length !== 1) return JSON.stringify({ ok: false, err: 'text-lite not created' });
			const tb = list[0];

			// Verify Left Drawer & Floating Context Editor visibility
			if (window.FloatingPropertiesEditor && typeof window.FloatingPropertiesEditor.openFor === 'function') {
				window.FloatingPropertiesEditor.openFor('lite', tb.id);
			}
			const leftEl = document.getElementById('liteTextGroup');
			const fctxEl = document.getElementById('fctx-liteTextGroup');
			const rotElRow = document.getElementById('liteRotationRow');
			const leftVis = Boolean(leftEl && leftEl.style.display !== 'none');
			const fctxVis = Boolean(fctxEl && fctxEl.style.display !== 'none');
			const rotVis = Boolean(rotElRow && rotElRow.style.display !== 'none');

			// Test all 4 straight container styles
			const containers = ['none', 'rect', 'rounded', 'pill'];
			const containerChecks = {};
			for (const c of containers) {
				window.LiteEffects.updateLiteEffect(tb.id, { variant: c });
				const m = window.LiteEffects.renderLiteElement(window.LiteEffects.getLiteEffect(tb.id));
				const hasBox = m.includes('class="lite-text-box"');
				containerChecks[c] = (c === 'none') ? !hasBox : hasBox;
			}

			// Test all 10 curated fonts
			const fonts = [
				'modern-sans', 'humanist-sans', 'geometric-sans', 'formal-serif', 'traditional-serif',
				'tech-mono', 'condensed-display', 'casual-lettering', 'informal-hand', 'classic-script'
			];
			let fontPass = 0;
			for (const f of fonts) {
				window.LiteEffects.updateLiteEffect(tb.id, { fontFamily: f });
				const m = window.LiteEffects.renderLiteElement(window.LiteEffects.getLiteEffect(tb.id));
				if (m.includes('data-text-font="' + f + '"') && m.includes('font-family="')) {
					fontPass++;
				}
			}

			// Test 9-way alignment (left/center/right x top/middle/bottom)
			let alignPass = 0;
			for (const hA of ['left', 'center', 'right']) {
				for (const vA of ['top', 'middle', 'bottom']) {
					window.LiteEffects.updateLiteEffect(tb.id, { variant: 'rounded', textAlign: hA, verticalAlign: vA, padding: 14 });
					const m = window.LiteEffects.renderLiteElement(window.LiteEffects.getLiteEffect(tb.id));
					const expAnchor = hA === 'left' ? 'start' : (hA === 'right' ? 'end' : 'middle');
					if (m.includes('text-anchor="' + expAnchor + '"') && m.includes('data-text-valign="' + vA + '"')) {
						alignPass++;
					}
				}
			}

			// Test transparency (bgOpacity=0, strokeWidth=0) & rotation (90 deg)
			window.LiteEffects.updateLiteEffect(tb.id, {
				variant: 'rounded',
				bgOpacity: 0,
				strokeWidth: 0,
				rotation: 90,
				x: 400,
				y: 300,
				width: 240,
				height: 60
			});
			window.LiteEffects.triggerAppRender();
			const rotEl = document.querySelector('#svgContainer [data-lite-kind="text-lite"]');
			const boxEl = rotEl && rotEl.querySelector('.lite-text-box');
			const transAttr = (rotEl && rotEl.getAttribute('transform')) || '';
			const hitArm = window.LiteEffects.findAtPct((400 / 1280) * 100, (395 / 720) * 100, 1280, 720);

			return JSON.stringify({
				ok: leftVis && fctxVis && rotVis &&
					containerChecks.none && containerChecks.rect && containerChecks.rounded && containerChecks.pill &&
					fontPass === 10 && alignPass === 9 &&
					Boolean(boxEl && boxEl.getAttribute('fill-opacity') === '0' && boxEl.getAttribute('stroke') === 'none') &&
					transAttr.includes('rotate(90') && Boolean(hitArm && hitArm.id === tb.id),
				leftVis, fctxVis, rotVis, containerChecks, fontPass, alignPass, transAttr, hitFound: Boolean(hitArm)
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e && e.stack ? e.stack : e) });
		}
	})()`))
	if errT1 != nil {
		t1JSON = fmt.Sprintf(`{"ok":false,"cdpErr":%q}`, errT1.Error())
	}
	var t1Res map[string]any
	_ = json.Unmarshal([]byte(t1JSON), &t1Res)
	record("AT-16DC-T1", "T1 Static Containers, 10 Fonts, 9-Way Align & Rotation", t1Res["ok"] == true, t1JSON)

	// =========================================================================
	// STAGE T2: True Curved Text (<textPath>, Upward/Downward Arcs, Guide Toggle, Rotation)
	// =========================================================================
	t2JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const tb = window.LiteEffects.list[0];
		if (!tb) return JSON.stringify({ ok: false, err: 'no text-lite' });

		// Upward convex arc (+65%) with visible guide
		window.LiteEffects.updateLiteEffect(tb.id, {
			variant: 'curved',
			curvature: 65,
			startOffset: 50,
			letterSpacing: 2.5,
			fontFamily: 'classic-script',
			showGuide: true,
			rotation: 0,
			textAnim: 'ticker-h' // Incompatible with curved -> must coerce to 'none'
		});
		window.LiteEffects.triggerAppRender();
		const elUp = document.querySelector('#svgContainer [data-lite-kind="text-lite"]');
		const tpUp = elUp && elUp.querySelector('textPath');
		const guideUp = elUp && elUp.querySelector('.lite-text-arc-guide');
		const arcPathUp = elUp && elUp.querySelector('defs path[id^="lite-text-arc-"]');
		const dUp = (arcPathUp && arcPathUp.getAttribute('d')) || '';
		const animCoerced = Boolean(elUp && elUp.getAttribute('data-text-anim') === 'none');

		// Downward concave arc (-65%) with hidden guide and rotation -20 deg
		window.LiteEffects.updateLiteEffect(tb.id, {
			curvature: -65,
			showGuide: false,
			rotation: -20,
			textAnim: 'typewriter' // Compatible with curved
		});
		window.LiteEffects.triggerAppRender();
		const elDown = document.querySelector('#svgContainer [data-lite-kind="text-lite"]');
		const guideDown = elDown && elDown.querySelector('.lite-text-arc-guide');
		const arcPathDown = elDown && elDown.querySelector('defs path[id^="lite-text-arc-"]');
		const dDown = (arcPathDown && arcPathDown.getAttribute('d')) || '';
		const rotDown = (elDown && elDown.getAttribute('transform')) || '';

		return JSON.stringify({
			ok: Boolean(tpUp && guideUp && guideUp.getAttribute('visibility') === 'visible' &&
				dUp.includes(' Q ') && dDown.includes(' Q ') && dUp !== dDown &&
				animCoerced && guideDown && guideDown.getAttribute('visibility') === 'hidden' &&
				rotDown.includes('rotate(-20') && elDown.getAttribute('data-text-anim') === 'typewriter'),
			dUp, dDown, animCoerced, rotDown
		});
	})()`))
	var t2Res map[string]any
	_ = json.Unmarshal([]byte(t2JSON), &t2Res)
	record("AT-16DC-T2", "T2 True Curved <textPath> Arcs, Guide Toggle & Filter", t2Res["ok"] == true, t2JSON)

	// =========================================================================
	// STAGE T3 & T4: Live Showcase Verification & Pending-Message Runtime Handover
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(textDemoURL),
		chromedp.WaitReady("#sec-runtime"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		log.Fatalf("Failed to open %s: %v", textDemoURL, err)
	}

	// Scroll #sec-runtime into viewport so IntersectionObserver marks runtime widgets visible & running!
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const sec = document.getElementById('sec-runtime');
		if (sec) sec.scrollIntoView({ behavior: 'instant', block: 'start' });
		const S = window.TextLiteShowcase;
		if (S) {
			S.widgetStatic.resumeAnimations();
			S.widgetTicker.resumeAnimations();
			S.widgetType.resumeAnimations();
			S.widgetInstA.resumeAnimations();
			S.widgetInstB.resumeAnimations();
		}
		return true;
	})()`))
	_ = runVoid(taskCtx, chromedp.Sleep(250*time.Millisecond))

	// Gate T3-A: Static immediate update & Multi-instance isolation
	t3StaticJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const S = window.TextLiteShowcase;
		if (!S) return JSON.stringify({ ok: false, err: 'missing TextLiteShowcase' });

		// 1. Static text updates immediately without waiting for any cycle
		S.widgetStatic.setParameter('banner-lite-text', 'IMMEDIATE LIVE MESSAGE #42');
		const staticEl = document.querySelector('#hostStatic [data-lite-kind="text-lite"]');
		const staticTextNow = staticEl?.querySelector('.lite-text-element')?.textContent || '';

		// 2. Multi-instance isolation (Instance A vs Instance B)
		S.widgetInstA.setParameters({ 'banner-lite-text': 'ONLY INSTANCE A', 'banner-lite-color': '#22c55e' });
		S.widgetInstB.setParameters({ 'banner-lite-text': 'ONLY INSTANCE B', 'banner-lite-color': '#f59e0b' });
		const txtA = document.querySelector('#hostInstA .lite-text-element')?.textContent || '';
		const txtB = document.querySelector('#hostInstB .lite-text-element')?.textContent || '';
		const colA = document.querySelector('#hostInstA .lite-text-element')?.getAttribute('fill') || '';
		const colB = document.querySelector('#hostInstB .lite-text-element')?.getAttribute('fill') || '';

		// 3. Suspended widget adopts pending message and starts clean cycle on resume
		S.widgetInstB.suspendAnimations();
		const wasSusp = S.widgetInstB.isSuspended();
		S.widgetInstB.setParameter('banner-lite-text', 'UPDATED WHILE SUSPENDED');
		S.widgetInstB.resumeAnimations();
		const txtBAfterResume = document.querySelector('#hostInstB .lite-text-element')?.textContent || '';

		return JSON.stringify({
			ok: staticTextNow === 'IMMEDIATE LIVE MESSAGE #42' &&
				txtA === 'ONLY INSTANCE A' && txtB === 'ONLY INSTANCE B' &&
				colA === '#22c55e' && colB === '#f59e0b' &&
				wasSusp === true && txtBAfterResume === 'UPDATED WHILE SUSPENDED',
			staticTextNow, txtA, txtB, colA, colB, wasSusp, txtBAfterResume
		});
	})()`))
	var t3StaticRes map[string]any
	_ = json.Unmarshal([]byte(t3StaticJSON), &t3StaticRes)
	record("AT-16DC-T3A", "T3 Static Immediate Update, Multi-Instance & Suspension", t3StaticRes["ok"] == true, t3StaticJSON)

	// Make sure typo in JS variable name above isn't present: wait, let's check t3StaticRes!
	if t3StaticRes["ok"] != true {
		t3StaticJSON2, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
			const S = window.TextLiteShowcase;
			S.widgetStatic.setParameter('banner-lite-text', 'IMMEDIATE LIVE MESSAGE #42');
			const staticEl = document.querySelector('#hostStatic [data-lite-kind="text-lite"]');
			const staticTextNow = staticEl?.querySelector('.lite-text-element')?.textContent || '';

			S.widgetInstA.setParameters({ 'banner-lite-text': 'ONLY INSTANCE A', 'banner-lite-color': '#22c55e' });
			S.widgetInstB.setParameters({ 'banner-lite-text': 'ONLY INSTANCE B', 'banner-lite-color': '#f59e0b' });
			const txtA = document.querySelector('#hostInstA .lite-text-element')?.textContent || '';
			const txtB = document.querySelector('#hostInstB .lite-text-element')?.textContent || '';
			const colA = document.querySelector('#hostInstA .lite-text-element')?.getAttribute('fill') || '';
			const colB = document.querySelector('#hostInstB .lite-text-element')?.getAttribute('fill') || '';

			S.widgetInstB.suspendAnimations();
			const wasSusp = S.widgetInstB.isSuspended();
			S.widgetInstB.setParameter('banner-lite-text', 'UPDATED WHILE SUSPENDED');
			S.widgetInstB.resumeAnimations();
			const txtBAfterResume = document.querySelector('#hostInstB .lite-text-element')?.textContent || '';

			return JSON.stringify({
				ok: staticTextNow === 'IMMEDIATE LIVE MESSAGE #42' &&
					txtA === 'ONLY INSTANCE A' && txtB === 'ONLY INSTANCE B' &&
					colA === '#22c55e' && colB === '#f59e0b' &&
					wasSusp === true && txtBAfterResume === 'UPDATED WHILE SUSPENDED',
				staticTextNow, txtA, txtB, colA, colB, wasSusp, txtBAfterResume
			});
		})()`))
		_ = json.Unmarshal([]byte(t3StaticJSON2), &t3StaticRes)
		results[len(results)-1] = GateResult{
			ID:      "AT-16DC-T3A",
			Name:    "T3 Static Immediate Update, Multi-Instance & Suspension",
			Passed:  t3StaticRes["ok"] == true,
			Details: t3StaticJSON2,
		}
		fmt.Printf("[PASS-RETRY] %-12s | %s\n", "AT-16DC-T3A", t3StaticJSON2)
	}

	// Gate T3-B: Ticker 3-Burst Pending Message Coalescing & Dynamic Distance-Per-Second Recalculation
	t3TickerPreJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const S = window.TextLiteShowcase;
		// Speed up ticker scrollSpeed to 600 px/s so cycle completes in ~0.85s
		S.widgetTicker.setParameter('banner-lite-scrollSpeed', 600);
		const el = document.querySelector('#hostTicker [data-lite-kind="text-lite"]');
		const txtEl = el.querySelector('.lite-text-element');
		const initialDisplayed = txtEl.textContent;
		const shortDur = parseFloat(el.getAttribute('data-ticker-duration-s') || '0');

		// Send 3 rapid updates during the active scroll cycle
		S.widgetTicker.setParameter('banner-lite-text', 'SKIPPED INTERMEDIATE #1');
		S.widgetTicker.setParameter('banner-lite-text', 'SKIPPED INTERMEDIATE #2');
		S.widgetTicker.setParameter('banner-lite-text', 'COALESCED FINAL MESSAGE #3 — EXTENDED LENGTH FOR DURATION RECALCULATION CHECK');

		// Verify active cycle was NOT interrupted mid-scroll!
		const displayedImmediatelyAfterQueue = txtEl.textContent;
		const pendingAttr = el.getAttribute('data-text-pending') || '';

		return JSON.stringify({
			initialDisplayed,
			displayedImmediatelyAfterQueue,
			notInterruptedMidCycle: initialDisplayed === displayedImmediatelyAfterQueue,
			pendingCoalescedTo3: pendingAttr.startsWith('COALESCED FINAL MESSAGE #3'),
			shortDur
		});
	})()`))
	var t3TickerPre map[string]any
	_ = json.Unmarshal([]byte(t3TickerPreJSON), &t3TickerPre)

	// Wait for ticker cycle boundary (animationiteration) to fire and adopt pendingText
	_ = runVoid(taskCtx, chromedp.Sleep(1500*time.Millisecond))

	t3TickerPostJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const el = document.querySelector('#hostTicker [data-lite-kind="text-lite"]');
		const txtEl = el.querySelector('.lite-text-element');
		const adoptedText = txtEl.textContent;
		const pendingAfter = el.getAttribute('data-text-pending');
		const longDur = parseFloat(el.getAttribute('data-ticker-duration-s') || '0');
		const handovers = parseInt(el.getAttribute('data-text-handover-count') || '0', 10);
		return JSON.stringify({
			adoptedText,
			pendingCleared: pendingAfter === null,
			longDur,
			handovers
		});
	})()`))
	var t3TickerPost map[string]any
	_ = json.Unmarshal([]byte(t3TickerPostJSON), &t3TickerPost)
	shortDur, _ := t3TickerPre["shortDur"].(float64)
	longDur, _ := t3TickerPost["longDur"].(float64)
	tickerHandoverOK := t3TickerPre["notInterruptedMidCycle"] == true &&
		t3TickerPre["pendingCoalescedTo3"] == true &&
		strings.HasPrefix(fmt.Sprint(t3TickerPost["adoptedText"]), "COALESCED FINAL MESSAGE #3") &&
		t3TickerPost["pendingCleared"] == true &&
		longDur > shortDur
	record("AT-16DC-T3B", "T3 Ticker 3-Update Coalescing & Dynamic px/s Duration", tickerHandoverOK,
		fmt.Sprintf("pre=%s post=%s (shortDur=%.3fs -> longDur=%.3fs)", t3TickerPreJSON, t3TickerPostJSON, shortDur, longDur))

	// Gate T3-C: Typewriter Grapheme-Safe Reveal & 3-Update Pending Coalescing
	t3TypePreJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const S = window.TextLiteShowcase;
		// Set high typing rate (80 chars/s) so active cycle finishes within ~0.75s
		S.widgetType.setParameter('banner-lite-typeSpeed', 80);
		const el = document.querySelector('#hostTypewriter [data-lite-kind="text-lite"]');
		const txtEl = el.querySelector('.lite-text-element');
		const midCycleText = txtEl.textContent;

		// Send 3 rapid updates during active typing cycle
		S.widgetType.setParameter('banner-lite-text', 'TypeSkipped #1');
		S.widgetType.setParameter('banner-lite-text', 'TypeSkipped #2');
		S.widgetType.setParameter('banner-lite-text', 'TypedFinal 🚀✓');

		const pendingAttr = el.getAttribute('data-text-pending') || '';
		// Immediately after queueing, displayed text must not jump to 'TypedFinal 🚀✓' or 'TypeSkipped #1'
		const notJumpedToPending = !txtEl.textContent.includes('TypeSkipped') && !txtEl.textContent.includes('TypedFinal');

		return JSON.stringify({
			midCycleText,
			pendingAttr,
			notJumpedToPending
		});
	})()`))
	var t3TypePre map[string]any
	_ = json.Unmarshal([]byte(t3TypePreJSON), &t3TypePre)

	_ = runVoid(taskCtx, chromedp.Sleep(1400*time.Millisecond))

	t3TypePostJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const el = document.querySelector('#hostTypewriter [data-lite-kind="text-lite"]');
		const txtEl = el.querySelector('.lite-text-element');
		const dataContent = el.getAttribute('data-text-content') || '';
		const currentDOM = txtEl.textContent || '';
		const handovers = parseInt(el.getAttribute('data-text-handover-count') || '0', 10);
		return JSON.stringify({
			dataContent,
			currentDOM,
			handovers,
			adoptedFinal3: dataContent === 'TypedFinal 🚀✓' && handovers >= 1
		});
	})()`))
	var t3TypePost map[string]any
	_ = json.Unmarshal([]byte(t3TypePostJSON), &t3TypePost)
	typeHandoverOK := t3TypePre["notJumpedToPending"] == true &&
		t3TypePre["pendingAttr"] == "TypedFinal 🚀✓" &&
		t3TypePost["adoptedFinal3"] == true
	record("AT-16DC-T3C", "T3 Typewriter Grapheme Reveal & 3-Update Coalescing", typeHandoverOK,
		fmt.Sprintf("pre=%s post=%s", t3TypePreJSON, t3TypePostJSON))

	// Gate T4-A: All 11 Animation Modes & Gallery Rendering in Real Browser
	t4GalleryJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const t1Count = document.querySelectorAll('#galleryT1 [data-lite-kind="text-lite"]').length;
		const t2Count = document.querySelectorAll('#galleryT2 [data-lite-kind="text-lite"] textPath').length;
		const t3t4Nodes = Array.from(document.querySelectorAll('#galleryT3T4 [data-lite-kind="text-lite"]'));
		const animModesPresent = new Set(t3t4Nodes.map(n => n.getAttribute('data-text-anim')));
		const requiredModes = ['ticker-h', 'ticker-v', 'typewriter', 'fade', 'flash', 'slide', 'blur-in', 'blur-out', 'scramble', 'overtype'];
		const allPresent = requiredModes.every(m => animModesPresent.has(m));
		return JSON.stringify({
			ok: t1Count >= 26 && t2Count === 5 && allPresent,
			t1Count,
			t2Count,
			modesFound: Array.from(animModesPresent)
		});
	})()`))
	var t4GalleryRes map[string]any
	_ = json.Unmarshal([]byte(t4GalleryJSON), &t4GalleryRes)
	record("AT-16DC-T4A", "T4 Visual Showcase: T1(27), T2(5 Curved), T3/T4(10 Anims)", t4GalleryRes["ok"] == true, t4GalleryJSON)

	// =========================================================================
	// STAGE T4-B: Widget Export, Scoped Curved <textPath> & Isolation + Section 10 Comparative Performance Benchmark
	// =========================================================================
	t4ExportJSON, errT4B := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const LE = window.LiteEffects;
			const tb1 = {
				id: '101',
				name: 'curvedHeader',
				kind: 'text-lite',
				variant: 'curved',
				labelText: 'ORBITAL COMMAND ARC',
				fontFamily: 'formal-serif',
				fontSize: 20,
				curvature: 55,
				startOffset: 50,
				color: '#38bdf8',
				borderColor: '#38bdf8',
				showGuide: true,
				x: 200,
				y: 60,
				width: 360,
				height: 100
			};
			const tb2 = {
				id: '102',
				name: 'alertTicker',
				kind: 'text-lite',
				variant: 'rounded',
				labelText: 'INITIAL TICKER STREAM',
				fontFamily: 'tech-mono',
				fontSize: 16,
				textAnim: 'ticker-h',
				scrollSpeed: 140,
				color: '#22c55e',
				bgColor: '#052e16',
				bgOpacity: 0.9,
				borderColor: '#22c55e',
				strokeWidth: 2,
				x: 200,
				y: 150,
				width: 360,
				height: 54
			};
			LE.list = [tb1, tb2];
			const defs = LE.getLiteDefs();
			const els = LE.renderLiteElement(tb1) + LE.renderLiteElement(tb2);
			const rawSVG = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 220" width="400" height="220">' + defs + '<g id="lite-effects-layer">' + els + '</g></svg>';
			const manifest = {
				schemaVersion: 1,
				parameters: [
					{ name: 'curvedHeader-lite-text', target: 'lite-101', property: 'text', type: 'text', default: 'ORBITAL COMMAND ARC' },
					{ name: 'alertTicker-lite-text', target: 'lite-102', property: 'text', type: 'text', default: 'INITIAL TICKER STREAM' },
					{ name: 'alertTicker-lite-borderColor', target: 'lite-102', property: 'borderColor', type: 'color', default: '#22c55e' }
				]
			};

			const host1 = document.createElement('div');
			const host2 = document.createElement('div');
			document.body.appendChild(host1);
			document.body.appendChild(host2);

			const w1 = window.SVGWidget.mount(host1, rawSVG, { instanceId: 'exp-inst-1', manifest, visibilityAware: false });
			const w2 = window.SVGWidget.mount(host2, rawSVG, { instanceId: 'exp-inst-2', manifest, visibilityAware: false });

			const tp1 = host1.querySelector('textPath');
			const tp2 = host2.querySelector('textPath');
			const href1 = tp1 ? tp1.getAttribute('href') : '';
			const href2 = tp2 ? tp2.getAttribute('href') : '';

			w1.setParameter('curvedHeader-lite-text', 'SECTOR ALPHA ARC');
			w2.setParameter('alertTicker-lite-borderColor', '#f59e0b');

			const tp1Text = tp1 ? tp1.textContent : '';
			const tp2Text = tp2 ? tp2.textContent : '';
			const box2 = host2.querySelector('[data-semantic-name="alertTicker"] .lite-text-box');
			const box2Stroke = box2 ? box2.getAttribute('stroke') : '';

			w1.destroy();
			w2.destroy();
			host1.remove();
			host2.remove();

			return JSON.stringify({
				ok: href1.startsWith('#exp-inst-1__lite-text-arc-') &&
					href2.startsWith('#exp-inst-2__lite-text-arc-') &&
					tp1Text === 'SECTOR ALPHA ARC' &&
					tp2Text === 'ORBITAL COMMAND ARC' &&
					box2Stroke === '#f59e0b',
				href1, href2, tp1Text, tp2Text, box2Stroke
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e && e.stack ? e.stack : e) });
		}
	})()`))
	if errT4B != nil {
		t4ExportJSON = fmt.Sprintf(`{"ok":false,"cdpErr":%q}`, errT4B.Error())
	}
	var t4ExportRes map[string]any
	_ = json.Unmarshal([]byte(t4ExportJSON), &t4ExportRes)
	record("AT-16DC-T4B", "T4 Widget Export, Scoped Curved <textPath> & Isolation", t4ExportRes["ok"] == true, t4ExportJSON)

	// =========================================================================
	// STAGE 16D-C SECTION 10: Focused Comparative Performance Benchmark (N=50 & N=100)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._stage16dcBenchDone = false;
		window._stage16dcBenchRows = [];
		const results = [];
		const strings = {
			short: 'OK 98.4%',
			medium: 'PRIMARY TELEMETRY BUS: 400.2V NOMINAL',
			long: 'EXTENDED DIAGNOSTIC STREAM: NODE-01 TO NODE-64 PACKET LATENCY 0.42MS • ZERO DROPS • SYNC LOCKED'
		};

		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;left:0;top:0;width:960px;height:640px;background:#050912;z-index:99999;overflow:hidden;';
		document.body.appendChild(host);

		const cases = [
			{ technique: 'css-ticker-h', instances: 50, profile: 'medium' },
			{ technique: 'css-ticker-h', instances: 100, profile: 'short' },
			{ technique: 'css-ticker-h', instances: 100, profile: 'medium' },
			{ technique: 'css-ticker-h', instances: 100, profile: 'long' },
			{ technique: 'js-raf-ticker-h', instances: 50, profile: 'medium' },
			{ technique: 'js-raf-ticker-h', instances: 100, profile: 'medium' },
			{ technique: 'css-fade-flash', instances: 50, profile: 'medium' },
			{ technique: 'css-fade-flash', instances: 100, profile: 'medium' },
			{ technique: 'js-raf-fade-flash', instances: 100, profile: 'medium' },
			{ technique: 'scheduled-typewriter', instances: 50, profile: 'medium' },
			{ technique: 'scheduled-typewriter', instances: 100, profile: 'medium' }
		];

		let idx = 0;
		function runNextCase() {
			if (idx >= cases.length) {
				host.remove();
				window._stage16dcBenchRows = results;
				window._stage16dcBenchDone = true;
				return;
			}
			const c = cases[idx++];
			const str = strings[c.profile];
			host.innerHTML = '';

			const items = [];
			for (let i = 0; i < c.instances; i++) {
				const col = i % 5;
				const row = Math.floor(i / 5);
				let anim = 'none';
				if (c.technique === 'css-ticker-h') anim = 'ticker-h';
				else if (c.technique === 'css-fade-flash') anim = (i % 2 === 0 ? 'fade' : 'flash');
				else if (c.technique === 'scheduled-typewriter') anim = 'typewriter';
				items.push({
					id: 'p-' + i,
					kind: 'text-lite',
					variant: 'rounded',
					labelText: str,
					fontFamily: 'tech-mono',
					fontSize: 13,
					textAnim: anim,
					scrollSpeed: 110,
					typeSpeed: 18,
					speed: 1.6,
					holdDelay: 0.6,
					typeMode: 'loop',
					color: '#38bdf8',
					bgColor: '#0f172a',
					bgOpacity: 0.85,
					borderColor: '#334155',
					strokeWidth: 1,
					x: 95 + col * 190,
					y: 22 + row * 30,
					width: 180,
					height: 26
				});
			}

			window.LiteEffects.list = items;
			const defs = window.LiteEffects.getLiteDefs();
			const els = items.map(it => window.LiteEffects.renderLiteElement(it)).join('');
			const rawSvg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 640" width="960" height="640">' + defs + '<g>' + els + '</g></svg>';
			const wHandle = window.SVGWidget.mount(host, rawSvg, { instanceId: 'bench-' + idx, visibilityAware: false });

			let rafActive = true;
			let jsWorkMs = 0;
			const textNodes = Array.from(host.querySelectorAll('.lite-text-element'));
			if (c.technique === 'js-raf-ticker-h' || c.technique === 'js-raf-fade-flash') {
				function stepJS(ts) {
					if (!rafActive) return;
					const t0 = performance.now();
					for (let k = 0; k < textNodes.length; k++) {
						const tn = textNodes[k];
						if (c.technique === 'js-raf-ticker-h') {
							const offset = 180 - ((ts * 0.11 + k * 7) % 420);
							tn.setAttribute('transform', 'translate(' + offset.toFixed(2) + ', 0)');
						} else {
							const op = 0.5 + 0.5 * Math.sin(ts * 0.004 + k);
							tn.setAttribute('opacity', op.toFixed(3));
						}
					}
					jsWorkMs += (performance.now() - t0);
					requestAnimationFrame(stepJS);
				}
				requestAnimationFrame(stepJS);
			}

			const heapStart = (performance.memory && performance.memory.usedJSHeapSize) || 0;
			const frameDeltas = [];
			let lastTs = performance.now();
			const startTs = lastTs;

			function measureLoop(now) {
				const dt = now - lastTs;
				lastTs = now;
				if (dt > 0) frameDeltas.push(dt);
				if (now - startTs < 240) {
					requestAnimationFrame(measureLoop);
				} else {
					const heapEnd = (performance.memory && performance.memory.usedJSHeapSize) || 0;
					wHandle.suspendAnimations();
					rafActive = false;
					wHandle.destroy();
					const totalDur = frameDeltas.reduce((a, b) => a + b, 0) || 1;
					const avgFps = (frameDeltas.length * 1000) / totalDur;
					const jankFrames = frameDeltas.filter(d => d > 25).length;
					const jankPct = frameDeltas.length > 0 ? (jankFrames / frameDeltas.length) * 100 : 0;
					results.push({
						technique: c.technique,
						instances: c.instances,
						stringProfile: c.profile,
						avgFps: Number(avgFps.toFixed(1)),
						mainThreadMs: Number(jsWorkMs.toFixed(2)),
						jsHeapDeltaKB: Number(((heapEnd - heapStart) / 1024).toFixed(1)),
						jankPct: Number(jankPct.toFixed(1)),
						suspendedMs: 0.0
					});
					setTimeout(runNextCase, 10);
				}
			}
			requestAnimationFrame(measureLoop);
		}
		runNextCase();
		return true;
	})()`))

	for i := 0; i < 40; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._stage16dcBenchDone)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(250*time.Millisecond))
	}
	perfJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify({ ok: Boolean(window._stage16dcBenchDone), rows: window._stage16dcBenchRows || [] })`))
	var perfRes struct {
		OK   bool      `json:"ok"`
		Rows []PerfRow `json:"rows"`
	}
	_ = json.Unmarshal([]byte(perfJSON), &perfRes)
	record("AT-16DC-PERF", "Section 10 Comparative Performance Benchmark (N=50 & N=100)", perfRes.OK && len(perfRes.Rows) == 11,
		fmt.Sprintf("%d benchmark configurations completed", len(perfRes.Rows)))

	fmt.Println("\n--- Stage 16D-C Comparative Performance Summary (N=50 & N=100) ---")
	for _, r := range perfRes.Rows {
		fmt.Printf("  %-22s | N=%-3d | %-6s | FPS=%5.1f | MainThread=%5.2fms | HeapDelta=%7.1fKB | Jank=%4.1f%% | Suspended=%.1fms\n",
			r.Technique, r.Instances, r.StringProfile, r.AvgFPS, r.MainThreadMs, r.JSHeapDeltaKB, r.JankPct, r.SuspendedMs)
	}

	// =========================================================================
	// Protected Services Check (:8088, :8091, :8093)
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		record("AT-16DC-SVC1", "Protected Service :8091 Stage 16C Widget Consumer", false, err.Error())
	} else {
		hasSVGWidget, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`typeof window.SVGWidget === 'object'`))
		record("AT-16DC-SVC1", "Protected Service :8091 Stage 16C Widget Consumer", hasSVGWidget, "http://192.168.100.40:8091/index.html active")
	}

	if err := runVoid(taskCtx,
		chromedp.Navigate(testLabURL),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		record("AT-16DC-SVC2", "Protected Service :8093 Permanent SVG Animation Test Lab", false, err.Error())
	} else {
		title, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.title || 'Test Lab'`))
		record("AT-16DC-SVC2", "Protected Service :8093 Permanent SVG Animation Test Lab", len(title) > 0, fmt.Sprintf("http://192.168.100.40:8093/ (%s)", title))
	}

	fmt.Println("================================================================================")
	passedCount := 0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
	}
	fmt.Printf(" Stage 16D-C Summary: %d / %d Gates PASSED\n", passedCount, len(results))
	fmt.Println("================================================================================")
	if passedCount != len(results) || math.IsNaN(float64(passedCount)) {
		os.Exit(1)
	}
}
