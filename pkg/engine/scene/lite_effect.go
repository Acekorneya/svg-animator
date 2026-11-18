package scene

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
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
	dashLen := math.Round((unit*(l/totalParts))*100) / 100
	gapLen := math.Round((unit*(g/totalParts))*100) / 100
	return dashLen, gapLen
}

// ValidateLiteEffect validates operational bounds for Stage 16D Lite effects.
func ValidateLiteEffect(l *LiteEffect) error {
	if l == nil {
		return errors.New("nil lite effect")
	}
	switch l.Kind {
	case "perimeter-tracer", "pulse", "flash":
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
		case "pulse":
			l.Color = "#22c55e"
		default:
			l.Color = "#38bdf8"
		}
	}
	if l.Opacity <= 0 {
		l.Opacity = 0.9
	}
	if l.StrokeWidth <= 0 {
		l.StrokeWidth = 2.5
	}
	if l.Radius <= 0 {
		l.Radius = 28
	}
	if l.Width <= 0 {
		if l.Kind == "pulse" || l.Kind == "flash" {
			l.Width = l.Radius * 2
		} else {
			l.Width = 140
		}
	}
	if l.Height <= 0 {
		if l.Kind == "pulse" || l.Kind == "flash" {
			l.Height = l.Radius * 2
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
		if l.Period <= 0 {
			l.Period = 3.0 // gentle repeating interval if Repeat=true
		}
		if l.Intensity <= 0 {
			l.Intensity = 0.75 // avoid high-contrast jarring default
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
	if v, ok := patch["lineGapRatio"].(string); ok && v != "" {
		existing.LineGapRatio = v
	}
	if v, ok := patch["segmentStyle"].(string); ok && v != "" {
		existing.SegmentStyle = v
	}
	if v, ok := toFloat(patch["startOffset"]); ok {
		existing.StartOffset = v
	}
	if v, ok := patch["direction"].(string); ok && (v == "cw" || v == "ccw") {
		existing.Direction = v
	}
	if v, ok := patch["playbackMode"].(string); ok && (v == "loop" || v == "pingpong" || v == "once") {
		existing.PlaybackMode = v
	}
	if v, ok := patch["entryExit"].(string); ok && (v == "none" || v == "fade" || v == "grow" || v == "fade-grow") {
		existing.EntryExit = v
	}
	if v, ok := toFloat(patch["speed"]); ok && v > 0 {
		existing.Speed = v
	}
	if v, ok := toFloat(patch["period"]); ok && v > 0 {
		existing.Period = v
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
		existing.MinScale = math.Max(0.1, math.Min(3.0, v))
	}
	if v, ok := toFloat(patch["maxScale"]); ok && v > 0 {
		existing.MaxScale = math.Max(0.1, math.Min(4.0, v))
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
