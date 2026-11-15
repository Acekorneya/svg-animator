package scene

import (
	"encoding/json"
	"fmt"
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
		reExistingManifest := regexp.MustCompile(`(?i)<script\b[^>]*\bid=["']sva-widget-manifest["']`)
		if !reExistingManifest.MatchString(transformed) {
			manifestBytes, err := json.Marshal(opts.Manifest)
			if err == nil {
				manifestTag := fmt.Sprintf("\n  <script type=\"application/json\" id=\"sva-widget-manifest\">%s</script>\n", string(manifestBytes))
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
