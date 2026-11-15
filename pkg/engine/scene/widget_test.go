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
