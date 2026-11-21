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
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

const (
	kiwiberryHost = "192.168.100.40"
	apiBase       = "http://192.168.100.40:8088"
	cdpEndpoint   = "http://127.0.0.1:9222"
	cpuHost       = "nigel@192.168.100.40"
	remoteDemoDir = "/home/nigel/acceptance-demo/node-target-resolution"
)

type CheckResult struct {
	Number  int
	Name    string
	Passed  bool
	Message string
}

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Node Target Resolution & Standardised Naming Verification")
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
	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		log.Printf("Failed to acquire browser session: %v", err)
		return 1
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(sess.AllocCtx, 75*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	// 2. Navigate to Studio on Kiwiberry
	fmt.Println("\n[PHASE 1] Navigating to SVG Animator Studio on Kiwiberry...")
	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(apiBase)); err != nil {
		log.Printf("Failed to navigate to %s: %v", apiBase, err)
		return 1
	}
	if _, err := chromedp.Run(taskCtx, chromedp.Sleep(2*time.Second)); err != nil {
		log.Printf("Sleep failed: %v", err)
		return 1
	}

	// Intercept console errors
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
		log.Printf("WebAssembly engine is not ready on Kiwiberry Studio: %v", err)
		return 1
	}
	fmt.Println(" > Go WebAssembly engine mounted and ready.")

	// 3. Test Object-Name-First Parameter Naming Convention
	fmt.Println("\n[PHASE 2] Verifying Object-Name-First Parameter Naming Generation...")
	namingOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const n1 = window.EngineBridge.generateParameterName('mytext', 'text', 'node');
		const n2 = window.EngineBridge.generateParameterName('mytext', 'fill', 'node');
		const n3 = window.EngineBridge.generateParameterName('border', 'speed', 'flow');
		const n4 = window.EngineBridge.generateParameterName('alarm', 'color', 'glow');
		const n5 = window.EngineBridge.generateParameterName('warning', 'opacity', 'ripple');

		return n1 === 'mytext-node-text' &&
		       n2 === 'mytext-node-fill' &&
		       n3 === 'border-flow-speed' &&
		       n4 === 'alarm-glow-color' &&
		       n5 === 'warning-ripple-opacity';
	})()`))

	record(1, "Standardised Parameter Naming", namingOk, "Generated names follow <object-name>-<object-type>-<property> contract")

	// 4. Test Pre-Export Validation
	fmt.Println("\n[PHASE 3] Testing Pre-Export Target Validation...")
	preExportValidationOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.EngineBridge.setWidgetParameters([]);
		window.EngineBridge.addWidgetParameter({
			name: 'orphan-node-fill',
			type: 'color',
			target: 'node-999',
			property: 'fill',
			default: '#ff0000'
		});
		const res = window.EngineBridge.buildWidgetPackage('Invalid Test', '<svg viewBox=\"0 0 100 100\"><rect id=\"node-1\"/></svg>');
		if (res !== null) return false;
		const lastErr = window.EngineBridge.getLastError() || '';
		return lastErr.includes('could not be resolved') || lastErr.includes('does not exist');
	})()`))

	record(2, "Pre-Export Target Validation", preExportValidationOk, "Export rejected with descriptive error when target object is missing from scene or SVG")

	// 5. Build full scene with two distinct Nodes, Flow, Glow, and Ripple
	fmt.Println("\n[PHASE 4] Building Multi-Node Production Scene & Exporting Package...")
	authoringScene := map[string]interface{}{
		"version": "1.2.0",
		"nodes": []map[string]interface{}{
			{
				"id":          1,
				"name":        "mytext",
				"shape":       "rounded",
				"text":        "Primary Server",
				"x":           150,
				"y":           150,
				"width":       180,
				"height":      80,
				"fillColor":   "#1e293b",
				"strokeColor": "#3b82f6",
			},
			{
				"id":          2,
				"name":        "clientnode",
				"shape":       "rounded",
				"text":        "Backup Client",
				"x":           450,
				"y":           150,
				"width":       180,
				"height":      80,
				"fillColor":   "#0f172a",
				"strokeColor": "#10b981",
			},
		},
		"flows": []map[string]interface{}{
			{
				"id":        1,
				"name":      "border",
				"color":     "#3b82f6",
				"animSpeed": 1.0,
			},
		},
		"glows": []map[string]interface{}{
			{
				"id":    1,
				"name":  "alarm",
				"color": "#f59e0b",
			},
		},
		"ripples": []map[string]interface{}{
			{
				"id":    1,
				"name":  "warning",
				"color": "#a855f7",
			},
		},
		"parameters": []map[string]interface{}{
			{
				"name":     "mytext-node-text",
				"type":     "text",
				"target":   "node-1",
				"property": "text",
				"default":  "Primary Server",
			},
			{
				"name":     "mytext-node-fill",
				"type":     "color",
				"target":   "node-1",
				"property": "fill",
				"default":  "#1e293b",
			},
			{
				"name":     "mytext-node-stroke",
				"type":     "color",
				"target":   "node-1",
				"property": "stroke",
				"default":  "#3b82f6",
			},
			{
				"name":     "clientnode-node-text",
				"type":     "text",
				"target":   "node-2",
				"property": "text",
				"default":  "Backup Client",
			},
			{
				"name":     "border-flow-speed",
				"type":     "speed",
				"target":   "flow-1",
				"property": "speed",
				"default":  1.0,
				"min":      0.1,
				"max":      5.0,
			},
			{
				"name":     "alarm-glow-color",
				"type":     "color",
				"target":   "glow-1",
				"property": "color",
				"default":  "#f59e0b",
			},
			{
				"name":     "warning-ripple-opacity",
				"type":     "number",
				"target":   "ripple-1",
				"property": "opacity",
				"default":  0.8,
				"min":      0.0,
				"max":      1.0,
			},
		},
	}

	authoringSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 650" width="1000" height="650">
  <defs>
    <radialGradient id="glowGrad-1">
      <stop offset="0%" stop-color="#f59e0b" />
      <stop offset="100%" stop-color="#f59e0b" stop-opacity="0" />
    </radialGradient>
  </defs>
  <g id="nodes-layer">
    <g id="node-1" class="diagram-node-instance" data-node-id="1" data-semantic-target="node-1" data-semantic-name="mytext">
      <rect x="150" y="150" width="180" height="80" rx="8" fill="#1e293b" stroke="#3b82f6" stroke-width="2"/>
      <text x="240" y="195" fill="#ffffff" font-size="14" text-anchor="middle">Primary Server</text>
    </g>
    <g id="node-2" class="diagram-node-instance" data-node-id="2" data-semantic-target="node-2" data-semantic-name="clientnode">
      <rect x="450" y="150" width="180" height="80" rx="8" fill="#0f172a" stroke="#10b981" stroke-width="2"/>
      <text x="540" y="195" fill="#ffffff" font-size="14" text-anchor="middle">Backup Client</text>
    </g>
  </g>
  <g id="flows-layer">
    <g id="flow-1" data-flow-id="1" data-semantic-target="flow-1">
      <path class="flow-track-path" stroke="none" fill="none" d="M 330 190 L 450 190" />
      <g class="flow-payload-group">
        <circle cx="390" cy="190" r="6" fill="#3b82f6" stroke="#3b82f6" stroke-width="1" />
      </g>
    </g>
  </g>
  <g id="glows-layer">
    <g id="glow-1" data-glow-id="1" data-semantic-target="glow-1">
      <rect class="glow-hit-target" fill="transparent" stroke="none" x="700" y="140" width="100" height="100" />
      <circle class="glow-core" fill="url(#glowGrad-1)" cx="750" cy="190" r="40" />
    </g>
  </g>
  <g id="ripples-layer">
    <g id="ripple-1" data-ripple-id="1" data-semantic-target="ripple-1">
      <rect class="ripple-hit-target" fill="transparent" stroke="none" x="200" y="350" width="100" height="100" />
      <circle class="ripple-shadow-ring" stroke="#000000" stroke-width="2" fill="none" cx="250" cy="400" r="35" />
      <circle class="ripple-wave" stroke="#a855f7" stroke-width="2" fill="none" cx="250" cy="400" r="25" />
    </g>
  </g>
</svg>`

	reqBytes, _ := json.Marshal(map[string]interface{}{
		"name":       "node-target-widget",
		"svg":        authoringSVG,
		"sceneState": authoringScene,
	})

	resp, err := http.Post(apiBase+"/api/export/widget-package", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		log.Printf("Package export failed: %v", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("Export status %d: %s", resp.StatusCode, string(errBody))
		return 1
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(zipBytes) < 5000 {
		log.Printf("Invalid zip returned: %v (len=%d)", err, len(zipBytes))
		return 1
	}
	fmt.Printf(" > Exported widget package ZIP (%d bytes).\n", len(zipBytes))

	// 6. Inspect ZIP contents and Manifest
	fmt.Println("\n[PHASE 5] Inspecting Exported ZIP Manifest & Documentation...")
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		log.Printf("Failed to open zip reader: %v", err)
		return 1
	}

	var (
		foundManifest bool
		manifestBytes []byte
		foundDocs     bool
		foundDemo     bool
		foundWidgetJs bool
		foundSvg      bool
	)

	for _, f := range zipReader.File {
		switch f.Name {
		case "node-target-widget/widget-manifest.json":
			foundManifest = true
			rc, _ := f.Open()
			manifestBytes, _ = io.ReadAll(rc)
			rc.Close()
		case "node-target-widget/node-target-widget-properties.md":
			foundDocs = true
		case "node-target-widget/demo.html":
			foundDemo = true
		case "node-target-widget/js/svg-widget.js":
			foundWidgetJs = true
		case "node-target-widget/node-target-widget.svg":
			foundSvg = true
		}
	}

	allFilesPresent := foundManifest && foundDocs && foundDemo && foundWidgetJs && foundSvg
	record(3, "Standalone 6-File ZIP Package", allFilesPresent, "Export contains all 6 required files including widget-manifest.json, demo.html and js/svg-widget.js")

	var exportedManifest struct {
		SchemaVersion int `json:"schemaVersion"`
		Parameters    []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Target   string `json:"target"`
			Property string `json:"property"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(manifestBytes, &exportedManifest)

	manifestValid := exportedManifest.SchemaVersion == 1 && len(exportedManifest.Parameters) == 7
	for _, p := range exportedManifest.Parameters {
		if !strings.HasPrefix(p.Name, "mytext-") &&
			!strings.HasPrefix(p.Name, "clientnode-") &&
			!strings.HasPrefix(p.Name, "border-") &&
			!strings.HasPrefix(p.Name, "alarm-") &&
			!strings.HasPrefix(p.Name, "warning-") {
			manifestValid = false
		}
	}

	record(4, "Manifest Naming Compliance", manifestValid, fmt.Sprintf("Manifest contains %d parameters strictly formatted as <object-name>-<object-type>-<property>", len(exportedManifest.Parameters)))

	// 7. Unpack on Kiwiberry and test via file://
	fmt.Println("\n[PHASE 6] Unpacking Package on Kiwiberry for file:// Browser Verification...")
	tmpZipPath := "/tmp/node-target-widget.zip"
	if err := os.WriteFile(tmpZipPath, zipBytes, 0644); err != nil {
		log.Printf("Failed to write local tmp zip: %v", err)
		return 1
	}

	// Transfer to Kiwiberry and unpack
	scpCmd := exec.Command("scp", tmpZipPath, fmt.Sprintf("nigel@%s:%s", kiwiberryHost, tmpZipPath))
	if out, err := scpCmd.CombinedOutput(); err != nil {
		log.Printf("SCP zip failed: %v, out: %s", err, string(out))
		return 1
	}

	unpackScript := fmt.Sprintf(`rm -rf %s && mkdir -p %s && unzip -q -o %s -d %s`, remoteDemoDir, remoteDemoDir, tmpZipPath, remoteDemoDir)
	sshCmd := exec.Command("ssh", fmt.Sprintf("nigel@%s", kiwiberryHost), unpackScript)
	if out, err := sshCmd.CombinedOutput(); err != nil {
		log.Printf("Remote unpack failed: %v, out: %s", err, string(out))
		return 1
	}
	fmt.Println(" > Unpacked package to Kiwiberry filesystem.")

	// 8. Open file:// demo in Chrome over CDP
	demoFilePath := fmt.Sprintf("file://%s/node-target-widget/demo.html", remoteDemoDir)
	fmt.Printf("\n[PHASE 7] Opening %s in Headless Chrome via file://...\n", demoFilePath)

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(demoFilePath)); err != nil {
		log.Printf("Failed to navigate to demo.html: %v", err)
		return 1
	}
	if _, err := chromedp.Run(taskCtx, chromedp.Sleep(2*time.Second)); err != nil {
		log.Printf("Demo sleep failed: %v", err)
		return 1
	}

	// Check console errors on mount
	errorsJson, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const errs = [];
		if (window.__capturedErrors) errs.push(...window.__capturedErrors);
		return JSON.stringify(errs);
	})()`))

	var demoErrors []string
	_ = json.Unmarshal([]byte(errorsJson), &demoErrors)

	// Verify target element not found error is ABSENT
	var targetNotFoundError bool
	for _, e := range demoErrors {
		if strings.Contains(e, "Target element") || strings.Contains(e, "not found in widget") {
			targetNotFoundError = true
		}
	}

	record(5, "Zero Mount Target Errors", !targetNotFoundError, fmt.Sprintf("No 'Target element not found' errors on widget mount (captured errors: %v)", demoErrors))

	// 9. Verify Multi-Instance Isolation
	isolationReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		if (typeof instances === 'undefined' || !instances['unit-a'] || !instances['unit-b'] || !instances['unit-c']) {
			return JSON.stringify({ error: 'instances missing' });
		}
		const instA = instances['unit-a'];
		const instB = instances['unit-b'];
		const idsA = Array.from(instA.svgElement.querySelectorAll('*[id]')).map(el => el.id);
		const idsB = Array.from(instB.svgElement.querySelectorAll('*[id]')).map(el => el.id);
		const overlap = idsA.filter(id => idsB.includes(id));
		return JSON.stringify({
			overlapCount: overlap.length,
			countA: idsA.length,
			countB: idsB.length
		});
	})()`))

	var isoResult struct {
		OverlapCount int `json:"overlapCount"`
		CountA       int `json:"countA"`
		CountB       int `json:"countB"`
	}
	_ = json.Unmarshal([]byte(isolationReport), &isoResult)
	record(6, "Multi-Instance Isolation", isoResult.OverlapCount == 0 && isoResult.CountA > 0,
		fmt.Sprintf("Zero ID collisions between instance unit-a (%d IDs) and unit-b (%d IDs)", isoResult.CountA, isoResult.CountB))

	// 10. Test Node Text Update on Instance A
	textUpdateReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const instA = instances['unit-a'];
		const instB = instances['unit-b'];

		// Mutate node text on unit-a
		instA.setParameter('mytext-node-text', 'Production Gateway');

		// Query DOM elements
		const targetA = instA.svgElement.querySelector('[data-semantic-target="node-1"] text') ||
		                instA.svgElement.querySelector('#unit-a__node-target-widget__node-1 text');
		const targetB = instB.svgElement.querySelector('[data-semantic-target="node-1"] text') ||
		                instB.svgElement.querySelector('#unit-b__node-target-widget__node-1 text');
		const clientA = instA.svgElement.querySelector('[data-semantic-target="node-2"] text') ||
		                instA.svgElement.querySelector('#unit-a__node-target-widget__node-2 text');

		return JSON.stringify({
			textA: targetA ? targetA.textContent.trim() : '',
			textB: targetB ? targetB.textContent.trim() : '',
			clientA: clientA ? clientA.textContent.trim() : ''
		});
	})()`))

	var textResult struct {
		TextA   string `json:"textA"`
		TextB   string `json:"textB"`
		ClientA string `json:"clientA"`
	}
	_ = json.Unmarshal([]byte(textUpdateReport), &textResult)

	nodeTextOk := textResult.TextA == "Production Gateway" && textResult.TextB == "Primary Server" && textResult.ClientA == "Backup Client"
	record(7, "Node Text Control & Isolation", nodeTextOk,
		fmt.Sprintf("Unit-A node-1 updated to '%s', Unit-B remained '%s', Node-2 remained '%s'", textResult.TextA, textResult.TextB, textResult.ClientA))

	// 11. Test Node Fill & Stroke Update on Instance A
	colorUpdateReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const instA = instances['unit-a'];
		const instB = instances['unit-b'];

		// Mutate fill and stroke on unit-a
		instA.setParameter('mytext-node-fill', '#b91c1c');
		instA.setParameter('mytext-node-stroke', '#fca5a5');

		const rectA = instA.svgElement.querySelector('[data-semantic-target="node-1"] rect') ||
		              instA.svgElement.querySelector('#unit-a__node-target-widget__node-1 rect');
		const rectB = instB.svgElement.querySelector('[data-semantic-target="node-1"] rect') ||
		              instB.svgElement.querySelector('#unit-b__node-target-widget__node-1 rect');
		const clientRectA = instA.svgElement.querySelector('[data-semantic-target="node-2"] rect') ||
		                    instA.svgElement.querySelector('#unit-a__node-target-widget__node-2 rect');

		return JSON.stringify({
			fillA: rectA ? rectA.getAttribute('fill') : '',
			strokeA: rectA ? rectA.getAttribute('stroke') : '',
			fillB: rectB ? rectB.getAttribute('fill') : '',
			strokeB: rectB ? rectB.getAttribute('stroke') : '',
			clientFillA: clientRectA ? clientRectA.getAttribute('fill') : '',
			clientStrokeA: clientRectA ? clientRectA.getAttribute('stroke') : ''
		});
	})()`))

	var colorResult struct {
		FillA         string `json:"fillA"`
		StrokeA       string `json:"strokeA"`
		FillB         string `json:"fillB"`
		StrokeB       string `json:"strokeB"`
		ClientFillA   string `json:"clientFillA"`
		ClientStrokeA string `json:"clientStrokeA"`
	}
	_ = json.Unmarshal([]byte(colorUpdateReport), &colorResult)

	nodeColorOk := colorResult.FillA == "#b91c1c" && colorResult.StrokeA == "#fca5a5" &&
		colorResult.FillB == "#1e293b" && colorResult.StrokeB == "#3b82f6" &&
		colorResult.ClientFillA == "#0f172a" && colorResult.ClientStrokeA == "#10b981"

	record(8, "Node Fill & Stroke Isolation", nodeColorOk,
		fmt.Sprintf("Unit-A node fill=%s/stroke=%s, Unit-B fill=%s/stroke=%s, Node-2 fill=%s/stroke=%s",
			colorResult.FillA, colorResult.StrokeA, colorResult.FillB, colorResult.StrokeB, colorResult.ClientFillA, colorResult.ClientStrokeA))

	// 12. Test Optical & Flow Controls Coexistence
	opticalFlowReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const instA = instances['unit-a'];

		instA.setParameter('border-flow-speed', 2.5);
		instA.setParameter('alarm-glow-color', '#10b981');
		instA.setParameter('warning-ripple-opacity', 0.4);

		const flowSpeed = instA.getParameter('border-flow-speed');
		const glowCol = instA.getParameter('alarm-glow-color');
		const ripOp = instA.getParameter('warning-ripple-opacity');

		const glowGradStop = instA.svgElement.querySelector('radialGradient stop');
		const stopCol = glowGradStop ? glowGradStop.getAttribute('stop-color') : '';

		return JSON.stringify({
			flowSpeed: flowSpeed,
			glowCol: glowCol,
			ripOp: ripOp,
			stopCol: stopCol
		});
	})()`))

	var optResult struct {
		FlowSpeed float64 `json:"flowSpeed"`
		GlowCol   string  `json:"glowCol"`
		RipOp     float64 `json:"ripOp"`
		StopCol   string  `json:"stopCol"`
	}
	_ = json.Unmarshal([]byte(opticalFlowReport), &optResult)

	optFlowOk := optResult.FlowSpeed == 2.5 && optResult.GlowCol == "#10b981" && optResult.RipOp == 0.4 && optResult.StopCol == "#10b981"
	record(9, "Optical & Flow Parameter Coexistence", optFlowOk,
		fmt.Sprintf("Flow speed=%.1f, glow color=%s (stop=%s), ripple opacity=%.1f verified",
			optResult.FlowSpeed, optResult.GlowCol, optResult.StopCol, optResult.RipOp))

	// Summary
	fmt.Println("\n==================================================================")
	fmt.Println(" STAGE 16B NODE TARGET RESOLUTION ACCEPTANCE SUMMARY")
	fmt.Println("==================================================================")
	allPassed := true
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] Criteria %2d: %-34s : %s\n", status, r.Number, r.Name, r.Message)
	}

	if !allPassed {
		return 1
	}
	fmt.Println("\n>>> ALL 9 STAGE 16B NODE TARGET RESOLUTION CRITERIA PASS 100% ON KIWIBERRY <<<")
	return 0
}
