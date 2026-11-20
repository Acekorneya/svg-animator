package scene

import (
	"strings"
	"testing"

	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/engine/geometry"
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

func TestLiteEffectLayersAndGroupingCompatibility(t *testing.T) {
	sc := NewScene()
	t1 := sc.CreateLiteEffect(LiteEffect{Name: "t1", Kind: "perimeter-tracer", X: 240, Y: 180, Width: 140, Height: 80})
	p2 := sc.CreateLiteEffect(LiteEffect{Name: "p2", Kind: "pulse", X: 400, Y: 240, Width: 60, Height: 60})

	// All 4 layer ordering operations on SelectionLite
	if changed, err := sc.BringForward([]SelectionItem{{Type: SelectionLite, ID: t1.ID}}); err != nil || !changed {
		t.Fatalf("BringForward(lite) failed: changed=%v err=%v", changed, err)
	}
	if changed, err := sc.SendBackward([]SelectionItem{{Type: SelectionLite, ID: t1.ID}}); err != nil || !changed {
		t.Fatalf("SendBackward(lite) failed: changed=%v err=%v", changed, err)
	}
	if changed, err := sc.BringToFront([]SelectionItem{{Type: SelectionLite, ID: t1.ID}}); err != nil || !changed {
		t.Fatalf("BringToFront(lite) failed: changed=%v err=%v", changed, err)
	}
	if changed, err := sc.SendToBack([]SelectionItem{{Type: SelectionLite, ID: t1.ID}}); err != nil || !changed {
		t.Fatalf("SendToBack(lite) failed: changed=%v err=%v", changed, err)
	}

	// SetSelection for SelectionLite
	sel, err := sc.SetSelection(SelectionLite, t1.ID)
	if err != nil || sel.Type != SelectionLite || sel.ID != t1.ID {
		t.Fatalf("SetSelection(SelectionLite) failed: %+v err=%v", sel, err)
	}

	// Grouping, MoveGroup, ComputeGroupBounds, and DeleteGroup
	grp, err := sc.CreateGroup("lite-grp", []GroupMember{
		{Type: SelectionLite, ID: t1.ID},
		{Type: SelectionLite, ID: p2.ID},
	})
	if err != nil || len(grp.Members) != 2 {
		t.Fatalf("CreateGroup with lite members failed: %+v err=%v", grp, err)
	}
	if err := sc.MoveGroup(grp.ID, 10, 5); err != nil {
		t.Fatalf("MoveGroup with lite members failed: %v", err)
	}
	t1After, _ := sc.GetLiteEffect(t1.ID)
	if t1After.X != 368 || t1After.Y != 216 {
		t.Fatalf("expected t1 moved by (128, 36) px to (368, 216), got (%v, %v)", t1After.X, t1After.Y)
	}
	minX, minY, maxX, maxY, err := sc.ComputeGroupBounds(grp.ID, 1280, 720)
	if err != nil || maxX <= minX || maxY <= minY {
		t.Fatalf("ComputeGroupBounds failed: (%v,%v)-(%v,%v) err=%v", minX, minY, maxX, maxY, err)
	}
	if err := sc.DeleteGroup(grp.ID, true); err != nil {
		t.Fatalf("DeleteGroup(deleteMembers=true) failed: %v", err)
	}
	if len(sc.GetLiteEffects()) != 0 {
		t.Fatalf("expected 0 lite effects after DeleteGroup(deleteMembers=true), got %d", len(sc.GetLiteEffects()))
	}
}

func TestFlowLiteMultiPointBendsEndpointsAndWidgetScoping(t *testing.T) {
	sc := NewScene()

	// 1. Default Flow Lite creation
	fl := sc.CreateLiteEffect(LiteEffect{
		Name: "mainPipe",
		Kind: "flow-lite",
	})
	if fl.Kind != "flow-lite" || fl.Variant != "dashed" {
		t.Fatalf("expected flow-lite dashed defaults, got %+v", fl)
	}
	if len(fl.Points) != 2 || len(fl.Bends) != 1 {
		t.Fatalf("expected 2 points and 1 bend by default, got points=%v bends=%v", fl.Points, fl.Bends)
	}
	if fl.StartMode != "tunnel" || fl.EndMode != "tunnel" || fl.Direction != "forward" || fl.PlaybackMode != "loop" {
		t.Fatalf("unexpected endpoint/direction defaults: %+v", fl)
	}
	if fl.DashLength != 18 || fl.GapLength != 14 || fl.DotSize != 5 || fl.StrokeWidth != 3.0 {
		t.Fatalf("unexpected pattern defaults: %+v", fl)
	}

	// 2. Update to 4-point path with straight and curved sections + independent endpoint modes + delays
	pts := []geometry.Point{
		{X: 10, Y: 50},
		{X: 35, Y: 50},
		{X: 65, Y: 25},
		{X: 90, Y: 50},
	}
	bends := []float64{0, 45, -35}
	upd, err := sc.UpdateLiteEffect(fl.ID, map[string]any{
		"variant":      "dotted",
		"points":       pts,
		"bends":        bends,
		"curvature":    5.0,
		"dashLength":   24.0,
		"gapLength":    18.0,
		"dotSize":      8.0,
		"strokeWidth":  4.0,
		"startMode":    "tunnel",
		"endMode":      "abrupt",
		"direction":    "reverse",
		"playbackMode": "pingpong",
		"speed":        1.6,
		"delay":        0.4,
		"endDelay":     0.7,
	})
	if err != nil {
		t.Fatalf("UpdateLiteEffect(flow-lite) failed: %v", err)
	}
	if len(upd.Points) != 4 || len(upd.Bends) != 3 || upd.Bends[1] != 45 || upd.Bends[2] != -35 {
		t.Fatalf("expected 4 points and 3 bends [0,45,-35], got points=%v bends=%v", upd.Points, upd.Bends)
	}
	if upd.StartMode != "tunnel" || upd.EndMode != "abrupt" || upd.Delay != 0.4 || upd.EndDelay != 0.7 {
		t.Fatalf("unexpected flow-lite timing/endpoint update: %+v", upd)
	}

	// 3. Geometry path generation, length, bounds, and hit distance
	pathD := geometry.GenerateFlowLitePath(upd.Points, upd.Bends, upd.Curvature, 1280, 720)
	if !strings.Contains(pathD, "M ") || !strings.Contains(pathD, " Q ") {
		t.Fatalf("expected M and Q commands in pathD, got %q", pathD)
	}
	pathLen := geometry.ComputeFlowLiteLength(upd.Points, upd.Bends, upd.Curvature, 1280, 720)
	if pathLen < 900 {
		t.Fatalf("expected path length > 900px across 1280x720 canvas, got %v", pathLen)
	}
	bounds, mid := geometry.ComputeFlowLiteBounds(upd.Points, upd.Bends, upd.Curvature, 1280, 720)
	if bounds.Width < 900 || bounds.Height < 100 || mid.X <= 0 {
		t.Fatalf("unexpected flow-lite bounds=%+v mid=%+v", bounds, mid)
	}
	distNear := geometry.DistanceToFlowLitePath(upd.Points, upd.Bends, upd.Curvature, 1280, 720, 128, 360)
	if distNear > 25 {
		t.Fatalf("expected near point (128,360) to be close to path start, got dist=%v", distNear)
	}

	// 4. MoveGroup translates all Flow Lite points and preserves bends
	p2 := sc.CreateLiteEffect(LiteEffect{Name: "p2", Kind: "pulse", X: 300, Y: 300})
	grp, err := sc.CreateGroup("flow-lite-grp", []GroupMember{
		{Type: SelectionLite, ID: fl.ID},
		{Type: SelectionLite, ID: p2.ID},
	})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := sc.MoveGroup(grp.ID, 5.0, -10.0); err != nil {
		t.Fatalf("MoveGroup failed: %v", err)
	}
	moved, _ := sc.GetLiteEffect(fl.ID)
	if moved.Points[0].X != 15.0 || moved.Points[0].Y != 40.0 || moved.Points[3].X != 95.0 || moved.Points[3].Y != 40.0 {
		t.Fatalf("expected flow-lite points translated by (+5, -10), got %+v", moved.Points)
	}
	if moved.Bends[1] != 45 || moved.Bends[2] != -35 {
		t.Fatalf("expected flow-lite bends preserved after MoveGroup, got %+v", moved.Bends)
	}

	// 5. Snapshot round-trip preserves all Flow Lite fields
	snap := sc.GetSnapshot()
	sc2 := NewScene()
	sc2.SetFromSnapshot(snap)
	restored, ok := sc2.GetLiteEffect(fl.ID)
	if !ok || len(restored.Points) != 4 || len(restored.Bends) != 3 || restored.EndMode != "abrupt" || restored.EndDelay != 0.7 || restored.DotSize != 8.0 {
		t.Fatalf("Flow Lite snapshot round-trip lost fields: %+v", restored)
	}

	// 6. Widget property catalog & IsolateSVGWidget for flow-lite (<use href>, <clipPath>, @keyframes)
	propTypes := map[string]string{
		"color":        "color",
		"opacity":      "number",
		"visibility":   "boolean",
		"speed":        "speed",
		"strokeWidth":  "number",
		"dashLength":   "number",
		"gapLength":    "number",
		"dotSize":      "number",
		"direction":    "enum",
		"playbackMode": "enum",
		"showGuide":    "boolean",
	}
	for prop, pType := range propTypes {
		pName := GenerateParameterName("mainPipe", prop, "lite")
		var opts []string
		if prop == "direction" {
			opts = []string{"forward", "reverse"}
		} else if prop == "playbackMode" {
			opts = []string{"loop", "pingpong", "once"}
		}
		if err := sc.AddParameter(WidgetParameter{
			Name:     pName,
			Type:     pType,
			Target:   "lite-" + string(fl.ID),
			Property: prop,
			Options:  opts,
		}); err != nil {
			t.Fatalf("AddParameter(%s) failed: %v", prop, err)
		}
	}
	rawSVG := `<svg id="mainSvg" viewBox="0 0 1280 720"><style>@keyframes sva-fl-1 { 0% { stroke-dashoffset: 18; } 100% { stroke-dashoffset: -400; } }</style><g id="lite-1" data-object-name="mainPipe"><defs><path id="lite-flow-track-1" d="M 100 100 L 500 100"/><clipPath id="lite-flow-clip-1"><rect x="100" y="90" width="400" height="20"/></clipPath></defs><g clip-path="url(#lite-flow-clip-1)"><use href="#lite-flow-track-1" class="lite-payload" style="animation: sva-fl-1 2s linear infinite;"/></g></g></svg>`
	scopedSVG, idMap, err := IsolateSVGWidget(rawSVG, WidgetIsolationOptions{Prefix: "w1__", ScopeCSS: true})
	if err != nil {
		t.Fatalf("IsolateSVGWidget failed: %v", err)
	}
	if idMap["lite-flow-track-1"] != "w1__lite-flow-track-1" || idMap["lite-flow-clip-1"] != "w1__lite-flow-clip-1" {
		t.Fatalf("expected flow-lite track and clip IDs mapped, got %+v", idMap)
	}
	if !strings.Contains(scopedSVG, `href="#w1__lite-flow-track-1"`) || !strings.Contains(scopedSVG, `url(#w1__lite-flow-clip-1)`) || !strings.Contains(scopedSVG, `w1__sva-fl-1`) {
		t.Fatalf("ScopeWidgetSVG did not scope flow-lite href, clip-path, or keyframes: %s", scopedSVG)
	}
}
