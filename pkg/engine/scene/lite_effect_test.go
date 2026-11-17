package scene

import (
	"testing"
)

func TestComputePerimeterDash(t *testing.T) {
	dash, gap := ComputePerimeterDash(2, "1:3")
	if dash != 125 || gap != 375 {
		t.Fatalf("expected (125, 375) for 2 segments @ 1:3, got (%v, %v)", dash, gap)
	}

	dash, gap = ComputePerimeterDash(4, "1:1")
	if dash != 125 || gap != 125 {
		t.Fatalf("expected (125, 125) for 4 segments @ 1:1, got (%v, %v)", dash, gap)
	}

	dash, gap = ComputePerimeterDash(1, "2:3")
	if dash != 400 || gap != 600 {
		t.Fatalf("expected (400, 600) for 1 segment @ 2:3, got (%v, %v)", dash, gap)
	}
}

func TestLiteEffectCRUDAndValidation(t *testing.T) {
	sc := NewScene()

	// Create Perimeter Tracer Lite
	tracer := sc.CreateLiteEffect(LiteEffect{
		Name:         "border",
		Kind:         "perimeter-tracer",
		Variant:      "rounded",
		Segments:     2,
		LineGapRatio: "1:3",
		Direction:    "cw",
		Speed:        2.5,
	})
	if tracer.ID != "1" || tracer.Kind != "perimeter-tracer" {
		t.Fatalf("unexpected tracer created: %+v", tracer)
	}

	// Create Pulse Lite
	pulse := sc.CreateLiteEffect(LiteEffect{
		Name:    "heartbeat",
		Kind:    "pulse",
		Variant: "diamond",
		Period:  1.6,
	})
	if pulse.ID != "2" || pulse.Variant != "diamond" {
		t.Fatalf("unexpected pulse created: %+v", pulse)
	}

	// Create Flash Lite
	flash := sc.CreateLiteEffect(LiteEffect{
		Name:     "alert",
		Kind:     "flash",
		Variant:  "star",
		Duration: 0.2,
	})
	if flash.ID != "3" || flash.Duration != 0.2 {
		t.Fatalf("unexpected flash created: %+v", flash)
	}

	// Update tracer segments & ratio
	updated, err := sc.UpdateLiteEffect(tracer.ID, map[string]any{
		"segments":     4,
		"lineGapRatio": "1:2",
		"direction":    "ccw",
	})
	if err != nil {
		t.Fatalf("UpdateLiteEffect failed: %v", err)
	}
	if updated.Segments != 4 || updated.LineGapRatio != "1:2" || updated.Direction != "ccw" {
		t.Fatalf("unexpected updated tracer: %+v", updated)
	}

	// Reject out-of-range flash duration (> 1.5s)
	if _, err := sc.UpdateLiteEffect(flash.ID, map[string]any{"duration": 4.0}); err == nil {
		t.Fatalf("expected validation error for excessive flash duration")
	}

	// Snapshot round-trip
	snap := sc.GetSnapshot()
	if len(snap.LiteEffects) != 3 {
		t.Fatalf("expected 3 lite effects in snapshot, got %d", len(snap.LiteEffects))
	}
	sc2 := NewScene()
	sc2.SetFromSnapshot(snap)
	if len(sc2.GetLiteEffects()) != 3 {
		t.Fatalf("expected 3 lite effects after SetFromSnapshot, got %d", len(sc2.GetLiteEffects()))
	}

	// Widget naming & pre-export validation for lite effects
	pName := GenerateParameterName("border", "color", "lite")
	if pName != "border-lite-color" {
		t.Fatalf("expected border-lite-color, got %q", pName)
	}
	if err := sc.AddParameter(WidgetParameter{
		Name:     pName,
		Type:     "color",
		Target:   "lite-1",
		Property: "color",
		Default:  "#38bdf8",
	}); err != nil {
		t.Fatalf("AddParameter for lite-1 failed: %v", err)
	}

	rawSVG := `<svg viewBox="0 0 200 200"><g id="lite-1" data-semantic-target="lite-1" data-semantic-name="border"><rect pathLength="1000" stroke="#38bdf8" fill="none"/></g></svg>`
	zipBytes, err := BuildWidgetPackageZip(sc, "lite-test", rawSVG, "window.SVGWidget={mount:()=>{}};")
	if err != nil || len(zipBytes) == 0 {
		t.Fatalf("BuildWidgetPackageZip failed for lite-1: %v", err)
	}

	// Delete
	if !sc.DeleteLiteEffect(flash.ID) {
		t.Fatalf("expected DeleteLiteEffect to return true")
	}
	if len(sc.GetLiteEffects()) != 2 {
		t.Fatalf("expected 2 lite effects after delete, got %d", len(sc.GetLiteEffects()))
	}
}

func TestLiteEffectLegacyMigrationAndNewProperties(t *testing.T) {
	// 1. Legacy perimeter-tracer without OriginMode (top-left X=100, Y=100, W=140, H=80)
	// must migrate to center X=170, Y=140 so top-left (X-W/2, Y-H/2) stays at (100, 100).
	sc := NewScene()
	snap := sc.GetSnapshot()
	snap.LiteEffects = []LiteEffect{
		{
			ID:     "1",
			Kind:   "perimeter-tracer",
			X:      100,
			Y:      100,
			Width:  140,
			Height: 80,
		},
		{
			ID:       "2",
			Kind:     "pulse",
			X:        200,
			Y:        200,
			Radius:   30,
			Width:    140,
			Height:   80,
		},
		{
			ID:       "3",
			Kind:     "flash",
			X:        300,
			Y:        300,
			Duration: 0.22,
		},
	}
	snap.LiteEffectsCounter = 3
	sc.SetFromSnapshot(snap)

	t1, ok := sc.GetLiteEffect("1")
	if !ok || t1.OriginMode != "center" || t1.X != 170 || t1.Y != 140 {
		t.Fatalf("legacy tracer migration failed: got %+v", t1)
	}
	if t1.ShowGuide == nil || !*t1.ShowGuide {
		t.Fatalf("expected ShowGuide default true on migrated tracer")
	}

	p2, ok := sc.GetLiteEffect("2")
	if !ok || p2.OriginMode != "center" || p2.Width != 60 || p2.Height != 60 || p2.MinScale != 0.85 || p2.MaxScale != 1.15 {
		t.Fatalf("legacy pulse migration failed: got %+v", p2)
	}

	f3, ok := sc.GetLiteEffect("3")
	if !ok || f3.Duration != 0.22 {
		t.Fatalf("legacy flash duration should be preserved (0.22), got %+v", f3)
	}

	// 2. New Flash defaults to 100ms (0.10s) and supports durationMs updates
	fNew := sc.CreateLiteEffect(LiteEffect{Kind: "flash"})
	if fNew.Duration != 0.10 {
		t.Fatalf("expected new flash default Duration=0.10 (100ms), got %v", fNew.Duration)
	}
	fUpd, err := sc.UpdateLiteEffect(fNew.ID, map[string]any{"durationMs": 180})
	if err != nil || fUpd.Duration != 0.18 {
		t.Fatalf("expected durationMs=180 to set Duration=0.18, got %v (err=%v)", fUpd.Duration, err)
	}

	// 3. Pulse minScale / maxScale validation
	if _, err := sc.UpdateLiteEffect("2", map[string]any{"minScale": 1.4, "maxScale": 0.9}); err == nil {
		t.Fatalf("expected error when minScale > maxScale")
	}
}
