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

---

## 5. Authoritative Object Property Eligibility Catalog

To maintain strict contract integrity between authoring, Go scene models, export pipelines, and runtime execution, property exposure is strictly validated against the authoritative `ObjectPropertyCatalog` in `pkg/engine/scene/widget.go`:

| Target Type | Supported Properties | Type | Default Value | Notes / Description |
| :--- | :--- | :--- | :--- | :--- |
| **`node`** | `fill` | `color` | Current shape fill (`#3b82f6`) | Background fill colour |
| | `stroke` | `color` | Current shape stroke (`#1d4ed8`) | Border outline colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Overall node opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| | `text` | `text` | Current node text label | Primary text label |
| **`flow`** | `stroke` | `color` | Current path colour (`#38bdf8`) | Connector path and particle colour |
| | `speed` | `speed` | `1.0` (range: `0.1`–`10.0`) | Flow animation rate multiplier |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Flow track opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`glow`** | `color` | `color` | Current glow colour (`#22c55e`) | Radial glow accent colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Glow intensity opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`ripple`** | `color` | `color` | Current wave colour (`#38bdf8`) | Ripple wave ring colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Wave pulse opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`image`** | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Image layer opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`group`** | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Composite group opacity |
| | `visibility` | `boolean` | `true` | Composite visibility toggle |
| **`root`** | `playbackRate` | `speed` | `1.0` (range: `0.1`–`5.0`) | Master widget playback speed |
| | `visibility` | `boolean` | `true` | Master widget container visibility |
| | `operatingState` | `enum` | `"running"` (`running`, `paused`, `stopped`) | High-level lifecycle state |

> [!IMPORTANT]
> Ineligible properties (e.g. attempting to bind `speed`, `height`, or `width` on a `node`) are strictly rejected at the Go engine boundary with: `property "%s" is not eligible for target "%s" of type "%s"`.

---

## 6. Unique Binding Contract

To prevent ambiguous states and duplicate UI controls, SVG Animator Studio enforces the **Unique Binding Contract**:

$$\text{One Object ID} + \text{One Controllable Property} + \text{One Scope} \le 1\text{ External Parameter Binding}$$

### Core Contract Rules:
1. **Collision Rejection on Add:** Attempting to expose a property on a target that is already bound (e.g. adding `flow1c` when `flow-1.stroke` is already bound to `flow1color`) returns an explicit collision error:
   `property "%s" on target "%s" is already bound to parameter "%s"`
2. **Multi-Property Exposure Permitted:** An object can expose multiple *different* eligible properties simultaneously (e.g. `flow-1` can expose both `stroke` as `flowColor` and `speed` as `flowSpeed`).
3. **Explicit Renaming (`RenameParameter`):** Renaming a parameter retains its target, property, type, and defaults while updating the public interface name.
4. **Single-Current-Value Runtime:** External parameters represent public controllable handles. No duplicate bindings or ambiguous overrides exist at runtime.

---

## 7. Global vs Local Playback Speed Coexistence

Global widget playback rate and local flow animation speed independently coexist. Setting the master playback rate or an individual flow's speed dynamically updates the element's actual playback according to the authoritative formula:

$$\text{Effective Speed} = \text{Master Playback Rate} \times \text{Targeted Flow Speed}$$

Both Web Animations API (`animation.playbackRate`) and SMIL declarative animations (`dur = baseDur / effectiveSpeed`) scale proportionally without drift or crosstalk.

---

## 8. Migration, Conflict Detection & Resolution

Legacy projects loaded from database or `.sva` files that contain ambiguous duplicate bindings from earlier versions are handled non-destructively:

1. **Non-Destructive Ingestion:** Conflicting parameters are preserved upon scene load without silent deletion.
2. **Conflict Detection:** The engine detects all duplicate `(target, property)` tuples via `DetectParameterConflicts()`.
3. **UI Conflict Resolution:** The Widget Properties drawer displays a visible warning banner listing all conflicting parameters, with one-click resolution buttons:
   `Keep "[paramName]"`
4. **Export Safety Blocking:** Exporting a widget package via `BuildWidgetPackageZip` is strictly blocked if unresolved conflicts exist, reporting:
   `cannot export widget package with conflicting parameter bindings: target.property (paramA, paramB). Please resolve duplicate bindings before exporting.`

---

## 9. Final Property Contract, Auto-Derived Naming & Effect Targeting Isolation

### A. Stable Internal Object Identity vs Unique Prefixed Object Names
1. **Stable Internal IDs:** Every object retains an immutable Go-owned ID (`StringID`) used for scene graph references, selection state, undo/redo history, and persistence.
2. **Unique User-Editable Object Names:** Objects enforce unique names within their type category, using fixed type prefixes:
   - `node-` (e.g. `node-gateway`, `node-database`)
   - `flow-` (e.g. `flow-border`, `flow-telemetry`)
   - `glow-` (e.g. `glow-status`, `glow-alarm`)
   - `ripple-` (e.g. `ripple-sensor`, `ripple-pulse`)
   - `image-` (e.g. `image-backdrop`, `image-overlay`)
   Uniqueness is validated by the authoritative Go engine via `ValidateObjectName(type, name, excludeID)` across object creation, duplication, import, and inline renaming. Empty strings `""` are permitted to clear explicit names and revert to default IDs.

### B. Automatic Parameter Name Generation
Parameter names are generated deterministically by `GenerateParameterName(objectName, property, targetType)`:
- Form: `<objectName>-<suffix>` (e.g. `node-gateway-fill`, `flow-border-color`, `flow-border-speed`, `glow-status-color`, `ripple-sensor-color`).
- Users never have to manually invent arbitrary parameter identifiers or pick variable types; types, limits, and defaults are derived authoritatively.
- Renaming an object automatically updates all bound public parameter names across the scene and synchronizes the UI and manifest.

### C. Effect Targeting Isolation in Runtime (`svg-widget.js`)
To ensure that parameter changes never degrade the visual structure of exported SVG graphics:
1. **Flow Targeting:**
   - Recolouring `color` or `stroke` changes only the moving payload (`.flow-payload-group` particles or dashes).
   - Motion guide track paths (`.flow-track-path` authored with `stroke="none"`) are strictly preserved as `stroke="none"` and never painted.
   - Individual flow speed controls scale connector animation duration without affecting master transport.
2. **Glow Targeting:**
   - Recolouring `color` targets radial gradient stops (`<radialGradient>`) and illuminated core geometry.
   - Transparent hit-test rectangles (`.glow-hit-target` with `fill="transparent"`) remain untouched and strictly transparent.
3. **Ripple Targeting:**
   - Recolouring `color` and `opacity` targets visible wave crests (`.ripple-wave`).
   - Transparent hit-test rectangles (`.ripple-hit-target`) and structural shadow rings (`.ripple-shadow-ring` with `stroke="#000000"`) are strictly preserved.
4. **Node & Image Targeting:**
   - Targets only attributes that can be faithfully altered at runtime (`fill`, `stroke`, `opacity`, `visibility`, `text`).

### D. Simplified Compact Cards Authoring UI
When an object is selected in SVG Animator Studio, the Widget drawer presents a compact card for each eligible property:
- **Checkbox:** `[ ] Expose` toggles the parameter binding immediately.
- **Badge:** Displays the auto-derived public parameter name (e.g. `node-gateway-fill`).
- **Default Value Editor:** Allows fine-tuning the initial authored default.
- **Duplicate Prevention:** If a property is already bound, the card offers an edit input and `✕ Disable` button, ensuring zero duplicate bindings.
