package scene

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWidgetDocsGeneration(t *testing.T) {
	sc := NewScene()
	sc.canvas.Width = 800
	sc.canvas.Height = 600

	node1 := sc.CreateNode(Node{
		Name:      "tankHull",
		Shape:     "rounded",
		Width:     200,
		Height:    300,
		FillColor: "#1f2937",
	}, nil, false)

	node2 := sc.CreateNode(Node{
		Name:      "fluidColumn",
		Shape:     "rect",
		Width:     180,
		Height:    150,
		FillColor: "#38bdf8",
	}, nil, false)

	glow1 := sc.CreateGlow(Glow{
		Name:  "warningBeacon",
		Color: "#22c55e",
		Size:  25,
	})

	minVal := 10.0
	maxVal := 250.0
	stepVal := 5.0
	err := sc.AddParameter(WidgetParameter{
		Name:        "hullColor",
		Target:      node1.Name,
		Property:    "fill",
		Type:        "color",
		Default:     "#1f2937",
		Description: "External tank body color",
	})
	if err != nil {
		t.Fatalf("AddParameter hullColor failed: %v", err)
	}

	err = sc.AddParameter(WidgetParameter{
		Name:        "fluidLevel",
		Target:      "node-" + string(node2.ID),
		Property:    "height",
		Type:        "number",
		Default:     150.0,
		Min:         &minVal,
		Max:         &maxVal,
		Step:        &stepVal,
		Unit:        "mm",
		Description: "Internal fluid level height",
	})
	if err != nil {
		t.Fatalf("AddParameter fluidLevel failed: %v", err)
	}

	err = sc.AddParameter(WidgetParameter{
		Name:        "beaconColor",
		Target:      "glow-" + string(glow1.ID),
		Property:    "color",
		Type:        "color",
		Default:     "#22c55e",
		Description: "Warning beacon color",
	})
	if err != nil {
		t.Fatalf("AddParameter beaconColor failed: %v", err)
	}

	manifest := &WidgetManifest{
		SchemaVersion: 1,
		Parameters:    sc.GetParameters(),
	}

	// 1. Verify Properties Guide
	guide := GenerateWidgetPropertiesGuide(sc, "Industrial Storage Tank", "industrial-storage-tank.svg", manifest)
	if !strings.Contains(guide, "# Industrial Storage Tank — Widget Properties & Integration Guide") {
		t.Errorf("Properties guide missing title")
	}
	if !strings.Contains(guide, "800 × 600 px") {
		t.Errorf("Properties guide missing dimensions")
	}
	if !strings.Contains(guide, "`hullColor`") || !strings.Contains(guide, "`fluidLevel`") || !strings.Contains(guide, "`beaconColor`") {
		t.Errorf("Properties guide missing parameter names")
	}
	if !strings.Contains(guide, "10.0 to 250.0") {
		t.Errorf("Properties guide missing min-max range for fluidLevel")
	}
	if !strings.Contains(guide, "mm") {
		t.Errorf("Properties guide missing unit 'mm'")
	}
	if !strings.Contains(guide, "myWidget.setParameter('hullColor', '#22c55e')") {
		t.Errorf("Properties guide missing real code example using hullColor")
	}
	if !strings.Contains(guide, "instA.setParameter('hullColor'") {
		t.Errorf("Properties guide missing multi-instance code example")
	}

	// 2. Verify README
	readme := GenerateWidgetReadme("Industrial Storage Tank", "industrial-storage-tank.svg", manifest)
	if !strings.Contains(readme, "# Industrial Storage Tank — Portable SVG Widget Package") {
		t.Errorf("Readme missing title")
	}
	if !strings.Contains(readme, "**Total Exposed Parameters:** 3") {
		t.Errorf("Readme missing parameter count, got:\n%s", readme)
	}
	if !strings.Contains(readme, "demo.html") || !strings.Contains(readme, "js/svg-widget.js") {
		t.Errorf("Readme missing file listing")
	}

	// 3. Verify Demo HTML
	demoHTML := GenerateWidgetDemoHTML("Industrial Storage Tank", "industrial-storage-tank.svg", manifest)
	if !strings.Contains(demoHTML, "<!DOCTYPE html>") {
		t.Errorf("Demo HTML missing doctype")
	}
	if !strings.Contains(demoHTML, "unit-a") || !strings.Contains(demoHTML, "unit-b") || !strings.Contains(demoHTML, "unit-c") {
		t.Errorf("Demo HTML missing 3 instance IDs")
	}
	if !strings.Contains(demoHTML, "SVGWidget.mount") {
		t.Errorf("Demo HTML missing SVGWidget.mount call")
	}
	if !strings.Contains(demoHTML, "industrial-storage-tank.svg") {
		t.Errorf("Demo HTML missing SVG asset filename")
	}

	// 4. Verify ZIP Package
	rawSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600"><rect id="tankHull" width="200" height="300"/></svg>`
	mockJS := `// Mock svg-widget.js runtime v1.2.0`

	zipBytes, err := BuildWidgetPackageZip(sc, "Industrial Storage Tank", rawSVG, mockJS)
	if err != nil {
		t.Fatalf("BuildWidgetPackageZip failed: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatalf("BuildWidgetPackageZip produced empty bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("Failed to open generated ZIP with archive/zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"industrial-storage-tank/industrial-storage-tank.svg":           false,
		"industrial-storage-tank/industrial-storage-tank-properties.md": false,
		"industrial-storage-tank/widget-manifest.json":                  false,
		"industrial-storage-tank/README.md":                             false,
		"industrial-storage-tank/demo.html":                             false,
		"industrial-storage-tank/js/svg-widget.js":                      false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
			rc, err := f.Open()
			if err != nil {
				t.Errorf("Failed opening zip entry %s: %v", f.Name, err)
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			if len(content) == 0 {
				t.Errorf("Zip entry %s is empty", f.Name)
			}
		}
	}

	for fname, found := range expectedFiles {
		if !found {
			t.Errorf("ZIP archive missing expected file: %s", fname)
		}
	}
}
