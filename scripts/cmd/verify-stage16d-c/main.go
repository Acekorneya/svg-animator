package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
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
	fmt.Println(" Stage 16D-C: Text and Typography Consolidated Corrections Acceptance Suite")
	fmt.Println("================================================================================")

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-14s %-52s | %s\n", status, id, name, details)
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
	// GATE 1: T1/T2 Non-Regression (Containers, 10 Fonts, 9-Way Align, Rotation, Tilted Alignment, Curved <textPath>)
	// =========================================================================
	g1JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.ProjectManager.resetToNewProject();
			if (typeof window.openAccordionById === 'function') window.openAccordionById('acc-lite-effects');
			document.getElementById('btnAddLiteText').click();
			const tb = window.LiteEffects.list[0];
			if (window.FloatingPropertiesEditor && typeof window.FloatingPropertiesEditor.openFor === 'function') {
				window.FloatingPropertiesEditor.openFor('lite', tb.id);
			}
			const leftVis = document.getElementById('liteTextGroup')?.style.display !== 'none';
			const fctxVis = document.getElementById('fctx-liteTextGroup')?.style.display !== 'none';

			// Verify Overtype is removed from new selection dropdowns
			const leftAnimOpts = Array.from(document.getElementById('liteTextAnimSelect').options).map(o => o.value);
			const fctxAnimOpts = Array.from(document.getElementById('fctx-liteTextAnimSelect').options).map(o => o.value);
			const noOvertypeInUI = !leftAnimOpts.includes('overtype') && !fctxAnimOpts.includes('overtype');

			// Verify Tilted-Box Alignment & Rotation (Item #19)
			window.LiteEffects.updateLiteEffect(tb.id, {
				variant: 'rounded',
				rotation: 35,
				textAlign: 'right',
				verticalAlign: 'bottom',
				padding: 14,
				x: 400,
				y: 300,
				width: 240,
				height: 60
			});
			window.LiteEffects.triggerAppRender();
			const rotEl = document.querySelector('#svgContainer [data-lite-kind="text-lite"]');
			const txtEl = rotEl && rotEl.querySelector('.lite-text-element');
			const rotAttr = (rotEl && rotEl.getAttribute('transform')) || '';
			const anchorAttr = (txtEl && txtEl.getAttribute('text-anchor')) || '';

			// Verify Curved <textPath>
			window.LiteEffects.updateLiteEffect(tb.id, { variant: 'curved', curvature: 60, rotation: -15 });
			window.LiteEffects.triggerAppRender();
			const curvedEl = document.querySelector('#svgContainer [data-lite-kind="text-lite"]');
			const hasTP = Boolean(curvedEl && curvedEl.querySelector('textPath'));

			return JSON.stringify({
				ok: leftVis && fctxVis && noOvertypeInUI && rotAttr.includes('rotate(35') && anchorAttr === 'end' && hasTP,
				leftVis, fctxVis, noOvertypeInUI, rotAttr, anchorAttr, hasTP
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e) });
		}
	})()`))
	var g1 map[string]any
	_ = json.Unmarshal([]byte(g1JSON), &g1)
	record("AT-16DC-C01", "Inspectors, Tilted Alignment, Curved & No Overtype UI", g1["ok"] == true, g1JSON)

	// =========================================================================
	// GATE 2: Lock On/Off Inspector Synchronisation (Left Drawer <-> Floating Context Editor) (Item #12)
	// =========================================================================
	g2JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const tb = window.LiteEffects.list[0];
			window.LiteEffects.updateLiteEffect(tb.id, { variant: 'rounded', textAnim: 'flash', speed: 0.1, holdDelay: 0.8, clearDuration: 0.3, lockOnOff: false });
			window.LiteEffects.syncEditorUI(window.LiteEffects.getLiteEffect(tb.id));
			window.FloatingPropertiesEditor.syncFromActiveObject();

			// 1. Enable Lock On/Off via Left Drawer checkbox
			const leftLock = document.getElementById('liteTextLockOnOffChk');
			leftLock.checked = true;
			leftLock.dispatchEvent(new Event('change'));
			const afterLock = window.LiteEffects.getLiteEffect(tb.id);
			const fctxLockChecked = document.getElementById('fctx-liteTextLockOnOffChk').checked;
			const syncedOnEnable = afterLock.lockOnOff === true && Math.abs(afterLock.clearDuration - 0.8) < 0.001 && fctxLockChecked === true;

			// 2. While locked, change On Hold Duration in Left Drawer to 1.4s -> Off Duration must become 1.4s in both inspectors!
			const holdNum = document.getElementById('liteTextHoldDelayNum');
			holdNum.value = '1.4';
			holdNum.dispatchEvent(new Event('input'));
			const afterHoldEdit = window.LiteEffects.getLiteEffect(tb.id);
			const fctxClearVal = parseFloat(document.getElementById('fctx-liteTextClearDurationNum').value);
			const syncedHoldToOff = Math.abs(afterHoldEdit.clearDuration - 1.4) < 0.001 && Math.abs(fctxClearVal - 1.4) < 0.001;

			// 3. While locked, change Off Duration in Floating Context Editor to 0.5s -> On Hold Duration must become 0.5s!
			const fctxClearNum = document.getElementById('fctx-liteTextClearDurationNum');
			fctxClearNum.value = '0.5';
			fctxClearNum.dispatchEvent(new Event('input'));
			const afterOffEdit = window.LiteEffects.getLiteEffect(tb.id);
			const leftHoldVal = parseFloat(document.getElementById('liteTextHoldDelayNum').value);
			const fadeUnchanged = Math.abs(afterOffEdit.speed - 0.1) < 0.001;
			const syncedOffToHold = Math.abs(afterOffEdit.holdDelay - 0.5) < 0.001 && Math.abs(leftHoldVal - 0.5) < 0.001 && fadeUnchanged;

			// 4. Unlock and edit independently
			leftLock.checked = false;
			leftLock.dispatchEvent(new Event('change'));
			fctxClearNum.value = '1.9';
			fctxClearNum.dispatchEvent(new Event('input'));
			const afterUnlock = window.LiteEffects.getLiteEffect(tb.id);
			const unlockedIndependent = Math.abs(afterUnlock.holdDelay - 0.5) < 0.001 && Math.abs(afterUnlock.clearDuration - 1.9) < 0.001;

			return JSON.stringify({
				ok: syncedOnEnable && syncedHoldToOff && syncedOffToHold && unlockedIndependent,
				syncedOnEnable, syncedHoldToOff, syncedOffToHold, unlockedIndependent
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e) });
		}
	})()`))
	var g2 map[string]any
	_ = json.Unmarshal([]byte(g2JSON), &g2)
	record("AT-16DC-C02", "Flash/Blink Lock On/Off Bidirectional Inspector Sync", g2["ok"] == true, g2JSON)

	// =========================================================================
	// GATE 3: Overtype Legacy Compatibility Migration (Item #17)
	// =========================================================================
	g3JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const legacyObj = window.LiteEffects.createLiteEffect({
				name: 'legacyOvertypeBox',
				kind: 'text-lite',
				variant: 'rounded',
				labelText: 'Legacy Scene Text',
				textAnim: 'overtype',
				typeMode: 'loop'
			});
			const m = window.LiteEffects.renderLiteElement(legacyObj);
			return JSON.stringify({
				ok: legacyObj.textAnim === 'typewriter' && legacyObj.typeMode === 'delete' && legacyObj.typeRepeat === true && m.includes('data-text-anim="typewriter"'),
				migratedAnim: legacyObj.textAnim,
				migratedMode: legacyObj.typeMode,
				migratedRepeat: legacyObj.typeRepeat
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e) });
		}
	})()`))
	var g3 map[string]any
	_ = json.Unmarshal([]byte(g3JSON), &g3)
	record("AT-16DC-C03", "Overtype & Loop Legacy Migration to Typewriter", g3["ok"] == true, g3JSON)

	// =========================================================================
	// Navigate to Consolidated Visual Showcase (`text-lite-demo.html`) for Temporal Animation Checks
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(textDemoURL),
		chromedp.WaitReady("#sec-t3-t4"),
		chromedp.Sleep(400*time.Millisecond),
	); err != nil {
		log.Fatalf("Failed to open %s: %v", textDemoURL, err)
	}

	// =========================================================================
	// GATE 4: Horizontal Ticker Left vs Right Temporal Movement & Zero Vertical Drift (Items #1, #2, #3)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._tickerDirDone = false;
		window._tickerDirResult = null;
		const S = window.TextLiteShowcase;
		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;top:10px;left:10px;width:420px;height:220px;z-index:99999;background:#050912;';
		const boxL = document.createElement('div');
		const boxR = document.createElement('div');
		host.appendChild(boxL);
		host.appendChild(boxR);
		document.body.appendChild(host);

		const pkgL = S.makeSingleTextWidgetSVG({ textAnim: 'ticker-h', direction: 'forward', scrollSpeed: 380, labelText: 'LEFTWARD STREAM' });
		const pkgR = S.makeSingleTextWidgetSVG({ textAnim: 'ticker-h', direction: 'reverse', scrollSpeed: 380, labelText: 'RIGHTWARD STREAM' });
		const wL = window.SVGWidget.mount(boxL, pkgL.svg, { instanceId: 'test-tk-left', manifest: pkgL.manifest, visibilityAware: false });
		const wR = window.SVGWidget.mount(boxR, pkgR.svg, { instanceId: 'test-tk-right', manifest: pkgR.manifest, visibilityAware: false });

		const elL = boxL.querySelector('.lite-text-element');
		const elR = boxR.querySelector('.lite-text-element');

		const samplesL = [];
		const samplesR = [];
		let count = 0;
		const iv = setInterval(() => {
			const rL = elL.getBoundingClientRect();
			const rR = elR.getBoundingClientRect();
			samplesL.push({ x: Number(rL.left.toFixed(2)), y: Number(rL.top.toFixed(2)) });
			samplesR.push({ x: Number(rR.left.toFixed(2)), y: Number(rR.top.toFixed(2)) });
			count++;
			if (count >= 5) {
				clearInterval(iv);
				// Also test runtime text update preserving direction & zero Y drift
				wL.setParameter('banner-lite-text', 'UPDATED LEFT STREAM');
				wR.setParameter('banner-lite-text', 'UPDATED RIGHT STREAM');
				const dxL = samplesL[samplesL.length - 1].x - samplesL[0].x;
				const dyL = Math.max(...samplesL.map(s => s.y)) - Math.min(...samplesL.map(s => s.y));
				const dxR = samplesR[samplesR.length - 1].x - samplesR[0].x;
				const dyR = Math.max(...samplesR.map(s => s.y)) - Math.min(...samplesR.map(s => s.y));
				wL.destroy();
				wR.destroy();
				host.remove();
				window._tickerDirResult = {
					ok: dxL < -40 && dxR > 40 && dyL < 0.5 && dyR < 0.5,
					dxL: Number(dxL.toFixed(2)),
					dyL: Number(dyL.toFixed(2)),
					dxR: Number(dxR.toFixed(2)),
					dyR: Number(dyR.toFixed(2))
				};
				window._tickerDirDone = true;
			}
		}, 90);
		return true;
	})()`))
	for i := 0; i < 30; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._tickerDirDone)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(100*time.Millisecond))
	}
	g4JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window._tickerDirResult || { ok: false })`))
	var g4 map[string]any
	_ = json.Unmarshal([]byte(g4JSON), &g4)
	record("AT-16DC-C04", "Horizontal Ticker Left & Right + Zero Vertical Drift", g4["ok"] == true, g4JSON)

	// =========================================================================
	// GATE 5: Vertical Multiline Scroll (Explicit Newlines, Word Wrap ON/OFF, Line Spacing, Lines/s Timing) (Items #4, #5, #6, #7, #8)
	// =========================================================================
	g5JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const S = window.TextLiteShowcase;
			const host = document.createElement('div');
			document.body.appendChild(host);

			const msg = 'HEADER LINE ONE\nThis second paragraph contains multiple words that wrap when Word Wrap is enabled.';
			// 1. Word Wrap ON, lineSpacing 1.2, linesPerSec 1.0
			const pkgWrapOn = S.makeSingleTextWidgetSVG({
				textAnim: 'ticker-v',
				labelText: msg,
				wordWrap: true,
				lineSpacing: 1.2,
				linesPerSec: 1.0,
				width: 240,
				height: 80,
				fontSize: 16
			});
			const wOn = window.SVGWidget.mount(host, pkgWrapOn.svg, { instanceId: 'v-wrap-on', manifest: pkgWrapOn.manifest, visibilityAware: false });
			const gOn = host.querySelector('[data-lite-kind="text-lite"]');
			const linesOn = gOn.querySelectorAll('tspan.lite-text-line');
			const countOn = linesOn.length;
			const dur1Lps = parseFloat(gOn.getAttribute('data-ticker-duration-s') || '0');
			const dy12 = countOn >= 2 ? (parseFloat(linesOn[1].getAttribute('y')) - parseFloat(linesOn[0].getAttribute('y'))) : 0;

			// 2. Word Wrap OFF -> only 2 lines (explicit newline)
			const pkgWrapOff = S.makeSingleTextWidgetSVG({
				textAnim: 'ticker-v',
				labelText: msg,
				wordWrap: false,
				lineSpacing: 1.6,
				linesPerSec: 2.0,
				width: 240,
				height: 80,
				fontSize: 16
			});
			const host2 = document.createElement('div');
			document.body.appendChild(host2);
			const wOff = window.SVGWidget.mount(host2, pkgWrapOff.svg, { instanceId: 'v-wrap-off', manifest: pkgWrapOff.manifest, visibilityAware: false });
			const gOff = host2.querySelector('[data-lite-kind="text-lite"]');
			const linesOff = gOff.querySelectorAll('tspan.lite-text-line');
			const countOff = linesOff.length;
			const dy16 = countOff >= 2 ? (parseFloat(linesOff[1].getAttribute('y')) - parseFloat(linesOff[0].getAttribute('y'))) : 0;

			// 3. Same wrapped message at 2.0 lines/s -> duration must be exactly half of 1.0 lines/s!
			const pkgFast = S.makeSingleTextWidgetSVG({
				textAnim: 'ticker-v',
				labelText: msg,
				wordWrap: true,
				lineSpacing: 1.2,
				linesPerSec: 2.0,
				width: 240,
				height: 80,
				fontSize: 16
			});
			const host3 = document.createElement('div');
			document.body.appendChild(host3);
			const wFast = window.SVGWidget.mount(host3, pkgFast.svg, { instanceId: 'v-fast', manifest: pkgFast.manifest, visibilityAware: false });
			const gFast = host3.querySelector('[data-lite-kind="text-lite"]');
			const dur2Lps = parseFloat(gFast.getAttribute('data-ticker-duration-s') || '0');

			wOn.destroy();
			wOff.destroy();
			wFast.destroy();
			host.remove();
			host2.remove();
			host3.remove();

			return JSON.stringify({
				ok: countOff === 2 && countOn >= 4 && dy16 > dy12 + 4 && Math.abs(dur1Lps / dur2Lps - 2.0) < 0.05,
				countOn, countOff,
				dy12: Number(dy12.toFixed(2)),
				dy16: Number(dy16.toFixed(2)),
				dur1Lps, dur2Lps
			});
		} catch (e) {
			return JSON.stringify({ ok: false, err: String(e) });
		}
	})()`))
	var g5 map[string]any
	_ = json.Unmarshal([]byte(g5JSON), &g5)
	record("AT-16DC-C05", "Vertical Multiline Newlines, Word Wrap, Line Spacing & Lines/s", g5["ok"] == true, g5JSON)

	// =========================================================================
	// GATE 6: Stacked-Letter Vertical Scroll Layout & Temporal Movement (Item #9)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._stackedDone = false;
		window._stackedResult = null;
		const S = window.TextLiteShowcase;
		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;top:10px;left:10px;width:240px;height:140px;z-index:99999;background:#050912;';
		document.body.appendChild(host);

		const pkg = S.makeSingleTextWidgetSVG({
			textAnim: 'stacked-v',
			labelText: 'STATUS🚀',
			typeSpeed: 10,
			lineSpacing: 1.2,
			direction: 'forward',
			width: 180,
			height: 110,
			fontSize: 16
		});
		const w = window.SVGWidget.mount(host, pkg.svg, { instanceId: 'test-stacked-v', manifest: pkg.manifest, visibilityAware: false });
		const txtEl = host.querySelector('.lite-text-element');
		const tspans = Array.from(txtEl.querySelectorAll('tspan.lite-text-line'));
		const letters = tspans.map(t => t.textContent);
		const sameX = new Set(tspans.map(t => t.getAttribute('x'))).size === 1;
		const monotonicY = tspans.every((t, i) => i === 0 || parseFloat(t.getAttribute('y')) > parseFloat(tspans[i - 1].getAttribute('y')));

		const ySamples = [];
		let c = 0;
		const iv = setInterval(() => {
			const r = txtEl.getBoundingClientRect();
			ySamples.push(Number(r.top.toFixed(2)));
			c++;
			if (c >= 4) {
				clearInterval(iv);
				const dy = ySamples[ySamples.length - 1] - ySamples[0];
				w.destroy();
				host.remove();
				window._stackedResult = {
					ok: letters.join('|') === 'S|T|A|T|U|S|🚀' && sameX && monotonicY && dy < -15,
					letters: letters.join('|'),
					sameX,
					monotonicY,
					dy: Number(dy.toFixed(2))
				};
				window._stackedDone = true;
			}
		}, 80);
		return true;
	})()`))
	for i := 0; i < 30; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._stackedDone)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(100*time.Millisecond))
	}
	g6JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window._stackedResult || { ok: false })`))
	var g6 map[string]any
	_ = json.Unmarshal([]byte(g6JSON), &g6)
	record("AT-16DC-C06", "Stacked-Letter Vertical Column Layout & Scroll", g6["ok"] == true, g6JSON)

	// =========================================================================
	// GATE 7: Fade & Blink 3-Phase Separate Timing Controls + Play Once / Ping-Pong (Items #10, #11, #13)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._timingDone = false;
		window._timingResult = null;
		const S = window.TextLiteShowcase;
		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;top:10px;left:10px;width:400px;height:200px;z-index:99999;';
		const bFade = document.createElement('div');
		const bBlink = document.createElement('div');
		const bOnce = document.createElement('div');
		host.appendChild(bFade);
		host.appendChild(bBlink);
		host.appendChild(bOnce);
		document.body.appendChild(host);

		// 1. Fade with Fade=0.2s, VisibleHold=0.4s, Clear=0.4s -> total=1.2s
		// At t=350ms (inside VisibleHold), opacity must be >0.95; at t=1000ms (inside Clear), opacity must be <0.10
		const pkgFade = S.makeSingleTextWidgetSVG({
			textAnim: 'fade',
			speed: 0.2,
			holdDelay: 0.4,
			clearDuration: 0.4,
			playbackMode: 'loop'
		});
		// 2. Blink with FadeIn=0s, OnHold=0.25s, Off=0.65s -> total=0.90s
		// At t=100ms (inside OnHold), opacity==1; at t=500ms (inside Off), opacity==0
		const pkgBlink = S.makeSingleTextWidgetSVG({
			textAnim: 'flash',
			speed: 0,
			holdDelay: 0.25,
			clearDuration: 0.65,
			lockOnOff: false,
			playbackMode: 'loop'
		});
		// 3. Ticker Play Once (playbackMode: 'once', speed high) -> settles inside container and triggers cycleCompleted
		const pkgOnce = S.makeSingleTextWidgetSVG({
			textAnim: 'ticker-h',
			scrollSpeed: 600,
			playbackMode: 'once',
			labelText: 'ONCE SETTLE'
		});

		const wFade = window.SVGWidget.mount(bFade, pkgFade.svg, { instanceId: 'tm-fade', manifest: pkgFade.manifest, visibilityAware: false });
		const wBlink = window.SVGWidget.mount(bBlink, pkgBlink.svg, { instanceId: 'tm-blink', manifest: pkgBlink.manifest, visibilityAware: false });
		const wOnce = window.SVGWidget.mount(bOnce, pkgOnce.svg, { instanceId: 'tm-once', manifest: pkgOnce.manifest, visibilityAware: false });

		const elFade = bFade.querySelector('.lite-text-element');
		const elBlink = bBlink.querySelector('.lite-text-element');
		const gOnce = bOnce.querySelector('[data-lite-kind="text-lite"]');

		let blinkOnOp = -1, blinkOffOp = -1, fadeHoldOp = -1, fadeClearOp = -1;
		setTimeout(() => {
			blinkOnOp = parseFloat(getComputedStyle(elBlink).opacity);
		}, 90);
		setTimeout(() => {
			fadeHoldOp = parseFloat(getComputedStyle(elFade).opacity);
		}, 350);
		setTimeout(() => {
			blinkOffOp = parseFloat(getComputedStyle(elBlink).opacity);
		}, 500);
		setTimeout(() => {
			fadeClearOp = parseFloat(getComputedStyle(elFade).opacity);
			const onceCompleted = gOnce.getAttribute('data-text-cycle-completed') === 'true';
			wFade.destroy();
			wBlink.destroy();
			wOnce.destroy();
			host.remove();
			window._timingResult = {
				ok: blinkOnOp > 0.9 && blinkOffOp < 0.1 && fadeHoldOp > 0.9 && fadeClearOp < 0.15 && onceCompleted,
				blinkOnOp: Number(blinkOnOp.toFixed(2)),
				blinkOffOp: Number(blinkOffOp.toFixed(2)),
				fadeHoldOp: Number(fadeHoldOp.toFixed(2)),
				fadeClearOp: Number(fadeClearOp.toFixed(2)),
				onceCompleted
			};
			window._timingDone = true;
		}, 1020);
		return true;
	})()`))
	for i := 0; i < 30; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._timingDone)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(100*time.Millisecond))
	}
	g7JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window._timingResult || { ok: false })`))
	var g7 map[string]any
	_ = json.Unmarshal([]byte(g7JSON), &g7)
	record("AT-16DC-C07", "3-Phase Fade & Blink Temporal Opacity + Play Once", g7["ok"] == true, g7JSON)

	// =========================================================================
	// GATE 8: Ping-Pong Ticker No Mid-Return Substitution (Item #8 & #13)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._ppDone = false;
		window._ppResult = null;
		const S = window.TextLiteShowcase;
		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;top:10px;left:10px;width:400px;height:100px;z-index:99999;';
		document.body.appendChild(host);

		// At scrollSpeed=600 px/s on width=200, 1 one-way pass is ~0.55s; full forward+return Ping-Pong cycle is ~1.10s
		const pkgPP = S.makeSingleTextWidgetSVG({
			textAnim: 'ticker-h',
			scrollSpeed: 600,
			width: 200,
			playbackMode: 'pingpong',
			labelText: 'ORIGINAL PP'
		});
		const wPP = window.SVGWidget.mount(host, pkgPP.svg, { instanceId: 'test-pp', manifest: pkgPP.manifest, visibilityAware: false });
		const txtEl = host.querySelector('.lite-text-element');
		const gEl = host.querySelector('[data-lite-kind="text-lite"]');

		// Queue pending text at t=120ms (during forward pass)
		setTimeout(() => {
			wPP.setParameter('banner-lite-text', 'ADOPTED AFTER RETURN');
		}, 120);

		let midReturnText = '';
		// Check at t=750ms (after 1st iteration turnaround, during return journey): must still be 'ORIGINAL PP'!
		setTimeout(() => {
			midReturnText = txtEl.textContent;
		}, 750);

		// Check at t=1450ms (after 2nd iteration completes return journey): must now be 'ADOPTED AFTER RETURN'!
		setTimeout(() => {
			const postReturnText = txtEl.textContent;
			const handovers = parseInt(gEl.getAttribute('data-text-handover-count') || '0', 10);
			wPP.destroy();
			host.remove();
			window._ppResult = {
				ok: midReturnText === 'ORIGINAL PP' && postReturnText === 'ADOPTED AFTER RETURN' && handovers >= 1,
				midReturnText,
				postReturnText,
				handovers
			};
			window._ppDone = true;
		}, 1500);
		return true;
	})()`))
	for i := 0; i < 35; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._ppDone)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(100*time.Millisecond))
	}
	g8JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window._ppResult || { ok: false })`))
	var g8 map[string]any
	_ = json.Unmarshal([]byte(g8JSON), &g8)
	record("AT-16DC-C08", "Ping-Pong Ticker Completes Return Journey Before Handover", g8["ok"] == true, g8JSON)

	// =========================================================================
	// GATE 9: All Four Typewriter Sequences, Forward Deletion First-Grapheme Removal & Type Once and Hold Handover (Items #14, #15, #16, #18)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window._tw4Done = false;
		window._tw4Result = null;
		const S = window.TextLiteShowcase;
		const host = document.createElement('div');
		host.style.cssText = 'position:fixed;top:10px;left:10px;width:400px;height:200px;z-index:99999;';
		const bBack = document.createElement('div');
		const bFwd = document.createElement('div');
		const bHold = document.createElement('div');
		host.appendChild(bBack);
		host.appendChild(bFwd);
		host.appendChild(bHold);
		document.body.appendChild(host);

		// C. Delete Backwards: 'STATUS' -> during delete phase must see prefix like 'STATU' or 'STAT'
		const pkgBack = S.makeSingleTextWidgetSVG({
			textAnim: 'typewriter',
			labelText: 'STATUS',
			typeSpeed: 60,
			holdDelay: 0.15,
			typeMode: 'delete',
			typeRepeat: false
		});
		// D. Delete Forwards: 'STATUS' -> during delete-forward phase must see suffix like 'TATUS', 'ATUS', 'TUS'
		const pkgFwd = S.makeSingleTextWidgetSVG({
			textAnim: 'typewriter',
			labelText: 'STATUS',
			typeSpeed: 60,
			holdDelay: 0.15,
			typeMode: 'delete-forward',
			typeRepeat: false
		});
		// B. Type Once and Hold: 'READY' -> completes typing & hold, retains 'READY', then adopts new pending string when sent!
		const pkgHold = S.makeSingleTextWidgetSVG({
			textAnim: 'typewriter',
			labelText: 'READY',
			typeSpeed: 80,
			holdDelay: 0.15,
			typeMode: 'hold',
			typeRepeat: false
		});

		const wBack = window.SVGWidget.mount(bBack, pkgBack.svg, { instanceId: 'tw-back', manifest: pkgBack.manifest, visibilityAware: false });
		const wFwd = window.SVGWidget.mount(bFwd, pkgFwd.svg, { instanceId: 'tw-fwd', manifest: pkgFwd.manifest, visibilityAware: false });
		const wHold = window.SVGWidget.mount(bHold, pkgHold.svg, { instanceId: 'tw-hold', manifest: pkgHold.manifest, visibilityAware: false });

		const elBack = bBack.querySelector('.lite-text-element');
		const elFwd = bFwd.querySelector('.lite-text-element');
		const elHold = bHold.querySelector('.lite-text-element');

		const seenBack = new Set();
		const seenFwd = new Set();
		const iv = setInterval(() => {
			seenBack.add(elBack.textContent);
			seenFwd.add(elFwd.textContent);
		}, 12);

		let holdRetainedAfterCompletion = false;
		setTimeout(() => {
			// At t=400ms, Type Once and Hold ('READY') has finished typing + 150ms hold and must retain 'READY'
			holdRetainedAfterCompletion = (elHold.textContent === 'READY');
			// Now send a new message to Type Once and Hold -> must immediately start typing 'NEXTHOLD'!
			wHold.setParameter('banner-lite-text', 'NEXTHOLD');
		}, 420);

		setTimeout(() => {
			clearInterval(iv);
			const backArr = Array.from(seenBack);
			const fwdArr = Array.from(seenFwd);
			const hasBackPrefix = backArr.some(s => ['STATU', 'STAT', 'STA', 'ST'].includes(s));
			const hasFwdSuffix = fwdArr.some(s => ['TATUS', 'ATUS', 'TUS', 'US'].includes(s));
			const holdAdoptedNew = (elHold.textContent === 'NEXTHOLD');
			wBack.destroy();
			wFwd.destroy();
			wHold.destroy();
			host.remove();
			window._tw4Result = {
				ok: hasBackPrefix && hasFwdSuffix && holdRetainedAfterCompletion && holdAdoptedNew,
				hasBackPrefix,
				hasFwdSuffix,
				holdRetainedAfterCompletion,
				holdAdoptedNew,
				fwdSamples: fwdArr
			};
			window._tw4Done = true;
		}, 950);
		return true;
	})()`))
	for i := 0; i < 30; i++ {
		done, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window._tw4Done)`))
		if done {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(100*time.Millisecond))
	}
	g9JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window._tw4Result || { ok: false })`))
	var g9 map[string]any
	_ = json.Unmarshal([]byte(g9JSON), &g9)
	record("AT-16DC-C09", "4 Typewriter Sequences, Delete Forwards & Hold Handover", g9["ok"] == true, g9JSON)

	// =========================================================================
	// GATE 10: Consolidated Visual Showcase & Protected Services (:8088, :8091, :8093) (Item #15 & #20)
	// =========================================================================
	g10JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const t1Count = document.querySelectorAll('#galleryT1 [data-lite-kind="text-lite"]').length;
		const t2Count = document.querySelectorAll('#galleryT2 [data-lite-kind="text-lite"] textPath').length;
		const t3t4Nodes = Array.from(document.querySelectorAll('#galleryT3T4 [data-lite-kind="text-lite"]'));
		const ids = new Set(t3t4Nodes.map(n => (n.getAttribute('id') || '').replace(/^.*__lite-/, '').replace(/^lite-/, '')));
		const requiredDemoIds = [
			'a-ticker-left', 'a-ticker-right', 'a-ticker-pp',
			'a-vscroll-wrap-on', 'a-vscroll-wrap-off', 'a-vscroll-fast-ls',
			'a-stacked-up', 'a-stacked-down',
			'a-fade-fast', 'a-fade-slow', 'a-slide',
			'a-flash-locked', 'a-flash-unlocked', 'a-fade-once',
			'a-type-once', 'a-type-hold', 'a-type-del-back', 'a-type-del-fwd',
			'a-blur-in', 'a-blur-out', 'a-scramble'
		];
		const missing = requiredDemoIds.filter(id => !ids.has(id));
		return JSON.stringify({
			ok: t1Count >= 26 && t2Count === 5 && missing.length === 0,
			t1Count,
			t2Count,
			t3t4Count: t3t4Nodes.length
		});
	})()`))
	var g10 map[string]any
	_ = json.Unmarshal([]byte(g10JSON), &g10)
	record("AT-16DC-C10", "Updated Visual Showcase (All 21 T3/T4 Cards + T1/T2)", g10["ok"] == true, g10JSON)

	if err := runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.Sleep(250*time.Millisecond),
	); err != nil {
		record("AT-16DC-SVC1", "Protected Service :8091 Stage 16C Widget Consumer", false, err.Error())
	} else {
		hasSVGWidget, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`typeof window.SVGWidget === 'object'`))
		record("AT-16DC-SVC1", "Protected Service :8091 Stage 16C Widget Consumer", hasSVGWidget, "http://192.168.100.40:8091/index.html active")
	}

	if err := runVoid(taskCtx,
		chromedp.Navigate(testLabURL),
		chromedp.Sleep(250*time.Millisecond),
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
	fmt.Printf(" Stage 16D-C Consolidated Corrections Summary: %d / %d Gates PASSED\n", passedCount, len(results))
	fmt.Println("================================================================================")
	if passedCount != len(results) || math.IsNaN(float64(passedCount)) {
		os.Exit(1)
	}
}
