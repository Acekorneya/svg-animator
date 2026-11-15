package scene

import (
	"fmt"
	"strings"
)

// FixedPrefixes defines the required prefix for user object names by type.
var FixedPrefixes = map[SelectionType]string{
	SelectionNode:   "node-",
	SelectionFlow:   "flow-",
	SelectionGlow:   "glow-",
	SelectionRipple: "ripple-",
	SelectionImage:  "image-",
}

// PrefixForType returns the canonical fixed prefix for an object selection type.
func PrefixForType(selType SelectionType) string {
	if p, ok := FixedPrefixes[selType]; ok {
		return p
	}
	return string(selType) + "-"
}

// NormalizeObjectName ensures the object name is formatted cleanly.
// If input already has the fixed prefix (e.g. "flow-border"), it is kept intact.
// If input is empty, fallbackID is formatted as "<prefix><fallbackID>".
func NormalizeObjectName(selType SelectionType, input string, fallbackID StringID) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		prefix := PrefixForType(selType)
		idStr := strings.TrimPrefix(string(fallbackID), prefix)
		return prefix + idStr
	}
	return trimmed
}

// ValidateObjectName verifies that an object name is unique within its object category in the scene.
func (s *Scene) ValidateObjectName(selType SelectionType, name string, excludeID StringID) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil // Clearing name is permitted for backward compatibility
	}

	lower := strings.ToLower(trimmed)

	switch selType {
	case SelectionNode:
		for id, node := range s.nodes {
			if id != excludeID && strings.ToLower(node.Name) == lower {
				return fmt.Errorf("node name %q is already in use by another node", trimmed)
			}
		}
	case SelectionFlow:
		for id, flow := range s.flows {
			if id != excludeID && strings.ToLower(flow.Name) == lower {
				return fmt.Errorf("flow name %q is already in use by another flow", trimmed)
			}
		}
	case SelectionGlow:
		for id, glow := range s.glows {
			if id != excludeID && strings.ToLower(glow.Name) == lower {
				return fmt.Errorf("glow name %q is already in use by another glow", trimmed)
			}
		}
	case SelectionRipple:
		for id, rip := range s.ripples {
			if id != excludeID && strings.ToLower(rip.Name) == lower {
				return fmt.Errorf("ripple name %q is already in use by another ripple", trimmed)
			}
		}
	case SelectionImage:
		for id, img := range s.images {
			if id != excludeID && strings.ToLower(img.Name) == lower {
				return fmt.Errorf("image name %q is already in use by another image", trimmed)
			}
		}
	}
	return nil
}

// EnsureUniqueObjectName returns name if unique for that type, or appends -2, -3...
func (s *Scene) EnsureUniqueObjectName(selType SelectionType, desiredName string, id StringID) string {
	trimmed := strings.TrimSpace(desiredName)
	if trimmed == "" {
		return ""
	}
	candidate := trimmed
	counter := 2
	for s.ValidateObjectName(selType, candidate, id) != nil {
		candidate = fmt.Sprintf("%s-%d", trimmed, counter)
		counter++
	}
	return candidate
}
