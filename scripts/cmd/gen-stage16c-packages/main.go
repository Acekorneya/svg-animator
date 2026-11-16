package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/engine/geometry"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/engine/scene"
)

func buildHostMonitorPackage(outDir, widgetName, nodeName string, includeRippleVisibility bool) error {
	sc := scene.NewScene()
	sc.SetCanvas(scene.CanvasConfig{
		Width:    360,
		Height:   220,
		Duration: 4.0,
		BgColor:  "#0f172a",
	})

	n1 := sc.CreateNode(scene.Node{
		ID:          "1",
		Name:        nodeName,
		Shape:       "rounded",
		Text:        "host-01",
		X:           40,
		Y:           70,
		Width:       150,
		Height:      80,
		FillColor:   "#1e293b",
		StrokeColor: "#38bdf8",
	}, nil, false)

	f1 := sc.CreateFlow(scene.Flow{
		ID:         "1",
		Name:       "traffic",
		FromNodeID: n1.ID,
		Nodes: []geometry.Point{
			{X: 190, Y: 110},
			{X: 310, Y: 110},
		},
		Color:   "#22c55e",
		Speed:   1.5,
		Opacity: 0.9,
	})

	g1 := sc.CreateGlow(scene.Glow{
		ID:       "1",
		Name:     "status",
		X:        290,
		Y:        60,
		Size:     28,
		Color:    "#22c55e",
		Opacity:  0.8,
		Duration: 1.8,
		Playback: "pingpong",
	})

	r1 := sc.CreateRipple(scene.Ripple{
		ID:       "1",
		Name:     "heartbeat",
		X:        290,
		Y:        155,
		InnerR:   6,
		OuterR:   32,
		Duration: 2.2,
		Color:    "#38bdf8",
		Opacity:  0.7,
		Rings:    2,
		Thick:    2,
	})

	params := []scene.WidgetParameter{
		{
			Name:     scene.GenerateParameterName(nodeName, "text", "node"),
			Type:     "text",
			Target:   "node-" + string(n1.ID),
			Property: "text",
			Default:  "host-01",
		},
		{
			Name:     scene.GenerateParameterName(nodeName, "fill", "node"),
			Type:     "color",
			Target:   "node-" + string(n1.ID),
			Property: "fill",
			Default:  "#1e293b",
		},
		{
			Name:     scene.GenerateParameterName(nodeName, "stroke", "node"),
			Type:     "color",
			Target:   "node-" + string(n1.ID),
			Property: "stroke",
			Default:  "#38bdf8",
		},
		{
			Name:     scene.GenerateParameterName("traffic", "color", "flow"),
			Type:     "color",
			Target:   "flow-" + string(f1.ID),
			Property: "color",
			Default:  "#22c55e",
		},
		{
			Name:     scene.GenerateParameterName("traffic", "speed", "flow"),
			Type:     "speed",
			Target:   "flow-" + string(f1.ID),
			Property: "speed",
			Default:  1.0,
			Min:      floatPtr(0.2),
			Max:      floatPtr(5.0),
		},
		{
			Name:     scene.GenerateParameterName("traffic", "visibility", "flow"),
			Type:     "boolean",
			Target:   "flow-" + string(f1.ID),
			Property: "visibility",
			Default:  true,
		},
		{
			Name:     scene.GenerateParameterName("status", "color", "glow"),
			Type:     "color",
			Target:   "glow-" + string(g1.ID),
			Property: "color",
			Default:  "#22c55e",
		},
		{
			Name:     scene.GenerateParameterName("status", "opacity", "glow"),
			Type:     "number",
			Target:   "glow-" + string(g1.ID),
			Property: "opacity",
			Default:  0.8,
			Min:      floatPtr(0.0),
			Max:      floatPtr(1.0),
		},
		{
			Name:     scene.GenerateParameterName("heartbeat", "opacity", "ripple"),
			Type:     "number",
			Target:   "ripple-" + string(r1.ID),
			Property: "opacity",
			Default:  0.7,
			Min:      floatPtr(0.0),
			Max:      floatPtr(1.0),
		},
	}

	if includeRippleVisibility {
		params = append(params, scene.WidgetParameter{
			Name:     scene.GenerateParameterName("heartbeat", "visibility", "ripple"),
			Type:     "boolean",
			Target:   "ripple-" + string(r1.ID),
			Property: "visibility",
			Default:  true,
		})
	}

	for _, p := range params {
		if err := sc.AddParameter(p); err != nil {
			return fmt.Errorf("AddParameter(%s): %w", p.Name, err)
		}
	}

	rawSVG := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 220" width="360" height="220" style="background-color:#0f172a">
  <defs>
    <radialGradient id="glowGrad-1">
      <stop offset="0%%" stop-color="#22c55e" stop-opacity="0.9" />
      <stop offset="100%%" stop-color="#22c55e" stop-opacity="0" />
    </radialGradient>
    <path id="flowPath-1" d="M 190 110 L 310 110" />
  </defs>
  <rect x="0" y="0" width="360" height="220" fill="#0f172a" rx="10" />
  <g id="nodes-layer">
    <g id="node-1" class="diagram-node-instance" data-node-id="1" data-semantic-target="node-1" data-semantic-name="%s">
      <rect x="40" y="70" width="150" height="80" rx="10" fill="#1e293b" stroke="#38bdf8" stroke-width="2.5" />
      <text x="115" y="115" fill="#f8fafc" font-family="system-ui, sans-serif" font-size="16" font-weight="600" text-anchor="middle">host-01</text>
    </g>
  </g>
  <g id="flows-layer">
    <g id="flow-1" class="diagram-flow-instance" data-flow-id="1" data-semantic-target="flow-1" data-semantic-name="traffic">
      <path class="flow-track-path" d="M 190 110 L 310 110" stroke="none" fill="none" />
      <line x1="190" y1="110" x2="310" y2="110" stroke="#334155" stroke-width="2" stroke-dasharray="4 4" />
      <g class="flow-payload-group">
        <circle cx="0" cy="0" r="6" fill="#22c55e">
          <animateMotion dur="1.5s" repeatCount="indefinite" rotate="auto">
            <mpath href="#flowPath-1" />
          </animateMotion>
        </circle>
      </g>
    </g>
  </g>
  <g id="glows-layer">
    <g id="glow-1" class="diagram-glow-instance" data-glow-id="1" data-semantic-target="glow-1" data-semantic-name="status">
      <rect class="glow-hit-target" x="262" y="32" width="56" height="56" fill="transparent" pointer-events="all" />
      <g pointer-events="none">
        <circle cx="290" cy="60" r="24" fill="url(#glowGrad-1)">
          <animate attributeName="opacity" values="0.35; 1; 0.35" dur="1.8s" repeatCount="indefinite" />
        </circle>
      </g>
    </g>
  </g>
  <g id="ripples-layer">
    <g id="ripple-1" class="diagram-ripple-instance" data-ripple-id="1" data-semantic-target="ripple-1" data-semantic-name="heartbeat">
      <circle class="ripple-hit-target" cx="290" cy="155" r="32" fill="transparent" pointer-events="all" />
      <g pointer-events="none">
        <circle class="ripple-wave" cx="290" cy="155" r="6" fill="none" stroke="#38bdf8" stroke-width="2">
          <animate attributeName="r" values="6; 32" dur="2.2s" repeatCount="indefinite" />
          <animate attributeName="stroke-opacity" values="0.9; 0" dur="2.2s" repeatCount="indefinite" />
        </circle>
      </g>
    </g>
  </g>
</svg>`, nodeName)

	widgetJSBytes, err := os.ReadFile("internal/web/static/js/svg-widget.js")
	if err != nil {
		return fmt.Errorf("read svg-widget.js: %w", err)
	}

	zipBytes, err := scene.BuildWidgetPackageZip(sc, widgetName, rawSVG, string(widgetJSBytes))
	if err != nil {
		return fmt.Errorf("BuildWidgetPackageZip: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		destPath := filepath.Join(outDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func floatPtr(v float64) *float64 { return &v }

func main() {
	outDir := "demos/stage16c-widget-consumer/packages"
	if err := os.RemoveAll(outDir); err != nil {
		log.Fatal(err)
	}
	if err := buildHostMonitorPackage(outDir, "host-monitor-v1", "hostname", true); err != nil {
		log.Fatalf("v1 failed: %v", err)
	}
	if err := buildHostMonitorPackage(outDir, "host-monitor-v2", "server", false); err != nil {
		log.Fatalf("v2 failed: %v", err)
	}

	// Create an intentionally inconsistent package for consistency validation testing
	brokenDir := filepath.Join(outDir, "broken-inconsistent")
	_ = os.MkdirAll(filepath.Join(brokenDir, "js"), 0755)
	_ = os.WriteFile(filepath.Join(brokenDir, "widget-manifest.json"), []byte(`{
  "schemaVersion": 1,
  "packageVersion": "9.9.9",
  "packageId": "broken-mismatched-id",
  "widgetName": "broken-inconsistent",
  "parameters": [
    { "name": "hostname-node-text", "type": "text", "default": "broken", "target": "node-1", "property": "text" }
  ]
}`), 0644)
	svgV1, err := os.ReadFile(filepath.Join(outDir, "host-monitor-v1", "host-monitor-v1.svg"))
	if err == nil {
		_ = os.WriteFile(filepath.Join(brokenDir, "broken-inconsistent.svg"), svgV1, 0644)
	}
	jsV1, err := os.ReadFile(filepath.Join(outDir, "host-monitor-v1", "js", "svg-widget.js"))
	if err == nil {
		_ = os.WriteFile(filepath.Join(brokenDir, "js", "svg-widget.js"), jsV1, 0644)
	}

	fmt.Println("Generated Stage 16C widget packages in", outDir)
}
