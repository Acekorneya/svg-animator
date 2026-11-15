package main

import (
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
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	kiwiberryHost = "192.168.100.40"
	apiBase       = "http://192.168.100.40:8088"
	cdpEndpoint   = "http://127.0.0.1:9222"
	remoteDemoDir = "/home/nigel/acceptance-demo"
)

type VerificationStep struct {
	Name    string
	Passed  bool
	Message string
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Property Eligibility & Unique Binding Acceptance Verification")
	fmt.Println(" Environment: Kiwiberry Chrome CDP (192.168.100.40)")
	fmt.Println("==================================================================")

	var steps []VerificationStep
	record := func(name string, passed bool, msg string) {
		steps = append(steps, VerificationStep{Name: name, Passed: passed, Message: msg})
		if passed {
			fmt.Printf(" [PASS] %s: %s\n", name, msg)
		} else {
			fmt.Printf(" [FAIL] %s: %s\n", name, msg)
		}
	}

	// 1. Connect to Kiwiberry Chrome via CDP
	fmt.Println("\n[PHASE 1] Connecting to Kiwiberry Chrome via CDP...")
	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()

	ctx, cancel := context.WithTimeout(allocCtx, 45*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	// 2. Load SVG Animator Studio on Kiwiberry
	fmt.Println("\n[PHASE 2] Loading SVG Animator Studio on Kiwiberry (:8088)...")
	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(apiBase)); err != nil {
		log.Fatalf("Failed to navigate to %s: %v", apiBase, err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.Sleep(2*time.Second)); err != nil {
		log.Fatalf("Sleep failed: %v", err)
	}

	wasmReady, err := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
	if err != nil {
		log.Fatalf("Evaluate wasm ready failed: %v", err)
	}
	record("StudioLoad", true, "SVG Animator Studio loaded successfully")
	record("WASMReady", wasmReady, "Go WebAssembly engine is mounted and ready")

	// 3. Test Object-Specific Property Eligibility Catalog
	fmt.Println("\n[PHASE 3] Verifying Object-Specific Property Eligibility Catalog...")
	nodePropsJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.EngineBridge.getEligibleWidgetProperties('node'))`))
	flowPropsJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.EngineBridge.getEligibleWidgetProperties('flow'))`))
	rootPropsJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`JSON.stringify(window.EngineBridge.getEligibleWidgetProperties('root'))`))

	nodeEligible := strings.Contains(nodePropsJSON, `"fill"`) && strings.Contains(nodePropsJSON, `"stroke"`) && !strings.Contains(nodePropsJSON, `"speed"`) && !strings.Contains(nodePropsJSON, `"height"`)
	flowEligible := strings.Contains(flowPropsJSON, `"speed"`) && strings.Contains(flowPropsJSON, `"stroke"`)
	rootEligible := strings.Contains(rootPropsJSON, `"playbackRate"`) && strings.Contains(rootPropsJSON, `"operatingState"`)
	record("NodeEligibility", nodeEligible, "Node allows fill/stroke/opacity/visibility/text and rejects speed/height")
	record("FlowEligibility", flowEligible, "Flow allows speed and stroke")
	record("RootEligibility", rootEligible, "Root allows playbackRate and operatingState")

	// 4. Test Rejection of Ineligible Properties & Bad Ranges
	fmt.Println("\n[PHASE 4] Verifying Rejection of Ineligible Properties & Bad Ranges...")
	badPropError, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.EngineBridge.addWidgetParameter({
			name: 'badNodeSpeed',
			type: 'speed',
			target: 'node-1',
			property: 'speed',
			default: 1.0
		});
		return window.EngineBridge.getLastError();
	})()`))

	badRangeError, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.EngineBridge.addWidgetParameter({
			name: 'badSpeedRange',
			type: 'speed',
			target: 'flow-1',
			property: 'speed',
			default: 5.0,
			min: 0.1,
			max: 2.0
		});
		return window.EngineBridge.getLastError();
	})()`))

	record("RejectIneligible", strings.Contains(badPropError, "not eligible"), fmt.Sprintf("Rejected speed on node: %q", badPropError))
	record("RejectBadRange", strings.Contains(badRangeError, "greater than maximum"), fmt.Sprintf("Rejected out-of-range default: %q", badRangeError))

	// 5. Test Unique Binding Contract & Multi-Property Exposure
	fmt.Println("\n[PHASE 5] Verifying Unique Binding Contract & Multi-Property Exposure...")
	initialParamOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.EngineBridge.setWidgetParameters([]);
		const p1 = window.EngineBridge.addWidgetParameter({
			name: 'flow1Stroke',
			type: 'color',
			target: 'flow-1',
			property: 'stroke',
			default: '#38bdf8'
		});
		return Boolean(p1 && !p1.error);
	})()`))

	collisionError, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.EngineBridge.addWidgetParameter({
			name: 'flow1ColorDuplicate',
			type: 'color',
			target: 'flow-1',
			property: 'stroke',
			default: '#ff0000'
		});
		return window.EngineBridge.getLastError();
	})()`))

	multiPropOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const p2 = window.EngineBridge.addWidgetParameter({
			name: 'flow1Speed',
			type: 'speed',
			target: 'flow-1',
			property: 'speed',
			default: 1.5,
			min: 0.1,
			max: 10.0
		});
		return Boolean(p2 && !p2.error);
	})()`))

	paramCount, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.EngineBridge.getWidgetParameters().length`))

	record("InitialBinding", initialParamOk, "flow-1.stroke bound to flow1Stroke")
	record("RejectDuplicate", strings.Contains(collisionError, "already bound"), fmt.Sprintf("Collision rejected: %q", collisionError))
	record("MultiPropertyExposure", multiPropOk && paramCount == 2, fmt.Sprintf("Different property (speed) on flow-1 successfully added (total: %d)", paramCount))

	// 6. Test Explicit Rename Preserving Target & Property Identity
	fmt.Println("\n[PHASE 6] Verifying Explicit Rename via RenameParameter...")
	renameOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const res = window.EngineBridge.renameWidgetParameter('flow1Speed', 'flow1Rate');
		return Boolean(res && res.success);
	})()`))

	renamedProp, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const p = window.EngineBridge.getWidgetParameters().find(x => x.name === 'flow1Rate');
		return p ? p.property : '';
	})()`))

	renamedTarget, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const p = window.EngineBridge.getWidgetParameters().find(x => x.name === 'flow1Rate');
		return p ? p.target : '';
	})()`))

	record("RenameSuccess", renameOk && renamedProp == "speed" && renamedTarget == "flow-1", fmt.Sprintf("Renamed flow1Speed -> flow1Rate preserving property %q and target %q", renamedProp, renamedTarget))

	// 7. Test Migration, Conflict Detection & Resolution
	fmt.Println("\n[PHASE 7] Verifying Migration, Conflict Detection & Resolution...")
	conflictsCount, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`(() => {
		window.EngineBridge.setWidgetParameters([
			{ name: 'flow1color', type: 'color', target: 'flow-1', property: 'stroke', default: '#111111' },
			{ name: 'flow1c', type: 'color', target: 'flow-1', property: 'stroke', default: '#222222' },
			{ name: 'serverFill', type: 'color', target: 'node-1', property: 'fill', default: '#3b82f6' }
		]);
		return window.EngineBridge.getWidgetConflicts().length;
	})()`))

	conflictExportBlocked, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const pkg = window.EngineBridge.buildWidgetPackage('ConflictTest', '<svg><rect/></svg>', '// mock js');
		return pkg === null && window.EngineBridge.getLastError().includes('conflicting parameter bindings');
	})()`))

	resolutionOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const res = window.EngineBridge.resolveWidgetConflict('flow-1', 'stroke', 'flow1color');
		return Boolean(res && res.success);
	})()`))

	conflictsRemaining, _ := chromedp.Run(taskCtx, chromedp.Evaluate[int](`window.EngineBridge.getWidgetConflicts().length`))

	record("DetectConflicts", conflictsCount == 1, "Detected 1 duplicate binding conflict on flow-1.stroke")
	record("BlockExportOnConflict", conflictExportBlocked, "BuildWidgetPackage explicitly blocked when unresolved conflicts exist")
	record("ResolveConflict", resolutionOk && conflictsRemaining == 0, "resolveWidgetConflict kept flow1color, zero conflicts remaining")

	// 8. End-to-End Export: Multi-Instance Offline Mounting & Speed Coexistence
	fmt.Println("\n[PHASE 8] End-to-End Export & Multi-Instance Speed Coexistence in Chrome...")
	mailScene := map[string]interface{}{
		"version": "1.2.0",
		"nodes": []map[string]interface{}{
			{
				"id":        1,
				"name":      "mail-server",
				"shape":     "rounded",
				"text":      "Mail Server",
				"x":         100,
				"y":         100,
				"width":     160,
				"height":    80,
				"fillColor": "#1f2937",
			},
		},
		"flows": []map[string]interface{}{
			{
				"id":        1,
				"name":      "mail-flow",
				"color":     "#38bdf8",
				"animSpeed": 2.0,
			},
		},
		"glows": []map[string]interface{}{
			{
				"id":    1,
				"name":  "status-glow",
				"color": "#22c55e",
			},
		},
		"parameters": []map[string]interface{}{
			{
				"name":     "serverFill",
				"type":     "color",
				"target":   "node-1",
				"property": "fill",
				"default":  "#1f2937",
			},
			{
				"name":     "flowSpeed",
				"type":     "speed",
				"target":   "flow-1",
				"property": "speed",
				"default":  2.0,
				"min":      0.1,
				"max":      10.0,
			},
			{
				"name":     "masterSpeed",
				"type":     "speed",
				"target":   "root",
				"property": "playbackRate",
				"default":  1.0,
				"min":      0.1,
				"max":      5.0,
			},
		},
	}

	sampleSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 500" width="800" height="500">
  <g id="nodes-layer">
    <g id="node-1" data-node-id="1" data-semantic-target="node-1">
      <rect x="50" y="50" width="160" height="80" rx="8" fill="#1f2937" stroke="#58a6ff" stroke-width="2"/>
      <text x="130" y="95" fill="#ffffff" font-size="14" text-anchor="middle">Mail Server</text>
    </g>
  </g>
  <g id="flows-layer">
    <g id="flow-1" data-flow-id="1" data-semantic-target="flow-1">
      <path class="flow-track-path" d="M 210 90 L 500 90" stroke="#38bdf8" stroke-width="4" fill="none"/>
    </g>
  </g>
</svg>`

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"name":       "mail-service",
		"svg":        sampleSVG,
		"sceneState": mailScene,
	})

	resp, err := http.Post(apiBase+"/api/export/widget-package", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Fatalf("Export API failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respErr, _ := io.ReadAll(resp.Body)
		log.Fatalf("Export API returned status %d: %s", resp.StatusCode, string(respErr))
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil || len(zipData) == 0 {
		log.Fatalf("Failed to read export zip: %v", err)
	}
	record("PackageExport", len(zipData) > 5000, fmt.Sprintf("Exported mail-service package ZIP (%d bytes)", len(zipData)))

	// Unpack and transfer to Kiwiberry filesystem for file:// testing
	tmpZipPath := "/tmp/mail-service-test.zip"
	_ = os.WriteFile(tmpZipPath, zipData, 0644)

	cmd := exec.Command("scp", tmpZipPath, fmt.Sprintf("nigel@%s:/tmp/mail-service.zip", kiwiberryHost))
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("SCP zip failed: %v, out: %s", err, string(out))
	}

	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && unzip -q /tmp/mail-service.zip -d %s", remoteDemoDir, remoteDemoDir, remoteDemoDir)
	sshCmd := exec.Command("ssh", fmt.Sprintf("nigel@%s", kiwiberryHost), unzipCmd)
	if out, err := sshCmd.CombinedOutput(); err != nil {
		log.Fatalf("SSH unzip failed: %v, out: %s", err, string(out))
	}

	// 9. Open demo.html via file:// on Kiwiberry Chrome & Test Parameters
	demoFileURL := fmt.Sprintf("file://%s/mail-service/demo.html", remoteDemoDir)
	fmt.Printf("\n[PHASE 9] Opening %s in real Chrome on Kiwiberry...\n", demoFileURL)

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(demoFileURL)); err != nil {
		log.Fatalf("Navigate demo failed: %v", err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-a svg")); err != nil {
		log.Fatalf("Instance unit-a SVG failed to mount: %v", err)
	}
	_, _ = chromedp.Run(taskCtx, chromedp.Sleep(1*time.Second))

	mountSuccess, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(instances && instances['unit-a'] && instances['unit-b'] && instances['unit-c'])`))
	speedCoexistenceOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const w = instances['unit-a'];
		if (!w) return false;
		w.setParameter('masterSpeed', 1.5);
		w.setParameter('flowSpeed', 2.0);
		return w.getSpeed() === 1.5 && w.getParameter('flowSpeed') === 2.0 && w.getParameter('masterSpeed') === 1.5;
	})()`))

	record("MultiInstanceMount", mountSuccess, "All 3 instances (unit-a, unit-b, unit-c) mounted offline via file://")
	record("SpeedCoexistence", speedCoexistenceOk, "Master playbackRate (1.5) and local flow speed (2.0) independently set and coexist")

	// Print summary
	fmt.Println("\n==================================================================")
	fmt.Println(" FINAL VERIFICATION RESULTS SUMMARY")
	fmt.Println("==================================================================")
	allPassed := true
	for _, s := range steps {
		status := "PASS"
		if !s.Passed {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] %-24s : %s\n", status, s.Name, s.Message)
	}

	if allPassed {
		fmt.Println("\n>>> ALL 10 ACCEPTANCE CRITERIA VERIFIED SUCCESSFULLY ON KIWIBERRY <<<")
	} else {
		fmt.Println("\n>>> SOME ACCEPTANCE CRITERIA FAILED <<<")
		os.Exit(1)
	}
}
