package scene

import (
	"testing"
)

func TestObjectNamingAndParameterGeneration(t *testing.T) {
	sc := NewScene()

	// 1. Create objects with fixed prefixes
	n1 := sc.CreateNode(Node{Name: "node-server", Shape: "rect"}, nil, false)
	if n1.Name != "node-server" {
		t.Fatalf("expected node name 'node-server', got %q", n1.Name)
	}

	f1 := sc.CreateFlow(Flow{Name: "flow-border", Color: "#38bdf8", Speed: 1.5})
	if f1.Name != "flow-border" {
		t.Fatalf("expected flow name 'flow-border', got %q", f1.Name)
	}

	g1 := sc.CreateGlow(Glow{Name: "glow-status", Color: "#22c55e"})
	if g1.Name != "glow-status" {
		t.Fatalf("expected glow name 'glow-status', got %q", g1.Name)
	}

	// 2. Add parameters with automatic naming
	pFlowSpeed := GenerateParameterName(f1.Name, "speed")
	if pFlowSpeed != "flow-border-speed" {
		t.Errorf("expected 'flow-border-speed', got %q", pFlowSpeed)
	}
	err := sc.AddParameter(WidgetParameter{
		Name:     pFlowSpeed,
		Target:   "flow-" + string(f1.ID),
		Property: "speed",
		Type:     "speed",
		Default:  1.5,
	})
	if err != nil {
		t.Fatalf("AddParameter flow-border-speed failed: %v", err)
	}

	pFlowColor := GenerateParameterName(f1.Name, "color")
	if pFlowColor != "flow-border-color" {
		t.Errorf("expected 'flow-border-color', got %q", pFlowColor)
	}
	err = sc.AddParameter(WidgetParameter{
		Name:     pFlowColor,
		Target:   "flow-" + string(f1.ID),
		Property: "color",
		Type:     "color",
		Default:  "#38bdf8",
	})
	if err != nil {
		t.Fatalf("AddParameter flow-border-color failed: %v", err)
	}

	pGlowColor := GenerateParameterName(g1.Name, "color")
	if pGlowColor != "glow-status-color" {
		t.Errorf("expected 'glow-status-color', got %q", pGlowColor)
	}
	err = sc.AddParameter(WidgetParameter{
		Name:     pGlowColor,
		Target:   "glow-" + string(g1.ID),
		Property: "color",
		Type:     "color",
		Default:  "#22c55e",
	})
	if err != nil {
		t.Fatalf("AddParameter glow-status-color failed: %v", err)
	}

	// 3. Test duplicate name rejection on rename
	f2 := sc.CreateFlow(Flow{Name: "flow-auxiliary"})
	ok, err := sc.RenameObject(SelectionFlow, f2.ID, "flow-border")
	if ok || err == nil {
		t.Fatalf("expected collision error renaming f2 to 'flow-border', got ok=%v, err=%v", ok, err)
	}

	// 4. Test RenameObject automatically updates public parameter names
	ok, err = sc.RenameObject(SelectionFlow, f1.ID, "flow-primary")
	if !ok || err != nil {
		t.Fatalf("failed to rename flow: %v", err)
	}
	if sc.GetFlow(f1.ID).Name != "flow-primary" {
		t.Errorf("expected flow name 'flow-primary', got %q", sc.GetFlow(f1.ID).Name)
	}

	params := sc.GetParameters()
	var foundSpeed, foundColor *WidgetParameter
	for i := range params {
		if params[i].Property == "speed" && params[i].Target == "flow-"+string(f1.ID) {
			foundSpeed = &params[i]
		}
		if params[i].Property == "color" && params[i].Target == "flow-"+string(f1.ID) {
			foundColor = &params[i]
		}
	}
	if foundSpeed == nil || foundSpeed.Name != "flow-primary-speed" {
		t.Errorf("expected renamed parameter 'flow-primary-speed', got %+v", foundSpeed)
	}
	if foundColor == nil || foundColor.Name != "flow-primary-color" {
		t.Errorf("expected renamed parameter 'flow-primary-color', got %+v", foundColor)
	}

	// 5. Root parameter name generation
	pRootSpeed := GenerateParameterName("root", "playbackRate")
	if pRootSpeed != "root-speed" {
		t.Errorf("expected 'root-speed', got %q", pRootSpeed)
	}
	pRootState := GenerateParameterName("root", "operatingState")
	if pRootState != "root-state" {
		t.Errorf("expected 'root-state', got %q", pRootState)
	}
}
