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
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

const (
	kiwiberryHost = "192.168.100.40"
	apiBase       = "http://192.168.100.40:8088"
	cdpEndpoint   = "http://127.0.0.1:9222"
	cpuHost       = "nigel@192.168.100.40"
	remoteDemoDir = "/home/nigel/birdscroll-demo"
)

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Birdscroll Scene End-to-End Kiwiberry Chrome Verification")
	fmt.Println("==================================================================")

	// Step 1: Define authoritative scene state and parameters
	fmt.Println("\n[STEP 1] Constructing Birdscroll scene state with 9 exposed parameters...")
	sceneState := map[string]interface{}{
		"version": "1.2.0",
		"nodes": []map[string]interface{}{
			{
				"id":          1,
				"name":        "bird-perch",
				"shape":       "rect",
				"text":        "Perch",
				"subtext":     "Oak Branch",
				"x":           50,
				"y":           300,
				"width":       220,
				"height":      30,
				"fillColor":   "#8B4513",
				"fillOpacity": 1.0,
				"strokeColor": "#5C2E0B",
				"strokeWidth": 2,
				"textColor":   "#FFFFFF",
				"fontSize":    12,
			},
			{
				"id":          2,
				"name":        "bird-body",
				"shape":       "rounded",
				"text":        "Swift",
				"subtext":     "Apus apus",
				"x":           140,
				"y":           210,
				"width":       90,
				"height":      65,
				"fillColor":   "#2563EB",
				"fillOpacity": 1.0,
				"strokeColor": "#1D4ED8",
				"strokeWidth": 2,
				"textColor":   "#FFFFFF",
				"fontSize":    13,
			},
			{
				"id":          3,
				"name":        "status-label",
				"shape":       "textbox",
				"text":        "MIGRATION IN FLIGHT",
				"subtext":     "AUTUMN TRANSLOCATION",
				"x":           220,
				"y":           40,
				"width":       260,
				"height":      40,
				"fillColor":   "transparent",
				"fillOpacity": 0,
				"strokeColor": "none",
				"strokeWidth": 0,
				"textColor":   "#E2E8F0",
				"fontSize":    15,
			},
		},
		"nodesCounter": 3,
		"glows": []map[string]interface{}{
			{
				"id":         1,
				"name":       "sun-glow",
				"xPct":       82,
				"yPct":       18,
				"shape":      "circle",
				"color":      "#F59E0B",
				"innerR":     15,
				"outerR":     48,
				"pulseSpeed": 1.5,
				"intensity":  0.9,
			},
		},
		"glowsCounter": 1,
		"ripples": []map[string]interface{}{
			{
				"id":         1,
				"name":       "pond-ripple",
				"xPct":       35,
				"yPct":       85,
				"innerR":     10,
				"outerR":     40,
				"color":      "#06B6D4",
				"speed":      1.0,
				"intensity":  0.75,
			},
		},
		"ripplesCounter": 1,
		"flows": []map[string]interface{}{
			{
				"id":         1,
				"name":       "wind-stream",
				"fromNodeId": 1,
				"toNodeId":   2,
				"color":      "#60A5FA",
				"animSpeed":  1.0,
				"flowStyle":  "particles",
			},
		},
		"flowsCounter": 1,
		"parameters": []map[string]interface{}{
			{
				"name":        "perchColor",
				"target":      "bird-perch",
				"property":    "fill",
				"type":        "color",
				"default":     "#8B4513",
				"description": "Perch wood fill color",
			},
			{
				"name":        "birdColor",
				"target":      "bird-body",
				"property":    "fill",
				"type":        "color",
				"default":     "#2563EB",
				"description": "Bird plumage color",
			},
			{
				"name":        "statusText",
				"target":      "status-label",
				"property":    "text",
				"type":        "text",
				"default":     "MIGRATION IN FLIGHT",
				"description": "Flight status indicator label",
			},
			{
				"name":        "sunColor",
				"target":      "sun-glow",
				"property":    "color",
				"type":        "color",
				"default":     "#F59E0B",
				"description": "Sun halo optical glow color",
			},
			{
				"name":        "sunVisible",
				"target":      "sun-glow",
				"property":    "visibility",
				"type":        "boolean",
				"default":     true,
				"description": "Sun optical glow visibility",
			},
			{
				"name":        "rippleColor",
				"target":      "pond-ripple",
				"property":    "color",
				"type":        "color",
				"default":     "#06B6D4",
				"description": "Pond ripple refraction tint",
			},
			{
				"name":        "flowSpeed",
				"target":      "flow-1",
				"property":    "speed",
				"type":        "speed",
				"default":     1.0,
				"min":         0.1,
				"max":         4.0,
				"step":        0.1,
				"description": "Wind stream particle animation speed",
			},
			{
				"name":        "playbackRate",
				"target":      "root",
				"property":    "playbackRate",
				"type":        "speed",
				"default":     1.0,
				"min":         0.2,
				"max":         3.0,
				"step":        0.1,
				"description": "Master playback speed multiplier",
			},
			{
				"name":        "flowing",
				"target":      "root",
				"property":    "visibility",
				"type":        "boolean",
				"default":     true,
				"description": "Master widget display toggle",
			},
		},
		"canvasWidth":  800,
		"canvasHeight": 600,
	}

	schemaBytes, err := json.Marshal(sceneState)
	if err != nil {
		log.Fatalf("JSON marshal error: %v", err)
	}

	// Step 2: Save to SQLite Library
	fmt.Println("\n[STEP 2] Saving Birdscroll project to SQLite Library on Kiwiberry...")
	saveReqBody, _ := json.Marshal(map[string]interface{}{
		"name":         "Birdscroll Studio Widget",
		"description":  "Studio-authored Birdscroll widget with node, optical, flow, and master controls",
		"canvasWidth":  800,
		"canvasHeight": 600,
		"schemaJson":   string(schemaBytes),
	})

	saveResp, err := http.Post(apiBase+"/api/projects/save", "application/json", bytes.NewReader(saveReqBody))
	if err != nil {
		log.Fatalf("HTTP save failed: %v", err)
	}
	defer saveResp.Body.Close()
	if saveResp.StatusCode != 200 {
		b, _ := io.ReadAll(saveResp.Body)
		log.Fatalf("HTTP save status %d: %s", saveResp.StatusCode, string(b))
	}
	var saveResult map[string]interface{}
	json.NewDecoder(saveResp.Body).Decode(&saveResult)
	projID := saveResult["id"]
	fmt.Printf("[PASS] Saved project to SQLite Library (Project ID: %v)\n", projID)

	// Step 3: Construct Raw SVG for Export
	fmt.Println("\n[STEP 3] Generating SVG markup with semantic target attributes...")
	rawSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600" width="800" height="600">
  <defs>
    <filter id="sunHaloFilter" x="-30%" y="-30%" width="160%" height="160%">
      <feGaussianBlur stdDeviation="6" result="blur" />
      <feMerge>
        <feMergeNode in="blur" />
        <feMergeNode in="SourceGraphic" />
      </feMerge>
    </filter>
  </defs>
  <rect width="800" height="600" fill="#0f172a" rx="12" />

  <!-- Node 1: Bird Perch -->
  <g id="bird-perch" data-node-id="1" data-semantic-target="bird-perch" transform="translate(160, 400)">
    <rect x="-110" y="-15" width="220" height="30" rx="6" fill="#8B4513" stroke="#5C2E0B" stroke-width="2" />
    <text x="0" y="5" fill="#FFFFFF" font-family="system-ui, sans-serif" font-size="12" text-anchor="middle">Perch</text>
  </g>

  <!-- Node 2: Bird Body -->
  <g id="bird-body" data-node-id="2" data-semantic-target="bird-body" transform="translate(185, 240)">
    <rect x="-45" y="-32" width="90" height="65" rx="16" fill="#2563EB" stroke="#1D4ED8" stroke-width="2" />
    <text x="0" y="5" fill="#FFFFFF" font-family="system-ui, sans-serif" font-size="13" font-weight="600" text-anchor="middle">Swift</text>
  </g>

  <!-- Node 3: Status Label -->
  <g id="status-label" data-node-id="3" data-semantic-target="status-label" transform="translate(350, 60)">
    <text x="0" y="0" fill="#E2E8F0" font-family="system-ui, sans-serif" font-size="15" font-weight="700" text-anchor="middle">MIGRATION IN FLIGHT</text>
  </g>

  <!-- Optical Effect 1: Sun Glow -->
  <g id="sun-glow" data-glow-id="1" data-semantic-target="sun-glow" transform="translate(650, 110)">
    <circle cx="0" cy="0" r="28" fill="#F59E0B" filter="url(#sunHaloFilter)">
      <animate attributeName="opacity" values="0.9;0.5;0.9" dur="2s" repeatCount="indefinite" />
    </circle>
  </g>

  <!-- Optical Effect 2: Pond Ripple -->
  <g id="pond-ripple" data-ripple-id="1" data-semantic-target="pond-ripple" transform="translate(280, 510)">
    <circle cx="0" cy="0" r="24" fill="none" stroke="#06B6D4" stroke-width="2" opacity="0.75">
      <animate attributeName="r" values="10;40" dur="2.5s" repeatCount="indefinite" />
      <animate attributeName="opacity" values="0.75;0" dur="2.5s" repeatCount="indefinite" />
    </circle>
  </g>

  <!-- Flow 1: Wind Stream -->
  <g id="flow-1" data-flow-id="1" data-semantic-target="flow-1">
    <path d="M 160 385 Q 220 310 185 275" fill="none" stroke="#60A5FA" stroke-width="3" stroke-dasharray="6,4">
      <animate attributeName="stroke-dashoffset" from="0" to="-40" dur="1.5s" repeatCount="indefinite" />
    </path>
  </g>
</svg>`

	// Step 4: Export Widget Package (ZIP) from Server
	fmt.Println("\n[STEP 4] Calling /api/export/widget-package to generate portable ZIP...")
	pkgReqBody, _ := json.Marshal(map[string]interface{}{
		"name":       "Birdscroll",
		"svg":        rawSVG,
		"schemaJson": sceneState,
	})

	pkgResp, err := http.Post(apiBase+"/api/export/widget-package", "application/json", bytes.NewReader(pkgReqBody))
	if err != nil {
		log.Fatalf("HTTP export package failed: %v", err)
	}
	defer pkgResp.Body.Close()
	if pkgResp.StatusCode != 200 {
		b, _ := io.ReadAll(pkgResp.Body)
		log.Fatalf("HTTP export package status %d: %s", pkgResp.StatusCode, string(b))
	}

	zipBytes, err := io.ReadAll(pkgResp.Body)
	if err != nil {
		log.Fatalf("Failed to read package ZIP: %v", err)
	}
	fmt.Printf("[PASS] Generated widget package ZIP (%d bytes)\n", len(zipBytes))

	// Step 5: Unzip & Validate Package Files
	fmt.Println("\n[STEP 5] Inspecting package files in ZIP...")
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		log.Fatalf("Failed to parse ZIP: %v", err)
	}

	packageFiles := make(map[string][]byte)
	for _, f := range zipReader.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatalf("Failed to open %s in ZIP: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		packageFiles[f.Name] = data
	}

	requiredFiles := []string{
		"birdscroll/birdscroll.svg",
		"birdscroll/birdscroll-properties.md",
		"birdscroll/widget-manifest.json",
		"birdscroll/README.md",
		"birdscroll/demo.html",
		"birdscroll/js/svg-widget.js",
	}

	for _, rf := range requiredFiles {
		data, exists := packageFiles[rf]
		if !exists {
			log.Fatalf("[FAIL] Missing required file in ZIP: %s", rf)
		}
		fmt.Printf("   ✓ %s (%d bytes)\n", rf, len(data))
	}

	// Step 6: Validate manifest contents
	fmt.Println("\n[STEP 6] Validating widget-manifest.json...")
	manifestBytes := packageFiles["birdscroll/widget-manifest.json"]
	var manifestData struct {
		SchemaVersion int                      `json:"schemaVersion"`
		Parameters    []map[string]interface{} `json:"parameters"`
	}
	if err := json.Unmarshal(manifestBytes, &manifestData); err != nil {
		log.Fatalf("Failed to parse widget-manifest.json: %v", err)
	}
	if len(manifestData.Parameters) != 9 {
		log.Fatalf("[FAIL] Expected 9 parameters in manifest, got %d", len(manifestData.Parameters))
	}
	fmt.Printf("[PASS] Manifest contains all 9 parameters:\n")
	for _, p := range manifestData.Parameters {
		fmt.Printf("   - %s (%s, target: %s, default: %v)\n", p["name"], p["type"], p["target"], p["default"])
	}

	// Step 7: Validate generated Markdown properties guide
	fmt.Println("\n[STEP 7] Validating birdscroll-properties.md documentation...")
	guideContent := string(packageFiles["birdscroll/birdscroll-properties.md"])
	if strings.Contains(guideContent, "'NORMAL'") || strings.Contains(guideContent, "\"NORMAL\"") {
		log.Fatalf("[FAIL] Properties guide contains invalid 'NORMAL' string for boolean parameters!")
	}
	if !strings.Contains(guideContent, "sunVisible") || !strings.Contains(guideContent, "flowSpeed") || !strings.Contains(guideContent, "perchColor") {
		log.Fatalf("[FAIL] Properties guide missing documented parameters!")
	}
	fmt.Println("[PASS] Properties guide correctly documents real types without invalid 'NORMAL' fallbacks.")

	// Step 8: Validate demo.html for offline double-click file:// support
	fmt.Println("\n[STEP 8] Validating demo.html offline embedded SVG support...")
	demoContent := string(packageFiles["birdscroll/demo.html"])
	if !strings.Contains(demoContent, "id=\"embedded-widget-svg\"") {
		log.Fatalf("[FAIL] demo.html does not contain embedded Base64 SVG payload for offline file:// execution!")
	}
	fmt.Println("[PASS] demo.html embeds Base64 SVG payload for double-click offline file:// operation.")

	// Step 9: Deploy extracted files to Kiwiberry
	fmt.Println("\n[STEP 9] Deploying package to Kiwiberry filesystem (/home/nigel/birdscroll-demo/)...")
	localTmpDir, _ := os.MkdirTemp("", "birdscroll-demo-*")
	defer os.RemoveAll(localTmpDir)

	for fname, data := range packageFiles {
		destPath := filepath.Join(localTmpDir, fname)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		os.WriteFile(destPath, data, 0644)
	}

	// SSH scp to Kiwiberry
	cmdMkdir := exec.Command("ssh", "nigel@"+kiwiberryHost, "rm -rf "+remoteDemoDir+" && mkdir -p "+remoteDemoDir)
	if out, err := cmdMkdir.CombinedOutput(); err != nil {
		log.Fatalf("Failed to create remote directory: %v, out: %s", err, string(out))
	}

	cmdScp := exec.Command("scp", "-r", localTmpDir+"/birdscroll", "nigel@"+kiwiberryHost+":"+remoteDemoDir+"/")
	if out, err := cmdScp.CombinedOutput(); err != nil {
		log.Fatalf("Failed to scp package to Kiwiberry: %v, out: %s", err, string(out))
	}
	fmt.Println("[PASS] Deployed package to Kiwiberry at " + remoteDemoDir + "/birdscroll/")

	// Step 10: Run Live Browser Acceptance Test in Google Chrome on Kiwiberry via CDP
	fmt.Println("\n[STEP 10] Running real Chrome acceptance tests via CDP on Kiwiberry...")
	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		log.Printf("Failed to acquire browser session: %v", err)
		return 1
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(sess.AllocCtx, 30*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	fileURL := "file://" + remoteDemoDir + "/birdscroll/demo.html"
	fmt.Printf("   Navigating Kiwiberry Chrome to: %s\n", fileURL)

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(fileURL)); err != nil {
		log.Printf("Failed to navigate to demo.html: %v", err)
		return 1
	}

	// Wait for DOM elements
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-a svg")); err != nil {
		log.Printf("Instance unit-a SVG failed to mount: %v", err)
		return 1
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-b svg")); err != nil {
		log.Printf("Instance unit-b SVG failed to mount: %v", err)
		return 1
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#container-unit-c svg")); err != nil {
		log.Printf("Instance unit-c SVG failed to mount: %v", err)
		return 1
	}
	fmt.Println("[PASS] All 3 instances (unit-a, unit-b, unit-c) mounted successfully in Chrome via file://")

	// Verification Test 1: Node fill parameter isolation
	fmt.Println("\n[VERIFICATION 1] Testing Node fill parameter isolation (perchColor)...")
	testPerchJS := `(() => {
		instances['unit-a'].setParameter('perchColor', '#FF0000');
		const elA = document.querySelector('#container-unit-a svg [id$="bird-perch"] rect');
		const elB = document.querySelector('#container-unit-b svg [id$="bird-perch"] rect');
		const elC = document.querySelector('#container-unit-c svg [id$="bird-perch"] rect');
		return {
			valA: instances['unit-a'].getParameter('perchColor'),
			valB: instances['unit-b'].getParameter('perchColor'),
			valC: instances['unit-c'].getParameter('perchColor'),
			fillA: elA ? elA.getAttribute('fill') : null,
			fillB: elB ? elB.getAttribute('fill') : null,
			fillC: elC ? elC.getAttribute('fill') : null
		};
	})()`

	var resPerch struct {
		ValA  string `json:"valA"`
		ValB  string `json:"valB"`
		ValC  string `json:"valC"`
		FillA string `json:"fillA"`
		FillB string `json:"fillB"`
		FillC string `json:"fillC"`
	}
	resPerchRaw, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testPerchJS)))
	if err != nil {
		log.Printf("Failed to test perchColor: %v", err)
		return 1
	}
	json.Unmarshal([]byte(resPerchRaw), &resPerch)

	if resPerch.ValA != "#FF0000" || resPerch.FillA != "#FF0000" {
		log.Printf("[FAIL] Unit A perchColor did not update to #FF0000: %+v", resPerch)
		return 1
	}
	if resPerch.ValB != "#8B4513" || resPerch.FillB != "#8B4513" {
		log.Printf("[FAIL] Unit B perchColor was improperly modified (crosstalk): %+v", resPerch)
		return 1
	}
	if resPerch.ValC != "#8B4513" || resPerch.FillC != "#8B4513" {
		log.Printf("[FAIL] Unit C perchColor was improperly modified (crosstalk): %+v", resPerch)
		return 1
	}
	fmt.Printf("[PASS] Perch color updated on Unit A (#FF0000) while Unit B and Unit C remained #8B4513 (zero crosstalk).\n")

	// Verification Test 2: Optical effect color parameter isolation (sunColor)
	fmt.Println("\n[VERIFICATION 2] Testing Optical Effect color isolation (sunColor on sun-glow)...")
	testSunJS := `(() => {
		instances['unit-b'].setParameter('sunColor', '#00FF00');
		const elA = document.querySelector('#container-unit-a svg [id$="sun-glow"] circle');
		const elB = document.querySelector('#container-unit-b svg [id$="sun-glow"] circle');
		const elC = document.querySelector('#container-unit-c svg [id$="sun-glow"] circle');
		return {
			valA: instances['unit-a'].getParameter('sunColor'),
			valB: instances['unit-b'].getParameter('sunColor'),
			valC: instances['unit-c'].getParameter('sunColor'),
			fillA: elA ? elA.getAttribute('fill') : null,
			fillB: elB ? elB.getAttribute('fill') : null,
			fillC: elC ? elC.getAttribute('fill') : null
		};
	})()`

	var resSun struct {
		ValA  string `json:"valA"`
		ValB  string `json:"valB"`
		ValC  string `json:"valC"`
		FillA string `json:"fillA"`
		FillB string `json:"fillB"`
		FillC string `json:"fillC"`
	}
	resSunRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testSunJS)))
	json.Unmarshal([]byte(resSunRaw), &resSun)

	if resSun.ValB != "#00FF00" || resSun.FillB != "#00FF00" {
		log.Printf("[FAIL] Unit B sunColor did not update to #00FF00: %+v", resSun)
		return 1
	}
	if resSun.ValA != "#F59E0B" || resSun.FillA != "#F59E0B" {
		log.Printf("[FAIL] Unit A sunColor was affected by Unit B update: %+v", resSun)
		return 1
	}
	if resSun.ValC != "#F59E0B" || resSun.FillC != "#F59E0B" {
		log.Printf("[FAIL] Unit C sunColor was affected by Unit B update: %+v", resSun)
		return 1
	}
	fmt.Printf("[PASS] Sun glow color updated on Unit B (#00FF00) while Unit A and Unit C remained #F59E0B.\n")

	// Verification Test 3: Optical effect visibility isolation (sunVisible)
	fmt.Println("\n[VERIFICATION 3] Testing Optical Effect visibility isolation (sunVisible on sun-glow)...")
	testSunVisJS := `(() => {
		instances['unit-c'].setParameter('sunVisible', false);
		const elA = document.querySelector('#container-unit-a svg [id$="sun-glow"]');
		const elB = document.querySelector('#container-unit-b svg [id$="sun-glow"]');
		const elC = document.querySelector('#container-unit-c svg [id$="sun-glow"]');
		return {
			valA: instances['unit-a'].getParameter('sunVisible'),
			valB: instances['unit-b'].getParameter('sunVisible'),
			valC: instances['unit-c'].getParameter('sunVisible'),
			visA: elA ? (elA.getAttribute('visibility') || elA.style.visibility || 'visible') : null,
			visB: elB ? (elB.getAttribute('visibility') || elB.style.visibility || 'visible') : null,
			visC: elC ? (elC.getAttribute('visibility') || elC.style.visibility) : null
		};
	})()`

	var resSunVis struct {
		ValA bool   `json:"valA"`
		ValB bool   `json:"valB"`
		ValC bool   `json:"valC"`
		VisA string `json:"visA"`
		VisB string `json:"visB"`
		VisC string `json:"visC"`
	}
	resSunVisRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testSunVisJS)))
	json.Unmarshal([]byte(resSunVisRaw), &resSunVis)

	if resSunVis.ValC != false || resSunVis.VisC != "hidden" {
		log.Printf("[FAIL] Unit C sun-glow was not hidden: %+v", resSunVis)
		return 1
	}
	if resSunVis.ValA != true || resSunVis.VisA == "hidden" {
		log.Printf("[FAIL] Unit A sun-glow was erroneously hidden: %+v", resSunVis)
		return 1
	}
	if resSunVis.ValB != true || resSunVis.VisB == "hidden" {
		log.Printf("[FAIL] Unit B sun-glow was erroneously hidden: %+v", resSunVis)
		return 1
	}
	fmt.Printf("[PASS] Sun glow hidden on Unit C (visibility: hidden) while remaining visible on Unit A and Unit B.\n")

	// Verification Test 4: Object-specific Flow animation speed vs whole-instance speed
	fmt.Println("\n[VERIFICATION 4] Testing targeted Flow animation speed (flowSpeed on flow-1) vs root playbackRate...")
	testSpeedJS := `(() => {
		// Set flowSpeed to 3.0 on Unit A
		instances['unit-a'].setParameter('flowSpeed', 3.0);
		
		const elFlowA = document.querySelector('#container-unit-a svg [id$="flow-1"] animate');
		const elFlowB = document.querySelector('#container-unit-b svg [id$="flow-1"] animate');
		
		return {
			flowSpeedA: instances['unit-a'].getParameter('flowSpeed'),
			flowSpeedB: instances['unit-b'].getParameter('flowSpeed'),
			instanceSpeedA: instances['unit-a'].getSpeed(),
			instanceSpeedB: instances['unit-b'].getSpeed(),
			flowDurA: elFlowA ? elFlowA.getAttribute('dur') : null,
			flowDurB: elFlowB ? elFlowB.getAttribute('dur') : null
		};
	})()`

	var resSpeed struct {
		FlowSpeedA     float64 `json:"flowSpeedA"`
		FlowSpeedB     float64 `json:"flowSpeedB"`
		InstanceSpeedA float64 `json:"instanceSpeedA"`
		InstanceSpeedB float64 `json:"instanceSpeedB"`
		FlowDurA       string  `json:"flowDurA"`
		FlowDurB       string  `json:"flowDurB"`
	}
	resSpeedRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testSpeedJS)))
	json.Unmarshal([]byte(resSpeedRaw), &resSpeed)

	if resSpeed.FlowSpeedA != 3.0 {
		log.Printf("[FAIL] Unit A flowSpeed not updated to 3.0: %+v", resSpeed)
		return 1
	}
	if resSpeed.InstanceSpeedA != 1.0 {
		log.Printf("[FAIL] Targeted flowSpeed unexpectedly modified Unit A's whole-instance playbackRate (must stay 1.0): %+v", resSpeed)
		return 1
	}
	if resSpeed.FlowSpeedB != 1.0 || resSpeed.InstanceSpeedB != 1.0 {
		log.Printf("[FAIL] Unit B speed was modified: %+v", resSpeed)
		return 1
	}
	fmt.Printf("[PASS] Targeted flowSpeed set to 3.0x on Unit A without affecting Unit A's whole-instance speed (%v) or Unit B (%v).\n", resSpeed.InstanceSpeedA, resSpeed.FlowSpeedB)

	// Verification Test 5: Whole-instance playbackRate control
	fmt.Println("\n[VERIFICATION 5] Testing whole-instance playbackRate on Unit B...")
	testGlobalSpeedJS := `(() => {
		instances['unit-b'].setParameter('playbackRate', 0.5);
		return {
			rateA: instances['unit-a'].getSpeed(),
			rateB: instances['unit-b'].getSpeed(),
			rateC: instances['unit-c'].getSpeed()
		};
	})()`

	var resGlobalSpeed struct {
		RateA float64 `json:"rateA"`
		RateB float64 `json:"rateB"`
		RateC float64 `json:"rateC"`
	}
	resGlobalSpeedRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testGlobalSpeedJS)))
	json.Unmarshal([]byte(resGlobalSpeedRaw), &resGlobalSpeed)

	if resGlobalSpeed.RateB != 0.5 {
		log.Printf("[FAIL] Unit B playbackRate not 0.5: %+v", resGlobalSpeed)
		return 1
	}
	if resGlobalSpeed.RateA != 1.0 || resGlobalSpeed.RateC != 1.0 {
		log.Printf("[FAIL] Unit A or Unit C affected by Unit B playbackRate change: %+v", resGlobalSpeed)
		return 1
	}
	fmt.Printf("[PASS] Whole-instance playbackRate set to 0.5 on Unit B while Unit A and Unit C remain at 1.0.\n")

	// Verification Test 6: Per-instance Play and Pause transport controls
	fmt.Println("\n[VERIFICATION 6] Testing per-instance Play and Pause transport controls...")
	testTransportJS := `(() => {
		instances['unit-a'].pause();
		const pausedA = instances['unit-a'].isPaused();
		const pausedB = instances['unit-b'].isPaused();
		const pausedC = instances['unit-c'].isPaused();
		
		instances['unit-a'].play();
		const resumedA = !instances['unit-a'].isPaused();
		
		return { pausedA, pausedB, pausedC, resumedA };
	})()`

	var resTransport struct {
		PausedA  bool `json:"pausedA"`
		PausedB  bool `json:"pausedB"`
		PausedC  bool `json:"pausedC"`
		ResumedA bool `json:"resumedA"`
	}
	resTransportRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testTransportJS)))
	json.Unmarshal([]byte(resTransportRaw), &resTransport)

	if !resTransport.PausedA || resTransport.PausedB || resTransport.PausedC || !resTransport.ResumedA {
		log.Printf("[FAIL] Independent Play/Pause failed: %+v", resTransport)
		return 1
	}
	fmt.Println("[PASS] Unit A independently paused and resumed while Unit B and Unit C continued active playback.")

	// Verification Test 7: Reset to Defaults
	fmt.Println("\n[VERIFICATION 7] Testing Reset Defaults on Unit A...")
	testResetJS := `(() => {
		instances['unit-a'].resetParameters();
		const elA = document.querySelector('#container-unit-a svg [id$="bird-perch"] rect');
		return {
			perchColorA: instances['unit-a'].getParameter('perchColor'),
			flowSpeedA: instances['unit-a'].getParameter('flowSpeed'),
			fillA: elA ? elA.getAttribute('fill') : null,
			perchColorB: instances['unit-b'].getParameter('perchColor'),
			sunColorB: instances['unit-b'].getParameter('sunColor')
		};
	})()`

	var resReset struct {
		PerchColorA string  `json:"perchColorA"`
		FlowSpeedA  float64 `json:"flowSpeedA"`
		FillA       string  `json:"fillA"`
		PerchColorB string  `json:"perchColorB"`
		SunColorB   string  `json:"sunColorB"`
	}
	resResetRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", testResetJS)))
	json.Unmarshal([]byte(resResetRaw), &resReset)

	if resReset.PerchColorA != "#8B4513" || resReset.FillA != "#8B4513" || resReset.FlowSpeedA != 1.0 {
		log.Printf("[FAIL] Unit A did not restore defaults: %+v", resReset)
		return 1
	}
	if resReset.SunColorB != "#00FF00" {
		log.Printf("[FAIL] Reset on Unit A improperly mutated Unit B state: %+v", resReset)
		return 1
	}
	fmt.Printf("[PASS] Reset on Unit A restored perchColor (#8B4513) and flowSpeed (1.0) without altering Unit B (#00FF00).\n")

	// Verification Test 8: Check for JavaScript console errors
	fmt.Println("\n[VERIFICATION 8] Checking for browser console errors...")
	checkErrorsJS := `(() => {
		return window.__errors || [];
	})()`
	var jsErrors []string
	jsErrorsRaw, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", checkErrorsJS)))
	json.Unmarshal([]byte(jsErrorsRaw), &jsErrors)

	if len(jsErrors) > 0 {
		log.Printf("[FAIL] Detected console errors during execution: %v", jsErrors)
		return 1
	}
	fmt.Println("[PASS] Zero console errors or uncaught exceptions reported in Google Chrome.")

	fmt.Println("\n==================================================================")
	fmt.Println(" ALL STAGE 16B BIRDSCROLL VERIFICATION TESTS COMPLETED SUCCESSFULLY!")
	fmt.Println("==================================================================")
	return 0
}
