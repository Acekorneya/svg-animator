package scene

import (
	"math"
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

	// 7. Flow Lite 30-second Cycle Duration boundary check
	upd30, err := sc.UpdateLiteEffect(fl.ID, map[string]any{"speed": 30.0})
	if err != nil || upd30.Speed != 30.0 {
		t.Fatalf("expected flow-lite 30.0s cycle duration accepted, got speed=%v err=%v", upd30.Speed, err)
	}
	updClamp, err := sc.UpdateLiteEffect(fl.ID, map[string]any{"speed": 45.0})
	if err != nil || updClamp.Speed != 30.0 {
		t.Fatalf("expected flow-lite >30.0s cycle duration clamped to 30.0s, got speed=%v err=%v", updClamp.Speed, err)
	}
}

func TestRadarPingLiteVariantsRingsScaleAndPersistence(t *testing.T) {
	sc := NewScene()

	// 1. Default Radar Ping Lite creation
	rd := sc.CreateLiteEffect(LiteEffect{
		Name: "beacon",
		Kind: "radar-lite",
	})
	if rd.Kind != "radar-lite" || rd.Variant != "multi" || rd.Rings != 3 {
		t.Fatalf("expected radar-lite multi with 3 rings by default, got %+v", rd)
	}
	if rd.MinScale != 0.15 || rd.MaxScale != 1.0 || rd.Period != 2.4 || rd.StrokeWidth != 2.0 || rd.Width != 80 || rd.Height != 80 || rd.Color != "#22c55e" || rd.Opacity != 0.9 {
		t.Fatalf("unexpected radar-lite defaults: %+v", rd)
	}
	if rd.ShowGuide == nil || !*rd.ShowGuide {
		t.Fatalf("expected radar-lite ShowGuide=true (centre marker enabled) by default")
	}

	// 2. Update variant to single -> rings normalized to 1
	singleUpd, err := sc.UpdateLiteEffect(rd.ID, map[string]any{
		"variant": "single",
	})
	if err != nil || singleUpd.Rings != 1 {
		t.Fatalf("expected single variant to normalize rings=1, got rings=%d err=%v", singleUpd.Rings, err)
	}

	// 3. Update to fading-ping with 6 rings, custom ellipse dimensions, min/max scale, and 30s cycle duration
	fadingUpd, err := sc.UpdateLiteEffect(rd.ID, map[string]any{
		"variant":     "fading-ping",
		"rings":       6,
		"minScale":    0.20,
		"maxScale":    1.25,
		"period":      30.0,
		"strokeWidth": 3.5,
		"width":       120.0,
		"height":      70.0,
		"color":       "#38bdf8",
		"opacity":     0.85,
		"showGuide":   false,
	})
	if err != nil {
		t.Fatalf("UpdateLiteEffect(radar-lite fading-ping) failed: %v", err)
	}
	if fadingUpd.Variant != "fading-ping" || fadingUpd.Rings != 6 || fadingUpd.MinScale != 0.20 || fadingUpd.MaxScale != 1.25 || fadingUpd.Period != 30.0 || fadingUpd.Width != 120 || fadingUpd.Height != 70 || (fadingUpd.ShowGuide != nil && *fadingUpd.ShowGuide) {
		t.Fatalf("unexpected radar-lite fading-ping update: %+v", fadingUpd)
	}

	// 4. Snapshot and History Undo/Redo round-trip
	sc.RecordHistory()
	_, _ = sc.UpdateLiteEffect(rd.ID, map[string]any{"rings": 4, "period": 1.6})
	sc.RecordHistory()
	if _, err := sc.UndoHistory(); err != nil {
		t.Fatalf("UndoHistory failed: %v", err)
	}
	afterUndo, _ := sc.GetLiteEffect(rd.ID)
	if afterUndo.Rings != 6 || afterUndo.Period != 30.0 {
		t.Fatalf("expected undo to restore rings=6 period=30.0, got %+v", afterUndo)
	}
	if _, err := sc.RedoHistory(); err != nil {
		t.Fatalf("RedoHistory failed: %v", err)
	}
	afterRedo, _ := sc.GetLiteEffect(rd.ID)
	if afterRedo.Rings != 4 || afterRedo.Period != 1.6 {
		t.Fatalf("expected redo to restore rings=4 period=1.6, got %+v", afterRedo)
	}

	// 5. Widget parameter binding for Radar Ping Lite
	radarProps := map[string]string{
		"color":       "color",
		"opacity":     "number",
		"visibility":  "boolean",
		"speed":       "speed",
		"strokeWidth": "number",
		"scale":       "number",
		"period":      "number",
		"showGuide":   "boolean",
	}
	for prop, pType := range radarProps {
		pName := GenerateParameterName("beacon", prop, "lite")
		if err := sc.AddParameter(WidgetParameter{
			Name:     pName,
			Type:     pType,
			Target:   "lite-" + string(rd.ID),
			Property: prop,
		}); err != nil {
			t.Fatalf("AddParameter(radar-lite %s) failed: %v", prop, err)
		}
	}
}

func TestGaugeLiteVariantsRangesThresholdsAndPersistence(t *testing.T) {
	sc := NewScene()

	// 1. Create default gauge-lite (ring variant, 0..100, value=65)
	g := sc.CreateLiteEffect(LiteEffect{
		Name: "rpmGauge",
		Kind: "gauge-lite",
	})
	if g.Variant != "ring" {
		t.Fatalf("expected default gauge-lite variant 'ring', got %q", g.Variant)
	}
	if g.MinValue == nil || *g.MinValue != 0.0 || g.MaxValue == nil || *g.MaxValue != 100.0 {
		t.Fatalf("expected default range [0, 100], got [%v, %v]", g.MinValue, g.MaxValue)
	}
	if g.Value == nil || math.Abs(*g.Value-65.0) > 0.01 {
		t.Fatalf("expected default value 65.0, got %v", g.Value)
	}
	if g.BgColor != "#1e293b" || g.WarnColor != "#f59e0b" || g.CritColor != "#ef4444" {
		t.Fatalf("unexpected default gauge colours: bg=%s warn=%s crit=%s", g.BgColor, g.WarnColor, g.CritColor)
	}

	// 2. Test all 5 gauge variants ("ring", "hbar", "vbar", "segmented", "level")
	for _, variant := range []string{"ring", "hbar", "vbar", "segmented", "level"} {
		upd, err := sc.UpdateLiteEffect(g.ID, map[string]any{
			"variant":  variant,
			"segments": 12,
		})
		if err != nil {
			t.Fatalf("UpdateLiteEffect(variant=%s) failed: %v", variant, err)
		}
		if upd.Variant != variant {
			t.Fatalf("expected variant %q, got %q", variant, upd.Variant)
		}
	}

	// 3. Test negative domain (-40 to +120 °C), valid zero value (0.0), and actual-domain thresholds
	updNeg, err := sc.UpdateLiteEffect(g.ID, map[string]any{
		"minValue":      -40.0,
		"maxValue":      120.0,
		"value":         0.0,
		"useThresholds": true,
		"warnThreshold": 85.0,
		"critThreshold": 105.0,
		"unit":          "°C",
		"labelText":     "Core Temp",
	})
	if err != nil {
		t.Fatalf("UpdateLiteEffect(negative domain & zero value) failed: %v", err)
	}
	if *updNeg.MinValue != -40.0 || *updNeg.MaxValue != 120.0 || *updNeg.Value != 0.0 {
		t.Fatalf("expected [-40, 120] with value 0.0, got [%v, %v] val=%v", *updNeg.MinValue, *updNeg.MaxValue, *updNeg.Value)
	}
	if !updNeg.UseThresholds || *updNeg.WarnThreshold != 85.0 || *updNeg.CritThreshold != 105.0 {
		t.Fatalf("expected actual-domain thresholds 85/105, got %v/%v", *updNeg.WarnThreshold, *updNeg.CritThreshold)
	}

	// 4. Snapshot and History Undo/Redo isolation
	sc.RecordHistory()
	_, _ = sc.UpdateLiteEffect(g.ID, map[string]any{"value": 98.0})
	sc.RecordHistory()
	if _, err := sc.UndoHistory(); err != nil {
		t.Fatalf("UndoHistory failed: %v", err)
	}
	afterUndo, _ := sc.GetLiteEffect(g.ID)
	if afterUndo.Value == nil || *afterUndo.Value != 0.0 {
		t.Fatalf("UndoHistory did not preserve valid zero gauge value: %+v", afterUndo)
	}

	// 5. Widget parameter bindings for gauge-lite properties
	gaugeProps := map[string]string{
		"value":       "number",
		"minValue":    "number",
		"maxValue":    "number",
		"color":       "color",
		"bgColor":     "color",
		"strokeWidth": "number",
		"opacity":     "number",
		"visibility":  "boolean",
		"showGuide":   "boolean",
	}
	for prop, pType := range gaugeProps {
		pName := GenerateParameterName("rpmGauge", prop, "lite")
		if err := sc.AddParameter(WidgetParameter{
			Name:     pName,
			Type:     pType,
			Target:   "lite-" + string(g.ID),
			Property: prop,
		}); err != nil {
			t.Fatalf("AddParameter(gauge-lite %s) failed: %v", prop, err)
		}
	}

	// 6. Test 4 independently enabled thresholds (including non-contiguous enabled bands & out-of-order disabled band)
	upd4, err := sc.UpdateLiteEffect(g.ID, map[string]any{
		"threshold1Enabled": true,
		"threshold1Value":   40.0,
		"threshold1Color":   "#22c55e",
		"threshold2Enabled": false,
		"threshold2Value":   999.0, // disabled out-of-order threshold must not interfere
		"threshold2Color":   "#eab308",
		"threshold3Enabled": true,
		"threshold3Value":   80.0,
		"threshold3Color":   "#f97316",
		"threshold4Enabled": true,
		"threshold4Value":   95.0,
		"threshold4Color":   "#ef4444",
	})
	if err != nil {
		t.Fatalf("UpdateLiteEffect(4 thresholds) failed: %v", err)
	}
	if len(upd4.Thresholds) != 4 {
		t.Fatalf("expected 4 threshold bands, got %d", len(upd4.Thresholds))
	}
	if !upd4.Thresholds[0].Enabled || upd4.Thresholds[1].Enabled || !upd4.Thresholds[2].Enabled || !upd4.Thresholds[3].Enabled {
		t.Fatalf("unexpected enabled state across 4 thresholds: %+v", upd4.Thresholds)
	}
}

func TestOscilloscopeLitePresetsModesSamplesAndPersistence(t *testing.T) {
	sc := NewScene()

	// 1. Create default oscilloscope-lite (ecg, animated, 180x76)
	osc := sc.CreateLiteEffect(LiteEffect{
		Name: "cardiacMonitor",
		Kind: "oscilloscope-lite",
	})
	if osc.Variant != "ecg" {
		t.Fatalf("expected default oscilloscope-lite variant 'ecg', got %q", osc.Variant)
	}
	if osc.PlaybackMode != "animated" {
		t.Fatalf("expected default playbackMode 'animated', got %q", osc.PlaybackMode)
	}
	if osc.Segments != 2 || osc.Intensity != 0.75 || osc.StartOffset != 50 {
		t.Fatalf("unexpected default oscilloscope params: segments=%d intensity=%v startOffset=%v", osc.Segments, osc.Intensity, osc.StartOffset)
	}

	// 2. Test all 7 waveform presets across static, animated, and sampled modes
	presets := []string{"ecg", "sine", "square", "triangle", "sawtooth", "polyline", "bezier"}
	modes := []string{"static", "animated", "sampled"}
	for _, preset := range presets {
		for _, mode := range modes {
			upd, err := sc.UpdateLiteEffect(osc.ID, map[string]any{
				"variant":      preset,
				"playbackMode": mode,
				"intensity":    0.85,
				"segments":     3,
				"startOffset":  45.0,
				"speed":        1.8,
				"direction":    "reverse",
				"samples":      "0, 0.5, 1.0, 0.2, -0.6, -1.0, -0.3, 0",
			})
			if err != nil {
				t.Fatalf("UpdateLiteEffect(variant=%s, mode=%s) failed: %v", preset, mode, err)
			}
			if upd.Variant != preset || upd.PlaybackMode != mode {
				t.Fatalf("expected (%s, %s), got (%s, %s)", preset, mode, upd.Variant, upd.PlaybackMode)
			}
		}
	}

	// 3. Reject invalid samples (fewer than 2, more than 64, or non-numeric)
	if _, err := sc.UpdateLiteEffect(osc.ID, map[string]any{"playbackMode": "sampled", "samples": "0.5"}); err == nil {
		t.Fatalf("expected single sample to be rejected")
	}
	if _, err := sc.UpdateLiteEffect(osc.ID, map[string]any{"playbackMode": "sampled", "samples": "0.1, bad, 0.5"}); err == nil {
		t.Fatalf("expected non-numeric sample to be rejected")
	}
	many := make([]string, 65)
	for i := range many {
		many[i] = "0.2"
	}
	if _, err := sc.UpdateLiteEffect(osc.ID, map[string]any{"playbackMode": "sampled", "samples": strings.Join(many, ",")}); err == nil {
		t.Fatalf("expected >64 samples to be rejected")
	}

	// 4. Undo/Redo round-trip
	sc.RecordHistory()
	_, _ = sc.UpdateLiteEffect(osc.ID, map[string]any{"variant": "sine", "samples": "0, 1, 0, -1"})
	sc.RecordHistory()
	if _, err := sc.UndoHistory(); err != nil {
		t.Fatalf("UndoHistory failed: %v", err)
	}
	afterUndo, _ := sc.GetLiteEffect(osc.ID)
	if afterUndo.Variant != "bezier" {
		t.Fatalf("expected undo to restore variant 'bezier', got %q", afterUndo.Variant)
	}

	// 5. Widget parameter bindings for oscilloscope-lite properties
	oscProps := map[string]string{
		"color":       "color",
		"bgColor":     "color",
		"opacity":     "number",
		"visibility":  "boolean",
		"speed":       "speed",
		"strokeWidth": "number",
		"direction":   "enum",
		"showGuide":   "boolean",
		"intensity":   "number",
		"samples":     "text",
	}
	for prop, pType := range oscProps {
		pName := GenerateParameterName("cardiacMonitor", prop, "lite")
		param := WidgetParameter{
			Name:     pName,
			Type:     pType,
			Target:   "lite-" + string(osc.ID),
			Property: prop,
		}
		if pType == "enum" {
			param.Options = []string{"forward", "reverse"}
		}
		if err := sc.AddParameter(param); err != nil {
			t.Fatalf("AddParameter(oscilloscope-lite %s) failed: %v", prop, err)
		}
	}
}

func TestTextLiteContainersFontsCurvedAnimationsAndWidgetScoping(t *testing.T) {
	sc := NewScene()

	// 1. Create text-lite with default properties
	txt := sc.CreateLiteEffect(LiteEffect{
		Name:      "statusBanner",
		Kind:      "text-lite",
		LabelText: "Backup Completed",
	})
	if txt.Kind != "text-lite" || txt.Variant != "rounded" {
		t.Fatalf("expected text-lite rounded default, got kind=%q variant=%q", txt.Kind, txt.Variant)
	}
	if txt.FontFamily != "modern-sans" || txt.FontSize != 18.0 || txt.FontWeight != "600" {
		t.Fatalf("unexpected font defaults: family=%q size=%v weight=%q", txt.FontFamily, txt.FontSize, txt.FontWeight)
	}
	if txt.TextAlign != "center" || txt.VerticalAlign != "middle" || txt.Padding != 0 {
		t.Fatalf("unexpected alignment defaults: align=%q valign=%q", txt.TextAlign, txt.VerticalAlign)
	}
	if txt.BgOpacity == nil || *txt.BgOpacity != 0.85 {
		t.Fatalf("expected default bgOpacity 0.85, got %v", txt.BgOpacity)
	}

	// 2. Test all 5 container variants, 10 curated fonts, 9-way alignments, rotation, and transparent bgOpacity=0
	variants := []string{"none", "rect", "rounded", "pill", "curved"}
	fonts := []string{
		"modern-sans", "humanist-sans", "geometric-sans", "formal-serif", "traditional-serif",
		"tech-mono", "condensed-display", "casual-lettering", "informal-hand", "classic-script",
	}
	hAligns := []string{"left", "center", "right"}
	vAligns := []string{"top", "middle", "bottom"}

	for _, v := range variants {
		for _, f := range fonts {
			upd, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
				"variant":       v,
				"fontFamily":    f,
				"fontSize":      24.0,
				"fontWeight":    "700",
				"letterSpacing": 1.5,
				"padding":       14.0,
				"bgOpacity":     0.0,
				"strokeWidth":   0.0,
				"rotation":      45.0,
				"curvature":     40.0,
			})
			if err != nil {
				t.Fatalf("UpdateLiteEffect(variant=%s, font=%s) failed: %v", v, f, err)
			}
			if upd.Variant != v || upd.FontFamily != f || upd.Rotation != 45.0 {
				t.Fatalf("mismatch after update: variant=%s font=%s rot=%v", upd.Variant, upd.FontFamily, upd.Rotation)
			}
			if upd.BgOpacity == nil || *upd.BgOpacity != 0.0 || upd.StrokeWidth != 0.0 {
				t.Fatalf("expected zero bgOpacity and strokeWidth preserved, got bgOp=%v sw=%v", upd.BgOpacity, upd.StrokeWidth)
			}
		}
	}

	for _, ha := range hAligns {
		for _, va := range vAligns {
			upd, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
				"variant":       "rounded",
				"textAlign":     ha,
				"verticalAlign": va,
			})
			if err != nil || upd.TextAlign != ha || upd.VerticalAlign != va {
				t.Fatalf("alignment (%s, %s) failed: %v", ha, va, err)
			}
		}
	}

	// 3. Test all 11 textAnim modes on straight container, legacy overtype -> typewriter migration, and curved compatibility filtering
	anims := []string{"none", "ticker-h", "ticker-v", "stacked-v", "typewriter", "fade", "flash", "slide", "blur-in", "blur-out", "scramble"}
	for _, anim := range anims {
		upd, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
			"variant":       "rounded",
			"textAnim":      anim,
			"scrollSpeed":   120.0,
			"linesPerSec":   2.5,
			"lineSpacing":   1.4,
			"wordWrap":      true,
			"typeSpeed":     18.0,
			"holdDelay":     2.0,
			"clearDuration": 1.0,
			"typeMode":      "delete-forward",
			"typeRepeat":    true,
			"playbackMode":  "pingpong",
		})
		if err != nil || upd.TextAnim != anim {
			t.Fatalf("textAnim %q on straight box failed: %v", anim, err)
		}
		if upd.LinesPerSec != 2.5 || upd.LineSpacing != 1.4 || upd.WordWrap == nil || !*upd.WordWrap {
			t.Fatalf("expected vertical scroll properties preserved, got lps=%v ls=%v ww=%v", upd.LinesPerSec, upd.LineSpacing, upd.WordWrap)
		}
		if upd.TypeMode != "delete-forward" || upd.PlaybackMode != "pingpong" {
			t.Fatalf("expected typeMode=delete-forward and playbackMode=pingpong, got %q / %q", upd.TypeMode, upd.PlaybackMode)
		}
	}

	// Legacy overtype migration -> typewriter, legacy typeMode="loop" -> "delete" + typeRepeat=true
	updLegacy, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
		"variant":  "rounded",
		"textAnim": "overtype",
		"typeMode": "loop",
	})
	if err != nil || updLegacy.TextAnim != "typewriter" || updLegacy.TypeMode != "delete" || updLegacy.TypeRepeat == nil || !*updLegacy.TypeRepeat {
		t.Fatalf("expected overtype -> typewriter and loop -> delete+typeRepeat, got err=%v anim=%q mode=%q rep=%v",
			err, updLegacy.TextAnim, updLegacy.TypeMode, updLegacy.TypeRepeat)
	}

	// Lock On/Off synchronisation in Go engine
	updLock, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
		"textAnim":      "flash",
		"holdDelay":     1.8,
		"clearDuration": 0.4,
		"lockOnOff":     true,
	})
	if err != nil || updLock.LockOnOff == nil || !*updLock.LockOnOff || updLock.ClearDuration != 1.8 {
		t.Fatalf("expected lockOnOff=true to sync clearDuration=holdDelay=1.8, got err=%v clear=%v", err, updLock.ClearDuration)
	}
	updLockClear, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
		"clearDuration": 0.75,
	})
	if err != nil || updLockClear.HoldDelay != 0.75 || updLockClear.ClearDuration != 0.75 {
		t.Fatalf("expected changing clearDuration while locked to sync holdDelay=0.75, got hold=%v clear=%v", updLockClear.HoldDelay, updLockClear.ClearDuration)
	}

	// Curved variant allows fade, flash, typewriter, blur-in, blur-out, none, and auto-resets unsupported ticker/slide/scramble/overtype
	updCurved, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
		"variant":  "curved",
		"textAnim": "fade",
	})
	if err != nil || updCurved.TextAnim != "fade" {
		t.Fatalf("expected curved fade to succeed, got err=%v anim=%q", err, updCurved.TextAnim)
	}
	updCurvedAuto, err := sc.UpdateLiteEffect(txt.ID, map[string]any{
		"variant":  "curved",
		"textAnim": "ticker-h",
	})
	if err != nil || updCurvedAuto.TextAnim != "none" {
		t.Fatalf("expected curved ticker-h to normalize to 'none', got err=%v anim=%q", err, updCurvedAuto.TextAnim)
	}

	// 4. Test IsolateSVGWidget rewrites <textPath href="#lite-text-arc-1"> and clip-path="url(#lite-text-clip-1)"
	rawSVG := `<svg xmlns="http://www.w3.org/2000/svg"><defs><clipPath id="lite-text-clip-1"><rect width="200" height="50"/></clipPath><path id="lite-text-arc-1" d="M 10 25 Q 100 5 190 25"/></defs><g id="lite-1" clip-path="url(#lite-text-clip-1)"><text><textPath href="#lite-text-arc-1">Curved Banner</textPath></text></g></svg>`
	isoSVG, _, err := IsolateSVGWidget(rawSVG, WidgetIsolationOptions{InstanceID: "instA", ScopeCSS: true})
	if err != nil {
		t.Fatalf("IsolateSVGWidget failed: %v", err)
	}
	if !strings.Contains(isoSVG, `id="instA__lite-text-arc-1"`) || !strings.Contains(isoSVG, `href="#instA__lite-text-arc-1"`) {
		t.Fatalf("expected textPath href and arc path ID to be isolated, got: %s", isoSVG)
	}

	// 5. Test widget property bindings for text-lite
	textProps := map[string]string{
		"text":        "text",
		"color":       "color",
		"bgColor":     "color",
		"borderColor": "color",
		"opacity":     "number",
		"visibility":  "boolean",
		"scrollSpeed": "number",
		"typeSpeed":   "number",
	}
	for prop, pType := range textProps {
		pName := GenerateParameterName("statusBanner", prop, "lite")
		if err := sc.AddParameter(WidgetParameter{
			Name:     pName,
			Type:     pType,
			Target:   "lite-" + string(txt.ID),
			Property: prop,
		}); err != nil {
			t.Fatalf("AddParameter(text-lite %s) failed: %v", prop, err)
		}
	}
}
