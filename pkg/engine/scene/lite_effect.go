package scene

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/engine/geometry"
)

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, !math.IsNaN(val) && !math.IsInf(val, 0)
	case float32:
		f := float64(val)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil && !math.IsNaN(f) {
			return f, true
		}
	}
	return 0, false
}

// ParseLineGapRatio parses a repeating pattern ratio such as "1:3" or "2:5".
// Defaults to (1, 3) if empty or malformed.
func ParseLineGapRatio(ratio string) (float64, float64) {
	trimmed := strings.TrimSpace(ratio)
	if trimmed == "" {
		return 1, 3
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) != 2 {
		return 1, 3
	}
	l, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	g, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || l <= 0 || g <= 0 || math.IsNaN(l) || math.IsNaN(g) {
		return 1, 3
	}
	return l, g
}

// ComputePerimeterDash calculates stroke-dasharray (dashLen, gapLen) in pathLength=1000 units
// so N segments are evenly distributed around the perimeter with the given line:gap ratio
// within each repeating segment pattern.
func ComputePerimeterDash(segments int, ratio string) (float64, float64) {
	if segments < 1 {
		segments = 1
	}
	if segments > 16 {
		segments = 16
	}
	l, g := ParseLineGapRatio(ratio)
	unit := 1000.0 / float64(segments)
	totalParts := l + g
	dashLen := math.Round((unit*(l/totalParts))*1000) / 1000
	gapLen := math.Round((unit-dashLen)*1000) / 1000
	return dashLen, gapLen
}

// ValidateLiteEffect validates operational bounds for Stage 16D Lite effects.
func ValidateLiteEffect(l *LiteEffect) error {
	if l == nil {
		return errors.New("nil lite effect")
	}
	switch l.Kind {
	case "perimeter-tracer", "pulse", "flash", "flow-lite", "radar-lite", "gauge-lite":
	default:
		return fmt.Errorf("unsupported lite effect kind %q", l.Kind)
	}
	if l.Opacity < 0 || l.Opacity > 1.0 {
		return errors.New("opacity must be between 0.0 and 1.0")
	}
	if l.Kind == "perimeter-tracer" {
		if l.Segments < 1 || l.Segments > 16 {
			return errors.New("perimeter-tracer segments must be between 1 and 16")
		}
		if l.Speed <= 0 {
			return errors.New("speed must be greater than 0")
		}
		if l.PlaybackMode != "" && l.PlaybackMode != "loop" && l.PlaybackMode != "pingpong" && l.PlaybackMode != "once" {
			return fmt.Errorf("unsupported perimeter-tracer playbackMode %q", l.PlaybackMode)
		}
		if l.BurstCycles < 0 || l.BurstCycles > 100 {
			return errors.New("burstCycles must be between 0 and 100")
		}
		if l.PauseSeconds < 0 || l.PauseSeconds > 300 {
			return errors.New("pauseSeconds must be between 0 and 300")
		}
		if l.PauseVisibility != "" && l.PauseVisibility != "hide" && l.PauseVisibility != "hold" {
			return fmt.Errorf("unsupported pauseVisibility %q", l.PauseVisibility)
		}
	}
	if l.Kind == "pulse" {
		if l.Period <= 0 {
			return errors.New("pulse period must be greater than 0")
		}
		if l.Intensity < 0 || l.Intensity > 1.0 {
			return errors.New("intensity must be between 0.0 and 1.0")
		}
		if l.MinScale < 0 || l.MaxScale < 0 || (l.MinScale > 0 && l.MaxScale > 0 && l.MinScale > l.MaxScale) {
			return errors.New("pulse minScale cannot exceed maxScale")
		}
	}
	if l.Kind == "flash" {
		if l.Duration <= 0 || l.Duration > 1.5 {
			return errors.New("flash duration must be between 0.01 and 1.5 seconds")
		}
		if l.Intensity < 0 || l.Intensity > 1.0 {
			return errors.New("intensity must be between 0.0 and 1.0")
		}
	}
	if l.Kind == "flow-lite" {
		if len(l.Points) < 2 {
			return errors.New("flow-lite requires at least 2 waypoints")
		}
		for _, b := range l.Bends {
			if b < -100 || b > 100 {
				return errors.New("flow-lite section bend must be between -100 and 100")
			}
		}
		if l.Curvature < -100 || l.Curvature > 100 {
			return errors.New("flow-lite curvature must be between -100 and 100")
		}
		if l.Variant != "" && l.Variant != "dashed" && l.Variant != "dotted" && l.Variant != "single" {
			return fmt.Errorf("unsupported flow-lite variant %q", l.Variant)
		}
		if l.DashLength < 2 || l.DashLength > 400 {
			return errors.New("flow-lite dashLength must be between 2 and 400")
		}
		if l.GapLength < 2 || l.GapLength > 300 {
			return errors.New("flow-lite gapLength must be between 2 and 300")
		}
		if l.DotSize < 1 || l.DotSize > 32 {
			return errors.New("flow-lite dotSize must be between 1 and 32")
		}
		if l.StrokeWidth < 0.5 || l.StrokeWidth > 24 {
			return errors.New("flow-lite strokeWidth must be between 0.5 and 24")
		}
		if l.StartMode != "" && l.StartMode != "tunnel" && l.StartMode != "abrupt" {
			return fmt.Errorf("unsupported flow-lite startMode %q", l.StartMode)
		}
		if l.EndMode != "" && l.EndMode != "tunnel" && l.EndMode != "abrupt" {
			return fmt.Errorf("unsupported flow-lite endMode %q", l.EndMode)
		}
		if l.Direction != "" && l.Direction != "forward" && l.Direction != "reverse" && l.Direction != "cw" && l.Direction != "ccw" {
			return fmt.Errorf("unsupported flow-lite direction %q", l.Direction)
		}
		if l.PlaybackMode != "" && l.PlaybackMode != "loop" && l.PlaybackMode != "pingpong" && l.PlaybackMode != "once" {
			return fmt.Errorf("unsupported flow-lite playbackMode %q", l.PlaybackMode)
		}
		if l.Speed < 0.2 || l.Speed > 30.0 {
			return errors.New("flow-lite cycle duration (speed) must be between 0.2 and 30.0 seconds")
		}
		if l.Delay < 0 || l.Delay > 60 {
			return errors.New("flow-lite delay must be between 0 and 60")
		}
		if l.EndDelay < 0 || l.EndDelay > 60 {
			return errors.New("flow-lite endDelay must be between 0 and 60")
		}
	}
	if l.Kind == "radar-lite" {
		if l.Variant != "" && l.Variant != "single" && l.Variant != "multi" && l.Variant != "fading-ping" {
			return fmt.Errorf("unsupported radar-lite variant %q", l.Variant)
		}
		if l.Rings < 1 || l.Rings > 6 {
			return errors.New("radar-lite rings must be between 1 and 6")
		}
		if l.MinScale < 0.05 || l.MinScale > 2.0 {
			return errors.New("radar-lite minScale must be between 0.05 and 2.0")
		}
		if l.MaxScale < 0.1 || l.MaxScale > 3.0 {
			return errors.New("radar-lite maxScale must be between 0.1 and 3.0")
		}
		if l.MinScale >= l.MaxScale {
			return errors.New("radar-lite minScale must be strictly less than maxScale")
		}
		if l.Period < 0.2 || l.Period > 30.0 {
			return errors.New("radar-lite cycle duration (period) must be between 0.2 and 30.0 seconds")
		}
		if l.StrokeWidth < 0.5 || l.StrokeWidth > 20.0 {
			return errors.New("radar-lite strokeWidth must be between 0.5 and 20.0")
		}
	}
	if l.Kind == "gauge-lite" {
		if l.Variant != "" && l.Variant != "ring" && l.Variant != "hbar" && l.Variant != "vbar" && l.Variant != "segmented" && l.Variant != "level" {
			return fmt.Errorf("unsupported gauge-lite variant %q", l.Variant)
		}
		if l.MinValue == nil || l.MaxValue == nil || math.IsNaN(*l.MinValue) || math.IsNaN(*l.MaxValue) {
			return errors.New("gauge-lite requires valid finite minValue and maxValue")
		}
		if *l.MaxValue <= *l.MinValue {
			return errors.New("gauge-lite maxValue must be strictly greater than minValue")
		}
		if l.Value == nil || math.IsNaN(*l.Value) || math.IsInf(*l.Value, 0) {
			return errors.New("gauge-lite requires a valid finite value")
		}
		if l.Variant == "segmented" && (l.Segments < 2 || l.Segments > 32) {
			return errors.New("gauge-lite segmented variant requires segments between 2 and 32")
		}
		if l.UseThresholds && l.WarnThreshold != nil && l.CritThreshold != nil {
			if *l.CritThreshold < *l.WarnThreshold {
				return errors.New("gauge-lite critThreshold cannot be less than warnThreshold")
			}
		}
		if l.StrokeWidth < 0.5 || l.StrokeWidth > 32.0 {
			return errors.New("gauge-lite strokeWidth must be between 0.5 and 32.0")
		}
		if l.Direction != "" && l.Direction != "cw" && l.Direction != "ccw" && l.Direction != "forward" && l.Direction != "reverse" {
			return fmt.Errorf("unsupported gauge-lite direction %q", l.Direction)
		}
	}
	return nil
}

// MigrateLegacyLiteEffect upgrades pre-amendment LiteEffect snapshots so center-based
// coordinates, pulse dimensions, and flash durations do not reposition or corrupt saved objects.
func MigrateLegacyLiteEffect(l *LiteEffect) {
	if l == nil {
		return
	}
	if l.OriginMode == "" {
		if l.Kind == "perimeter-tracer" {
			w := l.Width
			if w <= 0 {
				w = 140
			}
			h := l.Height
			if h <= 0 {
				h = 80
			}
			l.X = l.X + w/2.0
			l.Y = l.Y + h/2.0
		} else if l.Kind == "pulse" {
			r := l.Radius
			if r <= 0 {
				r = 28
			}
			if l.Width <= 0 || (l.Width == 140 && l.Height == 80) {
				l.Width = r * 2
				l.Height = r * 2
			}
		}
		l.OriginMode = "center"
	}
	applyLiteEffectDefaults(l)
}

func applyLiteEffectDefaults(l *LiteEffect) {
	if l.Kind == "" {
		l.Kind = "perimeter-tracer"
	}
	if l.OriginMode == "" {
		l.OriginMode = "center"
	}
	if l.Color == "" {
		switch l.Kind {
		case "flash":
			l.Color = "#f59e0b"
		case "pulse", "radar-lite":
			l.Color = "#22c55e"
		default:
			l.Color = "#38bdf8"
		}
	}
	if l.Opacity <= 0 {
		l.Opacity = 0.9
	}
	if l.StrokeWidth <= 0 {
		if l.Kind == "radar-lite" {
			l.StrokeWidth = 2.0
		} else {
			l.StrokeWidth = 2.5
		}
	}
	if l.Radius <= 0 {
		l.Radius = 28
	}
	if l.Width <= 0 {
		if l.Kind == "pulse" || l.Kind == "flash" {
			l.Width = l.Radius * 2
		} else if l.Kind == "radar-lite" {
			l.Width = 80
		} else {
			l.Width = 140
		}
	}
	if l.Height <= 0 {
		if l.Kind == "pulse" || l.Kind == "flash" {
			l.Height = l.Radius * 2
		} else if l.Kind == "radar-lite" {
			l.Height = 80
		} else {
			l.Height = 80
		}
	}
	if l.X == 0 {
		l.X = 240
	}
	if l.Y == 0 {
		l.Y = 180
	}

	switch l.Kind {
	case "perimeter-tracer":
		if l.Variant == "" {
			l.Variant = "rounded"
		}
		if l.CornerRadius <= 0 {
			l.CornerRadius = 10
		}
		if l.Segments <= 0 {
			l.Segments = 2
		}
		if l.LineGapRatio == "" {
			l.LineGapRatio = "1:3"
		}
		if l.SegmentStyle == "" {
			l.SegmentStyle = "rounded"
		}
		if l.Direction == "" {
			l.Direction = "cw"
		}
		if l.PlaybackMode == "" {
			l.PlaybackMode = "loop"
		}
		if l.EntryExit == "" {
			l.EntryExit = "none"
		}
		if l.Speed <= 0 {
			l.Speed = 3.0 // 3.0s circuit duration
		}
		if l.ShowGuide == nil {
			sg := true
			l.ShowGuide = &sg
		}
		if l.PauseVisibility == "" {
			l.PauseVisibility = "hide"
		}
	case "pulse":
		if l.Variant == "" {
			l.Variant = "circle"
		}
		if l.Period <= 0 {
			l.Period = 1.8
		}
		if l.Intensity <= 0 {
			l.Intensity = 0.85
		}
		if l.MinScale <= 0 {
			l.MinScale = 0.85
		}
		if l.MaxScale <= 0 {
			l.MaxScale = 1.15
		}
		if l.ArmThickness <= 0 {
			l.ArmThickness = 3.0
		}
		if l.MaintainAspectRatio == nil {
			mar := true
			l.MaintainAspectRatio = &mar
		}
	case "flash":
		if l.Variant == "" {
			l.Variant = "diamond"
		}
		if l.Duration <= 0 {
			l.Duration = 0.10 // 100 ms default brief flash
		}
		if l.RepeatInterval <= 0 {
			l.RepeatInterval = 2.0 // 2.0s interval between flashes when Repeat=true
		}
		if l.Period <= 0 {
			l.Period = l.RepeatInterval
		}
		if l.ArmThickness <= 0 {
			l.ArmThickness = 3.0
		}
		if l.Intensity <= 0 {
			l.Intensity = 0.75 // avoid high-contrast jarring default
		}
	case "flow-lite":
		if l.Variant == "" {
			l.Variant = "dashed"
		}
		if len(l.Points) < 2 {
			l.Points = []geometry.Point{
				{X: 25, Y: 50},
				{X: 75, Y: 50},
			}
		}
		if len(l.Bends) != len(l.Points)-1 {
			newBends := make([]float64, len(l.Points)-1)
			for i := 0; i < len(newBends) && i < len(l.Bends); i++ {
				newBends[i] = l.Bends[i]
			}
			l.Bends = newBends
		}
		if l.DashLength <= 0 {
			if l.Variant == "single" {
				l.DashLength = 36
			} else {
				l.DashLength = 18
			}
		}
		if l.GapLength <= 0 {
			l.GapLength = 14
		}
		if l.DotSize <= 0 {
			l.DotSize = 5
		}
		if l.StrokeWidth <= 0 || l.StrokeWidth == 2.5 {
			l.StrokeWidth = 3.0
		}
		if l.SegmentStyle == "" {
			l.SegmentStyle = "rounded"
		}
		if l.StartMode == "" {
			l.StartMode = "tunnel"
		}
		if l.EndMode == "" {
			l.EndMode = "tunnel"
		}
		if l.Direction == "" || l.Direction == "cw" {
			l.Direction = "forward"
		} else if l.Direction == "ccw" {
			l.Direction = "reverse"
		}
		if l.PlaybackMode == "" {
			l.PlaybackMode = "loop"
		}
		if l.Speed <= 0 {
			l.Speed = 2.0
		} else if l.Speed > 30.0 {
			l.Speed = 30.0
		}
		if l.ShowGuide == nil {
			sg := true
			l.ShowGuide = &sg
		}
	case "radar-lite":
		if l.Variant == "" {
			l.Variant = "multi"
		}
		if l.Variant == "single" {
			l.Rings = 1
		} else if l.Rings <= 0 {
			l.Rings = 3
		} else if l.Rings > 6 {
			l.Rings = 6
		}
		if l.MinScale <= 0 {
			l.MinScale = 0.15
		}
		if l.MaxScale <= 0 {
			l.MaxScale = 1.0
		}
		if l.MinScale >= l.MaxScale {
			l.MinScale = math.Max(0.05, l.MaxScale*0.5)
		}
		if l.Period <= 0 {
			if l.Speed > 0 {
				l.Period = l.Speed
			} else {
				l.Period = 2.4
			}
		}
		if l.Speed <= 0 {
			l.Speed = l.Period
		}
		if l.StrokeWidth <= 0 || l.StrokeWidth == 2.5 {
			l.StrokeWidth = 2.0
		}
		if l.ShowGuide == nil {
			sg := true
			l.ShowGuide = &sg
		}
		if l.MaintainAspectRatio == nil {
			mar := true
			l.MaintainAspectRatio = &mar
		}
	case "gauge-lite":
		if l.Variant == "" {
			l.Variant = "ring"
		}
		if l.MinValue == nil {
			minV := 0.0
			l.MinValue = &minV
		}
		if l.MaxValue == nil {
			maxV := 100.0
			l.MaxValue = &maxV
		}
		if *l.MaxValue <= *l.MinValue {
			maxV := *l.MinValue + 100.0
			l.MaxValue = &maxV
		}
		if l.Value == nil {
			defV := *l.MinValue + (*l.MaxValue-*l.MinValue)*0.65
			l.Value = &defV
		}
		if l.BgColor == "" {
			l.BgColor = "#1e293b"
		}
		if l.WarnColor == "" {
			l.WarnColor = "#f59e0b"
		}
		if l.CritColor == "" {
			l.CritColor = "#ef4444"
		}
		if l.WarnThreshold == nil {
			wt := *l.MinValue + (*l.MaxValue-*l.MinValue)*0.70
			l.WarnThreshold = &wt
		}
		if l.CritThreshold == nil {
			ct := *l.MinValue + (*l.MaxValue-*l.MinValue)*0.90
			l.CritThreshold = &ct
		}
		if *l.CritThreshold < *l.WarnThreshold {
			ct := *l.WarnThreshold
			l.CritThreshold = &ct
		}
		if l.Segments <= 0 {
			l.Segments = 10
		} else if l.Segments < 2 {
			l.Segments = 2
		} else if l.Segments > 32 {
			l.Segments = 32
		}
		if l.Direction == "" {
			l.Direction = "forward"
		}
		if l.ShowGuide == nil {
			sg := true
			l.ShowGuide = &sg
		}
		if l.StrokeWidth <= 0 || l.StrokeWidth == 2.5 {
			if l.Variant == "ring" {
				l.StrokeWidth = 8.0
			} else {
				l.StrokeWidth = 2.0
			}
		}
		if l.Width == 140 && l.Height == 80 {
			switch l.Variant {
			case "ring":
				l.Width = 96
				l.Height = 96
			case "hbar", "segmented":
				l.Width = 160
				l.Height = 32
			case "vbar":
				l.Width = 36
				l.Height = 140
			case "level":
				l.Width = 84
				l.Height = 120
			}
		}
	}
}

// CreateLiteEffect adds a new LiteEffect to the scene with authoritative defaults and identity.
func (s *Scene) CreateLiteEffect(initial LiteEffect) LiteEffect {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.liteEffectsCounter++
	if initial.ID == "" {
		initial.ID = StringID(strconv.FormatInt(s.liteEffectsCounter, 10))
	} else if idNum, err := strconv.ParseInt(string(initial.ID), 10, 64); err == nil && idNum > s.liteEffectsCounter {
		s.liteEffectsCounter = idNum
	}

	applyLiteEffectDefaults(&initial)
	if initial.Visible == nil {
		v := true
		initial.Visible = &v
	}

	s.liteEffects[initial.ID] = initial
	s.liteEffectOrder = append(s.liteEffectOrder, initial.ID)
	s.layers = append(s.layers, LayerEntry{Type: SelectionLite, ID: initial.ID})
	s.revision++
	return initial
}

// UpdateLiteEffect modifies an existing LiteEffect.
func (s *Scene) UpdateLiteEffect(id StringID, patch map[string]any) (LiteEffect, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.liteEffects[id]
	if !ok {
		return LiteEffect{}, fmt.Errorf("lite effect not found: %s", id)
	}

	oldName := existing.Name
	nameChanged := false
	if v, ok := patch["name"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed != existing.Name {
			existing.Name = trimmed
			nameChanged = true
		}
	}
	if v, ok := patch["kind"].(string); ok && v != "" {
		existing.Kind = v
	}
	if v, ok := patch["variant"].(string); ok && v != "" {
		existing.Variant = v
	}
	if v, ok := patch["targetId"].(string); ok {
		existing.TargetID = v
	}
	if v, ok := toFloat(patch["x"]); ok {
		existing.X = v
	}
	if v, ok := toFloat(patch["y"]); ok {
		existing.Y = v
	}
	if v, ok := toFloat(patch["width"]); ok && v > 0 {
		existing.Width = v
	}
	if v, ok := toFloat(patch["height"]); ok && v > 0 {
		existing.Height = v
	}
	if v, ok := toFloat(patch["radius"]); ok && v > 0 {
		existing.Radius = v
	}
	if v, ok := toFloat(patch["cornerRadius"]); ok && v >= 0 {
		existing.CornerRadius = v
	}
	if v, ok := patch["color"].(string); ok && v != "" {
		existing.Color = v
	}
	if v, ok := toFloat(patch["opacity"]); ok {
		existing.Opacity = math.Max(0, math.Min(1, v))
	}
	if v, ok := toFloat(patch["strokeWidth"]); ok && v > 0 {
		existing.StrokeWidth = v
	}
	if v, ok := toFloat(patch["segments"]); ok && v >= 1 {
		existing.Segments = int(math.Round(v))
	}
	if v, ok := toFloat(patch["rings"]); ok && v >= 1 {
		existing.Rings = int(math.Max(1, math.Min(6, math.Round(v))))
	}
	if existing.Kind == "radar-lite" && existing.Variant == "single" {
		existing.Rings = 1
	} else if existing.Kind == "radar-lite" && (existing.Variant == "multi" || existing.Variant == "fading-ping") && existing.Rings < 1 {
		existing.Rings = 3
	}
	if v, ok := patch["lineGapRatio"].(string); ok && v != "" {
		existing.LineGapRatio = v
	}
	if v, ok := patch["segmentStyle"].(string); ok && v != "" {
		existing.SegmentStyle = v
	}
	if v, ok := toFloat(patch["startOffset"]); ok {
		existing.StartOffset = v
	}
	if v, ok := patch["direction"].(string); ok && (v == "cw" || v == "ccw" || v == "forward" || v == "reverse") {
		existing.Direction = v
	}
	if v, ok := patch["playbackMode"].(string); ok && (v == "loop" || v == "pingpong" || v == "once") {
		existing.PlaybackMode = v
	}
	if v, ok := patch["entryExit"].(string); ok && (v == "none" || v == "fade" || v == "grow" || v == "fade-grow") {
		existing.EntryExit = v
	}
	if v, ok := toFloat(patch["burstCycles"]); ok {
		existing.BurstCycles = int(math.Max(0, math.Min(100, math.Round(v))))
	}
	if v, ok := toFloat(patch["pauseSeconds"]); ok {
		existing.PauseSeconds = math.Max(0, math.Min(300, v))
	}
	if v, ok := patch["pauseVisibility"].(string); ok && (v == "hide" || v == "hold") {
		existing.PauseVisibility = v
	}
	if v, ok := toFloat(patch["armThickness"]); ok && v > 0 {
		existing.ArmThickness = math.Max(1, math.Min(20, v))
	}
	if v, ok := toFloat(patch["repeatInterval"]); ok && v > 0 {
		existing.RepeatInterval = math.Max(0.2, math.Min(60, v))
	}
	if v, ok := toFloat(patch["speed"]); ok && v > 0 {
		if existing.Kind == "flow-lite" {
			existing.Speed = math.Max(0.2, math.Min(30.0, v))
		} else {
			existing.Speed = v
		}
	}
	if v, ok := toFloat(patch["period"]); ok && v > 0 {
		if existing.Kind == "radar-lite" {
			existing.Period = math.Max(0.2, math.Min(30.0, v))
			existing.Speed = existing.Period
		} else {
			existing.Period = v
		}
	}
	if v, ok := toFloat(patch["dur"]); ok && v > 0 {
		existing.Duration = v
	}
	if v, ok := toFloat(patch["duration"]); ok && v > 0 {
		existing.Duration = v
	}
	if v, ok := toFloat(patch["durationMs"]); ok && v > 0 {
		existing.Duration = v / 1000.0
	}
	if v, ok := toFloat(patch["intensity"]); ok {
		existing.Intensity = math.Max(0, math.Min(1, v))
	}
	if v, ok := toFloat(patch["minScale"]); ok && v > 0 {
		minBound := 0.1
		if existing.Kind == "radar-lite" {
			minBound = 0.05
		}
		existing.MinScale = math.Max(minBound, math.Min(3.0, v))
	}
	if v, ok := toFloat(patch["maxScale"]); ok && v > 0 {
		existing.MaxScale = math.Max(0.1, math.Min(4.0, v))
	}
	if existing.Kind == "radar-lite" && existing.MinScale >= existing.MaxScale {
		if _, hasMin := patch["minScale"]; hasMin {
			existing.MaxScale = math.Min(3.0, existing.MinScale+0.1)
		} else {
			existing.MinScale = math.Max(0.05, existing.MaxScale-0.1)
		}
	}
	if v, ok := patch["originMode"].(string); ok && v != "" {
		existing.OriginMode = v
	}
	if v, ok := patch["maintainAspectRatio"].(bool); ok {
		existing.MaintainAspectRatio = &v
	}
	if v, ok := patch["showGuide"].(bool); ok {
		existing.ShowGuide = &v
	}
	if v, ok := patch["repeat"].(bool); ok {
		existing.Repeat = v
	}
	if v, ok := toFloat(patch["triggerNonce"]); ok {
		existing.TriggerNonce = int64(v)
	}
	if v, ok := patch["visible"].(bool); ok {
		existing.Visible = &v
	}
	if v, ok := patch["locked"].(bool); ok {
		existing.Locked = v
	}
	if rawPts, ok := patch["points"]; ok {
		var parsedPts []geometry.Point
		switch pts := rawPts.(type) {
		case []geometry.Point:
			parsedPts = append(parsedPts, pts...)
		case []any:
			for _, item := range pts {
				if m, ok := item.(map[string]any); ok {
					x, okX := toFloat(m["x"])
					y, okY := toFloat(m["y"])
					if okX && okY {
						parsedPts = append(parsedPts, geometry.Point{X: x, Y: y})
					}
				}
			}
		}
		if len(parsedPts) >= 2 {
			existing.Points = parsedPts
			if len(existing.Bends) != len(existing.Points)-1 {
				newBends := make([]float64, len(existing.Points)-1)
				for i := 0; i < len(newBends) && i < len(existing.Bends); i++ {
					newBends[i] = existing.Bends[i]
				}
				existing.Bends = newBends
			}
		}
	}
	if rawBends, ok := patch["bends"]; ok {
		var parsedBends []float64
		switch bs := rawBends.(type) {
		case []float64:
			parsedBends = append(parsedBends, bs...)
		case []any:
			for _, item := range bs {
				if f, ok := toFloat(item); ok {
					parsedBends = append(parsedBends, math.Max(-100, math.Min(100, f)))
				}
			}
		}
		if len(existing.Points) >= 2 {
			newBends := make([]float64, len(existing.Points)-1)
			for i := 0; i < len(newBends) && i < len(parsedBends); i++ {
				newBends[i] = math.Max(-100, math.Min(100, parsedBends[i]))
			}
			existing.Bends = newBends
		} else {
			existing.Bends = parsedBends
		}
	}
	if v, ok := toFloat(patch["curvature"]); ok {
		existing.Curvature = math.Max(-100, math.Min(100, v))
	}
	if v, ok := toFloat(patch["dashLength"]); ok && v > 0 {
		existing.DashLength = math.Max(2, math.Min(400, v))
	}
	if v, ok := toFloat(patch["gapLength"]); ok && v > 0 {
		existing.GapLength = math.Max(2, math.Min(300, v))
	}
	if v, ok := toFloat(patch["dotSize"]); ok && v > 0 {
		existing.DotSize = math.Max(1, math.Min(32, v))
	}
	if v, ok := patch["startMode"].(string); ok && (v == "tunnel" || v == "abrupt") {
		existing.StartMode = v
	}
	if v, ok := patch["endMode"].(string); ok && (v == "tunnel" || v == "abrupt") {
		existing.EndMode = v
	}
	if v, ok := toFloat(patch["delay"]); ok {
		existing.Delay = math.Max(0, math.Min(60, v))
	}
	if v, ok := toFloat(patch["endDelay"]); ok {
		existing.EndDelay = math.Max(0, math.Min(60, v))
	}
	if v, ok := toFloat(patch["value"]); ok {
		val := v
		existing.Value = &val
	}
	if v, ok := toFloat(patch["minValue"]); ok {
		minV := v
		existing.MinValue = &minV
	}
	if v, ok := toFloat(patch["maxValue"]); ok {
		maxV := v
		existing.MaxValue = &maxV
	}
	if existing.Kind == "gauge-lite" && existing.MinValue != nil && existing.MaxValue != nil && *existing.MaxValue <= *existing.MinValue {
		if _, hasMin := patch["minValue"]; hasMin && patch["maxValue"] == nil {
			maxV := *existing.MinValue + 1.0
			existing.MaxValue = &maxV
		} else if _, hasMax := patch["maxValue"]; hasMax && patch["minValue"] == nil {
			minV := *existing.MaxValue - 1.0
			existing.MinValue = &minV
		}
	}
	if v, ok := patch["bgColor"].(string); ok && v != "" {
		existing.BgColor = v
	}
	if v, ok := patch["useThresholds"].(bool); ok {
		existing.UseThresholds = v
	}
	if v, ok := toFloat(patch["warnThreshold"]); ok {
		wt := v
		existing.WarnThreshold = &wt
	}
	if v, ok := patch["warnColor"].(string); ok && v != "" {
		existing.WarnColor = v
	}
	if v, ok := toFloat(patch["critThreshold"]); ok {
		ct := v
		existing.CritThreshold = &ct
	}
	if v, ok := patch["critColor"].(string); ok && v != "" {
		existing.CritColor = v
	}
	if existing.Kind == "gauge-lite" && existing.WarnThreshold != nil && existing.CritThreshold != nil && *existing.CritThreshold < *existing.WarnThreshold {
		if _, hasWarn := patch["warnThreshold"]; hasWarn && patch["critThreshold"] == nil {
			ct := *existing.WarnThreshold
			existing.CritThreshold = &ct
		} else if _, hasCrit := patch["critThreshold"]; hasCrit && patch["warnThreshold"] == nil {
			wt := *existing.CritThreshold
			existing.WarnThreshold = &wt
		}
	}
	if v, ok := patch["unit"].(string); ok {
		existing.Unit = strings.TrimSpace(v)
	}
	if v, ok := patch["labelText"].(string); ok {
		existing.LabelText = strings.TrimSpace(v)
	}
	if existing.Kind == "gauge-lite" {
		applyLiteEffectDefaults(&existing)
	}

	if err := ValidateLiteEffect(&existing); err != nil {
		return LiteEffect{}, err
	}

	s.liteEffects[id] = existing
	if nameChanged {
		targetID := "lite-" + string(id)
		targetName := existing.Name
		if targetName == "" {
			targetName = targetID
		}
		for i := range s.parameters {
			p := &s.parameters[i]
			if p.Target == string(id) || p.Target == targetID || (oldName != "" && p.Target == oldName) {
				p.Name = GenerateParameterName(targetName, p.Property, "lite")
			}
		}
	}
	s.revision++
	return existing, nil
}

func (s *Scene) removeLiteEffectLocked(id StringID) bool {
	if _, ok := s.liteEffects[id]; !ok {
		return false
	}
	delete(s.liteEffects, id)
	for i, oid := range s.liteEffectOrder {
		if oid == id {
			s.liteEffectOrder = append(s.liteEffectOrder[:i], s.liteEffectOrder[i+1:]...)
			break
		}
	}
	s.pruneSelectionLocked(SelectionLite, id)
	s.pruneGroupMembersLocked(SelectionLite, id)
	s.removeLayerEntryLocked(SelectionLite, id)
	return true
}

// DeleteLiteEffect removes a LiteEffect from the scene.
func (s *Scene) DeleteLiteEffect(id StringID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.removeLiteEffectLocked(id) {
		return false
	}
	s.revision++
	return true
}

// GetLiteEffect returns a single LiteEffect by ID.
func (s *Scene) GetLiteEffect(id StringID) (LiteEffect, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.liteEffects[id]
	return l, ok
}

// GetLiteEffects returns all LiteEffects in creation/order sequence.
func (s *Scene) GetLiteEffects() []LiteEffect {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LiteEffect, 0, len(s.liteEffectOrder))
	for _, id := range s.liteEffectOrder {
		if l, ok := s.liteEffects[id]; ok {
			out = append(out, l)
		}
	}
	return out
}
