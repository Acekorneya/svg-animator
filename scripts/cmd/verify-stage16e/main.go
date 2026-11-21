package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

const (
	studioURL        = "http://192.168.100.40:8088/"
	compositeDemoURL = "http://192.168.100.40:8088/composite-widget-demo.html"
	consumerURL      = "http://192.168.100.40:8091/index.html"
	testLabURL       = "http://192.168.100.40:8093/"
	cdpEndpoint      = "http://127.0.0.1:9222"
	cpuHost          = "nigel@192.168.100.40"
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
	fmt.Println("================================================================================")
	fmt.Println(" Stage 16E: Groups, Semantic Property Mappings & Reusable Composite Objects")
	fmt.Println("================================================================================")

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-14s %-56s | %s\n", status, id, name, details)
	}

	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire browser session: %v\n", err)
		return 1
	}
	defer sess.Close()

	ctx, cancelTimeout := context.WithTimeout(sess.AllocCtx, 180*time.Second)
	defer cancelTimeout()
	taskCtx, cancelTask := chromedp.NewContext(ctx)
	defer cancelTask()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	// =========================================================================
	// GATE 1: Studio Authoring (M1 Group Targets/Speed, M2 Semantic Mappings, M3 Composite Templates & Sequential Instances)
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1440, 960),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open Studio at %s: %v\n", studioURL, err)
		return 1
	}

	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(300*time.Millisecond))
	}

	g1JSON, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			window.confirm = () => true;
			window.alert = () => {};
			window.ProjectManager.resetToNewProject();
			window.EngineBridge.setWidgetParameters([]);

			const nIn = window.DiagramNodes.createNode({ shape: 'rounded', text: 'CoreIn', name: 'CoreIn', x: 18, y: 24, width: 80, height: 40 });
			const nOut = window.DiagramNodes.createNode({ shape: 'rounded', text: 'CoreOut', name: 'CoreOut', x: 30, y: 24, width: 80, height: 40 });
			const fl = window.DataFlows.createFlow({
				fromNodeId: nIn.id,
				toNodeId: nOut.id,
				color: '#38bdf8',
				speed: 1.0,
				nodes: [{ x: 18, y: 24 }, { x: 30, y: 24 }]
			});
			const leGauge = window.LiteEffects.createLiteEffect({
				kind: 'gauge-lite',
				variant: 'ring',
				name: 'CpuGauge',
				x: 300,
				y: 260,
				width: 120,
				height: 120,
				value: 40,
				minValue: 0,
				maxValue: 100
			});
			const leText = window.LiteEffects.createLiteEffect({
				kind: 'text-lite',
				variant: 'rounded',
				name: 'CpuReadout',
				x: 300,
				y: 360,
				width: 150,
				height: 40,
				labelText: '40'
			});

			window.AppCoordinator.setMultiSelection([
				{ type: 'node', id: String(nIn.id) },
				{ type: 'node', id: String(nOut.id) },
				{ type: 'flow', id: String(fl.id) },
				{ type: 'lite', id: String(leGauge.id) },
				{ type: 'lite', id: String(leText.id) }
			]);
			const grp1 = window.AppCoordinator.createGroupFromSelection('cpuwindow-1');
			window.AppCoordinator.selectGroup(grp1.id);

			// Check Right Drawer Widget Authoring Deck is visible for group
			const deckVis = document.getElementById('drawerWidgetAuthoringDeck')?.style.display !== 'none';
			const semanticSecVis = Boolean(document.querySelector('.group-semantic-mappings-section'));

			// M2: Define semantic mapping cpu -> gauge.value + text.text
			const mapRes = window.EngineBridge.setGroupMapping(grp1.id, {
				alias: 'cpu',
				label: 'CPU Load (%)',
				type: 'number',
				default: 40,
				min: 0,
				max: 100,
				targets: [
					{ memberType: 'lite', memberId: String(leGauge.id), target: 'lite-' + leGauge.id, property: 'value' },
					{ memberType: 'lite', memberId: String(leText.id), target: 'lite-' + leText.id, property: 'text' }
				]
			});

			// M3: Save as Composite Template & place 2 more instances
			const comp = window.AppCoordinator.saveSelectedGroupAsComposite('cpuwindow', 'CPU Monitor Template');
			const grp2 = window.AppCoordinator.instantiateComposite(comp.id, '', 30, 0);
			const grp3 = window.AppCoordinator.instantiateComposite(comp.id, '', 60, 0);

			window.renderCompositesList();
			const compListCount = document.querySelectorAll('#compositesListContainer .composite-template-item').length;
			const paramNames = window.EngineBridge.getWidgetParameters().map(p => p.name);

			return JSON.stringify({
				ok: Boolean(grp1 && deckVis && semanticSecVis && mapRes && mapRes.success && comp && grp2 && grp3 &&
					grp2.name === 'cpuwindow-2' && grp3.name === 'cpuwindow-3' && compListCount === 1 &&
					paramNames.includes('cpuwindow-1-group-cpu') &&
					paramNames.includes('cpuwindow-2-group-cpu') &&
					paramNames.includes('cpuwindow-3-group-cpu')),
				grp1Name: grp1 && grp1.name,
				grp2Name: grp2 && grp2.name,
				grp3Name: grp3 && grp3.name,
				compListCount,
				paramNames
			});
		} catch (e) {
			return JSON.stringify({ ok: false, error: e.message });
		}
	})()`))
	if err != nil {
		record("16E-GATE-01", "Studio Authoring (M1 Group Deck, M2 Mappings, M3 Composites)", false, err.Error())
	} else {
		var r map[string]any
		_ = json.Unmarshal([]byte(g1JSON), &r)
		record("16E-GATE-01", "Studio Authoring (M1 Group Deck, M2 Mappings, M3 Composites)", r["ok"] == true, g1JSON)
	}

	// =========================================================================
	// GATE 2: Consolidated Showcase (composite-widget-demo.html) — Multi-Target Fan-Out, Group Speed/Opacity/Visibility
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(compositeDemoURL),
		chromedp.WaitReady("#compositeClusterHost svg"),
	); err != nil {
		record("16E-GATE-02", "Consolidated Showcase Mount & 3 Placed Composite Instances", false, err.Error())
	} else {
		g2JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
			try {
				const w = window.clusterWidget;
				if (!w) return JSON.stringify({ ok: false, error: 'clusterWidget not mounted' });

				// 1. Update cpuwindow-1-group-cpu to 77 -> must update both gauge-12 value AND readout-13 text
				w.setParameter('cpuwindow-1-group-cpu', 77);
				const g12Val = document.querySelector('[data-semantic-target="lite-12"]')?.getAttribute('data-gauge-value');
				const r13Txt = document.querySelector('[data-semantic-target="lite-13"] text')?.textContent;

				// 2. Verify cpuwindow-2 and cpuwindow-3 remain independent (68 and 91)
				const g22Val = document.querySelector('[data-semantic-target="lite-22"]')?.getAttribute('data-gauge-value');
				const g32Val = document.querySelector('[data-semantic-target="lite-32"]')?.getAttribute('data-gauge-value');

				// 3. Update cpuwindow-2-group-status-color to #ef4444 -> fans out to tracer-21, flow-2, node-21, ticker-24
				w.setParameter('cpuwindow-2-group-status-color', '#ef4444');
				const tracer21Stroke = document.querySelector('[data-semantic-target="lite-21"] .lite-payload')?.getAttribute('stroke');
				const node21Stroke = document.querySelector('[data-semantic-target="node-21"] rect')?.getAttribute('stroke');

				// 4. Update cpuwindow-3-group-speed to 3.2 -> scales tracer-31 duration from 3.2s to 1.0s
				w.setParameter('cpuwindow-3-group-speed', 3.2);
				const tracer31StyleDur = document.querySelector('[data-semantic-target="lite-31"] .lite-payload')?.style.animationDuration || '';
				const tracer31Dur = parseFloat(tracer31StyleDur || '0');

				// 5. Update cpuwindow-1-group-opacity to 0.45 and cpuwindow-3-group-visibility to false
				w.setParameter('cpuwindow-1-group-opacity', 0.45);
				w.setParameter('cpuwindow-3-group-visibility', false);
				const grp1Op = document.querySelector('[data-semantic-target="group-1"]')?.getAttribute('opacity');
				const grp3Vis = document.querySelector('[data-semantic-target="group-3"]')?.getAttribute('visibility');
				w.setParameter('cpuwindow-3-group-visibility', true);

				return JSON.stringify({
					ok: g12Val === '77' && r13Txt === '77' && g22Val === '68' && g32Val === '91' &&
						tracer21Stroke === '#ef4444' && node21Stroke === '#ef4444' &&
						Math.abs(tracer31Dur - 1.0) < 0.05 && grp1Op === '0.45' && grp3Vis === 'hidden',
					g12Val, r13Txt, g22Val, g32Val, tracer21Stroke, node21Stroke, tracer31Dur, tracer31StyleDur, grp1Op, grp3Vis
				});
			} catch (e) {
				return JSON.stringify({ ok: false, error: e.message });
			}
		})()`))
		var r map[string]any
		_ = json.Unmarshal([]byte(g2JSON), &r)
		record("16E-GATE-02", "Showcase Fan-Out, Independence & Group Speed/Opacity/Vis", r["ok"] == true, g2JSON)
	}

	// =========================================================================
	// GATE 3: Same-Batch Precedence, TextLite Cycle-Boundary Handover & Atomic Invalid Rejection
	// =========================================================================
	g3JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		try {
			const w = window.clusterWidget;
			// 1. Same-Batch Precedence: pass member override first, group mapping second in object literal;
			// runtime must apply group mapping first and direct member override second!
			window.runPrecedenceDemo();
			const g12AfterPrec = document.querySelector('[data-semantic-target="lite-12"]')?.getAttribute('data-gauge-value');
			const r13AfterPrec = document.querySelector('[data-semantic-target="lite-13"] text')?.textContent;

			// 2. Atomic Rejection: invalid batch value must throw and leave DOM untouched
			window.runAtomicRejectionDemo();
			const g12AfterReject = document.querySelector('[data-semantic-target="lite-12"]')?.getAttribute('data-gauge-value');

			// 3. Animated TextLite pendingText handover via group semantic mapping
			w.setParameter('cpuwindow-1-group-headline', 'CLUSTER-01: HANDOVER QUEUED');
			const pendingAttr = document.querySelector('[data-semantic-target="lite-14"]')?.getAttribute('data-text-pending');

			return JSON.stringify({
				ok: g12AfterPrec === '85' && r13AfterPrec === '85 [OVR]' &&
					g12AfterReject === '85' && pendingAttr === 'CLUSTER-01: HANDOVER QUEUED',
				g12AfterPrec, r13AfterPrec, g12AfterReject, pendingAttr
			});
		} catch (e) {
			return JSON.stringify({ ok: false, error: e.message });
		}
	})()`))
	{
		var r map[string]any
		_ = json.Unmarshal([]byte(g3JSON), &r)
		record("16E-GATE-03", "Same-Batch Precedence, Pending-Text Handover & Atomic Reject", r["ok"] == true, g3JSON)
	}

	// =========================================================================
	// GATE 4: Existing Services Availability (:8091 Consumer & :8093 Test Lab)
	// =========================================================================
	err8091 := runVoid(taskCtx, chromedp.Navigate(consumerURL), chromedp.WaitReady("body"))
	err8093 := runVoid(taskCtx, chromedp.Navigate(testLabURL), chromedp.WaitReady("body"))
	record("16E-GATE-04", "Existing Services (:8091 Consumer & :8093 Test Lab)", err8091 == nil && err8093 == nil,
		fmt.Sprintf("8091=%v, 8093=%v", err8091 == nil, err8093 == nil))

	fmt.Println("================================================================================")
	allPassed := true
	for _, res := range results {
		if !res.Passed {
			allPassed = false
		}
	}
	if !allPassed {
		fmt.Println("RESULT: FAILED")
		return 1
	}
	fmt.Println("RESULT: ALL STAGE 16E BROWSER VERIFICATION GATES PASSED")
	return 0
}
