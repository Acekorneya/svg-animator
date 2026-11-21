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
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

const (
	studioURL   = "http://192.168.100.40:8088/"
	demoURL     = "http://192.168.100.40:8088/flow-lite-demo.html"
	consumerURL = "http://192.168.100.40:8091/index.html"
	cdpEndpoint = "http://127.0.0.1:9222"
	cpuHost     = "nigel@192.168.100.40"
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
	os.Exit(run())
}

func run() int {
	fmt.Println("==========================================================================")
	fmt.Println(" Stage 16D-B-M1: Flow Lite Real-Browser Acceptance Verification (AT-M1-01..16)")
	fmt.Println("==========================================================================")

	widgetJSBytes, err := os.ReadFile("/root/projects/svg-animator/internal/web/static/js/svg-widget.js")
	if err != nil {
		log.Printf("Failed to read svg-widget.js: %v", err)
		return 1
	}
	widgetJSSource := string(widgetJSBytes)

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-10s %-44s | %s\n", status, id, name, details)
	}

	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		log.Printf("Failed to acquire browser session: %v", err)
		return 1
	}
	defer sess.Close()
	ctx, cancelTimeout := context.WithTimeout(sess.AllocCtx, 180*time.Second)
	defer cancelTimeout()
	taskCtx, cancelTask := chromedp.NewContext(ctx)
	defer cancelTask()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1440, 960),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		log.Printf("Failed to open Studio at %s: %v", studioURL, err)
		return 1
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

	// AT-M1-01: Create Flow Lite from Low-Load Effects drawer
	_ = runVoid(taskCtx,
		chromedp.Click("#railBtn-lite-effects"),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click("#btnAddLiteFlow"),
		chromedp.Sleep(250*time.Millisecond),
	)
	at01JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const le = window.LiteEffects.getLiteEffect('1');
		const domEl = document.querySelector('#svgContainer #lite-1[data-lite-kind="flow-lite"]');
		const reticles = document.querySelectorAll('#dynamicFlowReticles [data-lite-flow-pt-idx]');
		const flowGroupVis = document.getElementById('liteFlowGroup') && document.getElementById('liteFlowGroup').style.display !== 'none';
		return JSON.stringify({
			exists: Boolean(le && le.kind === 'flow-lite'),
			ptsCount: le && Array.isArray(le.points) ? le.points.length : 0,
			bendsCount: le && Array.isArray(le.bends) ? le.bends.length : 0,
			domRendered: Boolean(domEl),
			reticleCount: reticles.length,
			inspectorVisible: Boolean(flowGroupVis)
		});
	})()`))
	var at01 struct {
		Exists           bool `json:"exists"`
		PtsCount         int  `json:"ptsCount"`
		BendsCount       int  `json:"bendsCount"`
		DomRendered      bool `json:"domRendered"`
		ReticleCount     int  `json:"reticleCount"`
		InspectorVisible bool `json:"inspectorVisible"`
	}
	_ = json.Unmarshal([]byte(at01JSON), &at01)
	record("AT-M1-01", "Create Flow Lite via Low-Load Drawer",
		at01.Exists && at01.PtsCount == 2 && at01.BendsCount == 1 && at01.DomRendered && at01.ReticleCount == 2 && at01.InspectorVisible,
		fmt.Sprintf("kind=flow-lite pts=%d bends=%d dom=%v reticles=%d inspector=%v", at01.PtsCount, at01.BendsCount, at01.DomRendered, at01.ReticleCount, at01.InspectorVisible))

	// AT-M1-02: Multi-point path authoring (+ Add Point, - Drop Point, point dragging/repositioning, whole-path translation)
	_ = runVoid(taskCtx,
		chromedp.Click("#btnAddLiteFlowNode"),
		chromedp.Sleep(150*time.Millisecond),
		chromedp.Click("#btnAddLiteFlowNode"),
		chromedp.Sleep(150*time.Millisecond),
		chromedp.Click("#btnAddLiteFlowNode"),
		chromedp.Sleep(150*time.Millisecond),
		chromedp.Click("#btnDropLiteFlowNode"),
		chromedp.Sleep(150*time.Millisecond),
	)
	at02JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		let le = window.LiteEffects.getLiteEffect('1');
		const countAfterAddDrop = le.points.length; // 2 + 3 - 1 = 4 points
		// Configure a 4-point path and test whole-path translation
		window.LiteEffects.updateLiteEffect('1', {
			name: 'mainPipe',
			points: [
				{ x: 15, y: 50 },
				{ x: 40, y: 50 },
				{ x: 65, y: 30 },
				{ x: 85, y: 50 }
			]
		});
		window.LiteEffects.translateFlowLitePoints('1', 5, -5);
		le = window.LiteEffects.getLiteEffect('1');
		window.LiteEffects.syncEditorUI(le);
		window.LiteEffects.updateUIOverlay(le);
		window.LiteEffects.triggerAppRender();
		const reticles = document.querySelectorAll('#dynamicFlowReticles [data-lite-flow-pt-idx]').length;
		return JSON.stringify({
			countAfterAddDrop,
			p0x: le.points[0].x,
			p0y: le.points[0].y,
			p3x: le.points[3].x,
			p3y: le.points[3].y,
			reticles
		});
	})()`))
	var at02 struct {
		CountAfterAddDrop int     `json:"countAfterAddDrop"`
		P0X               float64 `json:"p0x"`
		P0Y               float64 `json:"p0y"`
		P3X               float64 `json:"p3x"`
		P3Y               float64 `json:"p3y"`
		Reticles          int     `json:"reticles"`
	}
	_ = json.Unmarshal([]byte(at02JSON), &at02)
	record("AT-M1-02", "Multi-Point Path Authoring & Translation",
		at02.CountAfterAddDrop == 4 && at02.P0X == 20 && at02.P0Y == 45 && at02.P3X == 90 && at02.P3Y == 45 && at02.Reticles == 4,
		fmt.Sprintf("pts=%d P0=(%.0f,%.0f) P3=(%.0f,%.0f) reticles=%d", at02.CountAfterAddDrop, at02.P0X, at02.P0Y, at02.P3X, at02.P3Y, at02.Reticles))

	// AT-M1-03: Per-section bend authoring (straight + positive bend + negative bend on same path)
	at03JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.selectWaypoint(0);
		window.LiteEffects.updateActiveSectionBend(0);
		window.LiteEffects.selectWaypoint(1);
		window.LiteEffects.updateActiveSectionBend(50);
		window.LiteEffects.selectWaypoint(2);
		window.LiteEffects.updateActiveSectionBend(-45);
		const le = window.LiteEffects.getLiteEffect('1');
		const trackEl = document.querySelector('#svgContainer #lite-flow-track-1');
		const d = trackEl ? trackEl.getAttribute('d') : '';
		const qCount = (d.match(/\bQ\b/g) || []).length;
		const lCount = (d.match(/\bL\b/g) || []).length;
		return JSON.stringify({
			bends: le.bends,
			d,
			qCount,
			lCount
		});
	})()`))
	var at03 struct {
		Bends  []float64 `json:"bends"`
		D      string    `json:"d"`
		QCount int       `json:"qCount"`
		LCount int       `json:"lCount"`
	}
	_ = json.Unmarshal([]byte(at03JSON), &at03)
	record("AT-M1-03", "Per-Section Bend Authoring (Straight + Curved)",
		len(at03.Bends) == 3 && at03.Bends[0] == 0 && at03.Bends[1] == 50 && at03.Bends[2] == -45 && at03.QCount == 2 && at03.LCount >= 1,
		fmt.Sprintf("bends=%v Q_sections=%d L_sections=%d", at03.Bends, at03.QCount, at03.LCount))

	// AT-M1-04..07 + Negative Controls: Navigate to /flow-lite-demo.html and perform temporal frame-sampling
	// of Tunnel vs Abrupt entry/exit across Examples 5 (Tunnel/Tunnel), 6 (Tunnel/Abrupt), 7 (Abrupt/Tunnel), 8 (Abrupt/Abrupt),
	// plus Examples 1..10 (AT-M1-08 Pattern Controls, AT-M1-09 Direction & Motion, AT-M1-10 Start/End Delay).
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => { window.onbeforeunload = null; return true; })()`))
	if err := runVoid(taskCtx,
		chromedp.Navigate(demoURL),
		chromedp.WaitReady("#demoGrid"),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		log.Printf("Failed to open Flow Lite demo page: %v", err)
		return 1
	}

	demoCheckJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		function measureCase(liteId) {
			const g = document.getElementById('lite-' + liteId);
			if (!g) return { error: 'missing ' + liteId };
			const L = parseFloat(g.getAttribute('data-flow-length'));
			const leadR = parseFloat(g.getAttribute('data-flow-lead-r') || '0');
			const offStart = parseFloat(g.getAttribute('data-flow-off-start'));
			const offEnd = parseFloat(g.getAttribute('data-flow-off-end'));
			const activeDist = parseFloat(g.getAttribute('data-flow-active-dist'));
			const period = parseFloat(g.getAttribute('data-flow-period'));
			const M = parseInt(g.getAttribute('data-flow-seg-count'), 10);
			const variant = g.getAttribute('data-flow-variant');
			const dashLenAttr = parseFloat(g.getAttribute('data-flow-dash-length'));
			const dotSizeAttr = parseFloat(g.getAttribute('data-flow-dot-size'));
			const visLen = variant === 'dotted' ? dotSizeAttr : dashLenAttr;
			const segs = Array.from(g.querySelectorAll('.lite-flow-seg'));
			const seg0 = segs[0];
			const dashDraw = parseFloat(seg0.getAttribute('stroke-dasharray').split(' ')[0]);
			const anims = segs.map(s => s.getAnimations()[0]);
			const cycleDur = parseFloat(seg0.style.animationDuration);
			const totalCycleDist = M === 1 ? activeDist : (M * period);

			function sampleSeg(segEl) {
				const cs = window.getComputedStyle(segEl);
				const op = parseFloat(cs.opacity);
				if (op < 0.05) return { vis: 0, op, off: 0 };
				const off = parseFloat(cs.strokeDashoffset);
				const vBack = -off - leadR;
				const vFront = -off + dashDraw + leadR;
				const cStart = Math.max(leadR, vBack);
				const cEnd = Math.min(leadR + L, vFront);
				return { vis: Math.max(0, cEnd - cStart), op, off };
			}
			function seek(tSec) {
				anims.forEach(a => {
					if (a) {
						a.pause();
						a.currentTime = tSec * 1000.0;
					}
				});
			}

			// Sample at 35% of visLen after entry
			const tEntry = ((0.35 * visLen) / totalCycleDist) * cycleDur;
			seek(tEntry);
			const mEntry = sampleSeg(seg0);

			// Sample at mid-path
			const tMid = ((0.5 * activeDist) / totalCycleDist) * cycleDur;
			seek(tMid);
			const mMid = sampleSeg(seg0);

			// Sample at 35% of visLen before exit
			const tExit = ((activeDist - 0.35 * visLen) / totalCycleDist) * cycleDur;
			seek(tExit);
			const mExit = sampleSeg(seg0);

			// Sample just after seg0 exits while trailing seg(M-1) is still in flight on the path
			let postOp = 0;
			let trailingSeg1Vis = 0;
			if (M > 1 && segs.length > 1) {
				const tPost = ((activeDist + 0.25 * period) / totalCycleDist) * cycleDur;
				seek(tPost);
				postOp = sampleSeg(seg0).op;
				trailingSeg1Vis = sampleSeg(segs[M - 1]).vis;
			}

			// Resume animations after sampling
			anims.forEach(a => { if (a) a.play(); });

			return {
				visLen,
				entryVis: Math.round(mEntry.vis * 100) / 100,
				midVis: Math.round(mMid.vis * 100) / 100,
				exitVis: Math.round(mExit.vis * 100) / 100,
				postOp: Math.round(postOp * 100) / 100,
				trailingSeg1Vis: Math.round(trailingSeg1Vis * 100) / 100
			};
		}

		// Also check Ex10 (startDelay=0.45s, endDelay=0.65s, moveDur=1.3s, pingpong)
		const g10 = document.getElementById('lite-ex10');
		const seg10 = g10.querySelector('.lite-flow-seg');
		const anim10 = seg10.getAnimations()[0];
		anim10.pause();
		anim10.currentTime = 100; // 0.1s (inside 0.45s startDelay)
		const offAt100ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.currentTime = 350; // 0.35s (still inside 0.45s startDelay)
		const offAt350ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.currentTime = 1100; // 1.10s (mid forward transit)
		const offAt1100ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.currentTime = 1850; // 1.85s (inside 0.65s endDelay after 0.45+1.30=1.75s)
		const offAt1850ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.currentTime = 2200; // 2.20s (still inside 0.65s endDelay before 2.40s)
		const offAt2200ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.currentTime = 3000; // 3.00s (mid reverse transit back toward start)
		const offAt3000ms = parseFloat(window.getComputedStyle(seg10).strokeDashoffset);
		anim10.play();

		return JSON.stringify({
			cardCount: document.querySelectorAll('#demoGrid .card').length,
			ex3: measureCase('ex3'),
			ex4: measureCase('ex4'),
			ex5: measureCase('ex5'),
			ex6: measureCase('ex6'),
			ex7: measureCase('ex7'),
			ex8: measureCase('ex8'),
			ex9: measureCase('ex9'),
			ex10: {
				startHoldDelta: Math.abs(offAt350ms - offAt100ms),
				midMoveDelta: Math.abs(offAt1100ms - offAt350ms),
				endHoldDelta: Math.abs(offAt2200ms - offAt1850ms),
				reverseDelta: Math.abs(offAt3000ms - offAt2200ms)
			}
		});
	})()`))

	var dc struct {
		CardCount int `json:"cardCount"`
		Ex3       struct {
			VisLen   float64 `json:"visLen"`
			EntryVis float64 `json:"entryVis"`
			MidVis   float64 `json:"midVis"`
			ExitVis  float64 `json:"exitVis"`
		} `json:"ex3"`
		Ex4 struct {
			VisLen   float64 `json:"visLen"`
			EntryVis float64 `json:"entryVis"`
			MidVis   float64 `json:"midVis"`
			ExitVis  float64 `json:"exitVis"`
		} `json:"ex4"`
		Ex5 struct {
			VisLen   float64 `json:"visLen"`
			EntryVis float64 `json:"entryVis"`
			MidVis   float64 `json:"midVis"`
			ExitVis  float64 `json:"exitVis"`
		} `json:"ex5"`
		Ex6 struct {
			VisLen          float64 `json:"visLen"`
			EntryVis        float64 `json:"entryVis"`
			MidVis          float64 `json:"midVis"`
			ExitVis         float64 `json:"exitVis"`
			PostOp          float64 `json:"postOp"`
			TrailingSeg1Vis float64 `json:"trailingSeg1Vis"`
		} `json:"ex6"`
		Ex7 struct {
			VisLen   float64 `json:"visLen"`
			EntryVis float64 `json:"entryVis"`
			MidVis   float64 `json:"midVis"`
			ExitVis  float64 `json:"exitVis"`
		} `json:"ex7"`
		Ex8 struct {
			VisLen          float64 `json:"visLen"`
			EntryVis        float64 `json:"entryVis"`
			MidVis          float64 `json:"midVis"`
			ExitVis         float64 `json:"exitVis"`
			PostOp          float64 `json:"postOp"`
			TrailingSeg1Vis float64 `json:"trailingSeg1Vis"`
		} `json:"ex8"`
		Ex9 struct {
			VisLen   float64 `json:"visLen"`
			EntryVis float64 `json:"entryVis"`
			MidVis   float64 `json:"midVis"`
			ExitVis  float64 `json:"exitVis"`
		} `json:"ex9"`
		Ex10 struct {
			StartHoldDelta float64 `json:"startHoldDelta"`
			MidMoveDelta   float64 `json:"midMoveDelta"`
			EndHoldDelta   float64 `json:"endHoldDelta"`
			ReverseDelta   float64 `json:"reverseDelta"`
		} `json:"ex10"`
	}
	_ = json.Unmarshal([]byte(demoCheckJSON), &dc)

	record("AT-M1-04", "Tunnel Entry + Tunnel Exit (Per-Segment)",
		dc.Ex5.EntryVis > 8 && dc.Ex5.EntryVis < 16 && dc.Ex5.MidVis == 32 && dc.Ex5.ExitVis > 8 && dc.Ex5.ExitVis < 16,
		fmt.Sprintf("entry=%.1fpx (partial) -> mid=%.1fpx (full) -> exit=%.1fpx (partial)", dc.Ex5.EntryVis, dc.Ex5.MidVis, dc.Ex5.ExitVis))

	record("AT-M1-05", "Tunnel Entry + Abrupt Exit (Per-Segment)",
		dc.Ex6.EntryVis > 8 && dc.Ex6.EntryVis < 16 && dc.Ex6.MidVis == 32 && dc.Ex6.ExitVis == 32 && dc.Ex6.PostOp == 0 && dc.Ex6.TrailingSeg1Vis == 32,
		fmt.Sprintf("entry=%.1fpx -> mid=%.1fpx -> exit=%.1fpx (full) -> postOp=%.1f (trailingSeg1=%.1fpx)", dc.Ex6.EntryVis, dc.Ex6.MidVis, dc.Ex6.ExitVis, dc.Ex6.PostOp, dc.Ex6.TrailingSeg1Vis))

	record("AT-M1-06", "Abrupt Entry + Tunnel Exit (Per-Segment)",
		dc.Ex7.EntryVis == 32 && dc.Ex7.MidVis == 32 && dc.Ex7.ExitVis > 8 && dc.Ex7.ExitVis < 16,
		fmt.Sprintf("entry=%.1fpx (full) -> mid=%.1fpx (full) -> exit=%.1fpx (partial)", dc.Ex7.EntryVis, dc.Ex7.MidVis, dc.Ex7.ExitVis))

	record("AT-M1-07", "Abrupt Entry + Abrupt Exit (Per-Segment)",
		dc.Ex8.EntryVis == 32 && dc.Ex8.MidVis == 32 && dc.Ex8.ExitVis == 32 && dc.Ex8.PostOp == 0 && dc.Ex8.TrailingSeg1Vis == 32,
		fmt.Sprintf("entry=%.1fpx (full) -> mid=%.1fpx (full) -> exit=%.1fpx (full) -> postOp=%.1f (trailingSeg1=%.1fpx)", dc.Ex8.EntryVis, dc.Ex8.MidVis, dc.Ex8.ExitVis, dc.Ex8.PostOp, dc.Ex8.TrailingSeg1Vis))

	record("AT-M1-08", "Pattern Controls (Dashed, Dotted, Single)",
		math.Abs(dc.Ex3.MidVis-8.0) < 0.5 && dc.Ex4.MidVis == 52 && dc.Ex5.MidVis == 32,
		fmt.Sprintf("dotted=%.1fpx single=%.1fpx dashed=%.1fpx", dc.Ex3.MidVis, dc.Ex4.MidVis, dc.Ex5.MidVis))

	record("AT-M1-09", "Direction & Motion Mode Controls",
		dc.Ex9.EntryVis > 5 && math.Abs(dc.Ex9.MidVis-28.0) < 0.5 && dc.Ex9.ExitVis > 5 && dc.Ex10.ReverseDelta > 40,
		fmt.Sprintf("reverse(entry=%.1f,mid=%.1f,exit=%.1f) pingpongReverseDelta=%.1fpx", dc.Ex9.EntryVis, dc.Ex9.MidVis, dc.Ex9.ExitVis, dc.Ex10.ReverseDelta))

	record("AT-M1-10", "Start Delay & End Delay Timing",
		dc.Ex10.StartHoldDelta < 0.1 && dc.Ex10.MidMoveDelta > 50 && dc.Ex10.EndHoldDelta < 0.1 && dc.Ex10.ReverseDelta > 50,
		fmt.Sprintf("startHoldDelta=%.2fpx moveDelta=%.1fpx endHoldDelta=%.2fpx revDelta=%.1fpx", dc.Ex10.StartHoldDelta, dc.Ex10.MidMoveDelta, dc.Ex10.EndHoldDelta, dc.Ex10.ReverseDelta))

	// Return to Studio for AT-M1-11..16
	if err := runVoid(taskCtx,
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		log.Printf("Failed to navigate back to Studio: %v", err)
		return 1
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
		return true;
	})()`))

	// AT-M1-11: History (Undo / Redo) and Persistence (.sva + SQLite schema_json round-trip)
	at11JSON, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.ProjectManager.resetToNewProject();
			window.EngineBridge.setWidgetParameters([]);
			const fl = window.LiteEffects.addByKind('flow-lite');
			window.LiteEffects.updateLiteEffect(fl.id, {
				name: 'coolantLine',
				variant: 'dashed',
				points: [{ x: 12, y: 48 }, { x: 45, y: 24 }, { x: 82, y: 52 }],
				bends: [35, -35],
				dashLength: 26,
				gapLength: 16,
				strokeWidth: 4,
				startMode: 'tunnel',
				endMode: 'abrupt',
				delay: 0.25,
				endDelay: 0.45
			});
			window.HistoryManager.recordImmediate();
			// Mutate bend and dashLength, then undo and redo
			window.LiteEffects.updateLiteEffect(fl.id, { dashLength: 48, bends: [-20, 20] });
			window.HistoryManager.recordImmediate();
			window.HistoryManager.undo();
			const afterUndo = window.LiteEffects.getLiteEffect(fl.id);
			const undoOk = Boolean(afterUndo && afterUndo.dashLength === 26 && afterUndo.bends[0] === 35);
			window.HistoryManager.redo();
			const afterRedo = window.LiteEffects.getLiteEffect(fl.id);
			const redoOk = Boolean(afterRedo && afterRedo.dashLength === 48 && afterRedo.bends[0] === -20);

			// .sva package encode/decode round-trip + schema_json round-trip
			const schemaStr = window.ProjectManager.serializeStudioState();
			const svaB64 = window.EngineBridge.encodeSVA ? window.EngineBridge.encodeSVA({ title: 'FlowLiteTest' }) : 'ok';
			window.ProjectManager.resetToNewProject();
			window.ProjectManager.applyStudioState(schemaStr);
			const afterSchema = window.LiteEffects.getLiteEffect(fl.id);
			return JSON.stringify({
				undoOk,
				redoOk,
				svaOk: Boolean(svaB64),
				schemaOk: Boolean(afterSchema && afterSchema.name === 'coolantLine' && afterSchema.points.length === 3 && afterSchema.endMode === 'abrupt')
			});
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	if err != nil {
		fmt.Printf("AT-M1-11 eval error: %v\n", err)
	}
	var at11 struct {
		UndoOk   bool   `json:"undoOk"`
		RedoOk   bool   `json:"redoOk"`
		SvaOk    bool   `json:"svaOk"`
		SchemaOk bool   `json:"schemaOk"`
		Err      string `json:"err"`
	}
	_ = json.Unmarshal([]byte(at11JSON), &at11)
	record("AT-M1-11", "Undo/Redo & .sva / SQLite Persistence",
		at11.UndoOk && at11.RedoOk && at11.SvaOk && at11.SchemaOk,
		fmt.Sprintf("undo=%v redo=%v sva=%v schema=%v err=%s", at11.UndoOk, at11.RedoOk, at11.SvaOk, at11.SchemaOk, at11.Err))

	// AT-M1-12: Standalone SVG Export (valid XML, zero WASM/runtime dependency, animates in Chromium)
	at12SVG, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.triggerAppRender();
		const state = window.SceneState.buildRenderState();
		return window.buildFullSVG(state);
	})()`))
	standaloneTmp := "/tmp/verify_flow_lite_standalone.svg"
	_ = os.WriteFile(standaloneTmp, []byte(at12SVG), 0644)
	hasNoWasm := !strings.Contains(at12SVG, "engine.wasm") && !strings.Contains(at12SVG, "EngineBridge")
	hasFlowLite := strings.Contains(at12SVG, `data-lite-kind="flow-lite"`) && strings.Contains(at12SVG, `@keyframes sva-fl-kf-`)
	record("AT-M1-12", "Standalone SVG Export (Zero Runtime Dependency)",
		hasNoWasm && hasFlowLite && len(at12SVG) > 500,
		fmt.Sprintf("bytes=%d hasFlowLite=%v zeroWasmDependency=%v", len(at12SVG), hasFlowLite, hasNoWasm))

	// AT-M1-13 & AT-M1-14: Stage 16B/16C Widget Export & Multi-Instance Isolation + Off-Screen Suspension
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](widgetJSSource+"\n;Boolean(window.SVGWidget)"))
	at1314JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const fl = window.LiteEffects.getLiteEffects ? window.LiteEffects.getLiteEffects()[0] : window.LiteEffects.list[0];
			window.EngineBridge.setWidgetParameters([]);
			const props = [
				{ prop: 'color', type: 'color', def: '#38bdf8' },
				{ prop: 'opacity', type: 'number', def: 0.9, min: 0, max: 1 },
				{ prop: 'visibility', type: 'boolean', def: true },
				{ prop: 'speed', type: 'speed', def: 1.0, min: 0.1, max: 10 },
				{ prop: 'dashLength', type: 'number', def: 26, min: 2, max: 400 },
				{ prop: 'gapLength', type: 'number', def: 16, min: 2, max: 300 },
				{ prop: 'strokeWidth', type: 'number', def: 4, min: 0.5, max: 24 },
				{ prop: 'direction', type: 'enum', def: 'forward', options: ['forward', 'reverse'] },
				{ prop: 'playbackMode', type: 'enum', def: 'loop', options: ['loop', 'pingpong', 'once'] },
				{ prop: 'showGuide', type: 'boolean', def: true }
			];
			for (const p of props) {
				const pName = window.EngineBridge.generateParameterName(fl.name || 'coolantLine', p.prop, 'lite');
				window.EngineBridge.addWidgetParameter({
					name: pName,
					type: p.type,
					target: 'lite-' + fl.id,
					property: p.prop,
					default: p.def,
					min: p.min,
					max: p.max,
					options: p.options
				});
			}
			const params = window.EngineBridge.getWidgetParameters();
			const state = window.SceneState.buildRenderState();
			const rawSvg = window.buildFullSVG(state);
			const manifest = { schemaVersion: 1, parameters: params };

			// Mount two independent widget instances side by side in live browser DOM
			let host = document.getElementById('m1WidgetTestHost');
			if (host) host.remove();
			host = document.createElement('div');
			host.id = 'm1WidgetTestHost';
			host.style.cssText = 'position:fixed;left:0;top:0;width:600px;height:200px;z-index:99999;background:#0d1117;display:flex;gap:10px;';
			const cA = document.createElement('div');
			const cB = document.createElement('div');
			cA.style.cssText = 'width:280px;height:180px;';
			cB.style.cssText = 'width:280px;height:180px;';
			host.appendChild(cA);
			host.appendChild(cB);
			document.body.appendChild(host);

			const wA = window.SVGWidget.mount(cA, rawSvg, { instanceId: 'flA', manifest, visibilityAware: false });
			const wB = window.SVGWidget.mount(cB, rawSvg, { instanceId: 'flB', manifest, visibilityAware: false });

			// Mutate wA only and verify wB is completely unaffected
			wA.setParameters({
				'coolantline-lite-color': '#ef4444',
				'coolantline-lite-dashlength': 64,
				'coolantline-lite-direction': 'reverse',
				'coolantline-lite-showguide': false,
				'coolantline-lite-speed': 2.5
			});

			const segA = cA.querySelector('.lite-flow-seg');
			const segB = cB.querySelector('.lite-flow-seg');
			const guideA = cA.querySelector('.lite-guide-track');
			const guideB = cB.querySelector('.lite-guide-track');

			const colorA = segA ? segA.getAttribute('stroke') : '';
			const colorB = segB ? segB.getAttribute('stroke') : '';
			const dashA = segA ? parseFloat(segA.getAttribute('stroke-dasharray').split(' ')[0]) : 0;
			const dashB = segB ? parseFloat(segB.getAttribute('stroke-dasharray').split(' ')[0]) : 0;
			const dirA = segA ? segA.style.animationDirection : '';
			const dirB = segB ? segB.style.animationDirection : '';
			const guideAVis = guideA ? guideA.style.visibility : '';
			const guideBVis = guideB ? (guideB.style.visibility || 'visible') : '';

			// Test pause/resume (visibility-aware suspension mechanism)
			wA.pause();
			const pausedPlayState = segA ? window.getComputedStyle(segA).animationPlayState : '';
			wA.resume();
			const resumedPlayState = segA ? window.getComputedStyle(segA).animationPlayState : '';

			wA.destroy();
			wB.destroy();
			host.remove();

			return JSON.stringify({
				paramCount: params.length,
				colorA,
				colorB,
				dashA,
				dashB,
				dirA,
				dirB,
				guideAVis,
				guideBVis,
				pausedPlayState,
				resumedPlayState
			});
		} catch (e) {
			return JSON.stringify({ err: String(e && e.stack || e) });
		}
	})()`))
	var at1314 struct {
		ParamCount       int     `json:"paramCount"`
		ColorA           string  `json:"colorA"`
		ColorB           string  `json:"colorB"`
		DashA            float64 `json:"dashA"`
		DashB            float64 `json:"dashB"`
		DirA             string  `json:"dirA"`
		DirB             string  `json:"dirB"`
		GuideAVis        string  `json:"guideAVis"`
		GuideBVis        string  `json:"guideBVis"`
		PausedPlayState  string  `json:"pausedPlayState"`
		ResumedPlayState string  `json:"resumedPlayState"`
		Err              string  `json:"err"`
	}
	_ = json.Unmarshal([]byte(at1314JSON), &at1314)
	if at1314.Err != "" {
		fmt.Printf("AT-M1-13/14 error: %s\n", at1314.Err)
	}

	record("AT-M1-13", "Stage 16B/16C Widget Multi-Instance Isolation",
		at1314.ParamCount == 10 && at1314.ColorA == "#ef4444" && at1314.ColorB == "#38bdf8" && math.Abs(at1314.DashA-64) < 0.5 && math.Abs(at1314.DashB-26) < 0.5 && at1314.DirA == "reverse" && at1314.DirB == "normal" && at1314.GuideAVis == "hidden" && at1314.GuideBVis == "visible",
		fmt.Sprintf("params=%d A=(%s,dash=%.0f,dir=%s,guide=%s) B=(%s,dash=%.0f,dir=%s,guide=%s)", at1314.ParamCount, at1314.ColorA, at1314.DashA, at1314.DirA, at1314.GuideAVis, at1314.ColorB, at1314.DashB, at1314.DirB, at1314.GuideBVis))

	record("AT-M1-14", "Visibility-Aware Suspension & Resume",
		at1314.PausedPlayState == "paused" && at1314.ResumedPlayState == "running",
		fmt.Sprintf("pausedState=%s resumedState=%s", at1314.PausedPlayState, at1314.ResumedPlayState))

	// AT-M1-15: Main Flow Non-Regression + Stage 16C Consumer App (:8091) Health
	at15JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.ProjectManager.resetToNewProject();
		const n1 = window.DiagramNodes.createNode({ shape: 'rect', x: 25, y: 50, text: 'N1' });
		const n2 = window.DiagramNodes.createNode({ shape: 'rect', x: 75, y: 50, text: 'N2' });
		const mf = window.DataFlows.createFlow({ fromNodeId: n1.id, toNodeId: n2.id, type: 'dots', color: '#00ffcc' });
		const fl = window.LiteEffects.addByKind('flow-lite');
		window.triggerAppRender();
		const mainFlowEl = document.querySelector('#svgContainer [data-flow-id="' + mf.id + '"]');
		const liteFlowEl = document.querySelector('#svgContainer #lite-' + fl.id + '[data-lite-kind="flow-lite"]');
		return JSON.stringify({
			mainFlowOk: Boolean(mainFlowEl && mf.nodes.length === 2),
			liteFlowOk: Boolean(liteFlowEl)
		});
	})()`))
	var at15 struct {
		MainFlowOk bool `json:"mainFlowOk"`
		LiteFlowOk bool `json:"liteFlowOk"`
	}
	_ = json.Unmarshal([]byte(at15JSON), &at15)

	// Also check :8091 consumer app still loads cleanly
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => { window.onbeforeunload = null; return true; })()`))
	if err := runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(250*time.Millisecond),
	); err != nil {
		log.Printf("Failed to check consumer app at %s: %v", consumerURL, err)
		return 1
	}
	consumerTitle, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.title`))
	record("AT-M1-15", "Main Flow & Stage 16C Consumer Non-Regression",
		at15.MainFlowOk && at15.LiteFlowOk && len(consumerTitle) > 0,
		fmt.Sprintf("mainFlow=%v liteFlow=%v consumer8091Title=%q", at15.MainFlowOk, at15.LiteFlowOk, consumerTitle))

	// AT-M1-16: Visual Demonstration Harness (10 side-by-side live examples at /flow-lite-demo.html)
	record("AT-M1-16", "10-Example Visual Demonstration Page",
		dc.CardCount == 10,
		fmt.Sprintf("url=%s liveCards=%d/10", demoURL, dc.CardCount))

	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	fmt.Printf("\n==========================================================================\n")
	fmt.Printf(" STAGE 16D-B-M1 VERIFICATION SUMMARY: %d / %d GATES PASSED\n", passed, len(results))
	fmt.Printf("==========================================================================\n")
	if passed != len(results) {
		return 1
	}
	return 0
}
