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
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	kiwiberryHost = "192.168.100.40"
	apiBase       = "http://192.168.100.40:8088"
	cdpEndpoint   = "http://127.0.0.1:9222"
	remoteDemoDir = "/home/nigel/mail-demo"
)

func main() {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Mail Scene Property Authoring & Binding Kiwiberry CDP Verification")
	fmt.Println("==================================================================")

	// Step 1: Define Mail scene with Node, Flow, Glow, Ripple and clean parameters
	fmt.Println("\n[STEP 1] Constructing Mail scene state with clean object bindings...")
	sceneState := map[string]interface{}{
		"version": "1.2.0",
		"nodes": []map[string]interface{}{
			{
				"id":          1,
				"name":        "mail-server",
				"shape":       "rounded",
				"text":        "Mail Server",
				"subtext":     "SMTP / IMAP",
				"x":           120,
				"y":           180,
				"width":       180,
				"height":      80,
				"fillColor":   "#161b22",
				"fillOpacity": 0.92,
				"strokeColor": "#58a6ff",
				"strokeWidth": 2,
				"textColor":   "#ffffff",
				"fontSize":    14,
			},
			{
				"id":          2,
				"name":        "client-inbox",
				"shape":       "rounded",
				"text":        "Client Inbox",
				"subtext":     "User Terminal",
				"x":           500,
				"y":           180,
				"width":       180,
				"height":      80,
				"fillColor":   "#161b22",
				"fillOpacity": 0.92,
				"strokeColor": "#3fb950",
				"strokeWidth": 2,
				"textColor":   "#ffffff",
				"fontSize":    14,
			},
		},
		"nodesCounter": 2,
		"flows": []map[string]interface{}{
			{
				"id":         1,
				"name":       "mail-flow",
				"fromNodeId": 1,
				"toNodeId":   2,
				"color":      "#00ffcc",
				"animSpeed":  2.0,
				"flowStyle":  "particles",
			},
		},
		"flowsCounter": 1,
		"glows": []map[string]interface{}{
			{
				"id":         1,
				"name":       "server-status-glow",
				"xPct":       20,
				"yPct":       25,
				"shape":      "circle",
				"color":      "#22c55e",
				"innerR":     15,
				"outerR":     45,
				"pulseSpeed": 1.2,
				"intensity":  0.9,
			},
		},
		"glowsCounter": 1,
		"ripples": []map[string]interface{}{
			{
				"id":         1,
				"name":       "delivery-wave",
				"xPct":       65,
				"yPct":       25,
				"innerR":     12,
				"outerR":     50,
				"color":      "#38bdf8",
				"speed":      1.0,
				"intensity":  0.8,
			},
		},
		"ripplesCounter": 1,
		"parameters": []map[string]interface{}{
			{
				"name":        "serverFill",
				"target":      "node-1",
				"property":    "fill",
				"type":        "color",
				"default":     "#161b22",
				"description": "Mail server node fill color",
			},
			{
				"name":        "serverOpacity",
				"target":      "node-1",
				"property":    "opacity",
				"type":        "number",
				"default":     0.92,
				"min":         0.0,
				"max":         1.0,
				"description": "Mail server opacity [0, 1]",
			},
			{
				"name":        "flowColor",
				"target":      "flow-1",
				"property":    "stroke",
				"type":        "color",
				"default":     "#00ffcc",
				"description": "Mail transmission stream color",
			},
			{
				"name":        "flowSpeed",
				"target":      "flow-1",
				"property":    "speed",
				"type":        "speed",
				"default":     2.0,
				"min":         0.1,
				"max":         6.0,
				"step":        0.1,
				"description": "Mail particle flow speed",
			},
			{
				"name":        "flowOpacity",
				"target":      "flow-1",
				"property":    "opacity",
				"type":        "number",
				"default":     1.0,
				"min":         0.0,
				"max":         1.0,
				"description": "Flow opacity [0, 1]",
			},
			{
				"name":        "glowColor",
				"target":      "glow-1",
				"property":    "color",
				"type":        "color",
				"default":     "#22c55e",
				"description": "Server status glow color",
			},
			{
				"name":        "rippleColor",
				"target":      "ripple-1",
				"property":    "color",
				"type":        "color",
				"default":     "#38bdf8",
				"description": "Delivery ripple wave color",
			},
			{
				"name":        "playbackRate",
				"target":      "root",
				"property":    "playbackRate",
				"type":        "speed",
				"default":     1.0,
				"min":         0.1,
				"max":         5.0,
				"step":        0.1,
				"description": "Overall animation playback rate",
			},
		},
	}

	// Step 2: Save to SQLite library
	fmt.Println("\n[STEP 2] Saving Mail project to SQLite Library on Kiwiberry...")
	stateJSON, err := json.Marshal(sceneState)
	if err != nil {
		log.Fatalf("Failed to marshal scene state: %v", err)
	}

	projPayload := map[string]interface{}{
		"name":         "Mail",
		"description":  "Mail Network Topology Widget",
		"canvasWidth":  800,
		"canvasHeight": 500,
		"schemaJson":   string(stateJSON),
	}
	projBody, _ := json.Marshal(projPayload)
	resp, err := http.Post(apiBase+"/api/projects/save", "application/json", bytes.NewReader(projBody))
	if err != nil {
		log.Fatalf("Failed to create project via API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		log.Fatalf("Project save failed status %d: %s", resp.StatusCode, string(b))
	}

	var saveResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&saveResult); err != nil {
		log.Fatalf("Project creation response decode failed: %v", err)
	}
	projectID := saveResult["id"]
	fmt.Printf("[PASS] Saved Mail project (ID: %v)\n", projectID)

	// Step 3: Call /api/export/widget-package
	fmt.Println("\n[STEP 3] Calling /api/export/widget-package...")
	expPayload := map[string]interface{}{
		"name":       "mail",
		"sceneState": sceneState,
		"svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 500" width="800" height="500">
  <defs>
    <filter id="nodeDropShadow"><feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#000000" flood-opacity="0.3"/></filter>
  </defs>
  <g id="nodes-layer">
    <g id="node-1" data-node-id="1" data-semantic-target="node-1" data-semantic-name="mail-server" transform="translate(120, 180)">
      <rect x="-90" y="-40" width="180" height="80" rx="8" fill="#161b22" stroke="#58a6ff" stroke-width="2"/>
      <text x="0" y="5" fill="#ffffff" font-size="14" text-anchor="middle">Mail Server</text>
    </g>
    <g id="node-2" data-node-id="2" data-semantic-target="node-2" data-semantic-name="client-inbox" transform="translate(500, 180)">
      <rect x="-90" y="-40" width="180" height="80" rx="8" fill="#161b22" stroke="#3fb950" stroke-width="2"/>
      <text x="0" y="5" fill="#ffffff" font-size="14" text-anchor="middle">Client Inbox</text>
    </g>
  </g>
  <g id="flows-layer">
    <g id="flow-1" data-flow-id="1" data-semantic-target="flow-1" data-semantic-name="mail-flow" class="diagram-flow-instance">
      <path class="flow-track-path" d="M 210,180 L 410,180" stroke="#00ffcc" stroke-width="3" fill="none"/>
    </g>
  </g>
  <g id="glows-layer">
    <g id="glow-1" data-glow-id="1" data-semantic-target="glow-1" data-semantic-name="server-status-glow" class="diagram-glow-instance">
      <circle class="glow-core" cx="160" cy="125" r="20" fill="#22c55e" opacity="0.9"/>
    </g>
  </g>
  <g id="ripples-layer">
    <g id="ripple-1" data-ripple-id="1" data-semantic-target="ripple-1" data-semantic-name="delivery-wave" class="diagram-ripple-instance">
      <circle class="ripple-wave" cx="520" cy="125" r="30" stroke="#38bdf8" stroke-width="2" fill="none" opacity="0.8"/>
    </g>
  </g>
</svg>`,
	}
	expBody, _ := json.Marshal(expPayload)
	expResp, err := http.Post(apiBase+"/api/export/widget-package", "application/json", bytes.NewReader(expBody))
	if err != nil {
		log.Fatalf("Failed to request widget package: %v", err)
	}
	defer expResp.Body.Close()

	if expResp.StatusCode != 200 {
		b, _ := io.ReadAll(expResp.Body)
		log.Fatalf("Widget package export returned status %d: %s", expResp.StatusCode, string(b))
	}

	zipData, err := io.ReadAll(expResp.Body)
	if err != nil {
		log.Fatalf("Failed to read zip payload: %v", err)
	}
	fmt.Printf("[PASS] Generated Mail package ZIP (%d bytes)\n", len(zipData))

	// Step 4: Inspect ZIP contents and manifest
	fmt.Println("\n[STEP 4] Inspecting Mail package files in ZIP...")
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		log.Fatalf("Failed to parse zip archive: %v", err)
	}

	var manifestContent []byte
	packageFiles := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatalf("Failed to open %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		fmt.Printf("   ✓ %s (%d bytes)\n", f.Name, len(data))
		packageFiles[f.Name] = data
		if strings.HasSuffix(f.Name, "widget-manifest.json") {
			manifestContent = data
		}
	}

	if len(manifestContent) == 0 {
		log.Fatalf("widget-manifest.json not found in ZIP")
	}

	// Validate manifest parameters
	fmt.Println("\n[STEP 5] Validating widget-manifest.json parameter structure...")
	var manifest struct {
		SchemaVersion int `json:"schemaVersion"`
		Parameters    []struct {
			Name        string      `json:"name"`
			Type        string      `json:"type"`
			Target      string      `json:"target"`
			Property    string      `json:"property"`
			Default     interface{} `json:"default"`
			Min         *float64    `json:"min,omitempty"`
			Max         *float64    `json:"max,omitempty"`
			Description string      `json:"description"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		log.Fatalf("Failed to parse widget-manifest.json: %v", err)
	}

	if len(manifest.Parameters) != 8 {
		log.Fatalf("FAIL: Expected 8 parameters in manifest, got %d", len(manifest.Parameters))
	}

	// Verify absence of min/max on color types
	for _, param := range manifest.Parameters {
		name := param.Name
		if param.Type == "color" || param.Type == "boolean" || param.Type == "text" {
			if param.Min != nil || param.Max != nil {
				log.Fatalf("FAIL: Non-numeric parameter %q of type %q defines numeric min/max", name, param.Type)
			}
		}
		if param.Property == "opacity" {
			if param.Min != nil && *param.Min < 0 {
				log.Fatalf("FAIL: Opacity parameter %q defines min < 0", name)
			}
			if param.Max != nil && *param.Max > 1 {
				log.Fatalf("FAIL: Opacity parameter %q defines max > 1", name)
			}
		}
		fmt.Printf("   ✓ Param %q (type: %s, target: %s.%s, default: %v)\n", name, param.Type, param.Target, param.Property, param.Default)
	}
	fmt.Println("[PASS] All manifest parameters verified: zero orphan min/max and strict opacity bounds.")

	// Step 6: Deploy package to Kiwiberry filesystem
	fmt.Println("\n[STEP 6] Deploying package to Kiwiberry filesystem (/home/nigel/mail-demo/)...")
	localTmpDir, _ := os.MkdirTemp("", "mail-pkg-*")
	defer os.RemoveAll(localTmpDir)

	for fname, data := range packageFiles {
		destPath := filepath.Join(localTmpDir, fname)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		os.WriteFile(destPath, data, 0644)
	}

	cmdMkdir := exec.Command("ssh", "nigel@"+kiwiberryHost, "rm -rf "+remoteDemoDir+" && mkdir -p "+remoteDemoDir)
	if out, err := cmdMkdir.CombinedOutput(); err != nil {
		log.Fatalf("Failed to create remote directory: %v, out: %s", err, string(out))
	}

	cmdScp := exec.Command("scp", "-r", localTmpDir+"/mail", "nigel@"+kiwiberryHost+":"+remoteDemoDir+"/")
	if out, err := cmdScp.CombinedOutput(); err != nil {
		log.Fatalf("Failed to scp package to Kiwiberry: %v, out: %s", err, string(out))
	}
	fmt.Println("[PASS] Deployed package to Kiwiberry at " + remoteDemoDir + "/mail/")

	// Step 7: Drive Google Chrome on Kiwiberry via CDP
	fmt.Println("\n[STEP 7] Driving Google Chrome on Kiwiberry via CDP...")
	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()

	ctx, cancel := context.WithTimeout(allocCtx, 30*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	fileURL := "file://" + remoteDemoDir + "/mail/demo.html"
	fmt.Printf("   Navigating Kiwiberry Chrome to: %s\n", fileURL)

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(fileURL)); err != nil {
		log.Fatalf("Failed to navigate to demo.html: %v", err)
	}

	// Wait for DOM elements
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-a svg")); err != nil {
		log.Fatalf("Instance unit-a SVG failed to mount: %v", err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-b svg")); err != nil {
		log.Fatalf("Instance unit-b SVG failed to mount: %v", err)
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-c svg")); err != nil {
		log.Fatalf("Instance unit-c SVG failed to mount: %v", err)
	}
	fmt.Println("[PASS] All 3 instances (unit-a, unit-b, unit-c) mounted successfully via file://")

	// Verification 1: Node Fill Isolation (serverFill on node-1)
	fmt.Println("\n[VERIFICATION 1] Testing Node fill property isolation (serverFill on node-1)...")
	testFillJS := `(() => {
		instances['unit-a'].setParameter('serverFill', '#ff0055');
		const elA = document.querySelector('#container-unit-a svg [id$="node-1"] rect, #container-unit-a svg [data-node-id="1"] rect');
		const elB = document.querySelector('#container-unit-b svg [id$="node-1"] rect, #container-unit-b svg [data-node-id="1"] rect');
		const elC = document.querySelector('#container-unit-c svg [id$="node-1"] rect, #container-unit-c svg [data-node-id="1"] rect');
		return {
			valA: instances['unit-a'].getParameter('serverFill'),
			valB: instances['unit-b'].getParameter('serverFill'),
			valC: instances['unit-c'].getParameter('serverFill'),
			fillA: elA ? elA.getAttribute('fill') : null,
			fillB: elB ? elB.getAttribute('fill') : null,
			fillC: elC ? elC.getAttribute('fill') : null
		};
	})()`

	var resFill struct {
		ValA  string `json:"valA"`
		ValB  string `json:"valB"`
		ValC  string `json:"valC"`
		FillA string `json:"fillA"`
		FillB string `json:"fillB"`
		FillC string `json:"fillC"`
	}
	resFillRaw, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testFillJS)))
	if err != nil {
		log.Fatalf("Failed to test serverFill: %v", err)
	}
	json.Unmarshal([]byte(resFillRaw), &resFill)

	if resFill.ValA != "#ff0055" || resFill.FillA != "#ff0055" {
		log.Fatalf("FAIL: Unit A serverFill did not update to #ff0055: %+v", resFill)
	}
	if resFill.ValB != "#161b22" || resFill.FillB != "#161b22" {
		log.Fatalf("FAIL: Unit B serverFill crosstalked: %+v", resFill)
	}
	if resFill.ValC != "#161b22" || resFill.FillC != "#161b22" {
		log.Fatalf("FAIL: Unit C serverFill crosstalked: %+v", resFill)
	}
	fmt.Println("[PASS] Unit A serverFill updated to #ff0055; Unit B and C remained #161b22 (zero crosstalk).")

	// Verification 2: Flow Track Stroke Isolation (flowColor on flow-1)
	fmt.Println("\n[VERIFICATION 2] Testing Flow track stroke isolation (flowColor on flow-1)...")
	testStrokeJS := `(() => {
		instances['unit-b'].setParameter('flowColor', '#ffaa00');
		const elA = document.querySelector('#container-unit-a svg .flow-track-path, #container-unit-a svg [id$="flow-1"] path');
		const elB = document.querySelector('#container-unit-b svg .flow-track-path, #container-unit-b svg [id$="flow-1"] path');
		const elC = document.querySelector('#container-unit-c svg .flow-track-path, #container-unit-c svg [id$="flow-1"] path');
		return {
			valA: instances['unit-a'].getParameter('flowColor'),
			valB: instances['unit-b'].getParameter('flowColor'),
			valC: instances['unit-c'].getParameter('flowColor'),
			strokeA: elA ? elA.getAttribute('stroke') : null,
			strokeB: elB ? elB.getAttribute('stroke') : null,
			strokeC: elC ? elC.getAttribute('stroke') : null
		};
	})()`

	var resStroke struct {
		ValA    string `json:"valA"`
		ValB    string `json:"valB"`
		ValC    string `json:"valC"`
		StrokeA string `json:"strokeA"`
		StrokeB string `json:"strokeB"`
		StrokeC string `json:"strokeC"`
	}
	resStrokeRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testStrokeJS)))
	json.Unmarshal([]byte(resStrokeRaw), &resStroke)

	if resStroke.ValB != "#ffaa00" || resStroke.StrokeB != "#ffaa00" {
		log.Fatalf("FAIL: Unit B flowColor did not update to #ffaa00: %+v", resStroke)
	}
	if resStroke.ValA != "#00ffcc" || resStroke.StrokeA != "#00ffcc" {
		log.Fatalf("FAIL: Unit A flowColor was modified: %+v", resStroke)
	}
	if resStroke.ValC != "#00ffcc" || resStroke.StrokeC != "#00ffcc" {
		log.Fatalf("FAIL: Unit C flowColor was modified: %+v", resStroke)
	}
	fmt.Println("[PASS] Unit B flowColor updated to #ffaa00; Unit A and C remained #00ffcc.")

	// Verification 3: Glow Optical Isolation (glowColor on glow-1)
	fmt.Println("\n[VERIFICATION 3] Testing Glow optical color isolation (glowColor on glow-1)...")
	testGlowJS := `(() => {
		instances['unit-c'].setParameter('glowColor', '#e11d48');
		const elA = document.querySelector('#container-unit-a svg [id$="glow-1"] circle, #container-unit-a svg [data-semantic-target="glow-1"] circle');
		const elB = document.querySelector('#container-unit-b svg [id$="glow-1"] circle, #container-unit-b svg [data-semantic-target="glow-1"] circle');
		const elC = document.querySelector('#container-unit-c svg [id$="glow-1"] circle, #container-unit-c svg [data-semantic-target="glow-1"] circle');
		return {
			valA: instances['unit-a'].getParameter('glowColor'),
			valB: instances['unit-b'].getParameter('glowColor'),
			valC: instances['unit-c'].getParameter('glowColor'),
			fillA: elA ? elA.getAttribute('fill') : null,
			fillB: elB ? elB.getAttribute('fill') : null,
			fillC: elC ? elC.getAttribute('fill') : null
		};
	})()`

	var resGlow struct {
		ValA  string `json:"valA"`
		ValB  string `json:"valB"`
		ValC  string `json:"valC"`
		FillA string `json:"fillA"`
		FillB string `json:"fillB"`
		FillC string `json:"fillC"`
	}
	resGlowRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testGlowJS)))
	json.Unmarshal([]byte(resGlowRaw), &resGlow)

	if resGlow.ValC != "#e11d48" || resGlow.FillC != "#e11d48" {
		log.Fatalf("FAIL: Unit C glowColor did not update to #e11d48: %+v", resGlow)
	}
	if resGlow.ValA != "#22c55e" || resGlow.FillA != "#22c55e" {
		log.Fatalf("FAIL: Unit A glowColor modified: %+v", resGlow)
	}
	if resGlow.ValB != "#22c55e" || resGlow.FillB != "#22c55e" {
		log.Fatalf("FAIL: Unit B glowColor modified: %+v", resGlow)
	}
	fmt.Println("[PASS] Unit C glowColor updated to #e11d48; Unit A and B remained #22c55e.")

	// Verification 4: Ripple Wave Stroke Isolation (rippleColor on ripple-1)
	fmt.Println("\n[VERIFICATION 4] Testing Ripple wave stroke isolation (rippleColor on ripple-1)...")
	testRippleJS := `(() => {
		instances['unit-a'].setParameter('rippleColor', '#a855f7');
		const elA = document.querySelector('#container-unit-a svg [id$="ripple-1"] circle.ripple-wave, #container-unit-a svg [data-semantic-target="ripple-1"] circle');
		const elB = document.querySelector('#container-unit-b svg [id$="ripple-1"] circle.ripple-wave, #container-unit-b svg [data-semantic-target="ripple-1"] circle');
		const elC = document.querySelector('#container-unit-c svg [id$="ripple-1"] circle.ripple-wave, #container-unit-c svg [data-semantic-target="ripple-1"] circle');
		return {
			valA: instances['unit-a'].getParameter('rippleColor'),
			valB: instances['unit-b'].getParameter('rippleColor'),
			valC: instances['unit-c'].getParameter('rippleColor'),
			strokeA: elA ? elA.getAttribute('stroke') : null,
			strokeB: elB ? elB.getAttribute('stroke') : null,
			strokeC: elC ? elC.getAttribute('stroke') : null
		};
	})()`

	var resRipple struct {
		ValA    string `json:"valA"`
		ValB    string `json:"valB"`
		ValC    string `json:"valC"`
		StrokeA string `json:"strokeA"`
		StrokeB string `json:"strokeB"`
		StrokeC string `json:"strokeC"`
	}
	resRippleRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testRippleJS)))
	json.Unmarshal([]byte(resRippleRaw), &resRipple)

	if resRipple.ValA != "#a855f7" || resRipple.StrokeA != "#a855f7" {
		log.Fatalf("FAIL: Unit A rippleColor did not update to #a855f7: %+v", resRipple)
	}
	if resRipple.ValB != "#38bdf8" || resRipple.StrokeB != "#38bdf8" {
		log.Fatalf("FAIL: Unit B rippleColor modified: %+v", resRipple)
	}
	if resRipple.ValC != "#38bdf8" || resRipple.StrokeC != "#38bdf8" {
		log.Fatalf("FAIL: Unit C rippleColor modified: %+v", resRipple)
	}
	fmt.Println("[PASS] Unit A rippleColor updated to #a855f7; Unit B and C remained #38bdf8.")

	// Verification 5: Speed Control vs Root Playback Rate
	fmt.Println("\n[VERIFICATION 5] Testing targeted flowSpeed vs whole-instance playbackRate...")
	testSpeedJS := `(() => {
		instances['unit-a'].setParameter('flowSpeed', 4.5);
		return {
			speedA: instances['unit-a'].getParameter('flowSpeed'),
			pRateA: instances['unit-a'].getSpeed(),
			speedB: instances['unit-b'].getParameter('flowSpeed')
		};
	})()`

	var resSpeed struct {
		SpeedA float64 `json:"speedA"`
		PRateA float64 `json:"pRateA"`
		SpeedB float64 `json:"speedB"`
	}
	resSpeedRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testSpeedJS)))
	json.Unmarshal([]byte(resSpeedRaw), &resSpeed)

	if resSpeed.SpeedA != 4.5 || resSpeed.PRateA != 1.0 || resSpeed.SpeedB != 2.0 {
		log.Fatalf("FAIL: flowSpeed crosstalk or root interference: %+v", resSpeed)
	}
	fmt.Println("[PASS] flowSpeed set to 4.5x on Unit A without affecting Unit A root playback rate (1.0) or Unit B (2.0).")

	// Verification 6: Independent Play / Pause Transport
	fmt.Println("\n[VERIFICATION 6] Testing independent Play and Pause transport controls...")
	testPauseJS := `(() => {
		instances['unit-a'].pause();
		const pausedA = instances['unit-a'].isPaused();
		const pausedB = instances['unit-b'].isPaused();
		const pausedC = instances['unit-c'].isPaused();
		instances['unit-a'].play();
		const resumedA = !instances['unit-a'].isPaused();
		return {
			pausedA: pausedA,
			pausedB: pausedB,
			pausedC: pausedC,
			resumedA: resumedA
		};
	})()`

	var resPause struct {
		PausedA  bool `json:"pausedA"`
		PausedB  bool `json:"pausedB"`
		PausedC  bool `json:"pausedC"`
		ResumedA bool `json:"resumedA"`
	}
	resPauseRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testPauseJS)))
	json.Unmarshal([]byte(resPauseRaw), &resPause)

	if !resPause.PausedA || resPause.PausedB || resPause.PausedC || !resPause.ResumedA {
		log.Fatalf("FAIL: Independent pause crosstalk or resume failure: %+v", resPause)
	}
	fmt.Println("[PASS] Unit A independently paused and resumed while Unit B and C continued running.")

	// Verification 7: Reset Defaults on Unit A
	fmt.Println("\n[VERIFICATION 7] Testing parameter reset on Unit A...")
	testResetJS := `(() => {
		instances['unit-a'].resetParameters();
		const elB = document.querySelector('#container-unit-b svg .flow-track-path, #container-unit-b svg [id$="flow-1"] path');
		return {
			resetFillA: instances['unit-a'].getParameter('serverFill'),
			resetSpeedA: instances['unit-a'].getParameter('flowSpeed'),
			strokeB: elB ? elB.getAttribute('stroke') : null
		};
	})()`

	var resReset struct {
		ResetFillA  string      `json:"resetFillA"`
		ResetSpeedA interface{} `json:"resetSpeedA"`
		StrokeB     string      `json:"strokeB"`
	}
	resResetRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testResetJS)))
	json.Unmarshal([]byte(resResetRaw), &resReset)

	speedStr := fmt.Sprintf("%v", resReset.ResetSpeedA)
	if resReset.ResetFillA != "#161b22" || (speedStr != "2" && speedStr != "2.0") || resReset.StrokeB != "#ffaa00" {
		log.Fatalf("FAIL: Reset parameters failed: %+v", resReset)
	}
	fmt.Println("[PASS] Unit A reset restored defaults (#161b22, speed: 2) while Unit B preserved custom state (#ffaa00).")

	fmt.Println("\n==================================================================")
	fmt.Println(" ALL STAGE 16B MAIL SCENE ACCEPTANCE TESTS PASSED ON KIWIBERRY!")
	fmt.Println("==================================================================")
}
