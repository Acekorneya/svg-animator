package scene

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var widgetInstanceSeq uint64

// NextWidgetInstanceID returns a unique, collision-free instance identifier string.
func NextWidgetInstanceID() string {
	seq := atomic.AddUint64(&widgetInstanceSeq, 1)
	return fmt.Sprintf("sva-w-%d-%d", time.Now().UnixNano()%1000000, seq)
}

// WidgetIsolationOptions configures SVG widget isolation.
type WidgetIsolationOptions struct {
	InstanceID string          `json:"instanceId,omitempty"`
	Prefix     string          `json:"prefix,omitempty"`
	ScopeCSS   bool            `json:"scopeCss"`
	Manifest   *WidgetManifest `json:"manifest,omitempty"`
}

// IsolateSVGWidget rewrites an SVG document so that all element IDs, internal
// fragment references, CSS selectors, and animation keyframes are scoped to
// a unique instance namespace.
//
// This guarantees that multiple SVG instances can coexist inside the same HTML
// document or DOM tree without ID collisions, filter/gradient crosstalk, or
// CSS animation interference.
//
// Data URLs (data:image/...), external URLs (http://, https://), and arbitrary
// text containing '#' (e.g. '#1 Priority' or hex colors '#0d1117') are preserved untouched.
func IsolateSVGWidget(rawSVG string, opts WidgetIsolationOptions) (string, map[string]string, error) {
	if strings.TrimSpace(rawSVG) == "" {
		return "", nil, fmt.Errorf("rawSVG cannot be empty")
	}

	instanceID := strings.TrimSpace(opts.InstanceID)
	if instanceID == "" {
		instanceID = NextWidgetInstanceID()
	}

	// Sanitize instanceID to ensure valid XML/CSS identifier
	regInvalid := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	instanceID = regInvalid.ReplaceAllString(instanceID, "-")

	prefix := strings.TrimSpace(opts.Prefix)
	if prefix == "" {
		prefix = instanceID + "__"
	}

	rootWidgetID := "sva-widget-" + instanceID

	// 1. Scan and collect all declared IDs: id="VALUE" or id='VALUE'
	idMap := make(map[string]string)
	reDeclaredID := regexp.MustCompile(`\bid\s*=\s*["']([^"']+)["']`)
	matches := reDeclaredID.FindAllStringSubmatch(rawSVG, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			oldID := m[1]
			if oldID != "" && idMap[oldID] == "" {
				idMap[oldID] = prefix + oldID
			}
		}
	}

	// 2. Scan and collect all @keyframes names in <style>
	keyframeMap := make(map[string]string)
	reKeyframeName := regexp.MustCompile(`@keyframes\s+([a-zA-Z0-9_-]+)`)
	kfMatches := reKeyframeName.FindAllStringSubmatch(rawSVG, -1)
	for _, km := range kfMatches {
		if len(km) >= 2 {
			oldKF := km[1]
			if oldKF != "" && keyframeMap[oldKF] == "" {
				keyframeMap[oldKF] = prefix + oldKF
			}
		}
	}

	// 3. Transform CSS in <style>...</style> blocks
	reStyleBlock := regexp.MustCompile(`(?s)<style\b([^>]*)>(.*?)</style>`)
	transformed := reStyleBlock.ReplaceAllStringFunc(rawSVG, func(block string) string {
		sub := reStyleBlock.FindStringSubmatch(block)
		if len(sub) < 3 {
			return block
		}
		styleAttrs := sub[1]
		cssContent := sub[2]

		// Replace @keyframes names
		for oldKF, newKF := range keyframeMap {
			reKFDef := regexp.MustCompile(`@keyframes\s+` + regexp.QuoteMeta(oldKF) + `\b`)
			cssContent = reKFDef.ReplaceAllString(cssContent, "@keyframes "+newKF)

			reAnimProp := regexp.MustCompile(`\b(animation(?:-name)?\s*:\s*[^;}]*?)\b` + regexp.QuoteMeta(oldKF) + `\b`)
			cssContent = reAnimProp.ReplaceAllString(cssContent, "${1}"+newKF)
		}

		// Rewrite url(#oldID) inside CSS declarations
		for oldID, newID := range idMap {
			reCSSUrl := regexp.MustCompile(`url\(\s*["']?#` + regexp.QuoteMeta(oldID) + `["']?\s*\)`)
			cssContent = reCSSUrl.ReplaceAllString(cssContent, "url(#"+newID+")")
		}

		// Scope CSS rules to the widget root element
		if opts.ScopeCSS || true {
			cssContent = scopeCSSRules(cssContent, rootWidgetID, idMap)
		}

		return "<style" + styleAttrs + ">" + cssContent + "</style>"
	})

	// 4. Transform tag attributes
	// Regex matches opening or self-closing tags: <tagname attrs...>
	reTag := regexp.MustCompile(`(?s)<([a-zA-Z0-9:_-]+)((?:\s+[^>]+)?)>`)
	isFirstSVG := true

	transformed = reTag.ReplaceAllStringFunc(transformed, func(tagMatch string) string {
		sub := reTag.FindStringSubmatch(tagMatch)
		if len(sub) < 3 {
			return tagMatch
		}
		tagName := sub[1]
		tagAttrs := sub[2]

		// If closing tag (e.g. </g>), do not touch
		if strings.HasPrefix(tagName, "/") {
			return tagMatch
		}

		// Check if this is the root <svg> element
		if tagName == "svg" && isFirstSVG {
			isFirstSVG = false
			// Inject or replace id on root SVG
			reRootID := regexp.MustCompile(`\bid\s*=\s*["'][^"']*["']`)
			if reRootID.MatchString(tagAttrs) {
				tagAttrs = reRootID.ReplaceAllString(tagAttrs, `id="`+rootWidgetID+`"`)
			} else {
				tagAttrs = ` id="` + rootWidgetID + `"` + tagAttrs
			}

			// Add data-sva-widget-instance attribute
			reInstAttr := regexp.MustCompile(`\bdata-sva-widget-instance\s*=\s*["'][^"']*["']`)
			if !reInstAttr.MatchString(tagAttrs) {
				tagAttrs += ` data-sva-widget-instance="` + instanceID + `"`
			}
		} else {
			// For all other elements, transform id attribute if present
			reIDAttr := regexp.MustCompile(`\bid\s*=\s*(["'])([^"']+)(["'])`)
			tagAttrs = reIDAttr.ReplaceAllStringFunc(tagAttrs, func(m string) string {
				parts := reIDAttr.FindStringSubmatch(m)
				if len(parts) >= 4 {
					oldID := parts[2]
					if newID, ok := idMap[oldID]; ok {
						return `id=` + parts[1] + newID + parts[3]
					}
				}
				return m
			})
		}

		// Rewrite href and xlink:href if value starts with # (internal fragment reference)
		reHref := regexp.MustCompile(`\b(xlink:href|href)\s*=\s*(["'])(#)([^"']+)(["'])`)
		tagAttrs = reHref.ReplaceAllStringFunc(tagAttrs, func(m string) string {
			parts := reHref.FindStringSubmatch(m)
			if len(parts) >= 6 {
				attrName := parts[1]
				q1 := parts[2]
				targetID := parts[4]
				q2 := parts[5]
				if newID, ok := idMap[targetID]; ok {
					return attrName + `=` + q1 + `#` + newID + q2
				}
			}
			return m
		})

		// Rewrite url(#oldID) inside attributes (fill, stroke, filter, clip-path, mask, marker-*, style, etc.)
		reAttrURL := regexp.MustCompile(`url\(\s*["']?#([a-zA-Z0-9:_-]+)["']?\s*\)`)
		tagAttrs = reAttrURL.ReplaceAllStringFunc(tagAttrs, func(m string) string {
			parts := reAttrURL.FindStringSubmatch(m)
			if len(parts) >= 2 {
				targetID := parts[1]
				if newID, ok := idMap[targetID]; ok {
					return "url(#" + newID + ")"
				}
			}
			return m
		})

		// Rewrite aria-labelledby and aria-describedby space-separated ID tokens
		reAria := regexp.MustCompile(`\b(aria-labelledby|aria-describedby)\s*=\s*(["'])([^"']+)(["'])`)
		tagAttrs = reAria.ReplaceAllStringFunc(tagAttrs, func(m string) string {
			parts := reAria.FindStringSubmatch(m)
			if len(parts) >= 5 {
				attrName := parts[1]
				q1 := parts[2]
				tokens := strings.Fields(parts[3])
				q2 := parts[4]
				newTokens := make([]string, len(tokens))
				for i, tok := range tokens {
					if mapped, ok := idMap[tok]; ok {
						newTokens[i] = mapped
					} else {
						newTokens[i] = tok
					}
				}
				return attrName + `=` + q1 + strings.Join(newTokens, " ") + q2
			}
			return m
		})

		// Rewrite SMIL event timing references: begin="anim1.begin" / end="anim1.end+2s"
		reTiming := regexp.MustCompile(`\b(begin|end)\s*=\s*(["'])([^"']+)(["'])`)
		tagAttrs = reTiming.ReplaceAllStringFunc(tagAttrs, func(m string) string {
			parts := reTiming.FindStringSubmatch(m)
			if len(parts) >= 5 {
				attrName := parts[1]
				q1 := parts[2]
				timingVal := parts[3]
				q2 := parts[4]
				for oldID, newID := range idMap {
					reEvt := regexp.MustCompile(`\b` + regexp.QuoteMeta(oldID) + `\.`)
					timingVal = reEvt.ReplaceAllString(timingVal, newID+".")
				}
				return attrName + `=` + q1 + timingVal + q2
			}
			return m
		})

		return "<" + tagName + tagAttrs + ">"
	})

	// Inject manifest if provided and not already present
	if opts.Manifest != nil && len(opts.Manifest.Parameters) > 0 {
		reExistingManifest := regexp.MustCompile(`(?i)<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["']`)
		if !reExistingManifest.MatchString(transformed) {
			manifestBytes, err := json.Marshal(opts.Manifest)
			if err == nil {
				manifestTag := fmt.Sprintf("\n  <script type=\"application/json\" id=\"%ssva-widget-manifest\">%s</script>\n", prefix, string(manifestBytes))
				if idx := strings.LastIndex(transformed, "</svg>"); idx != -1 {
					transformed = transformed[:idx] + manifestTag + transformed[idx:]
				} else {
					transformed += manifestTag
				}
			}
		}
	}

	return transformed, idMap, nil
}

// scopeCSSRules scopes CSS rules to the widget root ID so they don't affect sibling elements.
func scopeCSSRules(css string, rootID string, idMap map[string]string) string {
	// Temporarily extract @keyframes blocks so their inner rules are not scoped
	var kfBlocks []string
	reKFBlock := regexp.MustCompile(`(?s)@keyframes\s+[^{]+\{(?:[^{}]*\{[^{}]*\}[^{}]*)*\}`)
	cssWithoutKF := reKFBlock.ReplaceAllStringFunc(css, func(m string) string {
		kfBlocks = append(kfBlocks, m)
		return fmt.Sprintf("/*__SVA_KF_BLOCK_%d__*/", len(kfBlocks)-1)
	})

	// Rule matcher: selector { declarations }
	reRule := regexp.MustCompile(`(?s)([^{}/]+)\{([^{}]+)\}`)
	scoped := reRule.ReplaceAllStringFunc(cssWithoutKF, func(m string) string {
		sub := reRule.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		rawSel := strings.TrimSpace(sub[1])
		body := sub[2]

		if strings.HasPrefix(rawSel, "@") || strings.HasPrefix(rawSel, "/*") {
			return m
		}

		selList := strings.Split(rawSel, ",")
		var scopedList []string
		for _, s := range selList {
			cleanSel := strings.TrimSpace(s)
			if cleanSel == "" {
				continue
			}
			// Remap any #oldID in selector
			for oldID, newID := range idMap {
				reOldSelID := regexp.MustCompile(`#` + regexp.QuoteMeta(oldID) + `\b`)
				cleanSel = reOldSelID.ReplaceAllString(cleanSel, "#"+newID)
			}
			scopedList = append(scopedList, "#"+rootID+" "+cleanSel)
		}

		if len(scopedList) == 0 {
			return m
		}

		return "\n    " + strings.Join(scopedList, ", ") + " {" + body + "}"
	})

	// Restore @keyframes blocks
	for i, kf := range kfBlocks {
		placeholder := fmt.Sprintf("/*__SVA_KF_BLOCK_%d__*/", i)
		scoped = strings.Replace(scoped, placeholder, kf, 1)
	}

	return scoped
}

// EligibleProperty describes a supported controllable property for an object type.
type EligibleProperty struct {
	Property    string   `json:"property"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Options     []string `json:"options,omitempty"`
	Default     any      `json:"default"`
}

func floatPtr(v float64) *float64 {
	return &v
}

// ObjectPropertyCatalog maps each object category to its genuinely supported properties.
var ObjectPropertyCatalog = map[string][]EligibleProperty{
	"node": {
		{Property: "fill", Type: "color", Label: "Fill Colour", Description: "Primary interior fill color of node shape", Default: "#3b82f6"},
		{Property: "stroke", Type: "color", Label: "Border Stroke", Description: "Outline and border stroke color", Default: "#1d4ed8"},
		{Property: "opacity", Type: "number", Label: "Opacity", Description: "Transparency of the node (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of the node", Default: true},
		{Property: "text", Type: "text", Label: "Label Text", Description: "Primary text label displayed inside the node", Default: "Node"},
	},
	"flow": {
		{Property: "stroke", Type: "color", Label: "Line Colour", Description: "Connector line and particle stroke color", Default: "#38bdf8"},
		{Property: "speed", Type: "speed", Label: "Flow Speed", Description: "Animation speed multiplier for this flow connector (0.1 to 10.0)", Min: floatPtr(0.1), Max: floatPtr(10.0), Default: 1.0},
		{Property: "opacity", Type: "number", Label: "Opacity", Description: "Transparency of the connector (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of this flow connector", Default: true},
	},
	"glow": {
		{Property: "color", Type: "color", Label: "Glow Colour", Description: "Accent color of the glow and beacon pulse", Default: "#f59e0b"},
		{Property: "opacity", Type: "number", Label: "Glow Intensity", Description: "Transparency / intensity of the glow element (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of this glow hotspot", Default: true},
	},
	"ripple": {
		{Property: "color", Type: "color", Label: "Wave Colour", Description: "Stroke color of the propagating ripple rings", Default: "#06b6d4"},
		{Property: "opacity", Type: "number", Label: "Wave Opacity", Description: "Transparency of the ripple effect (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of this ripple effect", Default: true},
	},
	"image": {
		{Property: "opacity", Type: "number", Label: "Image Opacity", Description: "Transparency of the image layer (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of the image layer", Default: true},
	},
	"group": {
		{Property: "opacity", Type: "number", Label: "Group Opacity", Description: "Transparency of the group (0.0 to 1.0)", Min: floatPtr(0), Max: floatPtr(1), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Toggle visibility of the group", Default: true},
	},
	"root": {
		{Property: "playbackRate", Type: "speed", Label: "Master Speed", Description: "Overall playback speed rate multiplier for the widget (0.1 to 5.0)", Min: floatPtr(0.1), Max: floatPtr(5.0), Default: 1.0},
		{Property: "visibility", Type: "boolean", Label: "Visibility", Description: "Master visibility toggle for the entire widget", Default: true},
		{Property: "operatingState", Type: "enum", Label: "Operating State", Description: "Lifecycle operating state of the widget (running, paused, stopped)", Options: []string{"running", "paused", "stopped"}, Default: "running"},
	},
}

// InferTargetType deduces the object category from a target identifier.
func InferTargetType(target string) string {
	t := strings.TrimSpace(strings.ToLower(target))
	switch {
	case t == "" || t == "root" || t == "global" || t == "stage" || t == "widget":
		return "root"
	case strings.HasPrefix(t, "node-") || strings.HasPrefix(t, "node_"):
		return "node"
	case strings.HasPrefix(t, "flow-") || strings.HasPrefix(t, "flow_"):
		return "flow"
	case strings.HasPrefix(t, "glow-") || strings.HasPrefix(t, "glow_"):
		return "glow"
	case strings.HasPrefix(t, "ripple-") || strings.HasPrefix(t, "ripple_"):
		return "ripple"
	case strings.HasPrefix(t, "image-") || strings.HasPrefix(t, "image_"):
		return "image"
	case strings.HasPrefix(t, "group-") || strings.HasPrefix(t, "group_"):
		return "group"
	default:
		return ""
	}
}

// GetEligibleProperties returns the slice of supported properties for a given target type.
func GetEligibleProperties(targetType string) []EligibleProperty {
	norm := strings.TrimSpace(strings.ToLower(targetType))
	if norm == "global" || norm == "stage" || norm == "widget" {
		norm = "root"
	}
	if list, ok := ObjectPropertyCatalog[norm]; ok {
		out := make([]EligibleProperty, len(list))
		copy(out, list)
		return out
	}
	return []EligibleProperty{}
}

// ParameterConflict reports an ambiguous duplicate binding where multiple parameters bind to the same (target, property).
type ParameterConflict struct {
	Target      string            `json:"target"`
	Property    string            `json:"property"`
	Parameters  []WidgetParameter `json:"parameters"`
	Description string            `json:"description"`
}

// ValidateWidgetParameter validates a WidgetParameter definition according to the Stage 16B schema.
func ValidateWidgetParameter(p WidgetParameter) error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("parameter name cannot be empty")
	}
	// Check valid identifier: must start with letter, followed by letters, digits, underscores, or hyphens
	reIdent := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
	if !reIdent.MatchString(name) {
		return fmt.Errorf("invalid parameter name %q: must start with a letter and contain only alphanumeric characters, underscores, or hyphens (max 64 chars)", name)
	}

	target := strings.TrimSpace(p.Target)
	if target == "" {
		return fmt.Errorf("parameter target cannot be empty")
	}

	// Supported types
	switch p.Type {
	case "color", "number", "boolean", "text", "enum", "speed":
		// valid
	default:
		return fmt.Errorf("unsupported parameter type %q: must be color, number, boolean, text, enum, or speed", p.Type)
	}

	// Supported safe properties
	prop := strings.TrimSpace(p.Property)
	if prop == "" {
		return fmt.Errorf("parameter property cannot be empty")
	}

	// Reject unsafe properties or script injection vectors
	lowerProp := strings.ToLower(prop)
	if strings.Contains(lowerProp, "script") ||
		(strings.HasPrefix(lowerProp, "on") && len(lowerProp) > 2) ||
		strings.Contains(lowerProp, "handler") ||
		strings.Contains(lowerProp, "href") ||
		strings.Contains(lowerProp, "style") ||
		strings.Contains(lowerProp, "javascript") {
		return fmt.Errorf("unsafe property %q is rejected", prop)
	}

	// Prohibit min/max on non-numeric types
	if p.Type != "number" && p.Type != "speed" {
		if p.Min != nil || p.Max != nil {
			return fmt.Errorf("parameter %q of type %q cannot define numeric min or max limits", name, p.Type)
		}
	}

	// Validate min/max ordering on numeric types
	if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
		return fmt.Errorf("parameter %q min (%v) cannot be greater than max (%v)", name, *p.Min, *p.Max)
	}

	// Check property eligibility against ObjectPropertyCatalog if target type is recognized
	targetType := InferTargetType(target)
	if targetType != "" {
		allowed := GetEligibleProperties(targetType)
		if len(allowed) > 0 {
			var found *EligibleProperty
			for _, ep := range allowed {
				if ep.Property == prop || (targetType == "flow" && prop == "playbackRate" && ep.Property == "speed") {
					found = &ep
					break
				}
			}
			if found == nil {
				return fmt.Errorf("property %q is not eligible for target %q of type %q", prop, target, targetType)
			}
			if p.Type != found.Type {
				return fmt.Errorf("property %q on %s requires type %q (got %q)", prop, targetType, found.Type, p.Type)
			}
		}
	}

	// Validate property compatibility with type
	switch p.Type {
	case "color":
		if prop != "fill" && prop != "stroke" && prop != "color" {
			return fmt.Errorf("color parameter does not support property %q (allowed: fill, stroke, color)", prop)
		}
	case "boolean":
		if prop != "visibility" && prop != "display" {
			return fmt.Errorf("boolean parameter does not support property %q (allowed: visibility, display)", prop)
		}
	case "text":
		if prop != "text" && prop != "textContent" {
			return fmt.Errorf("text parameter does not support property %q (allowed: text, textContent)", prop)
		}
	case "speed":
		if prop != "playbackRate" && prop != "speed" {
			return fmt.Errorf("speed parameter does not support property %q (allowed: playbackRate, speed)", prop)
		}
		if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
			return fmt.Errorf("speed min (%v) cannot be greater than max (%v)", *p.Min, *p.Max)
		}
		if p.Default != nil {
			defNum := toFloat64(p.Default, math.NaN())
			if !math.IsNaN(defNum) {
				if p.Min != nil && defNum < *p.Min {
					return fmt.Errorf("speed default value (%v) cannot be less than minimum (%v)", defNum, *p.Min)
				}
				if p.Max != nil && defNum > *p.Max {
					return fmt.Errorf("speed default value (%v) cannot be greater than maximum (%v)", defNum, *p.Max)
				}
			}
		}
	case "enum":
		if len(p.Options) == 0 {
			return fmt.Errorf("enum parameter must have at least one allowed option")
		}
		for _, opt := range p.Options {
			if strings.TrimSpace(opt) == "" {
				return fmt.Errorf("enum options cannot contain empty strings")
			}
		}
	case "number":
		if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
			return fmt.Errorf("numeric min (%v) cannot be greater than max (%v)", *p.Min, *p.Max)
		}
		if prop == "opacity" {
			if p.Min != nil && *p.Min < 0 {
				return fmt.Errorf("opacity min (%v) cannot be less than 0", *p.Min)
			}
			if p.Max != nil && *p.Max > 1.0 {
				return fmt.Errorf("opacity max (%v) cannot exceed 1.0", *p.Max)
			}
		}
		if p.Default != nil {
			defNum := toFloat64(p.Default, math.NaN())
			if !math.IsNaN(defNum) {
				if p.Min != nil && defNum < *p.Min {
					return fmt.Errorf("numeric default value (%v) cannot be less than minimum (%v)", defNum, *p.Min)
				}
				if p.Max != nil && defNum > *p.Max {
					return fmt.Errorf("numeric default value (%v) cannot be greater than maximum (%v)", defNum, *p.Max)
				}
			}
		}
	}

	return nil
}

// ExtractWidgetManifest parses an embedded sva-widget-manifest script tag from raw SVG markup.
func ExtractWidgetManifest(rawSVG string) *WidgetManifest {
	re := regexp.MustCompile(`(?i)<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["'][^>]*>([\s\S]*?)</script>`)
	matches := re.FindStringSubmatch(rawSVG)
	if len(matches) < 2 {
		return nil
	}
	rawJSON := strings.TrimSpace(matches[1])
	if rawJSON == "" || rawJSON == "{}" {
		return nil
	}
	var manifest WidgetManifest
	if err := json.Unmarshal([]byte(rawJSON), &manifest); err != nil {
		return nil
	}
	if len(manifest.Parameters) == 0 {
		return nil
	}
	return &manifest
}
