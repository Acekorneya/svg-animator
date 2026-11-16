# SVG Animator Studio — Animation Architecture and Extensibility Review

## Executive Summary

Stage 16B established a stable, authoritative widget foundation:
- Go/WASM-authoritative scene state, identity, and object renaming.
- Canonical `<object-name>-<object-type>-<property>` external parameter naming.
- Strict pre-export validation (`ValidateWidgetExport`) catching orphan/unresolvable bindings before archive creation.
- Multi-instance isolation in `svg-widget.js` scoping IDs, CSS, and SMIL timing.
- Verified live execution across headless Chrome over CDP and real Linux acceptance environments (`192.168.100.40:8088`).

With Stage 16B formally accepted, this review audits the application's animation architecture. The purpose is to design a clean, minimal extension model that allows new animations and controllable properties to be added predictably without reopening or breaking the stable widget contract.

---

## 1. Baseline Verification & Integrity

Prior to conducting this review, the baseline repository state was verified:
- **Git Branch:** `main`
- **Baseline Commit:** `41118bf` (`feat(stage16b): node target resolution, standardised object-name-first parameter naming & pre-export validation`)
- **Go Engine Test Suite:** 100% PASS (`go test ./...`) across all packages (`pkg/engine/scene`, `pkg/engine/timeline`, `pkg/engine/geometry`, `pkg/engine/protocol`, `pkg/engine/sva`, `internal/web`, `internal/generator`, `internal/rasterizer`).
- **JavaScript Regression Harness:** 100% PASS (`node scripts/regression-harness.js`) across all 86 test scenarios.
- **Kiwiberry CDP Verification:** 100% PASS on `verify-final-contract` (10/10 criteria) and `verify-node-target-resolution` (9/9 criteria).
- **Integrity Safeguards:** SQLite schemas, user projects, `studio_library.db`, and port configurations (`8088`/`8090`) remain intact and untouched.

---

## 2. Animation Architecture Audit

SVG Animator Studio contains **six distinct animation systems**, each developed during different development phases. They exhibit key differences in data modeling, coordinate systems, attachment semantics, and playback execution.

### Detailed Audit by Animation Type

```
+-------------------------------------------------------------------------------------------------------------+
|                                    ANIMATION ARCHITECTURE AT A GLANCE                                       |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| Animation System  | Authoritative Go   | SVG Export Mechanism   | Editor Playback Mode | Coordinate Model   |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 1. Flow           | scene.Flow         | SMIL <animateMotion>   | Continuous SMIL +    | Multi-waypoint     |
|                   |                    | & <animate> (opacity)  | Go Evaluator (batch) | connector path     |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 2. Glow           | scene.Glow         | SMIL <animate>         | Continuous SMIL +    | Absolute canvas    |
|                   |                    | (opacity, radius)      | Go Evaluator (batch) | center (X, Y)      |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 3. Ripple         | scene.Ripple       | SMIL <animate>         | Continuous SMIL +    | Absolute canvas    |
|                   |                    | (r, stroke-opacity)    | Go Evaluator (batch) | center (X, Y)      |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 4. Camera         | scene.CameraConfig | CSS transform / matrix | Go Evaluator ->      | Global canvas-wide |
|                   |                    | & rasterizer playback  | RAF DOM transform    | 2D/3D viewport     |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 5. Optical        | scene.OpticalConfig| SVG <filter> (SMIL     | Go Evaluator ->      | Global filter /    |
|                   |                    | feGaussianBlur)        | feGaussianBlur patch | procedural mask    |
+-------------------+--------------------+------------------------+----------------------+--------------------+
| 6. Node Transform | scene.Node         | Declarative SMIL / CSS | Go Evaluator ->      | Object-local       |
|    (Pulse/Float)  |                    | keyframe pulse/float   | Opacity/Translate    | bounding box       |
+-------------------+--------------------+------------------------+----------------------+--------------------+
```

---

### A. Data Flow Connectors (`Flow`)

- **Authoritative Go Model:** `scene.Flow` in `pkg/engine/scene/model.go` and `flow.go`.
- **Core Properties:**
  - Geometry: `Nodes []geometry.Point`, `Curvature float64`, `RouteMode string` ("curve", "orthogonal", "straight").
  - Visual: `Color string`, `LineWidth float64`, `LineStyle string`, `Arrow string`, `Opacity float64`, `Scale float64`, `Type string` ("packet", "orb", "envelope", etc.), `ShowLine bool`, `Glow float64`.
  - Animation: `Speed float64` (seconds per transit), `Delay float64`, `EndDelay float64`, `HideOnDelay bool`.
  - Topology: `FromNodeID StringID`, `ToNodeID StringID`.
- **Editor Responsibilities:** `internal/web/static/js/flows.js` (`window.DataFlows`). Manages waypoint manipulation, endpoint dragging, snap-to-node snapping, and visual selection overlays.
- **SVG Generation Mechanism:**
  - Guide track: `<path id="flowPath-{id}" class="flow-track-path" stroke="none" fill="none" d="..." />`.
  - Animated payload: `<animateMotion ... rotate="auto"><mpath href="#flowPath-{id}"/></animateMotion>` and `<animate attributeName="opacity" values="0; op; op; 0" ... />`.
- **Attachment:** Endpoint waypoints can optionally bind to `FromNodeID` and `ToNodeID`. When nodes move, attached flow endpoints follow. Non-endpoint waypoints are free-floating canvas coordinates.
- **Persistence:** Serialized in `.sva` archive (`scene.json`) and SQLite `schema_json` (v1.2.0).
- **Editor vs. Export Playback:**
  - In Editor: Browser continuously runs SMIL `<animateMotion>` in canvas stage. `TimelinePlayer` additionally computes deterministic position `pt, angle` via Go `timeline.EvaluateSceneAtTime` for time-scrubbing.
  - In Widget Export: Isolated SMIL loops independently. Runtime supports flow-specific `speed` multiplier and `color` recoloring.
- **Helper Geometry vs. Payload:**
  - Motion guide path is marked `.flow-track-path` with `stroke="none"`.
  - Payload is wrapped in `.flow-payload-group`.
  - Runtime recoloring strictly paints only `.flow-payload-group` children, preserving `stroke="none"` on track path.

---

### B. Hotspots and Radar Beacons (`Glow`)

- **Authoritative Go Model:** `scene.Glow` in `pkg/engine/scene/model.go` and `glow.go`.
- **Core Properties:**
  - Geometry: `X float64`, `Y float64`, `Size float64`, `Shape string` ("circle", "square", "radar", "custom").
  - Visual: `Color string`, `Opacity float64`, `Scale float64`, `Blur float64`, `CoreR float64`, `Thick float64`, `RadarRings int`.
  - Animation: `Duration float64` (seconds per cycle), `Delay float64`, `EndDelay float64`, `Playback string` ("pingpong", "linear", "once", "static"), `RadarStagger float64`.
- **Editor Responsibilities:** `internal/web/static/js/glows.js` (`window.Glows`). Dragging position on stage, radius resizing, color/shape inspection in Context Editor.
- **SVG Generation Mechanism:**
  - Defs: `<radialGradient id="glowGrad-{id}">` with stops transitioning from `color` to `transparent`.
  - Animation: `<animate attributeName="opacity" values="0; op; 0" calcMode="spline" ... />` or radar ring expansion `<animate attributeName="r" values="coreR; size; size" ... />`.
  - Hit testing: `<rect class="glow-hit-target" fill="transparent" pointer-events="all" />`.
- **Attachment:** Free-floating absolute coordinates `(X, Y)`. Not attached to Node or Flow objects.
- **Persistence:** Serialized in `.sva` and SQLite `schema_json`.
- **Editor vs. Export Playback:**
  - Editor: Continuous SMIL looping + Go timeline batch evaluation.
  - Export: Isolated SMIL playback.
- **Helper Geometry vs. Effect:**
  - Hit-test bounds use `.glow-hit-target` (`fill="transparent"`).
  - Runtime recoloring updates `<radialGradient>` stops and core shapes without touching the transparent hit target.

---

### C. Refractive Water Ripples (`Ripple`)

- **Authoritative Go Model:** `scene.Ripple` in `pkg/engine/scene/model.go` and `ripple.go`.
- **Core Properties:**
  - Geometry: `X float64`, `Y float64`, `InnerR float64`, `OuterR float64`.
  - Visual: `Color string`, `Opacity float64`, `Rings int`, `Thick float64`, `Refraction float64`.
  - Animation: `Duration float64`, `Delay float64`, `EndDelay float64`.
- **Editor Responsibilities:** `internal/web/static/js/ripples.js` (`window.Ripples`). Direct canvas placement, inner/outer radius handles, refraction slider.
- **SVG Generation Mechanism:**
  - Defs: Dynamic `<filter id="rippleBlur-{id}">` and `<mask id="rippleMask-{id}">`.
  - Wave crests: `<circle r="0" stroke="{color}" filter="url(#rippleBlur-{id})"><animate attributeName="r" ... /><animate attributeName="stroke-opacity" ... /></circle>`.
  - Shadow rings: Scaled offset circle (`scale(0.92)`) with `stroke="black"`.
  - Refraction layer: Clipped duplicate of background photo scaled by `(1 + refraction/1000)`.
  - Hit testing: `.ripple-hit-target` (`fill="transparent"`).
- **Attachment:** Free-floating canvas coordinates `(X, Y)`. Refraction optionally samples underlying photo.
- **Persistence:** Serialized in `.sva` and SQLite `schema_json`.
- **Helper Geometry vs. Effect:**
  - Runtime recoloring strictly paints `.ripple-wave` stroke, preserving shadow rings as `#000000` and hit target as `transparent`.

---

### D. Viewport & Camera Motion (`Camera`)

- **Authoritative Go Model:** `scene.CameraConfig` in `pkg/engine/scene/model.go` and `camera.go`.
- **Core Properties:**
  - Motion: `Motion string` ("static", "ken-burns", "handheld-drift", "perspective-tilt").
  - Transforms: `StartX`, `StartY`, `StartZoom`, `EndX`, `EndY`, `EndZoom`, `Sway`, `Breathe`, `TiltX`, `TiltY`.
  - Timing: `Duration float64`, `Loop string` ("loop", "pingpong", "once").
- **Editor Responsibilities:** `internal/web/static/js/effects.js` and `timeline.js`. Start/End keyframe framing boxes, pan/zoom sliders, 3D tilt preview.
- **SVG Generation Mechanism:**
  - Wraps entire canvas content inside `<g id="cameraRig" class="camera-rig">` inside `<g id="cameraStage3D">`.
  - Exported as CSS `@keyframes` transform animation or rendered frame-by-frame for raster/video export.
- **Attachment:** Scene-wide root transform. Affects all objects on the stage simultaneously.
- **Playback Semantics:** Driven authoritatively by Go timeline evaluator (`pkg/engine/timeline/evaluator.go:340-357`). During editing, RAF loop applies `camRig.style.transform`.
- **Widget Export Status:** Currently exposed as instance-wide master `playbackRate` and `operatingState`. Camera motion parameters are authored at project level and not individually exposed as widget parameters.

---

### E. Optical Effects (`Optics`)

- **Authoritative Go Model:** `scene.OpticalConfig` in `pkg/engine/scene/model.go` and `optics.go`.
- **Core Properties:**
  - Type: `Type string` ("none", "focus-pull", "tilt-shift", "motion-blur").
  - Focus Pull: `StartBlur float64`, `EndBlur float64`, `FocusDur float64`.
  - Tilt Shift: `TSShape string` ("horizontal", "vertical", "radial"), `TSPosX`, `TSPos`, `TSWidth`, `TSBlur`.
  - Motion Blur: `MBInt float64`, `MBAng float64`.
- **Editor Responsibilities:** `internal/web/static/js/effects.js`. Sliders for blur depth, band width, angle, and feathering.
- **SVG Generation Mechanism:**
  - Injected as `<filter id="opt-focus">`, `<filter id="opt-ts-blur">`, `<mask id="ts-mask">`.
  - Focus pull animates `stdDeviation` via SMIL `<animate>` or procedural RAF patch.
- **Attachment:** Canvas-wide optical filter applied to background and artwork.
- **Widget Export Status:** Authored in studio; filter definitions bundled in export SVG. Not currently exposed as dynamic widget parameters.

---

### F. Node and Image Animations (`Node / Image`)

- **Authoritative Go Model:** `scene.Node.Animation` ("none", "pulse", "float", "fade") in `model.go`.
- **Editor Responsibilities:** `internal/web/static/js/nodes.js`. Node Property Inspector dropdown `<select id="nodeAnimSelect">`.
- **SVG Generation Mechanism:**
  - In static/WASM export: Declarative CSS keyframes `@keyframes node-pulse`, `@keyframes node-float`.
  - In Go timeline evaluator: Deterministic harmonic math:
    - Pulse: `op = 0.65 + 0.35 * (sin(t * 2pi / 2.6) + 1) / 2`
    - Float: `dy = -6.0 * (sin(t * 2pi / 3.2) + 1) / 2`
    - Fade: `op = 0.3 + 0.7 * (sin(t * 2pi / 3.0) + 1) / 2`
- **Attachment:** Object-local animation applied directly to the Node instance.
- **Widget Export Status:** Node text, fill, stroke, opacity, and visibility are exposed as widget parameters. Animation type itself is authored and fixed in exported SVG.

---

## 3. Capability Matrix

| Animation Type | Authored Properties | Externally Controllable (Widget) | Export Runtime Support | Attachment Scope | Primary Architectural Limitations |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Flow** | 18 properties (speed, delay, endDelay, curvature, routeMode, color, lineWidth, lineStyle, arrow, type, scale, glow, label, customImage, etc.) | `speed`, `color`, `stroke`, `opacity`, `visibility` | Full (SMIL + `svg-widget.js`) | Binds to 2 Nodes (from/to) + arbitrary waypoints | Cannot follow arbitrary node shape boundaries; cannot attach multiple flows to same visual packet. |
| **Glow** | 16 properties (x, y, size, shape, color, opacity, dur, delay, endDelay, playback, radarRings, stagger, scale, blur, thick, coreR) | `color`, `opacity`, `visibility` | Full (SMIL + `svg-widget.js`) | Free-floating (X, Y) | Cannot attach to follow a Node or Flow waypoint; fixed center coordinates. |
| **Ripple** | 12 properties (x, y, innerR, outerR, dur, delay, endDelay, color, opacity, rings, thick, refraction) | `color`, `opacity`, `visibility` | Full (SMIL + `svg-widget.js`) | Free-floating (X, Y) + photo mask | Refraction requires background image; cannot attach to Node or Flow. |
| **Camera** | 14 properties (motion, start/end zoom, start/end position, sway, breathe, tiltX, tiltY, duration, loop) | `playbackRate`, `operatingState` | Full (CSS / Chromium frame capture) | Entire Canvas Viewport | Scene-global only; cannot create local sub-cameras. |
| **Optics** | 11 properties (type, startBlur, endBlur, focusDur, tsShape, tsPosX, tsPos, tsWidth, tsBlur, mbInt, mbAng) | Fixed at export (not dynamically exposed) | Full (SVG Filters / Masks) | Canvas-wide | Procedural filters cannot target individual nodes or layers. |
| **Node Anim** | 2 properties (`animation` = pulse, float, fade) | `fill`, `stroke`, `text`, `opacity`, `visibility` | Full (CSS / SMIL) | Single Node | Fixed hard-coded waveforms; no custom timing, delay, or amplitude controls. |

---

## 4. Architectural Analysis & Key Findings

### What Already Works Well
1. **Authoritative Engine Ownership:** Go scene models (`pkg/engine/scene/`) authoritatively own identity, coordinates, naming, history, and serialization.
2. **Standardized Parameter Contract:** `<object-name>-<object-type>-<property>` reliably bridges Go models and external widget runtimes.
3. **Safe Effect Targeting:** Runtime isolations in `svg-widget.js` prevent external recoloring or opacity changes from destroying invisible track paths (`.flow-track-path`) or transparent hit targets (`.glow-hit-target`, `.ripple-hit-target`).
4. **Deterministic Timeline Evaluation:** `pkg/engine/timeline/evaluator.go` provides exact, reproducible math for time $T$, decoupling playback from browser frame rate.

### Inconsistencies & Architectural Friction
1. **Three Competing Animation Playback Strategies:**
   - **Strategy A (SMIL):** Flows, Glows, and Ripples generate declarative SMIL elements (`<animateMotion>`, `<animate>`) that loop automatically in the browser independent of timeline transport.
   - **Strategy B (Go Evaluator + RAF Patch):** Camera, Optics, and Timeline Scrubber evaluate in Go/WASM and patch DOM properties (`camRig.style.transform`, `feGaussianBlur.stdDeviation`) via `requestAnimationFrame`.
   - **Strategy C (CSS Keyframes):** Node pulse/float animations and camera video exports use CSS `@keyframes`.
   *Consequence:* In the Studio editor, when the user pauses the timeline player, Flows, Glows, and Ripples keep running because SMIL animations do not pause unless `svg.pauseAnimations()` is explicitly synchronized.

2. **Attachment Model Disparity:**
   - Nodes are anchored shapes with bounding boxes.
   - Flows anchor their start and end to Nodes, but midpoints are independent.
   - Glows and Ripples are strictly unattached canvas-coordinate points `(X, Y)`. They cannot move with a parent Node or along a Flow path.
   - Images are independent rectangular objects without native effect attachments.

3. **Rigid Object-Type Catalogs:**
   - `ObjectPropertyCatalog` in `pkg/engine/scene/widget.go` is a static map keyed strictly by object type (`node`, `flow`, `glow`, `ripple`, `image`, `group`, `root`).
   - If a new animation type is added (e.g. `border-tracer`, `pulse-glow`, `path-morph`), it currently requires adding a dedicated Go struct, scene map, CRUD methods, history serialization, and catalog entries across multiple files.

4. **Authored Properties vs. Widget Controllable Properties:**
   - Currently, adding an authored property to a Go struct does not automatically make it eligible for widget export.
   - There is no formal metadata schema in Go describing which struct fields can be exposed externally, what their valid ranges are, and how the widget runtime should apply them.

---

## 5. Separation of Concerns: The Three-Tier Property Model

To ensure that internal animation capabilities can expand freely without breaking or overloading external widget consumers, the architecture must maintain three distinct property tiers:

```
+-----------------------------------------------------------------------------------+
|                           THE THREE-TIER PROPERTY MODEL                           |
+-----------------------------------------------------------------------------------+
|                                                                                   |
|   Tier 1: Authored Properties (Studio Design Layer)                              |
|   - Internal settings used to author the animation in SVG Animator Studio.        |
|   - Persisted in Go scene models and .sva project packages.                       |
|   - Examples: waypoint curvature, radar ring stagger, fillet depth, blur radius. |
|                                   |                                               |
|                                   v                                               |
|   Tier 2: Externally Controllable Properties (Capability Layer)                   |
|   - Properties that the SVG markup structure and svg-widget.js runtime            |
|     can reliably mutate without corrupting artwork or helper elements.            |
|   - Defined in Go ObjectPropertyCatalog with strict type and limit constraints.   |
|   - Examples: payload color, flow speed, label text, wave opacity.                |
|                                   |                                               |
|                                   v                                               |
|   Tier 3: Exposed Widget Properties (Manifest & Host Binding Layer)               |
|   - The specific subset of Tier 2 properties that the author has chosen           |
|     to expose for a given project.                                                |
|   - Exported in widget-manifest.json with object-name-first identifiers.          |
|   - Bound to host inputs, REST endpoints, or Digital Twin telemetry.             |
|                                                                                   |
+-----------------------------------------------------------------------------------+
```

---

## 6. The Proposed Minimal Extension Architecture

Rather than introducing an experimental or parallel framework, the extension model builds upon the verified Go/WASM authoritative pattern:

```text
               [ Go Engine: pkg/engine/ ]
               1. Portable Data Model (scene/model.go)
               2. Validation & Defaults (scene/animation.go)
               3. Timeline Evaluation (timeline/evaluator.go)
               4. Widget Property Descriptor (scene/widget.go)
                           |
                           v
               [ WASM Bridge: cmd/wasm/ & engine-bridge.js ]
               5. Versioned Protocol Dispatch (protocol/commands.go)
                           |
                           v
               [ JS Presentation Adapter: internal/web/static/js/ ]
               6. DOM / SVG Markup Builder (Declarative SMIL + CSS)
               7. Context Editor Inspector & Compact Property Cards
                           |
                           v
               [ Standalone Runtime: svg-widget.js ]
               8. Scoped ID / Target Resolution & Property Application
```

### Necessary Changes vs. Optional Enhancements

#### Necessary (Minimal Foundation for Extensibility)
1. **Unified Animation Descriptor Contract (`pkg/engine/scene/`):**
   A structured interface or metadata definition in Go that registers:
   - Target entity types the animation can attach to (`node`, `flow`, `image`, `canvas`).
   - Authored property schema (types, defaults, min/max limits).
   - Widget export eligibility rules.
2. **Generic Target-to-Element Semantic Tagging:**
   Enforce that all SVG generators tag visual elements with:
   - `data-semantic-target="{targetId}"`
   - `data-semantic-name="{objectName}"`
   - `data-semantic-role="payload | guide | hit-target | container"`
   *(This eliminates hard-coded per-shape queries in `svg-widget.js`).*
3. **Editor Timeline Play/Pause Synchronization:**
   Harmonize editor transport so clicking "Pause" calls `svg.pauseAnimations()` on the canvas, aligning live editor SMIL behavior with the Go timeline evaluator.

#### Optional / Future Phases (Not Required for Initial Extension)
- Dynamic expression/scripting bindings.
- Multi-channel timeline keyframe editor.
- Arbitrary shape deformation / path tweening engine.

---

## 7. Assessment of Future Animation Capabilities

The following capabilities were assessed against the proposed extension architecture:

1. **Boundary & Perimeter Tracer Animations (e.g. dashed glow running around a Node border):**
   - *Feasibility:* High.
   - *Mechanism:* Extracted directly from `ShapeRegistry.buildShapeSVG` path data. Emits a stroke with `stroke-dasharray` and SMIL `dashoffset` animation.
   - *Widget Exposure:* Exposes `color`, `speed`, and `opacity`.
2. **Moving Effects Following Arbitrary Paths (e.g. particles orbiting a node or along freehand spline):**
   - *Feasibility:* High.
   - *Mechanism:* Leverages existing `geometry.SampleFlowPathAtProgress` math with `<animateMotion>`.
3. **Parent-Child Object Attachment (e.g. Glow attached to Node center):**
   - *Feasibility:* Moderate.
   - *Mechanism:* Add optional `ParentID StringID` to `scene.Glow`. If present, Glow coordinates dynamically derive from `(node.X + node.Width/2, node.Y + node.Height/2)`.
4. **Independent Speed Multipliers per Animation Instance:**
   - *Feasibility:* Implemented and verified in Stage 16B for Flows; directly applicable to future animation types.

---

## 8. Proposed Phased Implementation Sequence

To maintain continuous stability and avoid broad regressions, future animation work should proceed in small, self-contained batches:

```
+----------------------------------------------------------------------------------------------------------+
|                                    PROPOSED IMPLEMENTATION SEQUENCE                                      |
+----------------------------------------------------------------------------------------------------------+
|  Batch A: Semantic Target Role Standardisation & Editor Playback Harmonisation                            |
|  - Scope: Tag SVG output with data-semantic-role; sync TimelinePlayer pause/play with SMIL.              |
|  - Impact: internal/web/static/js/ (flows.js, glows.js, ripples.js, timeline.js, svg-widget.js)          |
|  - Pre-requisites: None (Builds directly on Stage 16B baseline).                                         |
+----------------------------------------------------------------------------------------------------------+
|                                                    |                                                     |
|                                                    v                                                     |
+----------------------------------------------------------------------------------------------------------+
|  Batch B: Object-Attached Optical Hotspots (Parent-Child Anchoring)                                      |
|  - Scope: Allow Glows to anchor to Nodes, following node drag and transform.                             |
|  - Impact: pkg/engine/scene/glow.go, flows.js, context-editor.js                                          |
|  - Pre-requisites: Batch A.                                                                              |
+----------------------------------------------------------------------------------------------------------+
|                                                    |                                                     |
|                                                    v                                                     |
+----------------------------------------------------------------------------------------------------------+
|  Batch C: Shape Boundary & Perimeter Path Animation Engine                                               |
|  - Scope: Introduce perimeter motion effects around arbitrary Shape Registry geometries.                 |
|  - Impact: pkg/engine/geometry/shapes.go, pkg/engine/scene/model.go, nodes.js, svg-widget.js             |
|  - Pre-requisites: Batch A.                                                                              |
+----------------------------------------------------------------------------------------------------------+
```

---

## 9. Risk Assessment

| Risk Area | Severity | Mitigation Strategy |
| :--- | :--- | :--- |
| **Stage 16B Widget Regressions** | High | All new animation parameters must strictly adhere to the established `<object-name>-<object-type>-<property>` naming and pass `ValidateWidgetExport`. |
| **SMIL vs. RAF DOM Conflicts** | Medium | Use `data-semantic-role="payload"` to separate declarative SMIL animated elements from procedural RAF transforms. |
| **Export File Size Growth** | Low | Standalone SVG export must reuse shared `<defs>` and parametric builders rather than embedding bloated inline animation scripts. |
| **Persistence Compatibility** | High | All new scene struct fields in Go must use `omitempty` and maintain backward compatibility with `schema_json` v1.2.0. |
| **Multi-Instance Collision** | High | All newly generated `<filter>`, `<mask>`, or `<path>` IDs must pass through Go `IsolateSVGWidget` prefixing. |

---

## 10. Summary Findings

1. **What already works architecturally:** The Stage 16B widget framework, naming standardisation, pre-export target validation, and multi-instance runtime isolation are solid and reliable.
2. **What is inconsistent:** Playback synchronization between continuous browser SMIL elements and the discrete Go timeline evaluator.
3. **What genuinely needs refactoring:** Semantic role tagging in SVG generation to cleanly identify payload vs. guide vs. hit-target elements across all types.
4. **What can be extended without refactoring:** Adding new properties to existing types in `ObjectPropertyCatalog` and extending `applyPropertyToElement` in `svg-widget.js`.
5. **Which small implementation batch should come first:** **Batch A** (Semantic Target Role Standardisation & Editor Playback Harmonisation).
6. **What should explicitly remain untouched:** The accepted Stage 16B public parameter contract, project serialization formats (v1.2.0), and the Go/WASM authority boundary.
