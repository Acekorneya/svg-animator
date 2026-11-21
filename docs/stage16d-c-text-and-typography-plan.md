# Stage 16D-C — Text and Typography Architecture and Development Plan

**Status:** **Stage 16D-C — CORRECTIONS IMPLEMENTED, AWAITING USER ACCEPTANCE**  
**Implementation Authorisation:** **ALL FOUR MILESTONES (T1–T4) + CONSOLIDATED VISUAL CORRECTIONS AUTHORISED & IMPLEMENTED**  
**Initial Implementation Commit:** `f07272f`  
**Consolidated Corrections Commit:** `ea1540e`  
**Project:** SVG Animator Studio  
**Stage:** Stage 16D-C (Text and Typography)  
**Prerequisite Baseline:** Commit `735c3f5` (Stage 16D-B Final Closeout; M1 Flow Lite `a3a7d24`, M2 Radar Ping Lite `ad814bb`, M3 Gauge / Level Lite `a086170`, and M4 Oscilloscope Lite & 4-Threshold Gauge Enhancement `698e6cf` all COMPLETE AND ACCEPTED)  
**Visual Showcase URL:** `http://192.168.100.40:8088/text-lite-demo.html`  
**Studio URL:** `http://192.168.100.40:8088/`  
**Engineering Guidance Reference:** [`docs/architecture/svg-animation-engineering-guidance.md`](architecture/svg-animation-engineering-guidance.md)  
**Test Lab Comparative Evidence:** [`testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md`](../testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md) (BookStack Book 468, Page 470; live viewer at `http://192.168.100.40:8093/`)

---

## 1. Current Implemented Text Capabilities

A direct audit of the current SVG Animator Studio repository (`pkg/engine/geometry/shapes.go`, `pkg/engine/scene/model.go`, `pkg/engine/scene/widget.go`, `internal/web/static/js/nodes.js`, `internal/web/static/js/lite-effects.js`, and `internal/web/static/js/svg-widget.js`) shows that text currently exists only as a secondary label attached to other objects rather than as a dedicated, typographically flexible authoring and animation capability:

1. **Diagram Node Labels (`scene.Node` in `pkg/engine/scene/model.go` & `pkg/engine/geometry/shapes.go`):**
   - Each diagram node has a `Text` string (with `|` used as a manual line separator), an optional `Subtext` string, a `TextColor` (`#e6edf3`), and a single `FontSize` (`15 px` default).
   - `ComputeTextPlacement` in `pkg/engine/geometry/shapes.go` (lines 210–286) and `renderNodeElement` in `internal/web/static/js/nodes.js` (lines 293–320) hardcode a single system sans-serif stack (`-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`) at fixed weight `600` for main lines, and a generic `monospace` font at `78%` size for `Subtext`.
   - Horizontal alignment is fixed to centred (`text-anchor="middle"`) for all shapes except the `axes` chart shape (`text-anchor="start"`). Vertical alignment is fixed to the vertical midpoint of the shape.
   - There is a `"textbox"` entry in `BuiltinShapes` (`"Floating Text Label Only"`), which simply suppresses the node's background rectangle while retaining the same fixed-font, centre-locked label renderer.
   - Diagram nodes have no `Rotation` field, no padding control, no font-family selector, no font-weight selector, no letter-spacing control, no curved text (`<textPath>`), and no text-specific animations.
2. **Data Flow Midpoint Badges (`scene.Flow` in `internal/web/static/js/flows.js`):**
   - Flows optionally render a small static midpoint `<text>` badge for the connector's `Label`. It has no typography, alignment, or animation controls.
3. **Gauge / Level Lite Numeric Readouts (`kind: "gauge-lite"` in `internal/web/static/js/lite-effects.js`):**
   - Renders an optional static numeric value + unit string (`85%`, `2950 RPM`) and short label (`CPU`, `Spindle`) using fixed system sans-serif typography.
4. **Stage 16C Widget Runtime Text Replacement (`internal/web/static/js/svg-widget.js`):**
   - `applyPropertyToElement` (`case 'text'` / `case 'textContent'`, lines 1003–1017) locates the first `<text>` element inside a target node and overwrites `textContent = String(value)` immediately.
   - It does not split multi-line text, does not measure rendered text width, does not recalculate ticker or typewriter durations, and has no cycle-boundary pending-message handover for active text animations.

---

## 2. Reusable Architecture

The existing Go/WASM-authoritative architecture and Stage 16C/16D widget infrastructure provide a strong foundation that can be reused directly without architectural replacement:

1. **Authoritative Go/WASM Low-Load Object Pipeline (`scene.LiteEffect` / `SelectionLite`):**
   - `scene.LiteEffect` in `pkg/engine/scene/model.go` and `pkg/engine/scene/lite_effect.go` already supports center-based `(X, Y)` percentage coordinates, pixel `Width` and `Height`, `CornerRadius`, `Rotation` (`0..360°`), `Color`, `BgColor`, `StrokeWidth`, `Opacity`, `Speed`, `Direction`, `PlaybackMode`, `LabelText`, `Visible`, and `Locked`.
   - `LiteEffect` already participates in Go `Scene` state, `HistoryManager` undo/redo (`maxDepth: 50` with `DeepCopyLiteEffect`), `.sva` binary archive packaging, SQLite `schema_json` (`v1.2.0`) persistence, the unified `LayerManager` (`layers-ui.js`), persistent object groups (`ObjectGroups`), canvas selection boxes (`#nodeSelectionBox`), and both the Left Drawer (`#acc-lite-effects`) and Floating Context Editor (`#floatingContextEditor`).
2. **Whole-Object Rotation & Hit-Testing (`lite-effects.js`):**
   - `LiteEffects._resolveGeometry(le)` and `renderLiteElement(le)` (line 832) already apply `transform="rotate(rot, cx, cy)"` at the outer `<g id="lite-{id}">` level, rotating the entire object (hit target, container, guide paths, and payload) around its centre `(cx, cy)` and preserving rotation across save/reload, standalone SVG export, and widget export.
3. **Stage 16A–16C Widget Isolation & Runtime Engine (`pkg/engine/scene/widget.go` & `internal/web/static/js/svg-widget.js`):**
   - `IsolateSVGWidget` in Go and `SVGWidget.mount()` in JS automatically rewrite internal SVG `<defs>` IDs (`<clipPath>`, `<path>`, `@keyframes`) with per-instance prefixes (`<instanceId>__...`) so multiple mounted widgets never collide.
   - `SVGWidget.mount()` already provides an instance-scoped closure with `setParameter()`, `setParameters()`, `resetParameters()`, `pause()`, `play()`, `destroy()`, and an `IntersectionObserver` (`visibilityObserver`) that suspends decorative animations off-screen while continuing to accept incoming data updates.
4. **Permanent SVG Animation Test Lab & Engineering Guidance (`cmd/anim-test-lab/`, `pkg/animlab/`, `docs/architecture/svg-animation-engineering-guidance.md`):**
   - Provides the validated benchmark harness (`measurementSpecVersion: 2.0.0`, real Chromium CDP + `/proc` process-tree CPU attribution) and the live viewer at `http://192.168.100.40:8093/` for evaluating any new text animation method if a genuine scaling or performance question arises.

---

## 3. Proposed Feature Scope

Stage 16D-C introduces **Text Box Lite (`kind: "text-lite"`)** as a dedicated Low-Load Text and Typography object family in SVG Animator Studio, alongside its Stage 16C widget runtime controller in `svg-widget.js`.

### In-Scope Capabilities
1. **Core Text Box Object (`kind: "text-lite"`):**
   - Five focused container/layout styles:
     1. **Floating Text (`variant: "none"`):** No visible background or border (transparent hit-target rectangle retained for easy selection and dragging in Studio).
     2. **Rectangle (`variant: "rect"`):** Sharp-cornered rectangular box with independent fill, border colour, and border thickness.
     3. **Rounded Rectangle (`variant: "rounded"`):** Rectangular box with authorable corner radius (`cornerRadius`).
     4. **Pill / Capsule (`variant: "pill"`):** Stadium capsule where `rx = ry = min(width, height) / 2`.
     5. **Curved Text on Arc (`variant: "curved"`):** Text rendered along a true SVG `<path>` via `<text><textPath href="#..."></textPath></text>`, with upward or downward convex arc curvature and optional visible/hidden guide curve.
2. **Complete Appearance & Typography Controls:**
   - Text content (`labelText` / `text`), background colour (`bgColor`) and background opacity/transparency (`bgOpacity`), text colour (`color`), border colour (`borderColor`), border thickness (`strokeWidth`, `0..24 px`, where `0` disables the border), corner radius (`cornerRadius`), curated font family (`fontFamily`, 10 curated stacks), font size (`fontSize`, `8..120 px`), font weight (`fontWeight`: `400`, `500`, `600`, `700`), letter spacing (`letterSpacing`, `-2..20 px`), horizontal alignment (`textAlign`: `left`, `center`, `right`), vertical alignment (`verticalAlign`: `top`, `middle`, `bottom`), internal padding (`padding`, `0..60 px`), whole-object opacity (`opacity`), width (`width`), height (`height`), and whole-object rotation (`rotation`, `0..360°`).
3. **Static & Animated Text Operating Modes (`textAnim`):**
   - **Static (`"none"`):** Immediate in-place text update on `setParameter()` with zero idle animation loops.
   - **Ticker (`"ticker-h"` horizontal and `"ticker-v"` vertical):** Distance-per-second scrolling (`scrollSpeed` in SVG units/px per second) where travel distance and cycle duration adapt automatically to the rendered text length and container dimensions.
   - **Typewriter (`"typewriter"`):** Character-per-second (`typeSpeed` in grapheme clusters per second) progressive reveal supporting `once`, `hold`, `delete` (type, hold, delete backwards), and `loop` (repeating type/hold/delete) modes with Unicode grapheme-safe segmentation.
   - **Approved Declarative & Lightweight Effects:** `fade` (fade in/out/loop), `flash` (attention blink/pulse), `slide` (slide in/out), `blur-in` (blur to clear), `blur-out` (clear to blur), `scramble` (cipher/character scramble to clear text), and `overtype` (in-place progressive character replacement from old string to new string).
4. **Deterministic Cycle-Boundary Pending-Message Handover:**
   - Single-slot `pendingText` buffer per widget instance: incoming `setParameter()` updates during an active animation never interrupt the current cycle; multiple updates overwrite `pendingText` without queueing; at the exact cycle boundary, the widget adopts `pendingText`, re-measures geometry/duration, and starts the new cycle.

### Out-of-Scope Boundaries
- No rich-text HTML/foreignObject word-processor features (no mixed inline bold/italic spans, bulleted lists, or HTML CSS layout engines).
- No hundreds of uncurated system fonts or bundling of proprietary commercial font binaries.
- No freeform multi-bezier text-on-path drawing editor or concave/tightly pinched S-curves in the initial implementation.
- No independently spinning text inside a stationary container (rotation applies to the whole text object).
- No message-priority interruption queue (urgent overrides belong to the consuming WebGUI).
- No Stage 16E group-level data bindings or Digital Twin presenter workflows.

---

## 4. Core Text Object Design

### 4.1 Architectural Ownership: Extending `scene.LiteEffect` (`kind: "text-lite"`)
Extending `scene.LiteEffect` with `kind: "text-lite"` (rather than creating a sixth top-level scene collection or overloading flowchart `DiagramNodes`) is recommended for three reasons:
1. **Zero Boilerplate Duplication:** `LiteEffect` already has full Go/WASM authority, undo/redo deep-copying, `.sva` and SQLite persistence, unified layer ordering, group membership, rotation, and Stage 16B/16C widget property exposure (`<name>-lite-<property>`).
2. **Clean Separation from Flowchart Nodes:** `DiagramNodes` (`scene.Node`) carry connector topology attachments (`fromNodeId` / `toNodeId` in `DataFlows`), flowchart shape presets, and two-line `text | subtext` conventions. Keeping `DiagramNodes` intact prevents any regression in network topology diagrams while giving `text-lite` purpose-built typography, container, curve, and animation controls.
3. **Consistent Widget Runtime & Off-Screen Suspension:** All `LiteEffect` objects already integrate with `svg-widget.js`'s `IntersectionObserver` suspension lifecycle and `ObjectPropertyCatalog["lite"]`.

### 4.2 Authoritative Go Model Extensions (`pkg/engine/scene/model.go`)
The following backward-compatible `omitempty` fields are proposed on `scene.LiteEffect`:

| Field | Go Type | JSON Tag | Default (`text-lite`) | Purpose & Validation Range |
|---|---|---|---|---|
| `Kind` | `string` | `json:"kind"` | `"text-lite"` | Discriminator for Text Box Lite |
| `Variant` | `string` | `json:"variant,omitempty"` | `"rounded"` | Container style: `"none"`, `"rect"`, `"rounded"`, `"pill"`, `"curved"` |
| `LabelText` | `string` | `json:"labelText,omitempty"` | `"System Ready"` | Displayed text content (`0..2000` characters) |
| `FontFamily` | `string` | `json:"fontFamily,omitempty"` | `"modern-sans"` | Curated font family key (10 approved keys; see Section 5) |
| `FontSize` | `float64` | `json:"fontSize,omitempty"` | `18.0` | Font size in SVG user units (`8.0 .. 120.0`) |
| `FontWeight` | `string` | `json:"fontWeight,omitempty"` | `"600"` | Font weight: `"400"`, `"500"`, `"600"`, `"700"` |
| `LetterSpacing` | `float64` | `json:"letterSpacing,omitempty"` | `0.0` | Additional tracking in SVG px (`-2.0 .. 20.0`) |
| `TextAlign` | `string` | `json:"textAlign,omitempty"` | `"center"` | Horizontal alignment: `"left"`, `"center"`, `"right"` |
| `VerticalAlign` | `string` | `json:"verticalAlign,omitempty"` | `"middle"` | Vertical alignment: `"top"`, `"middle"`, `"bottom"` |
| `Padding` | `float64` | `json:"padding,omitempty"` | `12.0` | Interior container padding in SVG px (`0.0 .. 60.0`) |
| `Color` | `string` | `json:"color"` | `"#f8fafc"` | Text fill colour |
| `BgColor` | `string` | `json:"bgColor,omitempty"` | `"#0f172a"` | Container background fill colour |
| `BgOpacity` | `*float64` | `json:"bgOpacity,omitempty"` | `0.85` (`0.0` for `"none"`) | Container background opacity (`0.0 .. 1.0`, pointer so `0.0` transparent is preserved) |
| `BorderColor` | `string` | `json:"borderColor,omitempty"` | `"#38bdf8"` | Container border stroke colour (distinct from text `Color`) |
| `StrokeWidth` | `float64` | `json:"strokeWidth"` | `1.5` (`0.0` for `"none"`) | Container border thickness (`0.0 .. 24.0 px`; `0` = no border) |
| `CornerRadius` | `float64` | `json:"cornerRadius"` | `8.0` | Corner radius for `"rounded"` (`0.0 .. 100.0 px`) |
| `Rotation` | `float64` | `json:"rotation"` | `0.0` | Whole-object rotation angle in degrees (`0.0 .. 360.0`) |
| `Curvature` | `float64` | `json:"curvature,omitempty"` | `35.0` | Convex arc sagitta percentage for `"curved"` (`-100 .. +100`, where `+` is upward arch and `-` is downward smile) |
| `StartOffset` | `float64` | `json:"startOffset"` | `50.0` | Arc anchor position percentage (`0 .. 100%`) for `"curved"` |
| `ShowGuide` | `bool` | `json:"showGuide"` | `false` | Show/hide visible arc guide path when `variant == "curved"` |
| `TextAnim` | `string` | `json:"textAnim,omitempty"` | `"none"` | Animation mode: `"none"`, `"ticker-h"`, `"ticker-v"`, `"typewriter"`, `"fade"`, `"flash"`, `"slide"`, `"blur-in"`, `"blur-out"`, `"scramble"`, `"overtype"` |
| `ScrollSpeed` | `float64` | `json:"scrollSpeed,omitempty"` | `90.0` | Ticker linear speed in SVG units/px per second (`10.0 .. 600.0 px/s`) |
| `TypeSpeed` | `float64` | `json:"typeSpeed,omitempty"` | `14.0` | Typewriter / overtype / scramble rate in characters per second (`1.0 .. 120.0 chars/s`) |
| `HoldDelay` | `float64` | `json:"holdDelay,omitempty"` | `1.5` | Hold pause at end of typewriter/slide/fade cycle in seconds (`0.0 .. 30.0 s`) |
| `TypeMode` | `string` | `json:"typeMode,omitempty"` | `"hold"` | Typewriter sub-mode: `"once"`, `"hold"`, `"delete"`, `"loop"` |
| `TickerGap` | `string` | `json:"tickerGap,omitempty"` | `"full-exit"` | Ticker exit rule: `"full-exit"` (default: scrolls completely out before next cycle) or `"continuousloop"` (optional future seamless wrap) |

### 4.3 Container Geometry, Alignment & Whole-Object Rotation
- **Coordinate System & Bounds:**
  - Consistent with all other `LiteEffect` objects, `(X, Y)` stores the center of the text box in canvas percentage coordinates (`0..100%`), while `Width` (`24..1280 px`, default `220 px`) and `Height` (`16..720 px`, default `56 px`) define its unrotated bounding box `[x, x + w] × [y, y + h]`, where `x = cx - w/2` and `y = cy - h/2`.
- **Horizontal & Vertical Alignment Mathematics (Straight Containers `none`, `rect`, `rounded`, `pill`):**
  - Let `pad = clamp(padding, 0, min(w, h)/2 - 2)`.
  - **Horizontal (`textAlign`):**
    - `"left"`: `textX = x + pad`, `text-anchor="start"`
    - `"center"`: `textX = cx`, `text-anchor="middle"`
    - `"right"`: `textX = x + w - pad`, `text-anchor="end"`
  - **Vertical (`verticalAlign`):**
    - Using explicit SVG `dominant-baseline` combined with deterministic `y` positioning:
    - `"top"`: `textY = y + pad + fontSize * 0.82` (or `dominant-baseline="hanging"` with `y + pad`)
    - `"middle"`: `textY = cy + fontSize * 0.35` (calibrated optical cap-height centering consistent with Gauge Lite readouts across Chromium, Firefox, and Safari)
    - `"bottom"`: `textY = y + h - pad - fontSize * 0.15`
- **Interior Clipping Region (`<clipPath id="lite-text-clip-{id}">`):**
  - Every boxed container (`rect`, `rounded`, `pill`) and ticker animation defines an interior `<clipPath>` matching the container's inner boundary (inset by `strokeWidth / 2`), ensuring that long strings or scrolling tickers are cleanly clipped to the container shape (including rounded corners and pill capsules).
- **Whole-Object Rotation (`0..360°`):**
  - Applied to the root `<g id="lite-{id}" transform="rotate(rot, cx, cy)">`.
  - Because rotation is applied to the outer group, the background box, border, clipping path, curved guide path, and text all rotate together as a single rigid body.
  - Hit-testing in `LiteEffects.findAtPct(xPct, yPct)` transforms the query point `(px, py)` into the object's local unrotated coordinate frame by rotating `(px - cx, py - cy)` by `-rot` radians before testing against `[-w/2, +w/2] × [-h/2, +h/2]`. This ensures click selection and dragging remain exact at any rotation angle (`0..360°`).

---

## 5. Font Handling and Curated Font Families

### 5.1 Curated 10-Font Catalog
Rather than exposing hundreds of fragile OS-specific font names, Studio will provide **10 curated font choices** covering every category requested by the user, each backed by a tested multi-platform CSS `font-family` fallback stack:

| Key (`fontFamily`) | Studio Label | Category | CSS `font-family` Stack | Platform Availability & Visual Behaviour |
|---|---|---|---|---|
| `modern-sans` | **Modern Sans (Default)** | Modern Sans-Serif | `system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif` | Native across Windows (`Segoe UI`), macOS/iOS (`SF Pro`), Linux (`Roboto` / `Cantarell` / `Ubuntu`). Clean UI & telemetry legibility. |
| `humanist-sans` | **Humanist Sans** | Humanist Sans-Serif | `'Trebuchet MS', 'Gill Sans', 'Gill Sans MT', 'Lucida Grande', 'DejaVu Sans', sans-serif` | Pre-installed on Windows & macOS (`Trebuchet MS` / `Gill Sans`); falls back cleanly to `DejaVu Sans` on Linux. Warm, open counters. |
| `Geometric-sans` (`geometric-sans`) | **Geometric / Display Sans** | Geometric Sans-Serif | `'Century Gothic', 'Avenir Next', Avenir, 'URW Gothic', 'Liberation Sans', sans-serif` | Wide circular geometric letterforms suitable for headers and control panels. |
| `formal-serif` | **Formal Serif (Times)** | Formal Serif | `'Times New Roman', Times, 'Liberation Serif', 'Nimbus Roman', serif` | Universal across Windows/macOS (`Times New Roman`) and Linux (`Liberation Serif` is metric-compatible with Times New Roman). |
| `traditional-serif` | **Traditional Readable Serif** | Traditional Serif | `Georgia, 'Palatino Linotype', Palatino, 'Book Antiqua', 'URW Bookman', serif` | Pre-installed on Windows & macOS (`Georgia` / `Palatino`); high x-height designed for on-screen readability. |
| `tech-mono` | **Technical Monospace** | Technical Monospace | `'JetBrains Mono', 'Fira Code', 'Cascadia Mono', Consolas, 'Liberation Mono', 'Courier New', monospace` | Tabular fixed-width glyphs (`Consolas` on Windows, `SF Mono`/`Menlo` on macOS, `Liberation Mono` on Linux). Essential for counters, IP addresses, and `scramble` / `overtype` effects. |
| `condensed-display` | **Condensed Display** | Condensed Display | `'Arial Narrow', 'Roboto Condensed', 'Helvetica Neue Condensed', Impact, 'Liberation Sans Narrow', sans-serif` | High-density narrow title lettering for compact industrial badges and alarm headers. |
| `casual-lettering` | **Casual Lettering** | Casual Lettering | `'Comic Sans MS', 'Chalkboard SE', 'Comic Neue', 'TSCu_Comic', cursive, sans-serif` | Informal rounded hand-printed lettering (distinct from connected cursive script). |
| `informal-hand` | **Informal Handwriting** | Informal Handwriting | `'Segoe Print', 'Bradley Hand', 'Marker Felt', 'Lucida Handwriting', cursive` | Natural pen/marker handwriting (`Segoe Print` on Windows, `Bradley Hand` / `Marker Felt` on macOS). |
| `classic-script` | **Classic Flowing Script / Calligraphy** | Formal Script / Calligraphy | `'Palace Script MT', 'Edwardian Script ITC', 'Snell Roundhand', 'Apple Chancery', 'URW Chancery L', 'Z003', 'Monotype Corsiva', cursive` | **User-requested traditional decorative flowing script** for formal or fantasy-style artwork (`Snell Roundhand` / `Apple Chancery` on macOS, `Edwardian Script` / `Monotype Corsiva` on Windows, `URW Chancery L` / `Z003` on Linux). |

### 5.2 Portability, Packaging & Licensing Analysis
- **Zero Proprietary Font Binary Bundling Required by Default:**
  - All 10 stacks above use standard OS-resident fonts plus metric-compatible open-source equivalents on Linux (`Liberation`, `DejaVu`, `URW Chancery L` / `Z003`).
  - No proprietary `.ttf` / `.woff2` files (such as Microsoft's `Edwardian Script ITC` or Apple's `Snell Roundhand`) will be copied into the repository or embedded into exported widgets, avoiding copyright and redistribution issues.
- **Special Consideration for `classic-script` (Formal Calligraphy / Fantasy Script):**
  - While Windows (`Monotype Corsiva` / `Gabriola`) and macOS (`Snell Roundhand` / `Apple Chancery`) ship with flowing script fonts, minimal Linux containers (including headless Chromium servers without the `fonts-urw-base35` package) sometimes map generic `cursive` to a plain sans-serif fallback if no script font is installed.
  - **Optional Open-Font Embedding Decision (See Section 13, Decision 2):** If the user wants identical flowing calligraphic glyph rendering across every machine—including bare Linux servers and standalone exported SVGs—we can bundle a single small (~25 KB `.woff2`), SIL Open Font License (OFL 1.1) calligraphic script font (such as *Great Vibes* or *Pinyon Script*) in `internal/web/static/fonts/`. However, consistent with the prompt's guidance, **font file packaging is not made a prerequisite**; the curated multi-platform CSS stack works immediately out of the box.

---

## 6. Curved Text Strategy

### 6.1 True SVG `<textPath>` Along a Parametric Convex Arc
A curved text object (`variant: "curved"`) will **never** put straight `<text>` inside a curved decorative box. Instead, the text itself is laid out along an authoritative SVG `<path id="lite-text-arc-{id}">` using native SVG `<text><textPath href="#lite-text-arc-{id}">...</textPath></text>`:

1. **Authorable Arc Geometry (`Curvature` `-100 .. +100`):**
   - Within the object's local bounding box `[x, x + w] × [y, y + h]`, the chord endpoints are placed at `P0 = (x + pad, cy)` and `P1 = (x + w - pad, cy)`.
   - `Curvature` (`-100 .. +100`, default `+35`) controls the signed vertical sagitta (arch height) `s = (curvature / 100) * (h * 0.45)`:
     - **Upward Convex Arch (`curvature > 0`):** The curve bows upward toward the top of the bounding box (classic rainbow/crest banner).
     - **Downward Convex Arc (`curvature < 0`):** The curve bows downward toward the bottom of the bounding box (smile/lower emblem rocker).
     - **Flat Baseline (`curvature == 0`):** Degenerates smoothly to a straight horizontal baseline `M P0.x cy L P1.x cy`.
   - Using a single symmetric quadratic Bézier `M P0.x P0.y Q cx (cy - 2*s) P1.x P1.y` (or circular arc `A`) guarantees a smooth, convex curve without inflection points or sharp kinks where glyphs collide.
2. **Positioning & Alignment Along the Arc (`textAlign` & `StartOffset`):**
   - `startOffset` (`0% .. 100%`, default `50%`) and `textAlign` control where the text sits along the arc:
     - `"left"`: `startOffset="0%"`, `text-anchor="start"`
     - `"center"`: `startOffset="50%"`, `text-anchor="middle"` (or custom `startOffset` slider `0..100%`)
     - `"right"`: `startOffset="100%"`, `text-anchor="end"`
3. **Optional Guide Curve (`showGuide: true / false`):**
   - When `showGuide: true`, a subtle `<path class="lite-guide-track">` renders the underlying arc in `borderColor` (`stroke-width`, `stroke-opacity="0.35"`).
   - When `showGuide: false` (default), the arc path lives only inside `<defs>` (plus a dashed `.lite-editor-marker` visible only in Studio when selected and stripped on export).

### 6.2 Kerning, Spacing & Legibility on Curves
- **Why Tightly Curved or Concave Text Degrades:**
  - In SVG `<textPath>`, each glyph is rotated independently to match the tangent of the path at the glyph's midpoint. On tight concave curves (where the baseline folds inward toward the cap height), adjacent glyph ascenders tilt toward each other and overlap; on flowing script fonts (`classic-script`), cursive entry/exit connecting strokes break apart when adjacent glyphs rotate by more than a few degrees.
- **Practical Legibility Controls & Documented Limits:**
  - **Bounded Convex Curvature:** Bounding the sagitta to `±45%` of container height keeps glyph-to-glyph tangent rotation gentle (`< 6°` per character at normal font sizes).
  - **Authorable Letter Spacing (`letterSpacing`, `-2 .. +20 px`):** Allows authors to open tracking slightly on upward/downward arcs so letters never pinch together, or keep `letterSpacing: 0` on `classic-script` where cursive joins should stay close.
  - **Explicitly Excluded Curve Types:** Multi-wave S-curves, tight spiral paths, and high-curvature inner-concave rings are excluded from the initial implementation and documented as known limitations (Section 12).

---

## 7. Animation Methods

In accordance with the permanent SVG Animation Engineering Guidance (`docs/architecture/svg-animation-engineering-guidance.md`), text animations are divided into **declarative CSS/SVG animations** (zero per-frame JavaScript) and **lightweight event/step-driven JavaScript controllers** (which touch the DOM only when characters step or when a new message arrives):

### 7.1 Ticker Animation (`"ticker-h"` & `"ticker-v"`) — Distance-Per-Second Model
- **Default Horizontal Ticker Behaviour (`"ticker-h"`):**
  1. Text is rendered on a single unwrapped line inside the container's `<clipPath id="lite-text-clip-{id}">`.
  2. In forward (right-to-left) scrolling, the text starts completely outside the right entrance edge (`translateX(W_box)` with `text-anchor="start"` at `x_left`).
  3. It travels across the container at constant linear speed `V = scrollSpeed` (`px/s`) and exits completely past the left exit edge (`translateX(-W_text)`).
  4. **Even when the text is shorter than the box (`W_text < W_box`), it travels the full distance `D = W_box + W_text` and scrolls completely out of the box before completing the cycle.**
  5. At the cycle completion event (`animationiteration` / `animationend`), the runtime checks `pendingText`; if a new message is waiting, it swaps the text, re-measures `W_text`, updates the travel distance and duration, and starts the next cycle cleanly from the entrance edge.
- **Vertical Ticker Behaviour (`"ticker-v"`):**
  - Scrolls vertically from outside the bottom entrance edge (`translateY(H_box)`) through the top exit edge (`translateY(-H_text)`), or top-to-bottom when `direction == "reverse"`, at speed `V = scrollSpeed` (`px/s`), where `D = H_box + H_text` and `duration = D / V`.
- **Speed & Duration Formula (Distance Per Second, Never Fixed Duration):**
  $$\text{Travel Distance } D = W_{\text{container}} + W_{\text{rendered\_text}}$$
  $$\text{Cycle Duration } T = \frac{D}{V_{\text{scrollSpeed}}}$$
  - **How Measurement Works Across Studio, Standalone SVG, and Widget Runtime:**
    - When mounted in a live browser DOM (Studio preview, standalone SVG with inline `<script>`, or `SVGWidget.mount()`), `textEl.getComputedTextLength()` (or `getBBox().width`) measures the exact rendered pixel width `W_text` of the current string in the active font.
    - If web/system fonts finish loading asynchronously (`document.fonts.ready`), the controller re-measures `getComputedTextLength()` once and updates `--sva-ticker-start`, `--sva-ticker-end`, and `animationDuration = (D / V) + 's'`.
    - For static server-side / initial SVG markup generation before DOM mount, Go estimates `W_text_est = runeCount * fontSize * 0.58 + letterSpacing * max(0, runeCount - 1)` so the initial `@keyframes` duration is reasonable even inside a script-disabled `<img>` preview, and the inline/widget script refines it to the exact `getComputedTextLength()` immediately on load.
  - **No Arbitrary Short Duration Cap or Character Limit:**
    - If a user supplies a 500-character log message at `100 px/s`, `W_text` (~4,500 px) produces a `~47 s` cycle duration and scrolls smoothly from start to finish using compositor-driven CSS `transform: translateX(...)`.

### 7.2 Typewriter Animation (`"typewriter"`) — Characters-Per-Second Model
- **Unicode & Grapheme-Cluster Safety:**
  - Splitting a JavaScript string via `str.split('')` corrupts surrogate pairs (emoji such as `🚀`, `🟢`) and combining diacritics (`e` + `\u0301`).
  - The typewriter controller segments strings using standard `Intl.Segmenter(undefined, { granularity: 'grapheme' })` (with an `Array.from(str)` code-point fallback in older environments), producing an array of true visible grapheme clusters `G = [g_0, g_1, ..., g_{N-1}]`.
- **Rate Model (`typeSpeed` in Characters Per Second):**
  - Each character step takes $\Delta t = \frac{1000}{\text{typeSpeed}}\text{ ms}$.
  - Typing a string of $N$ graphemes takes $T_{\text{type}} = \frac{N}{\text{typeSpeed}}\text{ seconds}$.
  - Deleting backwards takes $T_{\text{delete}} = \frac{N}{\text{typeSpeed} \times 1.5}\text{ seconds}$ (or `1.0x` rate).
- **Four Candidate Typewriter Sub-Modes (`typeMode`):**
  1. **`"once"` (Type Once):** Reveals `0 → N` graphemes at `typeSpeed` chars/s and stops at full text. When a new `setParameter()` message arrives after completion, it immediately begins typing the new message from `0 → N_new`; if a message arrives *while* typing is still in progress, it finishes typing the current string (plus a brief `0.4 s` readability hold) before typing the pending string.
  2. **`"hold"` (Type and Hold):** Types `0 → N` graphemes (`N / typeSpeed` s), holds the completed message for `holdDelay` seconds (`default 1.5 s`), and completes the cycle. If `pendingText` has arrived, it clears and types the new message on the next cycle; if no pending text is waiting, it remains held at full text without needlessly re-typing unchanged telemetry (or repeats only in `"loop"` mode).
  3. **`"delete"` (Type, Hold, Delete Backwards — Single Cycle per Message):** Types `0 → N`, holds for `holdDelay`, deletes backwards `N → 0`, and adopts `pendingText` at the empty boundary.
  4. **`"loop"` (Repeating Type / Hold / Delete Loop):** Continuously loops `Type (0 → N) → Hold (holdDelay) → Delete (N → 0) → Pause (0.3 s)`, adopting `pendingText` at the end of the delete phase (`0` visible characters) when the cycle completes.
- **DOM Efficiency:**
  - Mutates `textEl.textContent = G.slice(0, k).join('')` on a single `<text>` (or `<textPath>`) node at discrete timer intervals (`1..120 Hz` depending on `typeSpeed`), never recreating the widget or allocating per-character `<tspan>` trees.

### 7.3 Evaluation of Other Candidate Text Effects

| Candidate Effect | Mechanism | Practicality & Cost | Recommendation |
|---|---|---|---|
| **1. Fade In/Out (`"fade"`)** | Pure CSS `@keyframes` animating `opacity` (`0 ↔ 1`), with `animationiteration` boundary handover for `pendingText`. | **Very High Practicality / Zero Main-Thread Cost.** Works on all containers and curved text. | **Include in Initial Subset (Milestone T4).** |
| **2. Flash / Blink (`"flash"`)** | Pure CSS `@keyframes` step/pulse opacity (`1 → 0.15 → 1`), matching Stage 16D-A Flash Lite semantics. | **Very High Practicality / Zero Main-Thread Cost.** Ideal for alarm/status banners. | **Include in Initial Subset (Milestone T4).** |
| **3. Slide In/Out (`"slide"`)** | CSS `@keyframes` translating text from below/above or left into the clipped box (`translateY(100%) → 0 → hold → translateY(-100%)`), swapping `pendingText` while outside the clip box. | **High Practicality / Compositor-Friendly.** Very clean for status boards and event feeds. | **Include in Initial Subset (Milestone T4).** |
| **4. Blur to Clear (`"blur-in"`) & Clear to Blur (`"blur-out"`)** | Scoped SVG `<filter><feGaussianBlur stdDeviation="..."/></filter>` animated only during the transition (`1.2 s`), returning to `filter="none"` when clear. | **Moderate Cost.** Continuous Gaussian blur filters across 100+ widgets are expensive (as shown in Test Lab), so `"blur-in"` / `"blur-out"` should run as bounded transition cycles (`once` or `hold`-dominant) rather than high-frequency loops. | **Include in Milestone T4 with bounded blur radius (`stdDeviation <= 6`) and automatic filter removal when clear.** |
| **5. Scramble to Clear (`"scramble"`)** | Lightweight JS step timer (at `typeSpeed` chars/s) that progressively locks graphemes `0..k` to the target string while randomising unlocked non-space positions `k+1..N-1` from a compact cipher alphabet (`0-9A-Z#%&*`). | **High Visual Value for Cyber / Sci-Fi / Telemetry Dashboards.** Touching `textContent` on a single `<text>` node at `15–30 Hz` during a `1–2 s` reveal is very cheap. Best with `tech-mono`, usable with sans/serif. | **Include in Milestone T4.** |
| **6. Overtyping / Replacement (`"overtype"`)** | Lightweight JS step timer that transitions directly from `currentText` to `pendingText` character-by-character in-place: `G_new.slice(0, k) + '▌' + G_old.slice(k + 1)`, expanding or shrinking length smoothly from `len(old)` to `len(new)`. | **Very High Practicality for Data-Driven Widgets.** Eliminates the blank screen between messages by morphing the old reading directly into the new reading at `typeSpeed` chars/s. | **Include in Milestone T4.** |

---

## 8. Curved Text Animation Compatibility Matrix

Not every text animation is visually or geometrically appropriate on a curved `<textPath>`. Each combination has been evaluated individually:

| Animation Mode | Straight Box (`none`, `rect`, `rounded`, `pill`) | Curved Text (`variant: "curved"` via `<textPath>`) | Engineering Assessment & Policy |
|---|---|---|---|
| **Static (`"none"`)** | **Supported** | **Supported** | Direct `textPath.textContent` replacement; works across all 10 fonts. |
| **Fade (`"fade"`)** | **Supported** | **Supported** | Animates opacity on the `<text>` element; zero path distortion. |
| **Flash (`"flash"`)** | **Supported** | **Supported** | Animates opacity on the `<text>` element; zero path distortion. |
| **Typewriter (`"typewriter"`)** | **Supported** | **Supported (with two rules)** | 1. With `text-anchor="middle"`, typing `0 → N` characters on an arc causes already-typed letters to shift backward along the arc as the string grows. Therefore, typewriter on a curved arc should either anchor from the start of the string's final centred span or default to `startOffset` left-to-right progression.<br>2. On `classic-script`, cursive ligatures join as adjacent characters appear (documented visual characteristic). |
| **Ticker (`"ticker-h"`)** | **Supported** | **Not Equivalent (`translateX` breaks arc) — Optional Arc-Offset Mode Only** | Translating a curved `<text>` with CSS `translateX` moves the entire arc rigidly sideways off the curve! Moving text *along* a curve requires animating `<textPath startOffset="100% → -100%">`. Because a finite open arc segment clips glyphs abruptly at the path endpoints unless extended, **Ticker is restricted to straight containers (`none`, `rect`, `rounded`, `pill`) in the initial implementation.** |
| **Vertical Ticker (`"ticker-v"`)** | **Supported** | **Unsupported / Excluded** | Vertical scrolling perpendicular to an arc has no clean typographic meaning. Disabled when `variant == "curved"`. |
| **Slide In/Out (`"slide"`)** | **Supported** | **Unsupported / Excluded** | Sliding into a rectangular clip box does not apply to an open convex arc. Disabled when `variant == "curved"`. |
| **Blur to Clear / Clear to Blur (`"blur-in"`, `"blur-out"`)** | **Supported** | **Supported** | Applies the transition `<filter>` to the `<text>` element containing `<textPath>`. |
| **Scramble (`"scramble"`) & Overtype (`"overtype"`)** | **Supported** | **Unsupported / Excluded** | On a proportional `<textPath>`, swapping random cipher widths causes every downstream character along the curve to jitter tangentially on every step. Restricted to straight containers. |

**UI Enforcement:** When the author selects `variant: "curved"` in Studio, the Animation selector (`#liteTextAnimSelect`) automatically filters/disables unsupported curve animations (`ticker-h`, `ticker-v`, `slide`, `scramble`, `overtype`) and falls back to `"none"` (or keeps `"fade"`, `"flash"`, `"typewriter"`, `"blur-in"`, `"blur-out"`).

---

## 9. Runtime Update and Pending-String Model

### 9.1 Per-Instance State Machine in `svg-widget.js`
To satisfy the requirement that **the consuming WebGUI only calls `widget.setParameter("<param>", "New message")` while the exported widget owns all presentation, timing, and cycle-boundary handover**, `svg-widget.js` will instantiate a lightweight per-element `TextLiteController` stored in an instance-private `Map(el -> controller)` inside `SVGWidget.mount()`:

```
                  ┌──────────────────────────────────────────┐
                  │             IDLE / STATIC                │
                  │  currentText displayed, no active cycle  │
                  └────────────────────┬─────────────────────┘
                                       │
              setParameter(text)       │ (if textAnim == "none")
           ┌───────────────────────────┴───────────────────────────┐
           ▼                                                       ▼
┌─────────────────────────┐                             ┌─────────────────────────┐
│Update DOM immediately   │                             │  ACTIVE ANIMATION CYCLE │
│currentText = newText    │                             │  Displaying currentText │
└─────────────────────────┘                             │  pendingText = null     │
                                                        └───────┬─────────┬───────┘
                                                                │         │
                                       setParameter(newText)    │         │ Cycle Completes
                                       arrives mid-cycle:       │         │ (animationiteration /
                                       pendingText = newText    │         │  animationend / timer)
                                       (overwrites any prior    │         │
                                        pendingText; 1 slot!)   │         ▼
                                                                │  ┌──────────────────────────────┐
                                                                └──► If pendingText !== null:     │
                                                                   │   currentText = pendingText  │
                                                                   │   pendingText = null         │
                                                                   │   Measure new width / length │
                                                                   │   Recompute duration = D / V │
                                                                   │   Start new animation cycle  │
                                                                   └──────────────────────────────┘
```

### 9.2 Five Mandatory Handover Rules
1. **Never Interrupt an Active Cycle:** When `setParameter(name, nextStr)` is called on an animated `text-lite` element while a cycle is currently running (`isAnimating === true`), the DOM text element is **not** mutated mid-flight.
2. **Single-Slot Pending Replacement (Zero Queue Buildup):** The controller stores `controller.pendingText = String(nextStr)`. If 10 rapid telemetry updates arrive during a 4-second ticker pass, updates 1 through 9 are overwritten in `controller.pendingText`, and update 10 is displayed immediately when the 4-second pass finishes.
3. **Exact Cycle-Boundary Handover:**
   - For CSS-driven animations (`ticker-h`, `ticker-v`, `fade`, `flash`, `slide`, `blur-in`, `blur-out`), the controller listens to `animationiteration` (and `animationend` for single-cycle transitions). When the event fires and `controller.pendingText !== null`, the controller pauses the CSS animation at `0%`, swaps `textContent = controller.pendingText`, clears `controller.pendingText = null`, recalculates `duration` (for tickers), and restarts from `0%`.
   - For step-driven JS animations (`typewriter`, `scramble`, `overtype`), the step timer checks `controller.pendingText !== null` upon reaching the end of a complete cycle (after the hold/delete phase completes) and immediately begins the next cycle with `controller.pendingText`.
4. **Off-Screen Suspension Behavior (`IntersectionObserver`):**
   - If a widget is currently **suspended off-screen** (`isSuspended === true` via `IntersectionObserver`), no visual cycle is visible to the user. An incoming `setParameter()` update while suspended immediately promotes `pendingText` to `currentText`, updates the DOM text, and recomputes duration so that when the user scrolls the widget back into view, it is already showing the latest text without waiting for a stale hidden cycle to finish.
5. **Deterministic Cleanup on `unmount()` / `destroy()`:**
   - Every `TextLiteController` registers its `animationiteration` / `animationend` listeners and any active step timer ID (`setTimeout` / `setInterval`). Calling `widget.unmount()` or `widget.destroy()` invokes `controller.dispose()` on all controllers in that widget instance, clearing all timers and listeners with zero memory leaks or cross-instance interference.

---

## 10. Studio Authoring Workflow

To keep Studio intuitive and avoid overwhelming the inspector with unrelated controls, Text Box Lite will follow the established Low-Load Effects authoring pattern:

1. **Creation (`#acc-lite-effects`):**
   - Add a **`T Text Box`** button (`#btnAddLiteText`) to the Low-Load Effects creation bar in the left drawer alongside `⬡ Tracer`, `◉ Pulse`, `⚡ Flash`, `⇢ Flow`, `◎ Radar`, `⏲ Gauge`, and `∿ Oscilloscope`.
   - Clicking `T Text Box` creates a centred `text-lite` object (`variant: "rounded"`, `labelText: "System Ready"`, `width: 220`, `height: 56`) and selects it.
   - Double-clicking a `text-lite` object on the canvas focuses the Text Content input (`#liteTextContentInput` / `#fctx-liteTextContentInput`) for immediate typing.
2. **Contextual Inspector Sections (`#liteTextGroup` & `#fctx-liteTextGroup`):**
   - **Container Style (`#liteVariantSelect`):**
     - `Floating Text (No Box)` (`none`)
     - `Rectangle Box` (`rect`)
     - `Rounded Box` (`rounded`)
     - `Pill / Capsule` (`pill`)
     - `Curved Text (Arc)` (`curved`)
   - **Text & Typography Block (Always visible for `text-lite`):**
     - **Text Content:** Text input (`#liteTextContentInput`, supporting up to 2,000 characters).
     - **Font Family:** Dropdown (`#liteTextFontSelect`) with the 10 curated font choices.
     - **Font Size & Weight:** Font Size slider/number (`8..120 px`) and Weight dropdown (`Regular 400`, `Medium 500`, `Semi-Bold 600`, `Bold 700`).
     - **Letter Spacing:** Slider/number (`-2..20 px`).
     - **Text Colour:** Colour picker (`#liteColorPicker`).
   - **Container & Alignment Block (Context-filtered by `variant`):**
     - Shown for `rect`, `rounded`, `pill`: Background Colour (`bgColor`), Background Opacity (`0..1`, with a quick `Transparent` toggle), Border Colour (`borderColor`), Border Thickness (`0..24 px`), Padding (`0..60 px`), and Horizontal / Vertical Alignment (`Left / Center / Right`, `Top / Middle / Bottom`).
     - Corner Radius (`cornerRadius`) is shown only when `variant === "rounded"`.
     - For `none` (Floating Text): Background and Border controls are hidden; Horizontal / Vertical Alignment remains available.
   - **Curved Arc Block (Shown only when `variant === "curved"`):**
     - **Arc Curvature:** Slider/number (`-100 .. +100`, where positive bows upward and negative bows downward).
     - **Arc Position (`startOffset`):** Slider/number (`0 .. 100%`, default `50%`).
     - **Show Guide Curve (`showGuide`):** Checkbox to display or hide the visible arc line.
   - **Transform Block (Standard `LiteEffect` geometry controls):**
     - Position `X / Y (%)`, `Width / Height (px)`, and **Rotation (`0..360°`)**.
   - **Animation Block (Context-filtered by `textAnim`):**
     - **Animation Mode (`#liteTextAnimSelect`):** `None (Static)`, `Horizontal Ticker`, `Vertical Ticker`, `Typewriter`, `Fade In/Out`, `Flash / Blink`, `Slide In/Out`, `Blur to Clear`, `Clear to Blur`, `Scramble to Clear`, `Overtype Replacement`.
     - When `Horizontal Ticker` or `Vertical Ticker` is selected: shows **Scroll Speed (`px/s`, `10..600`)** and **Direction (`Forward` / `Reverse`)**.
     - When `Typewriter` is selected: shows **Typing Speed (`chars/s`, `1..120`)**, **Behaviour (`Type Once`, `Type & Hold`, `Type, Hold & Delete`, `Repeating Loop`)**, and **Hold Duration (`0..30 s`)**.
     - When `Scramble` or `Overtype` is selected: shows **Character Rate (`chars/s`, `1..120`)** and **Hold Duration (`0..30 s`)**.
     - When `Fade`, `Flash`, `Slide`, or `Blur` is selected: shows **Cycle Duration (`0.2..30.0 s`)** and **Hold Duration (`0..30 s`)**.
3. **Stage 16B Three-Tier Widget Property Exposure (`ObjectPropertyCatalog["lite"]`):**
   - Selecting a `text-lite` object and opening the Widget Properties drawer (or clicking `Expose as Widget Property`) presents eligible runtime properties:
     - **`text` (`labelText`):** Exposed as `<object-name>-lite-text` (or custom alias such as `message`), `type: "text"`.
     - **`color`:** Text colour (`type: "color"`).
     - **`bgColor`:** Container background colour (`type: "color"`, for boxed variants).
     - **`borderColor`:** Container border colour (`type: "color"`, for boxed/guide variants).
     - **`opacity`:** Whole-object opacity (`0..1`).
     - **`visibility`:** Show/hide toggle (`boolean`).
     - **`scrollSpeed` / `typeSpeed` / `speed`:** Animation speed control (filtered to match the authored `textAnim`).

---

## 11. Persistence and Export Architecture

### 11.1 Go Scene Persistence (`.sva`, SQLite `v1.2.0`, Undo/Redo)
- Because `text-lite` is stored in `Scene.LiteEffects` (`[]LiteEffect`), all new `omitempty` fields serialize automatically into:
  - Native `.sva` archives (`pkg/engine/sva/encoder.go` & `decoder.go`).
  - SQLite `projects.schema_json` (`version: "1.2.0"` in `internal/db/sqlite.go` and `js/projects.js`).
  - `HistoryManager` undo/redo snapshots (`pkg/engine/scene/deepcopy.go` & `history.go`).
- Existing saved projects without `text-lite` objects load unchanged with 100% backward compatibility.

### 11.2 Standalone SVG Export vs. Stage 16C Complete Widget Export
It is important to distinguish clearly between a **Standalone SVG Export** and a **Stage 16C Widget Package**:

| Aspect | Standalone SVG Export (`File → Export Still/Animated SVG`) | Stage 16C Widget Package (`Export Widget Package`) |
|---|---|---|
| **Primary Use Case** | Self-contained vector artwork opened directly in a browser tab, `<object>`, `<img>`, or vector editor. | Data-driven dashboard/HMI component mounted via `SVGWidget.mount()` and updated by a consuming WebGUI (`widget.setParameter()`). |
| **Displayed Text** | Displays the Studio-authored string (`labelText`). | Displays the Studio-authored default string on mount, then updates dynamically whenever `setParameter('<name>-lite-text', newStr)` is called. |
| **Pure CSS Animations (`ticker`, `fade`, `flash`, `slide`, `blur`)** | Run natively via embedded `@keyframes` inside `<defs><style>`, even inside script-disabled `<img>` tags (using the Go-computed/DOM-measured travel distance for the authored string). | Run via instance-isolated `@keyframes`, with `TextLiteController` recalculating ticker distance/duration and performing cycle-boundary `pendingText` handover on `setParameter()`. |
| **Step-Driven JS Animations (`typewriter`, `scramble`, `overtype`)** | Run via a compact self-contained inline `<script type="application/ecmascript">` (guarded by `if (!svg.hasAttribute('data-sva-runtime-managed'))`, identical to `getLiteBurstScript()`) when opened in a browser tab or `<object>`. In script-disabled `<img>` tags, the full authored text is present in `<text>` so the label remains readable. | Managed by `TextLiteController` inside `svg-widget.js`, supporting grapheme-safe typing, cycle-boundary `pendingText` replacement, `IntersectionObserver` suspension, and `destroy()` cleanup. |
| **Multi-Instance ID Isolation** | Each `text-lite` object in the SVG has unique IDs (`lite-text-clip-{id}`, `lite-text-arc-{id}`, `sva-text-kf-{id}`). | `IsolateSVGWidget` (Go) and `SVGWidget.mount()` (JS) prefix all IDs, `url(#...)` references, `href="#..."` `<textPath>` links, and `@keyframes` names with `<instanceId>__` so multiple widgets on the same page never collide. |

> **Implementation Note on `<textPath href="#...">` Isolation:**
> We will verify and ensure that `IsolateSVGWidget` in `pkg/engine/scene/widget.go` and `scopeSvgIds` in `internal/web/static/js/svg-widget.js` rewrite both `href="#lite-text-arc-1"` and `xlink:href="#lite-text-arc-1"` on `<textPath>` elements (just as they currently rewrite `<use href="#...">` for Flow Lite), guaranteeing that two mounted curved-text widgets never reference each other's arc path.

---

## 12. Proposed Development Milestones

To keep implementation disciplined, verifiable, and reviewable, a **four-milestone programme (`T1` through `T4`)** is proposed. Only one milestone may be authorised at a time, and each milestone concludes with an automated + real-browser verification suite, a visual demonstration page, and a user acceptance gate:

### Milestone T1 — Core Text Box Object, Containers, Curated Fonts, Alignment & Rotation (`16D-C-T1`)
- **Scope:**
  - Extend `scene.LiteEffect` in Go/WASM with `kind: "text-lite"`, validation, defaults, deep-copy, and unit tests.
  - Implement the 4 straight container variants (`none` floating text, `rect`, `rounded`, `pill`) with background colour, background opacity/transparency, border colour, border thickness (`0..24 px`), corner radius, padding (`0..60 px`), horizontal alignment (`left`, `center`, `right`), vertical alignment (`top`, `middle`, `bottom`), and interior `<clipPath>`.
  - Implement the 10 curated font family stacks (`fontFamily`), font size (`8..120 px`), font weight (`400..700`), and letter spacing (`-2..20 px`).
  - Implement whole-object rotation (`0..360°`) with rotated local-frame hit-testing so click selection and dragging work accurately at any angle.
  - Wire Studio Left Drawer (`#acc-lite-effects` → `T Text Box`), Floating Context Editor (`#fctx-liteTextGroup`), double-click text focus, Layers panel (`Text` / `T` badge), Undo/Redo, `.sva` & SQLite persistence, standalone SVG export, and Stage 16C static `setParameter('<name>-lite-text', ...)` updates.
- **Deliverable & Acceptance Gate:** `verify-stage16d-c-t1` browser suite + `text-lite-demo.html` visual showcase page.

### Milestone T2 — Curved Text on Convex Arcs (`16D-C-T2`)
- **Scope:**
  - Implement `variant: "curved"` using true SVG `<path id="lite-text-arc-{id}">` and `<text><textPath href="#lite-text-arc-{id}">`.
  - Support upward (`curvature > 0`) and downward (`curvature < 0`) convex arcs (`-100 .. +100`), arc position (`startOffset` `0..100%` + `left`/`center`/`right` anchor mapping), letter spacing (`-2..20 px`), whole-object rotation (`0..360°`), and optional visible/hidden guide path (`showGuide`).
  - Ensure `<textPath href="#...">` ID scoping works across multi-instance `SVGWidget.mount()` and standalone SVG export.
  - Enforce the curved-text animation compatibility filter in Go validation and the Studio inspector.
- **Deliverable & Acceptance Gate:** `verify-stage16d-c-t2` browser suite + curved text showcase cards (including `classic-script` formal calligraphy on upward and downward arcs).

### Milestone T3 — Data-Driven Ticker, Typewriter & Cycle-Boundary Pending-Message Handover (`16D-C-T3`)
- **Scope:**
  - Implement `TextLiteController` in `svg-widget.js` (and standalone inline script for exported SVGs) with per-instance `currentText` and single-slot `pendingText` state.
  - Implement **Horizontal Ticker (`"ticker-h"`)** and **Vertical Ticker (`"ticker-v"`)** using the **distance-per-second (`scrollSpeed` `px/s`)** model:
    $$\text{duration} = \frac{W_{\text{container}} + W_{\text{rendered\_text}}}{\text{scrollSpeed}}$$
    with full entrance-to-exit travel even when text is shorter than the box, accurate `getComputedTextLength()` measurement, font-ready re-measurement, and seamless cycle-boundary adoption of `pendingText`.
  - Implement **Typewriter (`"typewriter"`)** using the **characters-per-second (`typeSpeed` `chars/s`)** model across all 4 sub-modes (`once`, `hold`, `delete`, `loop`) with `Intl.Segmenter` Unicode grapheme-cluster safety and cycle-boundary `pendingText` handover.
  - Integrate with `IntersectionObserver` visibility suspension/resumption and `destroy()` cleanup.
- **Deliverable & Acceptance Gate:** `verify-stage16d-c-t3` browser suite + interactive ticker/typewriter live pending-message handover showcase.

### Milestone T4 — Additional Text Effects (`fade`, `flash`, `slide`, `blur`, `scramble`, `overtype`) & Export Hardening (`16D-C-T4`)
- **Scope:**
  - Implement the remaining approved text animations:
    - Declarative CSS/SVG effects: `fade`, `flash`, `slide`, `blur-in` (blur to clear), and `blur-out` (clear to blur), all with `animationiteration` / `animationend` cycle-boundary `pendingText` handover.
    - Lightweight step-driven effects: `scramble` (cipher scramble to clear text at `typeSpeed` chars/s) and `overtype` (progressive in-place character replacement from `currentText` to `pendingText` at `typeSpeed` chars/s).
  - Complete end-to-end export hardening across standalone SVG, animated MP4/WebP/APNG rasterization, `.sva` archives, and multi-instance Stage 16C widgets.
  - If any performance question arises for `blur-in` or `scramble` at dashboard scale (`N=100`), run a targeted experiment in the permanent SVG Animation Test Lab (`cmd/anim-test-lab`).
- **Deliverable & Acceptance Gate:** `verify-stage16d-c-t4` full browser acceptance suite + comprehensive Text & Typography showcase page.

---

## 13. Testing and Acceptance Criteria

Each milestone will be verified through three layers of automated testing (Go unit tests, Node regression harness scenarios, and real headless Chromium CDP browser verification against Kiwiberry `:8088` and `:8091`):

1. **Static Text & Container Geometry (`T1`):**
   - Verify all 4 straight container styles (`none`, `rect`, `rounded`, `pill`), background fill & transparency (`bgOpacity = 0`), border colour, border thickness (`0` = no border), corner radius, and interior `<clipPath>`.
   - Verify all 10 curated font family stacks, font sizes (`8..120 px`), font weights (`400..700`), and letter spacing (`-2..20 px`).
   - Verify all 9 combinations of Horizontal (`left`, `center`, `right`) × Vertical (`top`, `middle`, `bottom`) alignment and padding (`0..60 px`).
   - Verify whole-object rotation (`0..360°`) in DOM transform, rotated hit-testing, selection box overlay, save/reload, and export.
2. **Curved Text Layout (`T2`):**
   - Verify `<textPath>` binding to `<path id="lite-text-arc-{id}">`, upward (`+curvature`) and downward (`-curvature`) convex arc geometry, `startOffset` positioning, letter spacing, optional guide path (`showGuide`), whole-object rotation, and multi-instance ID isolation (`<instanceId>__lite-text-arc-...`).
3. **Edge-Case Strings (`T1–T4`):**
   - Verify empty strings (`""` — container remains selectable in Studio and valid in widget runtime without NaN/divide-by-zero duration), very long strings (`500+` characters), XML special characters (`< > & " '`), and multi-codepoint Unicode grapheme clusters (emoji `🚀`, `👨‍👩‍👧‍👦`, accented characters, CJK).
4. **Ticker Movement, Distance-Per-Second Timing & Pending Handover (`T3`):**
   - Verify `duration = (W_box + W_text) / scrollSpeed` for both short strings (`W_text < W_box`) and long strings (`W_text >> W_box`) by sampling rendered `getComputedTextLength()` and computed `animationDuration`.
   - Sample real-time browser `getBoundingClientRect()` / transform matrix positions across the cycle to prove smooth monotonic travel from outside the entrance edge to completely outside the exit edge (never relying on `requestAnimationFrame` callback Hz as proof of movement).
   - Send 3 rapid `setParameter()` updates mid-cycle (`"Msg 1"`, `"Msg 2"`, `"Msg 3"`): verify that the original string continues uninterrupted until the current pass exits, `"Msg 1"` and `"Msg 2"` are overwritten in `pendingText`, and `"Msg 3"` appears at the start of the very next cycle with recalculated duration.
5. **Typewriter Grapheme Progression & Rate (`T3`):**
   - Verify `typeSpeed` (`chars/s`) duration scaling with string length across `once`, `hold`, `delete`, and `loop` modes.
   - Verify that multi-byte Unicode emoji/graphemes appear as whole characters and are never split into broken surrogate halves (`\uD83D`).
   - Verify mid-cycle `pendingText` replacement and handover at the cycle boundary.
6. **Additional Text Effects, Multi-Instance Isolation & Suspension (`T4`):**
   - Verify `fade`, `flash`, `slide`, `blur-in`, `blur-out`, `scramble`, and `overtype`.
   - Mount two independent instances (`instA`, `instB`) of the same exported widget package: verify that updating text, speed, or colour on `instA` (including mid-cycle `pendingText` queuing) has zero effect on `instB`.
   - Verify `IntersectionObserver` off-screen suspension and resumption, and clean timer/listener disposal on `destroy()`.
7. **Persistence & Non-Regression (`T1–T4`):**
   - Verify `HistoryManager` undo/redo, `.sva` binary package round-trip, SQLite `schema_json` round-trip, standalone SVG export, and coexistence with all 7 existing `LiteEffect` kinds (`perimeter-tracer`, `pulse`, `flash`, `flow-lite`, `radar-lite`, `gauge-lite`, `oscilloscope-lite`) and main `Node`, `Flow`, `Glow`, and `Ripple` objects.

---

## 14. Technical Limitations and Deferred Features

1. **Font Availability Across Platforms vs. Bundled Font Files:**
   - Standard CSS font stacks rely on fonts installed on the viewing OS. While our 10 curated stacks provide metric-compatible fallbacks across Windows, macOS, and Linux, exact glyph shapes for decorative scripts (`classic-script`) vary slightly between macOS (`Snell Roundhand` / `Apple Chancery`), Windows (`Edwardian Script` / `Monotype Corsiva`), and Linux (`URW Chancery L` / `Z003`) unless an open-source `.woff2` script font is embedded.
2. **SVG `<textPath>` Kerning on Tight Curves:**
   - Native browser SVG `<textPath>` rotates each glyph around its baseline midpoint without reshaping glyph outlines. Tight concave curves or sharp bends cause glyph collisions and break cursive script joins. Therefore, curved text is deliberately scoped to smooth convex arcs (`curvature` `-100 .. +100` mapped to bounded sagitta `±45%` of height) with authorable `letterSpacing`.
3. **Curved Text Animation Restrictions:**
   - As documented in Section 8, `ticker-h`, `ticker-v`, `slide`, `scramble`, and `overtype` are restricted to straight containers (`none`, `rect`, `rounded`, `pill`) because applying linear translation or rapid proportional glyph-width swapping to an open `<textPath>` arc causes geometric distortion. Curved text supports `none` (static), `fade`, `flash`, `typewriter`, `blur-in`, and `blur-out`.
4. **Script-Disabled `<img>` Tags vs. Standalone `<svg>` / Widget Contexts:**
   - Inside an HTML `<img>` tag, browsers disable all inline `<script>` execution for security. Pure CSS animations (`ticker`, `fade`, `flash`, `slide`, `blur`) still animate in `<img>` using the authored text's pre-calculated duration, whereas JS step-driven animations (`typewriter`, `scramble`, `overtype`) display the static authored text in `<img>` and run their full character-step animation when opened as a standalone `.svg` document, `<object>`, or mounted Stage 16C widget.
5. **Single-Line vs. Multi-Line Scope:**
   - Ticker (`ticker-h`), Curved Text (`curved`), Scramble (`scramble`), and Overtype (`overtype`) operate on a single line of text. For Static (`none`), Fade (`fade`), Flash (`flash`), Slide (`slide`), and Blur (`blur-in` / `blur-out`) in straight containers, explicit line breaks (via `|` or `\n`) can be supported as stacked `<tspan>` lines if desired.

---

## 15. Implementation Evidence, Comparative Performance Findings & Acceptance Readiness

All four milestones (**T1 Static Text and Containers**, **T2 True Curved Text**, **T3 Data-Driven Ticker, Typewriter & Runtime Pending-Message Handover**, and **T4 Additional Text Effects & Export Hardening**) have been implemented and verified consecutively:

### 15.1 Milestone Implementation Status

| Milestone | Scope | Verification Evidence | Status |
| :--- | :--- | :--- | :--- |
| **T1 — Static Text and Containers** | `kind: "text-lite"` in Go/WASM & JS; 4 straight container styles (`none`, `rect`, `rounded`, `pill`); 10 curated font stacks (`modern-sans` through `classic-script` formal calligraphy); font size (`8..120`), weight (`400..700`), letter spacing (`-2..20`); 9-way alignment (`left`/`center`/`right` × `top`/`middle`/`bottom`); padding (`0..60`); background opacity (`0..1`) & border thickness (`0..24`); whole-object rotation (`-360..360°`) with inverse-rotated local hit-testing (`findAtPct`) and rotated `#nodeSelectionBox`. | `TestTextLiteContainersFontsCurvedAnimationsAndWidgetScoping` (`go test ./pkg/engine/...` PASS), Node Harness Scenario 93 (`W16DC-01..12` PASS), Real-Browser Gate `AT-16DC-T1` PASS | **IMPLEMENTED (AWAITING USER ACCEPTANCE)** |
| **T2 — Curved Text** | True SVG `<textPath>` bound to quadratic-Bézier `<path id="lite-text-arc-{id}">`; upward convex (`+curvature`) and downward concave (`-curvature`) arcs (`-100..+100`); `startOffset` (`0..100%`); letter-spacing; optional dashed arc guide (`showGuide`); curved animation compatibility filter; and multi-instance ID isolation (`<instanceId>__lite-text-arc-{id}`). | Go unit test `IsolateSVGWidget` `<textPath href>` scoping PASS, Node Harness Scenario 93 (`W16DC-13..18` PASS), Real-Browser Gates `AT-16DC-T2` & `AT-16DC-T4B` PASS | **IMPLEMENTED (AWAITING USER ACCEPTANCE)** |
| **T3 — Ticker, Typewriter & Runtime Message Handling** | Distance-per-second (`scrollSpeed` `10..600 px/s`) horizontal (`ticker-h`) and vertical (`ticker-v`) tickers with `getComputedTextLength()` width measurement and `duration = (W_box + W_text) / scrollSpeed`; characters-per-second (`typeSpeed` `1..120 chars/s`) typewriter (`once`, `hold`, `delete`, `loop`) with `Intl.Segmenter` Unicode grapheme-cluster safety; and per-instance `TextLiteController` (`currentText` + single-slot `pendingText`) cycle-boundary handover in `svg-widget.js`. | Real-Browser Gates `AT-16DC-T3A` (immediate static update, multi-instance isolation, off-screen suspension handover), `AT-16DC-T3B` (ticker 3-burst coalescing & dynamic duration recalculation `1.085s -> 2.003s`), and `AT-16DC-T3C` (typewriter emoji grapheme safety & 3-burst coalescing) PASS | **IMPLEMENTED (AWAITING USER ACCEPTANCE)** |
| **T4 — Additional Text Effects & Export Hardening** | Declarative CSS/SVG effects (`fade`, `flash`, `slide`, `blur-in`, `blur-out`) and character-step effects (`scramble`, `overtype`) with cycle-boundary `pendingText` handover; Stage 16C widget property catalog (`text`, `color`, `bgColor`, `borderColor`, `opacity`, `visibility`, `strokeWidth`, `showGuide`, `direction`, `scrollSpeed`, `typeSpeed`, `speed`); standalone SVG export; `.sva` & SQLite round-trip; and comprehensive visual showcase at `http://192.168.100.40:8088/text-lite-demo.html`. | Node Harness Scenario 93 (`W16DC-19..28` PASS), Real-Browser Gates `AT-16DC-T4A`, `AT-16DC-T4B`, `AT-16DC-PERF`, `AT-16DC-SVC1`, `AT-16DC-SVC2` (`10 / 10` PASS), plus Stage 16D-B M1–M4 non-regression (`63 / 63` gates PASS) | **IMPLEMENTED (AWAITING USER ACCEPTANCE)** |

### 15.2 Focused Comparative Performance Investigation (`AT-16DC-PERF` at `N=50` & `N=100`)

Measured in headless Chromium via CDP (`scripts/cmd/verify-stage16d-c/main.go`) across `N=50` and `N=100` concurrent `text-lite` instances with short (`8 chars`), medium (`36 chars`), and long (`96 chars`) strings:

| Technique | Instances (`N`) | String Profile | Avg FPS | Main-Thread JS Work (`ms`) | Suspended JS Cost (`ms`) | Recommendation |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **`css-ticker-h` (Declarative CSS `@keyframes`)** | `50` | `medium` (`36 chars`) | `25.3..28.2` | **`0.00 ms`** | `0.0 ms` | **Preferred for whole-text tickers** (`0.00 ms` per-frame JS work) |
| **`css-ticker-h` (Declarative CSS `@keyframes`)** | `100` | `short` / `medium` / `long` | `14.3..23.3` | **`0.00 ms`** | `0.0 ms` | Compositor-driven; recalculates `animationDuration` only at `animationiteration` handover |
| **`js-raf-ticker-h` (Per-Frame JS `requestAnimationFrame`)** | `50` / `100` | `medium` (`36 chars`) | `19.1..22.5` | `0.90..3.70 ms` | `0.0 ms` | **Avoid** — continuous main-thread DOM attribute mutation every frame |
| **`css-fade-flash` (Declarative CSS `@keyframes`)** | `50` / `100` | `medium` (`36 chars`) | `18.8..38.8` | **`0.00 ms`** | `0.0 ms` | **Preferred for `fade`, `flash`, `slide`** (`0.00 ms` main-thread JS) |
| **`js-raf-fade-flash` (Per-Frame JS `requestAnimationFrame`)** | `100` | `medium` (`36 chars`) | `20.0..32.0` | `1.10..2.60 ms` | `0.0 ms` | **Avoid** — unnecessary per-frame JS overhead |
| **`scheduled-typewriter` (Character-Interval `setTimeout`)** | `50` / `100` | `medium` (`36 chars`) | **`35.3..40.0`** | **`0.00 ms` (between steps)** | `0.0 ms` | **Preferred for `typewriter`, `scramble`** — wakes only at `1000 / typeSpeed` ms character intervals |

---

## 16. Consolidated Visual Acceptance Corrections (`Stage 16D-C — CORRECTIONS IMPLEMENTED, AWAITING USER ACCEPTANCE`)

Following the user's initial visual inspection of `f07272f`, the following targeted corrections were implemented across the Go engine (`pkg/engine/scene/`), WASM binary, Studio inspectors (`index.html`, `lite-effects.js`, `context-editor.js`), Stage 16C widget runtime (`svg-widget.js`), and visual showcase (`text-lite-demo.html`):

1. **Horizontal Ticker Directions & Zero Vertical Drift (`ticker-h`):**
   - Replaced `translateX(...)` with explicit 2D `translate(var(--sva-ticker-start), 0px)` → `translate(var(--sva-ticker-end), 0px)`, preserving the configured vertical alignment (`firstLineY`) and tilted-box alignment while guaranteeing zero vertical movement (`dy = 0.00 px`).
   - **Scroll Left (`direction: "forward"`):** Starts outside the right edge (`+w`) and exits completely through the left edge (`-tw`).
   - **Scroll Right (`direction: "reverse"`):** Starts outside the left edge (`-tw`) and exits completely through the right edge (`+w`).
   - Inspector labels updated to **"Scroll Left / Up (Forward)"** and **"Scroll Right / Down (Reverse)"**.
2. **Multiline Vertical Scroll (`ticker-v`) — Explicit Newlines, Word Wrap, Line Spacing & Lines per Second:**
   - Supports literal newline characters (`\n`) in both Studio `<textarea>` inputs and runtime `setParameter()` strings, rendering each line as an aligned `<tspan class="lite-text-line">`.
   - Added **Word Wrap (`wordWrap`, default `true`)** specifically for `ticker-v`: wraps at word spaces to fit `availW = width - 2 * padding` using canvas font metrics (`measureTextStringWidth`) while honouring explicit newlines; when disabled (`wordWrap: false`), only explicit `\n` breaks create new lines.
   - Added **Line Spacing (`lineSpacing`, `0.8..3.0×`, default `1.25×`)** controlling vertical line advance (`lineAdvance = fontSize * lineSpacing`) and total block height (`blockH = fontSize + (N - 1) * lineAdvance`).
   - Changed vertical scrolling speed to **Lines per Second (`linesPerSec`, `0.1..20.0 lines/s`, default `1.2 lines/s`)**, computing linear travel velocity `v = linesPerSec * lineAdvance` and full entry-to-exit duration `duration = (h + blockH) / v`.
3. **Additional Stacked-Letter Vertical Scroll (`stacked-v`):**
   - Added **Vertical Stacked Letters (`textAnim: "stacked-v"`)** as a distinct animation choice alongside `ticker-v`.
   - Segments the message into Unicode grapheme clusters (`Intl.Segmenter`) and stacks characters vertically one above another (`S / T / A / T / U / S / 🚀`) with uniform `lineSpacing`, scrolling the vertical column at **`typeSpeed` (`chars/s`)** (`v = typeSpeed * lineAdvance`) without applying Word Wrap.
4. **Fade — Separate 3-Phase Timing Controls (`fade`):**
   - Exposed three independent timing controls: **Fade Duration (`speed`, `0.05..30 s`)**, **Visible Hold Duration (`holdDelay`, `0..30 s`)**, and **Clear Duration (`clearDuration`, `0..30 s`)**.
   - Generates an exact per-element `@keyframes sva-lite-fade-{id}` stylesheet block with phase percentages `p1 = fadeDur / total`, `p2 = (fadeDur + visHold) / total`, `p3 = (2 * fadeDur + visHold) / total` (`opacity: 0 → 1 → 1 → 0 → 0`).
5. **Flash / Blink — Separate 3-Phase Timing Controls & Lock On/Off (`flash`):**
   - Exposed **Fade-In Duration (`speed`, `0..30 s`)**, **On Hold Duration (`holdDelay`, `0..30 s`)**, **Off Duration (`clearDuration`, `0..30 s`)**, and **Lock On/Off (`lockOnOff` checkbox)**.
   - When **Lock On/Off** is enabled, editing either On Hold Duration or Off Duration in either the Left Drawer or Floating Context Editor automatically keeps both values equal while leaving Fade-In Duration independent.
6. **Playback Modes (`playbackMode`: `once`, `loop`, `pingpong`):**
   - Exposed **Play Once (`once`)**, **Loop (`loop`)**, and **Ping-Pong (`pingpong`)** for all non-Typewriter animated effects (`ticker-h`, `ticker-v`, `stacked-v`, `fade`, `flash`, `slide`, `blur-in`, `blur-out`, `scramble`).
   - In **Ping-Pong** ticker mode, `TextLiteController` tracks `iterCount` and only adopts `pendingText` after even iteration counts (`iterCount % 2 === 0`), preventing mid-return text substitution.
   - Hidden when `textAnim === "typewriter"` so it never conflicts with Typewriter's explicit sequence controls.
7. **Four Distinct Typewriter Sequences & Repeat Toggle (`typeMode` + `typeRepeat`):**
   - **A. Type Once (`once`):** Reveals text from start to end once.
   - **B. Type Once and Hold (`hold`):** Reveals text once, holds for `holdDelay`, and retains the completed text; if a new `pendingText` arrives during or after the hold, it transitions cleanly at the cycle boundary without blocking updates indefinitely.
   - **C. Type, Hold, Delete Backwards (`delete`):** Types left-to-right, holds, and deletes from the right-hand end towards the left (`STATUS → STATU → STAT → STA → ST → S → ""`).
   - **D. Type, Hold, Delete Forwards (`delete-forward`):** Types left-to-right, holds, and deletes from the left-hand end towards the right (`STATUS → TATUS → ATUS → TUS → US → S → ""`).
   - Added an explicit **Repeat Sequence (`typeRepeat`)** checkbox and migrated legacy `typeMode: "loop"` to `typeMode: "delete"` with `typeRepeat: true`.
8. **Removal of Redundant Overtype Selection & Legacy Migration:**
   - Removed `overtype` from new selection dropdowns in both the Left Drawer and Floating Context Editor while preserving full backward-compatible migration (`textAnim: "overtype"` → `"typewriter"`) in Go (`applyLiteEffectDefaults`, `UpdateLiteEffect`) and JS (`_migrateLiteEffect`, `svg-widget.js`).
   - Preserved **Scramble to Clear (`scramble`)**, **Blur to Clear (`blur-in`)**, and **Clear to Blur (`blur-out`)** unchanged.
9. **Automated Verification of Consolidated Corrections (`12 / 12` Browser Gates PASS):**
   - `AT-16DC-C01`: Inspectors, tilted-box alignment (`rotation: 35°`, `text-anchor: end`), curved `<textPath>`, and removal of `overtype` from UI dropdowns — **PASS**
   - `AT-16DC-C02`: Flash/Blink **Lock On/Off** bidirectional synchronisation across Left Drawer and Floating Context Editor — **PASS**
   - `AT-16DC-C03`: Legacy `overtype` & `loop` migration to `typewriter` (`delete` + `typeRepeat: true`) — **PASS**
   - `AT-16DC-C04`: Horizontal Ticker Left (`dx = -139.33 px`, `dy = 0.00 px`) & Right (`dx = +139.33 px`, `dy = 0.00 px`) temporal sampling — **PASS**
   - `AT-16DC-C05`: Vertical multiline scroll explicit newlines, Word Wrap ON (`5 lines`) vs OFF (`2 lines`), Line Spacing (`1.2×` `19.2 px` vs `1.6×` `25.6 px`), and Lines/s duration scaling (`1.0 lines/s` `9.0 s` vs `2.0 lines/s` `4.5 s`) — **PASS**
   - `AT-16DC-C06`: Stacked-letter vertical column (`S|T|A|T|U|S|🚀`, identical `x`, monotonic `y`, vertical scroll `dy = -46.28 px`) — **PASS**
   - `AT-16DC-C07`: 3-phase Fade (`fadeHoldOp = 1.0`, `fadeClearOp = 0.0`) & Blink (`blinkOnOp = 1.0`, `blinkOffOp = 0.0`) temporal opacity sampling + Play Once completion — **PASS**
   - `AT-16DC-C08`: Ping-Pong ticker completes return journey (`midReturnText = "ORIGINAL PP"` → `postReturnText = "ADOPTED AFTER RETURN"`) before adopting `pendingText` — **PASS**
   - `AT-16DC-C09`: All 4 Typewriter sequences, Delete Forwards first-grapheme removal (`STATUS → TATUS → ATUS → TUS → US → S → ""`), and Type Once & Hold immediate adoption after hold completion — **PASS**
   - `AT-16DC-C10`, `AT-16DC-SVC1`, `AT-16DC-SVC2`: Updated visual showcase (`27` T1 + `5` T2 + `21` T3/T4 cards) and protected services (`:8088`, `:8091`, `:8093`) — **PASS**
