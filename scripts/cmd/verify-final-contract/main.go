package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	kiwiberryHost = "192.168.100.40"
	apiBase       = "http://192.168.100.40:8088"
	cdpEndpoint   = "http://127.0.0.1:9222"
	remoteDemoDir = "/home/nigel/acceptance-demo/final-contract"
)

type CheckResult struct {
	Number  int
	Name    string
	Passed  bool
	Message string
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Final Property Contract & Effect Targeting Acceptance")
	fmt.Println(" Target: Kiwiberry (192.168.100.40:8088) with Chrome CDP")
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
		fmt.Printf("[%s] Criteria %2d: %-34s - %s\n", status, num, name, msg)
	}

	// 1. Set up CDP allocator context
	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()

	ctx, cancel := context.WithTimeout(allocCtx, 60*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	// 2. Navigate to Studio
	fmt.Println("\n[PHASE 1] Navigating to SVG Animator Studio on Kiwiberry...")
	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(apiBase)); err != nil {
		log.Fatalf("Failed to navigate to %s: %v", apiBase, err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.Sleep(2*time.Second)); err != nil {
		log.Fatalf("Sleep failed: %v", err)
	}

	// Install console error interceptor and dialog override in studio
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.__capturedErrors = [];
		const origErr = console.error;
		console.error = function(...args) {
			window.__capturedErrors.push(args.map(a => String(a)).join(' '));
			if (typeof origErr === 'function') origErr.apply(console, args);
		};
		window.addEventListener('error', function(e) {
			window.__capturedErrors.push(e.message || String(e));
		});
		return true;
	})()`))

	// Verify WASM engine ready
	wasmReady, err := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
	if err != nil || !wasmReady {
		log.Fatalf("WebAssembly engine is not ready on Kiwiberry Studio: %v", err)
	}
	fmt.Println(" > Go WebAssembly engine mounted and ready.")

	// 3. Test UI authoring & Compact Property Cards in Studio
	fmt.Println("\n[PHASE 2] Testing UI Authoring, Compact Cards & Renaming in Studio...")
	uiAuthoringOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		// Reset to clean project
		if (window.ProjectManager) window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);

		// Create a Node
		const n = window.DiagramNodes.createNode({ shape: 'rounded', text: 'Gateway', x: 200, y: 150, width: 140, height: 70 });
		if (!n) return false;

		// Select the Node
		window.AppCoordinator.selectObject('node', n.id);

		// Render compact cards into drawer container
		const container = document.getElementById('drawerWidgetPropertiesList');
		if (!container) return false;
		window.renderWidgetPropertyCards(container, { type: 'node', id: n.id, name: 'node-gateway' });

		const cards = container.querySelectorAll('.widget-prop-card');
		if (!cards || cards.length < 5) return false;

		// Rename object via EngineBridge
		const ren = window.EngineBridge.renameObject('node', n.id, 'node-primary-gw');
		if (!ren || !ren.success) return false;

		// Check derived name generator (standardised object-name-first contract)
		const derivedName = window.EngineBridge.generateParameterName('node-primary-gw', 'fill', 'node');
		return derivedName === 'primary-gw-node-fill' || derivedName === 'node-primary-gw-node-fill';
	})()`))

	if !uiAuthoringOk {
		log.Fatalf("UI authoring test in Studio failed")
	}
	fmt.Println(" > Studio compact cards rendered and auto-derived naming verified.")

	// Test Undo / Redo in Studio (Acceptance 10)
	undoRedoOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const initParam = window.EngineBridge.addWidgetParameter({
			name: 'node-gw-fill',
			type: 'color',
			target: 'node-1',
			property: 'fill',
			default: '#3b82f6'
		});
		if (!initParam) return false;

		window.AppCoordinator.recordHistoryImmediate();
		const countBefore = window.EngineBridge.getWidgetParameters().length;

		// Add another parameter
		window.EngineBridge.addWidgetParameter({
			name: 'node-gw-stroke',
			type: 'color',
			target: 'node-1',
			property: 'stroke',
			default: '#1d4ed8'
		});
		window.AppCoordinator.recordHistoryImmediate();
		const countAfterAdd = window.EngineBridge.getWidgetParameters().length;
		if (countAfterAdd !== countBefore + 1) return false;

		// Undo
		window.HistoryManager.undo();
		const countAfterUndo = window.EngineBridge.getWidgetParameters().length;
		if (countAfterUndo !== countBefore) return false;

		// Redo
		window.HistoryManager.redo();
		const countAfterRedo = window.EngineBridge.getWidgetParameters().length;
		return countAfterRedo === countAfterAdd;
	})()`))
	record(10, "Undo/Redo Verification", undoRedoOk, "Parameter addition correctly recorded and restored across undo/redo stack")

	// Verify zero UI console errors (Acceptance 9)
	var studioErrors []string
	errorsRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.__capturedErrors || [])`))
	_ = json.Unmarshal([]byte(errorsRaw), &studioErrors)
	record(9, "Studio UI Zero Console Errors", len(studioErrors) == 0, fmt.Sprintf("Studio produced %d console errors during authoring (%s)", len(studioErrors), strings.Join(studioErrors, "; ")))

	// 4. Construct scene with Node, Flow, Glow, Ripple and author widget package
	fmt.Println("\n[PHASE 3] Generating Stage 16B Package with Node, Flow, Glow, Ripple...")

	authoringScene := map[string]interface{}{
		"version": "1.2.0",
		"nodes": []map[string]interface{}{
			{
				"id":          1,
				"name":        "node-gateway",
				"shape":       "rounded",
				"text":        "Gateway Node",
				"x":           150,
				"y":           150,
				"width":       160,
				"height":      80,
				"fillColor":   "#1e293b",
				"strokeColor": "#38bdf8",
			},
		},
		"flows": []map[string]interface{}{
			{
				"id":        1,
				"name":      "flow-border",
				"color":     "#38bdf8",
				"animSpeed": 1.0,
			},
		},
		"glows": []map[string]interface{}{
			{
				"id":    1,
				"name":  "glow-status",
				"color": "#10b981",
			},
		},
		"ripples": []map[string]interface{}{
			{
				"id":    1,
				"name":  "ripple-sensor",
				"color": "#a855f7",
			},
		},
		"parameters": []map[string]interface{}{
			{
				"name":     "node-gateway-fill",
				"type":     "color",
				"target":   "node-1",
				"property": "fill",
				"default":  "#1e293b",
			},
			{
				"name":     "node-gateway-text",
				"type":     "text",
				"target":   "node-1",
				"property": "text",
				"default":  "Gateway Node",
			},
			{
				"name":     "flow-border-color",
				"type":     "color",
				"target":   "flow-1",
				"property": "color",
				"default":  "#38bdf8",
			},
			{
				"name":     "flow-border-speed",
				"type":     "speed",
				"target":   "flow-1",
				"property": "speed",
				"default":  1.0,
				"min":      0.1,
				"max":      5.0,
			},
			{
				"name":     "glow-status-color",
				"type":     "color",
				"target":   "glow-1",
				"property": "color",
				"default":  "#10b981",
			},
			{
				"name":     "ripple-sensor-color",
				"type":     "color",
				"target":   "ripple-1",
				"property": "color",
				"default":  "#a855f7",
			},
			{
				"name":     "root-speed",
				"type":     "speed",
				"target":   "root",
				"property": "playbackRate",
				"default":  1.0,
				"min":      0.1,
				"max":      3.0,
			},
		},
	}

	authoringSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 600" width="900" height="600">
  <defs>
    <radialGradient id="glowGrad-1">
      <stop offset="0%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#10b981" stop-opacity="0" />
    </radialGradient>
  </defs>
  <g id="nodes-layer">
    <g id="node-1" data-node-id="1" data-semantic-target="node-1">
      <rect x="100" y="100" width="160" height="80" rx="8" fill="#1e293b" stroke="#38bdf8" stroke-width="2"/>
      <text x="180" y="145" fill="#ffffff" font-size="14" text-anchor="middle">Gateway Node</text>
    </g>
  </g>
  <g id="flows-layer">
    <g id="flow-1" data-flow-id="1" data-semantic-target="flow-1">
      <!-- Invisible motion guide track path -->
      <path class="flow-track-path" stroke="none" fill="none" d="M 260 140 L 600 140" />
      <!-- Moving payload particles -->
      <g class="flow-payload-group">
        <circle cx="300" cy="140" r="6" fill="#38bdf8" stroke="#38bdf8" stroke-width="1" />
        <circle cx="450" cy="140" r="6" fill="#38bdf8" stroke="#38bdf8" stroke-width="1" />
      </g>
    </g>
  </g>
  <g id="glows-layer">
    <g id="glow-1" data-glow-id="1" data-semantic-target="glow-1">
      <!-- Transparent hit test target -->
      <rect class="glow-hit-target" fill="transparent" stroke="none" x="650" y="100" width="100" height="100" />
      <!-- Illuminated glow graphic -->
      <circle class="glow-core" fill="url(#glowGrad-1)" cx="700" cy="150" r="40" />
    </g>
  </g>
  <g id="ripples-layer">
    <g id="ripple-1" data-ripple-id="1" data-semantic-target="ripple-1">
      <!-- Transparent hit test target -->
      <rect class="ripple-hit-target" fill="transparent" stroke="none" x="400" y="300" width="100" height="100" />
      <!-- Shadow ring (black) -->
      <circle class="ripple-shadow-ring" stroke="#000000" stroke-width="2" fill="none" cx="450" cy="350" r="35" />
      <!-- Visible wave ring -->
      <circle class="ripple-wave" stroke="#a855f7" stroke-width="2" fill="none" cx="450" cy="350" r="25" />
    </g>
  </g>
</svg>`

	reqBytes, _ := json.Marshal(map[string]interface{}{
		"name":       "final-contract-widget",
		"svg":        authoringSVG,
		"sceneState": authoringScene,
	})

	resp, err := http.Post(apiBase+"/api/export/widget-package", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		log.Fatalf("Package export failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		log.Fatalf("Export status %d: %s", resp.StatusCode, string(errBody))
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(zipBytes) < 5000 {
		log.Fatalf("Invalid zip returned: %v (len=%d)", err, len(zipBytes))
	}
	fmt.Printf(" > Exported widget package ZIP (%d bytes).\n", len(zipBytes))

	// 5. Inspect ZIP contents directly (Requirement 8)
	fmt.Println("\n[PHASE 4] Inspecting Exported Manifest & Documentation...")
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		log.Fatalf("Failed to open zip reader: %v", err)
	}

	var manifestContent string
	var docsContent string
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, "widget-manifest.json") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			manifestContent = string(b)
		} else if strings.HasSuffix(f.Name, "-properties.md") || strings.HasSuffix(f.Name, "README.md") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			docsContent += string(b)
		}
	}

	fmt.Printf("DEBUG manifestContent: %s\n", manifestContent)

	var parsedManifest map[string]interface{}
	_ = json.Unmarshal([]byte(manifestContent), &parsedManifest)
	rawParams, _ := parsedManifest["parameters"].([]interface{})

	var hasInvalidColorLimits bool
	for _, rp := range rawParams {
		if pm, ok := rp.(map[string]interface{}); ok {
			if pm["type"] == "color" {
				if _, hasMin := pm["min"]; hasMin {
					hasInvalidColorLimits = true
				}
				if _, hasMax := pm["max"]; hasMax {
					hasInvalidColorLimits = true
				}
			}
		}
	}
	manifestValid := len(rawParams) == 7 &&
		(strings.Contains(manifestContent, "gateway-node-fill") || strings.Contains(manifestContent, "node-gateway-fill")) &&
		(strings.Contains(manifestContent, "border-flow-color") || strings.Contains(manifestContent, "flow-border-color")) &&
		(strings.Contains(manifestContent, "border-flow-speed") || strings.Contains(manifestContent, "flow-border-speed")) &&
		(strings.Contains(manifestContent, "status-glow-color") || strings.Contains(manifestContent, "glow-status-color")) &&
		(strings.Contains(manifestContent, "sensor-ripple-color") || strings.Contains(manifestContent, "ripple-sensor-color")) &&
		strings.Contains(manifestContent, "root-speed") &&
		!hasInvalidColorLimits

	docsValid := len(docsContent) > 500 &&
		(strings.Contains(docsContent, "border-flow-color") || strings.Contains(docsContent, "flow-border-color")) &&
		!strings.Contains(docsContent, `"NORMAL"`) // no bogus strings for boolean/speed

	record(8, "Manifest & Docs Accuracy", manifestValid && docsValid, fmt.Sprintf("Manifest contains %d valid parameters with strict types and zero invalid limits", len(rawParams)))

	// 6. Transfer to Kiwiberry and unpack
	fmt.Println("\n[PHASE 5] Unpacking Package on Kiwiberry Filesystem for file:// Testing...")
	localZip := "/tmp/final-contract.zip"
	_ = os.WriteFile(localZip, zipBytes, 0644)

	scpCmd := exec.Command("scp", localZip, fmt.Sprintf("nigel@%s:/tmp/final-contract.zip", kiwiberryHost))
	if out, err := scpCmd.CombinedOutput(); err != nil {
		log.Fatalf("SCP failed: %v, %s", err, string(out))
	}

	unzipScript := fmt.Sprintf("rm -rf %s && mkdir -p %s && unzip -q /tmp/final-contract.zip -d %s", remoteDemoDir, remoteDemoDir, remoteDemoDir)
	sshUnzip := exec.Command("ssh", fmt.Sprintf("nigel@%s", kiwiberryHost), unzipScript)
	if out, err := sshUnzip.CombinedOutput(); err != nil {
		log.Fatalf("SSH unzip failed: %v, %s", err, string(out))
	}
	fmt.Println(" > Unpacked package to Kiwiberry filesystem.")

	// 7. Open demo.html via direct file:// on Kiwiberry Chrome
	demoFileURL := fmt.Sprintf("file://%s/final-contract-widget/demo.html", remoteDemoDir)
	fmt.Printf("\n[PHASE 6] Opening %s via file:// in Chrome...\n", demoFileURL)

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(demoFileURL)); err != nil {
		log.Fatalf("CDP navigation to file:// failed: %v", err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-a svg")); err != nil {
		log.Fatalf("Instance SVG failed to mount in demo.html: %v", err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.Sleep(1*time.Second)); err != nil {
		log.Fatalf("Sleep failed: %v", err)
	}

	// Requirement 1: Direct file:// operation
	fileOpSuccess, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(document.querySelector('#container-unit-a svg') && document.querySelector('#container-unit-b svg') && document.querySelector('#container-unit-c svg'))`))
	record(1, "Direct file:// Operation", fileOpSuccess, "All 3 instances rendered directly from filesystem without a web server")

	// Requirement 2: Multi-instance isolation
	isolationReport, isoErr := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		if (typeof instances === 'undefined' || !instances['unit-a'] || !instances['unit-b'] || !instances['unit-c']) {
			return JSON.stringify({ error: 'instances missing' });
		}
		const svgA = instances['unit-a'].svgElement;
		const svgB = instances['unit-b'].svgElement;
		if (!svgA || !svgB) {
			return JSON.stringify({ error: 'svgElement missing' });
		}
		const allElsA = Array.from(svgA.querySelectorAll('*'));
		const idsA = allElsA.map(el => el.getAttribute('id')).filter(Boolean);
		const allElsB = Array.from(svgB.querySelectorAll('*'));
		const idsB = allElsB.map(el => el.getAttribute('id')).filter(Boolean);
		const overlap = idsA.filter(id => idsB.includes(id));
		return JSON.stringify({ idsA, idsB, overlap, elCount: allElsA.length });
	})()`))
	if isoErr != nil {
		fmt.Printf("DEBUG isoErr: %v\n", isoErr)
	}
	fmt.Printf("DEBUG isolationReport: %q\n", isolationReport)
	var isoData struct {
		IDsA    []string `json:"idsA"`
		IDsB    []string `json:"idsB"`
		Overlap []string `json:"overlap"`
		ElCount int      `json:"elCount"`
		Error   string   `json:"error"`
	}
	_ = json.Unmarshal([]byte(isolationReport), &isoData)
	multiInstanceIsolated := len(isoData.Overlap) == 0 && len(isoData.IDsA) > 0 && isoData.Error == ""
	record(2, "Multi-Instance Isolation", multiInstanceIsolated, fmt.Sprintf("Zero ID collisions between instance unit-a (%d IDs) and unit-b (%d IDs), overlap: %v", len(isoData.IDsA), len(isoData.IDsB), isoData.Overlap))

	// Requirement 3: Node recolour & text
	nodeControlOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		const fillParam = w.getParameter('gateway-node-fill') !== undefined ? 'gateway-node-fill' : 'node-gateway-fill';
		const textParam = w.getParameter('gateway-node-text') !== undefined ? 'gateway-node-text' : 'node-gateway-text';
		w.setParameter(fillParam, '#10b981');
		w.setParameter(textParam, 'Core Switch');
		const nodeRect = w.svgElement.querySelector('rect[fill="#10b981"]');
		const nodeText = w.svgElement.querySelector('text');
		return Boolean(nodeRect && nodeText && nodeText.textContent.includes('Core Switch'));
	})()`))
	record(3, "Node Recolour & Text Control", nodeControlOk, "Node fill changed to #10b981 and text updated to 'Core Switch'")

	// Requirement 4: Flow recolour affects payload only; guide path remains invisible (stroke="none")
	flowTargetingOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		const colParam = w.getParameter('border-flow-color') !== undefined ? 'border-flow-color' : 'flow-border-color';
		// Apply new flow color
		w.setParameter(colParam, '#f43f5e');

		const trackPath = w.svgElement.querySelector('.flow-track-path');
		const payloadCircles = w.svgElement.querySelectorAll('.flow-payload-group circle');

		// Guide path MUST remain stroke="none"
		const trackPreserved = trackPath && (trackPath.getAttribute('stroke') === 'none' || trackPath.style.stroke === 'none');

		// Payload circle MUST receive #f43f5e
		let payloadUpdated = false;
		payloadCircles.forEach(c => {
			if (c.getAttribute('fill') === '#f43f5e' || c.style.fill === '#f43f5e' || c.getAttribute('stroke') === '#f43f5e') {
				payloadUpdated = true;
			}
		});

		return Boolean(trackPreserved && payloadUpdated);
	})()`))
	record(4, "Flow Payload vs Guide Isolation", flowTargetingOk, "Payload recoloured to #f43f5e while .flow-track-path motion guide remains stroke=none")

	// Requirement 5: Flow speed changes only that flow, preserving master playback
	flowSpeedCoexistenceOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		const speedParam = w.getParameter('border-flow-speed') !== undefined ? 'border-flow-speed' : 'flow-border-speed';
		w.setParameter('root-speed', 1.5);
		w.setParameter(speedParam, 2.0);
		return w.getSpeed() === 1.5 && w.getParameter(speedParam) === 2.0;
	})()`))
	record(5, "Flow Speed vs Master Coexistence", flowSpeedCoexistenceOk, "Flow speed (2.0) changed independently while preserving master playbackRate (1.5)")

	// Requirement 6: Glow recolour changes glow without turning hit-target opaque
	glowReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const w = instances['unit-a'];
		const glowParam = w.getParameter('status-glow-color') !== undefined ? 'status-glow-color' : 'glow-status-color';
		w.setParameter(glowParam, '#fbbf24');

		const hitTarget = w.svgElement.querySelector('.glow-hit-target');
		const hitTransparent = hitTarget && (hitTarget.getAttribute('fill') === 'transparent' || hitTarget.style.fill === 'transparent');

		const stops = w.svgElement.querySelectorAll('radialGradient stop');
		let stopColors = [];
		stops.forEach(s => {
			stopColors.push(s.getAttribute('stop-color') || s.style.stopColor);
		});

		return JSON.stringify({ hitTransparent, stopColors, gradCount: w.svgElement.querySelectorAll('radialGradient').length });
	})()`))
	var glowData struct {
		HitTransparent bool     `json:"hitTransparent"`
		StopColors     []string `json:"stopColors"`
		GradCount      int      `json:"gradCount"`
	}
	_ = json.Unmarshal([]byte(glowReport), &glowData)
	var glowStopUpdated bool
	for _, c := range glowData.StopColors {
		if c == "#fbbf24" {
			glowStopUpdated = true
		}
	}
	glowTargetingOk := glowData.HitTransparent && glowStopUpdated
	record(6, "Glow Hit-Target Preservation", glowTargetingOk, fmt.Sprintf("Glow hit-target transparent=%v, stops=%v, updated=%v", glowData.HitTransparent, glowData.StopColors, glowStopUpdated))

	// Requirement 7: Ripple recolour changes wave crests without touching hit-target or shadow rings
	rippleTargetingOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		const ripParam = w.getParameter('sensor-ripple-color') !== undefined ? 'sensor-ripple-color' : 'ripple-sensor-color';
		w.setParameter(ripParam, '#ec4899');

		const hitTarget = w.svgElement.querySelector('.ripple-hit-target');
		const hitTransparent = hitTarget && (hitTarget.getAttribute('fill') === 'transparent' || hitTarget.style.fill === 'transparent');

		const shadowRing = w.svgElement.querySelector('.ripple-shadow-ring');
		const shadowBlack = shadowRing && (shadowRing.getAttribute('stroke') === '#000000' || shadowRing.style.stroke === '#000000');

		const waveRing = w.svgElement.querySelector('.ripple-wave');
		const waveRecoloured = waveRing && (waveRing.getAttribute('stroke') === '#ec4899' || waveRing.style.stroke === '#ec4899');

		return Boolean(hitTransparent && shadowBlack && waveRecoloured);
	})()`))
	record(7, "Ripple Wave vs Hit/Shadow Isolation", rippleTargetingOk, "Wave crest recoloured to #ec4899; hit-target remains transparent; shadow ring remains #000000")

	// Final Summary
	fmt.Println("\n==================================================================")
	fmt.Println(" STAGE 16B FINAL ACCEPTANCE VERIFICATION SUMMARY")
	fmt.Println("==================================================================")
	allPassed := true
	for _, res := range results {
		s := "PASS"
		if !res.Passed {
			s = "FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] Criteria %2d: %-34s : %s\n", s, res.Number, res.Name, res.Message)
	}

	if allPassed && len(results) == 10 {
		fmt.Println("\n>>> ALL 10 STAGE 16B ACCEPTANCE CRITERIA PASS 100% ON KIWIBERRY <<<")
	} else {
		fmt.Printf("\n>>> ACCEPTANCE FAILED: %d of 10 criteria passed <<<\n", len(results))
		os.Exit(1)
	}
}
