# Stage 16B: Widget Parameter Manifest Schema Specification

## 1. Schema Overview

The SVG Animator Widget Manifest is an open, versioned JSON schema embedded directly within exported SVG files or stored in `.sva` project archives.

- **Schema Identifier:** `sva-widget-manifest`
- **Schema Version:** `1`
- **Embedding Format:** `<script type="application/json" id="sva-widget-manifest">...</script>` placed inside `<svg>...</svg>`.

---

## 2. Type Definition

### Go Definition (`pkg/engine/scene/model.go`)

```go
type WidgetParameter struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"`        // "color" | "number" | "boolean" | "text" | "enum" | "speed"
    Default     any      `json:"default"`
    Target      string   `json:"target"`      // Semantic target (e.g. "fluidLevel", "statusDot", "root")
    Property    string   `json:"property"`    // "fill" | "stroke" | "color" | "visibility" | "opacity" | "height" | "textContent" | "playbackRate" | "state"
    Min         *float64 `json:"min,omitempty"`
    Max         *float64 `json:"max,omitempty"`
    Options     []string `json:"options,omitempty"` // For "enum" type
    Description string   `json:"description,omitempty"`
}

type WidgetManifest struct {
    SchemaVersion int               `json:"schemaVersion"`
    Parameters    []WidgetParameter `json:"parameters"`
}
```

---

## 3. Supported Parameter Types and Validation Rules

| Type | Data Type | Validation Rule | Supported Properties |
| :--- | :--- | :--- | :--- |
| `color` | `string` | Safe hex (`#RGB`, `#RRGGBB`, `#RRGGBBAA`), `rgb()`, `rgba()`, `hsl()`, or named CSS color. Script/expression injection rejected. | `fill`, `stroke`, `color` |
| `number` | `number` | Finite number. Clamped or rejected against `min` and `max` constraints if defined. | `height`, `width`, `x`, `y`, `opacity`, custom attributes |
| `speed` | `number` | Finite non-negative float (`rate >= 0.0`). Controls instance playback rate. | `playbackRate`, `speed` |
| `boolean` | `boolean` | Coerced or strict boolean. | `visibility`, `display` |
| `enum` | `string` | String strictly matching one of the values in the declared `options` array. | `state`, `operatingState`, custom attributes |
| `text` | `string` | Arbitrary string applied safely via `element.textContent`. Direct HTML/script injection blocked. | `textContent`, `text` |

---

## 4. Canonical Manifest Example

```json
{
  "schemaVersion": 1,
  "parameters": [
    {
      "name": "operatingState",
      "type": "enum",
      "default": "running",
      "target": "root",
      "property": "state",
      "options": ["running", "paused", "stopped", "alarm"],
      "description": "Operating lifecycle state of the unit"
    },
    {
      "name": "statusColor",
      "type": "color",
      "default": "#22c55e",
      "target": "statusDot",
      "property": "fill",
      "description": "Status LED beacon accent color"
    },
    {
      "name": "fillLevel",
      "type": "number",
      "default": 65,
      "target": "fluidLevel",
      "property": "height",
      "min": 0,
      "max": 180,
      "description": "Tank fluid volume height in pixels"
    },
    {
      "name": "speed",
      "type": "speed",
      "default": 1.0,
      "target": "root",
      "property": "playbackRate",
      "min": 0.1,
      "max": 3.0,
      "description": "Animation playback speed rate"
    },
    {
      "name": "statusText",
      "type": "text",
      "default": "RUNNING",
      "target": "statusLabel",
      "property": "textContent",
      "description": "Badge text displayed on front panel"
    },
    {
      "name": "beaconVisible",
      "type": "boolean",
      "default": true,
      "target": "alarmBeacon",
      "property": "visibility",
      "description": "Top warning beacon visibility toggle"
    }
  ]
}
```
