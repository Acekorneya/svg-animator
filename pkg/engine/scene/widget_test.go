package scene

import (
	"strings"
	"testing"
)

func TestIsolateSVGWidgetBasic(t *testing.T) {
	raw := `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600">
  <defs>
    <filter id="glowBlur-1"><feGaussianBlur stdDeviation="3"/></filter>
    <radialGradient id="glowGrad-1"><stop offset="0%" stop-color="#38bdf8"/></radialGradient>
    <marker id="flowArrow-1"><path d="M 0 0 L 10 5 L 0 10 z" fill="#38bdf8"/></marker>
    <clipPath id="viewportClip"><rect width="800" height="600"/></clipPath>
  </defs>
  <style>
    .camera-rig {
      transform: translate3d(0, 0, 0);
      animation: camKenBurns 5s linear infinite;
    }
    @keyframes camKenBurns {
      0% { transform: scale(1); }
      100% { transform: scale(1.2); }
    }
  </style>
  <rect width="800" height="600" fill="url(#glowGrad-1)" clip-path="url(#viewportClip)"/>
  <path id="flowPath-1" d="M 10 10 L 100 100" marker-end="url(#flowArrow-1)"/>
  <animateMotion><mpath href="#flowPath-1" xlink:href="#flowPath-1"/></animateMotion>
  <image id="img-1" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="/>
  <text fill="#ffffff">Node #1 Status: #Active</text>
</svg>`

	opts := WidgetIsolationOptions{
		InstanceID: "widget-test1",
		Prefix:     "test1__",
		ScopeCSS:   true,
	}

	isolated, idMap, err := IsolateSVGWidget(raw, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Verify root SVG has isolated ID and instance attribute
	if !strings.Contains(isolated, `id="sva-widget-widget-test1"`) {
		t.Errorf("expected root svg to have id sva-widget-widget-test1, got: %s", isolated)
	}
	if !strings.Contains(isolated, `data-sva-widget-instance="widget-test1"`) {
		t.Errorf("expected data-sva-widget-instance attribute on root svg")
	}

	// 2. Verify all defs IDs have prefix
	expectedIDs := []string{"glowBlur-1", "glowGrad-1", "flowArrow-1", "viewportClip", "flowPath-1", "img-1"}
	for _, id := range expectedIDs {
		expectedNewID := "test1__" + id
		if idMap[id] != expectedNewID {
			t.Errorf("expected idMap[%s] = %s, got %s", id, expectedNewID, idMap[id])
		}
		if !strings.Contains(isolated, `id="`+expectedNewID+`"`) {
			t.Errorf("expected markup to contain id=\"%s\"", expectedNewID)
		}
	}

	// 3. Verify internal references remapped
	if !strings.Contains(isolated, `fill="url(#test1__glowGrad-1)"`) {
		t.Errorf("expected fill to reference #test1__glowGrad-1")
	}
	if !strings.Contains(isolated, `clip-path="url(#test1__viewportClip)"`) {
		t.Errorf("expected clip-path to reference #test1__viewportClip")
	}
	if !strings.Contains(isolated, `marker-end="url(#test1__flowArrow-1)"`) {
		t.Errorf("expected marker-end to reference #test1__flowArrow-1")
	}
	if !strings.Contains(isolated, `href="#test1__flowPath-1"`) {
		t.Errorf("expected mpath href to reference #test1__flowPath-1")
	}
	if !strings.Contains(isolated, `xlink:href="#test1__flowPath-1"`) {
		t.Errorf("expected mpath xlink:href to reference #test1__flowPath-1")
	}

	// 4. Verify data URL is completely preserved and untouched
	if !strings.Contains(isolated, `href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="`) {
		t.Errorf("expected data URL to remain untouched")
	}

	// 5. Verify text content containing '#' is preserved untouched
	if !strings.Contains(isolated, `Node #1 Status: #Active`) {
		t.Errorf("expected text content with # to remain untouched")
	}

	// 6. Verify CSS scoping and keyframe renaming
	if !strings.Contains(isolated, `@keyframes test1__camKenBurns`) {
		t.Errorf("expected @keyframes to be renamed to test1__camKenBurns")
	}
	if !strings.Contains(isolated, `animation: test1__camKenBurns 5s linear infinite`) {
		t.Errorf("expected animation property to reference test1__camKenBurns")
	}
	if !strings.Contains(isolated, `#sva-widget-widget-test1 .camera-rig`) {
		t.Errorf("expected .camera-rig selector to be scoped to #sva-widget-widget-test1")
	}
}

func TestIsolateSVGWidgetMultiInstanceCollisionFree(t *testing.T) {
	raw := `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360">
  <defs>
    <radialGradient id="gradMain"><stop stop-color="#ff0000"/></radialGradient>
    <filter id="blurMain"><feGaussianBlur stdDeviation="2"/></filter>
    <path id="sharedPath" d="M 0 0 L 100 100"/>
  </defs>
  <style>
    @keyframes animMain { from { opacity: 0; } to { opacity: 1; } }
    .box { animation: animMain 2s; }
  </style>
  <rect id="rect1" fill="url(#gradMain)" filter="url(#blurMain)"/>
  <use href="#sharedPath" xlink:href="#sharedPath"/>
</svg>`

	// Instance A
	isoA, mapA, errA := IsolateSVGWidget(raw, WidgetIsolationOptions{
		InstanceID: "inst-a",
		Prefix:     "inst_a__",
	})
	if errA != nil {
		t.Fatalf("instance A failed: %v", errA)
	}

	// Instance B
	isoB, mapB, errB := IsolateSVGWidget(raw, WidgetIsolationOptions{
		InstanceID: "inst-b",
		Prefix:     "inst_b__",
	})
	if errB != nil {
		t.Fatalf("instance B failed: %v", errB)
	}

	// Verify no ID collision between Instance A and Instance B
	setA := make(map[string]bool)
	for _, v := range mapA {
		setA[v] = true
	}
	for _, v := range mapB {
		if setA[v] {
			t.Errorf("ID collision detected between instances: %s", v)
		}
	}

	// Verify Instance A references only inst_a__ IDs
	if strings.Contains(isoA, "inst_b__") {
		t.Errorf("Instance A contains reference to Instance B prefix")
	}
	if strings.Contains(isoB, "inst_a__") {
		t.Errorf("Instance B contains reference to Instance A prefix")
	}

	// Verify root IDs are distinct
	if !strings.Contains(isoA, `id="sva-widget-inst-a"`) {
		t.Errorf("expected root ID sva-widget-inst-a")
	}
	if !strings.Contains(isoB, `id="sva-widget-inst-b"`) {
		t.Errorf("expected root ID sva-widget-inst-b")
	}
}

func TestIsolateSVGWidgetValidation(t *testing.T) {
	_, _, err := IsolateSVGWidget("", WidgetIsolationOptions{})
	if err == nil {
		t.Errorf("expected error for empty rawSVG")
	}

	// Auto-generation of instance ID when empty
	iso, mapAuto, err := IsolateSVGWidget(`<svg><circle id="c1"/></svg>`, WidgetIsolationOptions{})
	if err != nil {
		t.Fatalf("unexpected error with auto instance ID: %v", err)
	}
	if len(mapAuto) != 1 || !strings.Contains(mapAuto["c1"], "c1") {
		t.Errorf("expected c1 to be mapped in auto mode, got %v", mapAuto)
	}
	if !strings.Contains(iso, "data-sva-widget-instance") {
		t.Errorf("expected auto generated data-sva-widget-instance attribute")
	}
}

func TestIsolateSVGWidgetWithManifest(t *testing.T) {
	raw := `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="300">
  <rect id="statusBox" fill="#22c55e" width="100" height="50"/>
</svg>`

	minVal := 0.0
	maxVal := 100.0
	opts := WidgetIsolationOptions{
		InstanceID: "barrel-a",
		Prefix:     "barrel_a__",
		Manifest: &WidgetManifest{
			SchemaVersion: 1,
			Parameters: []WidgetParameter{
				{
					Name:        "statusColor",
					Type:        "color",
					Default:     "#22c55e",
					Target:      "statusBox",
					Property:    "fill",
					Description: "Status box fill color",
				},
				{
					Name:        "level",
					Type:        "number",
					Default:     75.0,
					Target:      "statusBox",
					Property:    "width",
					Min:         &minVal,
					Max:         &maxVal,
					Description: "Fill level percentage",
				},
			},
		},
	}

	isolated, idMap, err := IsolateSVGWidget(raw, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify manifest script tag injected with namespaced ID
	if !strings.Contains(isolated, `<script type="application/json" id="barrel_a__sva-widget-manifest">`) {
		t.Errorf("expected sva-widget-manifest script tag in isolated svg")
	}
	if !strings.Contains(isolated, `"name":"statusColor"`) {
		t.Errorf("expected statusColor in injected manifest")
	}
	if !strings.Contains(isolated, `"name":"level"`) {
		t.Errorf("expected level in injected manifest")
	}

	// Verify ID remapping
	if idMap["statusBox"] != "barrel_a__statusBox" {
		t.Errorf("expected statusBox mapped to barrel_a__statusBox, got %s", idMap["statusBox"])
	}
	if !strings.Contains(isolated, `id="barrel_a__statusBox"`) {
		t.Errorf("expected isolated SVG to contain id=\"barrel_a__statusBox\"")
	}
}

func TestValidateWidgetParameter(t *testing.T) {
	// 1. Valid parameter
	minVal := 0.0
	maxVal := 1.0
	valid := WidgetParameter{
		Name:     "tankOpacity",
		Type:     "number",
		Default:  0.8,
		Target:   "node-1",
		Property: "opacity",
		Min:      &minVal,
		Max:      &maxVal,
	}
	if err := ValidateWidgetParameter(valid); err != nil {
		t.Fatalf("expected valid parameter to pass, got: %v", err)
	}

	// 2. Invalid names (empty, starts with digit, contains space, symbols)
	invalidNames := []string{"", "123level", "tank level", "tank@color", "tank.val"}
	for _, badName := range invalidNames {
		p := valid
		p.Name = badName
		if err := ValidateWidgetParameter(p); err == nil {
			t.Errorf("expected error for invalid name %q, got nil", badName)
		}
	}

	// 3. Unsafe properties (script, onclick, href, style, javascript)
	unsafeProps := []string{"onclick", "onmouseover", "href", "xlink:href", "style", "script", "javascript:void(0)"}
	for _, badProp := range unsafeProps {
		p := valid
		p.Property = badProp
		if err := ValidateWidgetParameter(p); err == nil {
			t.Errorf("expected error for unsafe property %q, got nil", badProp)
		}
	}

	// 4. Incompatible type and property
	pColorWrong := WidgetParameter{
		Name:     "myColor",
		Type:     "color",
		Target:   "node-1",
		Property: "height",
	}
	if err := ValidateWidgetParameter(pColorWrong); err == nil {
		t.Errorf("expected error when color parameter has property 'height', got nil")
	}

	// 5. Min > Max
	badMin := 200.0
	badMax := 100.0
	pRange := WidgetParameter{
		Name:     "myRange",
		Type:     "number",
		Target:   "custom-tank",
		Property: "width",
		Min:      &badMin,
		Max:      &badMax,
	}
	if err := ValidateWidgetParameter(pRange); err == nil {
		t.Errorf("expected error when min > max, got nil")
	}

	// 6. Enum with empty options
	pEnumEmpty := WidgetParameter{
		Name:     "myState",
		Type:     "enum",
		Target:   "root",
		Property: "state",
		Options:  []string{},
	}
	if err := ValidateWidgetParameter(pEnumEmpty); err == nil {
		t.Errorf("expected error when enum has empty options, got nil")
	}
}

func TestSceneParameterPruningOnDelete(t *testing.T) {
	sc := NewScene()
	n1 := sc.CreateNode(Node{Shape: "rect", X: 10, Y: 10, Width: 100, Height: 50}, nil, false)

	err := sc.AddParameter(WidgetParameter{
		Name:     "nodeColor",
		Type:     "color",
		Default:  "#ff0000",
		Target:   "node-" + string(n1.ID),
		Property: "fill",
	})
	if err != nil {
		t.Fatalf("AddParameter failed: %v", err)
	}

	err = sc.AddParameter(WidgetParameter{
		Name:     "globalSpeed",
		Type:     "speed",
		Default:  1.0,
		Target:   "root",
		Property: "playbackRate",
	})
	if err != nil {
		t.Fatalf("AddParameter failed: %v", err)
	}

	if len(sc.GetParameters()) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(sc.GetParameters()))
	}

	// Remove node n1 -> parameter 'nodeColor' targeting 'node-1' should be safely pruned
	sc.RemoveNode(n1.ID)

	paramsAfter := sc.GetParameters()
	if len(paramsAfter) != 1 {
		t.Fatalf("expected 1 parameter after node removal, got %d", len(paramsAfter))
	}
	if paramsAfter[0].Name != "globalSpeed" {
		t.Errorf("expected globalSpeed remaining, got %s", paramsAfter[0].Name)
	}
}

func TestParameterUndoRedo(t *testing.T) {
	sc := NewScene()
	sc.RecordHistory() // Record baseline state

	p := WidgetParameter{
		Name:     "statusColor",
		Type:     "color",
		Default:  "#22c55e",
		Target:   "node-1",
		Property: "fill",
	}

	if err := sc.AddParameter(p); err != nil {
		t.Fatalf("AddParameter failed: %v", err)
	}
	if len(sc.GetParameters()) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(sc.GetParameters()))
	}

	// Undo addition
	_, err := sc.UndoHistory()
	if err != nil {
		t.Fatalf("expected undo to succeed, got %v", err)
	}
	if len(sc.GetParameters()) != 0 {
		t.Errorf("expected 0 parameters after undo, got %d", len(sc.GetParameters()))
	}

	// Redo addition
	_, err = sc.RedoHistory()
	if err != nil {
		t.Fatalf("expected redo to succeed, got %v", err)
	}
	if len(sc.GetParameters()) != 1 || sc.GetParameters()[0].Name != "statusColor" {
		t.Errorf("expected 1 parameter 'statusColor' after redo, got %+v", sc.GetParameters())
	}
}

func TestWidgetPropertyEligibilityAndUniquenessContract(t *testing.T) {
	// 1. Check Catalog and GetEligibleProperties
	nodeProps := GetEligibleProperties("node")
	if len(nodeProps) == 0 {
		t.Fatalf("expected non-empty node eligible properties")
	}
	hasFill := false
	hasSpeed := false
	for _, ep := range nodeProps {
		if ep.Property == "fill" {
			hasFill = true
		}
		if ep.Property == "speed" {
			hasSpeed = true
		}
	}
	if !hasFill {
		t.Errorf("expected node to have 'fill'")
	}
	if hasSpeed {
		t.Errorf("expected node NOT to have 'speed'")
	}

	// 2. Reject ineligible property on node
	badNodeParam := WidgetParameter{
		Name:     "nodeSpeed",
		Type:     "speed",
		Target:   "node-1",
		Property: "speed",
	}
	if err := ValidateWidgetParameter(badNodeParam); err == nil {
		t.Errorf("expected ValidateWidgetParameter to reject 'speed' on node-1, got nil")
	}

	// 3. Reject invalid type for property (e.g. number for fill)
	badTypeParam := WidgetParameter{
		Name:     "nodeFill",
		Type:     "number",
		Target:   "node-1",
		Property: "fill",
	}
	if err := ValidateWidgetParameter(badTypeParam); err == nil {
		t.Errorf("expected ValidateWidgetParameter to reject 'number' for 'fill', got nil")
	}

	// 4. Test Uniqueness Contract in Scene
	sc := NewScene()

	// Add flow-1 stroke -> flow1Color
	err := sc.AddParameter(WidgetParameter{
		Name:     "flow1Color",
		Type:     "color",
		Target:   "flow-1",
		Property: "stroke",
		Default:  "#38bdf8",
	})
	if err != nil {
		t.Fatalf("AddParameter flow1Color failed: %v", err)
	}

	// Attempt duplicate binding for flow-1 stroke with a different name -> flow1c
	err = sc.AddParameter(WidgetParameter{
		Name:     "flow1c",
		Type:     "color",
		Target:   "flow-1",
		Property: "stroke",
		Default:  "#ff0000",
	})
	if err == nil {
		t.Fatalf("expected duplicate binding on (flow-1, stroke) to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "already bound") {
		t.Errorf("expected error message to contain 'already bound', got: %v", err)
	}

	// Updating flow1Color in place (same name, same target & property) succeeds
	err = sc.AddParameter(WidgetParameter{
		Name:        "flow1Color",
		Type:        "color",
		Target:      "flow-1",
		Property:    "stroke",
		Default:     "#22c55e",
		Description: "Updated color",
	})
	if err != nil {
		t.Fatalf("updating flow1Color in place failed: %v", err)
	}
	params := sc.GetParameters()
	if len(params) != 1 || params[0].Default != "#22c55e" {
		t.Errorf("expected updated flow1Color default '#22c55e', got %+v", params)
	}

	// Adding a DIFFERENT property to flow-1 succeeds (speed)
	speedVal := 1.5
	minSpeed := 0.1
	maxSpeed := 5.0
	err = sc.AddParameter(WidgetParameter{
		Name:     "flow1Speed",
		Type:     "speed",
		Target:   "flow-1",
		Property: "speed",
		Default:  speedVal,
		Min:      &minSpeed,
		Max:      &maxSpeed,
	})
	if err != nil {
		t.Fatalf("AddParameter flow1Speed on flow-1 failed: %v", err)
	}
	if len(sc.GetParameters()) != 2 {
		t.Fatalf("expected 2 parameters on flow-1, got %d", len(sc.GetParameters()))
	}

	// 5. RenameParameter
	// Renaming to existing name is rejected
	err = sc.RenameParameter("flow1Speed", "flow1Color")
	if err == nil {
		t.Errorf("expected renaming to existing name to fail, got nil")
	}
	// Renaming non-existent name is rejected
	err = sc.RenameParameter("nonExistent", "newParam")
	if err == nil {
		t.Errorf("expected renaming non-existent parameter to fail, got nil")
	}
	// Renaming flow1Speed -> flow1Rate succeeds
	err = sc.RenameParameter("flow1Speed", "flow1Rate")
	if err != nil {
		t.Fatalf("RenameParameter flow1Speed -> flow1Rate failed: %v", err)
	}
	params = sc.GetParameters()
	var foundRenamed bool
	for _, p := range params {
		if p.Name == "flow1Rate" {
			foundRenamed = true
			if p.Target != "flow-1" || p.Property != "speed" {
				t.Errorf("renamed parameter lost target/property identity: %+v", p)
			}
		}
	}
	if !foundRenamed {
		t.Errorf("renamed parameter 'flow1Rate' not found")
	}

	// 6. Conflict Detection and Resolution
	// Simulate existing loaded project with duplicate bindings using SetParameters
	conflictingParams := []WidgetParameter{
		{Name: "flow1color", Type: "color", Target: "flow-1", Property: "stroke", Default: "#111111"},
		{Name: "flow1c", Type: "color", Target: "flow-1", Property: "stroke", Default: "#222222"},
		{Name: "flow1op", Type: "number", Target: "flow-1", Property: "opacity", Default: 0.5},
	}
	if err := sc.SetParameters(conflictingParams); err != nil {
		t.Fatalf("SetParameters with conflicts failed: %v", err)
	}
	conflicts := sc.DetectParameterConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Target != "flow-1" || conflicts[0].Property != "stroke" || len(conflicts[0].Parameters) != 2 {
		t.Errorf("unexpected conflict details: %+v", conflicts[0])
	}

	// Exporting package with conflicts must fail
	_, err = BuildWidgetPackageZip(sc, "TestWidget", "<svg></svg>", "// js")
	if err == nil {
		t.Fatalf("expected BuildWidgetPackageZip to fail on unresolved conflicts, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting parameter bindings") {
		t.Errorf("expected error to mention conflicting parameter bindings, got: %v", err)
	}

	// Resolve conflict by keeping flow1color
	err = sc.ResolveParameterConflict("flow-1", "stroke", "flow1color")
	if err != nil {
		t.Fatalf("ResolveParameterConflict failed: %v", err)
	}
	conflictsAfter := sc.DetectParameterConflicts()
	if len(conflictsAfter) != 0 {
		t.Errorf("expected 0 conflicts after resolution, got %d", len(conflictsAfter))
	}
	resolvedParams := sc.GetParameters()
	if len(resolvedParams) != 2 {
		t.Fatalf("expected 2 parameters after resolution, got %d", len(resolvedParams))
	}
	for _, p := range resolvedParams {
		if p.Name == "flow1c" {
			t.Errorf("flow1c should have been removed during conflict resolution")
		}
	}
}
