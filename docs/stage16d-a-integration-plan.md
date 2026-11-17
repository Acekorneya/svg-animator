# Stage 16D-A — Architecture Integration Audit & Persistent Implementation Plan

## Status & Baseline
- **Current Stage:** Stage 16D-A (Corrective First-Class Object Integration Pass)
- **Phase Gate:** **Phase 2 — Awaiting Explicit Project-Owner Approval (Planning Only)**
- **Git Branch & Baseline Commit:** `main` @ `01735aa` (builds on `830c866` Stage 16D-A initial implementation, `1ea7e23` Stage 16C, `41118bf` Stage 16B, and `v1.2.0` release baseline)
- **Deployed Environment:** Kiwiberry (`192.168.100.40`):
  - Studio development server: `http://192.168.100.40:8088/`
  - Stage 16C independent consumer harness: `http://192.168.100.40:8091/stage16c-consumer/index.html`
  - Stage 16D benchmark harness: `http://192.168.100.40:8092/stage16d-benchmark/index.html`

---

## A. Current Architecture

SVG Animator Studio enforces a strict three-layer architecture with Go/WASM as the single authoritative engine:

```text
Go Engine (Authoritative: pkg/engine/scene, geometry, protocol, sva)
   ──► WASM Bridge (cmd/wasm, internal/web/static/js/engine-bridge.js)
   ──► Coordinator & Layer/Property Adapters (coordinator.js, layers-ui.js, context-editor.js, app.js)
   ──► Browser SVG & Widget Runtime (scene-state.js, svg-widget.js)
```

### How Established First-Class Studio Objects Work (Stages 1–16C)
1. **Go/WASM Authoritative State & Identity (`pkg/engine/scene/`):**
   - Objects (`DiagramNode`, `DataFlow`, `GlowEffect`, `RippleEffect`, `ImageObject`) are created, validated, updated, and deleted in `Scene` with monotonic ID counters (`nodesCounter`, etc.), z-ordered `LayerEntry` slices (`s.layers`), and full `Snapshot` serialization (`GetState` / `SetState`, `HistoryManager` undo/redo, and `.sva` packaging).
2. **Coordinate System & Bounding Geometry:**
   - First-class positioned objects (`DiagramNode`, `GlowEffect`, `RippleEffect`, `ImageObject`) use **center-based coordinates** (`x`, `y`) decoupled from dimensions (`width`, `height` or `size`/`outerR`), so changing width or height expands/contracts symmetrically around `(x, y)` without translating the object's center.
3. **Unified Layers Management (`engine-bridge.js` `LayerManager` + `layers-ui.js`):**
   - `LayerManager` synchronizes `s.layers` with Go (`Layer.GetAll`, `Layer.SetAll`, `Layer.BringToFront`, `Layer.SendToBack`, `Layer.BringForward`, `Layer.SendBackward`, `Layer.SetVisibility`, `Layer.SetLocked`, `Layer.GetEffective`).
   - `layers-ui.js` renders every layer in reverse z-order inside `#layersListContainer`, supports click/Shift/Ctrl selection, double-click to open `#floatingContextEditor`, visibility/lock toggles, and synchronizes per-inspector layer controls (`syncInspectorLayerSections`).
4. **Selection, Hit Testing & Bounding Outlines (`coordinator.js`, `stage.js`, `nodes.js`):**
   - `AppCoordinator.selectObject(type, id)` coordinates single and multi-selection across sidebars, Layers, and `#floatingContextEditor`.
   - `AppCoordinator.updateActiveOverlays()` invokes `DiagramNodes.updateUIOverlay()`, which positions the dashed purple `#nodeSelectionBox` (`stroke="#a371f7"`) inside `#trajectorySvg` around the active object's bounding box and renders multi-selection outlines in `#multiSelectionGroup`.
5. **Custom Colour Picker & Eyedropper (`color-picker.js`):**
   - `window.CustomColorPicker` intercepts clicks on any `input[type="color"]` via document event delegation (`color-picker.js:159`) and opens `#customColorPickerModal` with HSV square, Hue/RGB sliders, HEX input, swatches, and `#btnColorPickerPipette`.
6. **Three-Tier Widget Property System (Stages 16B & 16C):**
   - **Tier 1 (Go-authored):** Full object model stored in `Scene`.
   - **Tier 2 (Runtime-eligible):** Defined in `ObjectPropertyCatalog` (`pkg/engine/scene/widget.go`) and queried via `EngineBridge.getEligibleWidgetProperties(type)`.
   - **Tier 3 (Author-exposed):** Selected by the user in `#fctx-widgetSection` (Floating Context Editor) or `#acc-widget` (Right Drawer) via `window.renderWidgetPropertyCards` (`app.js:2732`), stored in `Scene.widgetParameters`, validated by `ValidateWidgetExport`, emitted into `widget-manifest.json` + `*-properties.md`, and driven at runtime by `svg-widget.js` (`setParameter` / `setParameters`) and the Stage 16C consumer.

---

## B. Integration Gap Matrix

| Area | Existing API / Mechanism | Current Lite Implementation | Gap / Root Cause of Defect | Proposed Correction |
|---|---|---|---|---|
| **1. Authoritative State, Undo/Redo & Persistence** | `Scene` (`pkg/engine/scene/lite_effect.go`), `EngineBridge` (`LiteEffect.*`), `HistoryManager`, `ProjectManager` | `LiteEffect` exists in Go `Scene` and `Snapshot`, but `LiteEffects.setState` in `lite-effects.js` only updates `this.list` in JS without syncing back to Go `Scene` when `ProjectManager.applyStudioState` runs outside `Scene.SetState`, and lacks new fields (`width`/`height` for Pulse, `maintainAspectRatio`, `minScale`/`maxScale`, `showGuide`). | Partial Go/JS desync after project reload; missing Pulse/Tracer/Flash fields in Go `LiteEffect` struct and `UpdateLiteEffect`. | Extend `scene.LiteEffect` & validation in Go; ensure `LiteEffects.setState` syncs to Go `EngineBridge`; ensure all mutations record undo history via `AppCoordinator`. |
| **2. Layers Panel & Z-Ordering** | `LayerManager` (`engine-bridge.js:4394`), `layers-ui.js` (`renderLayersList`, `updateLayersToolbarState`, `syncInspectorLayerSections`) | Go `CreateLiteEffect` appends `SelectionLite` (`"lite"`) to `s.layers`, **but** JS `LayerManager` (`ensureAllObjectsRepresented`, `setVisibility`, `setLocked`, `getEffective`) and `layers-ui.js` hardcode `['node', 'flow', 'glow', 'ripple', 'image']` and omit `'lite'`. Furthermore, `LiteEffects.createLiteEffect` in JS does not call `LayerManager.syncFromEngine()` or `LayerManager.addEntry('lite', id)`. | **Lite effects never appear properly in Layers**, cannot be reordered/hidden/locked from Layers, and do not show a layer icon/name (`⚡ Tracer #1`, etc.). | Add `'lite'` to `LayerManager` (`ensureAllObjectsRepresented`, `setVisibility`, `setLocked`, `getEffective`), `layers-ui.js` (`updateLayersToolbarState`, `renderLayersList`, `syncInspectorLayerSections`), and `#liteLayerSection` in `#acc-lite-effects`. |
| **3. Selection Outlines & Bounding Boxes** | `AppCoordinator.updateActiveOverlays()` (`coordinator.js:1154`), `DiagramNodes.updateUIOverlay()` (`nodes.js:692`), `#nodeSelectionBox` in `#trajectorySvg` | `updateActiveOverlays()` only updates overlays for `group`, `node`, `flow`, `glow`, `ripple`, and `image`. When `activeSelection.type === 'lite'`, `#nodeSelectionBox` is hidden or stale. | Selected Lite effects have **no selection bounding box** on the stage; users cannot tell which Lite instance is selected when multiple instances exist. | Extend `LiteEffects.getBoundingBox(le)` and wire `activeSelection.type === 'lite'` into `updateActiveOverlays()` / `DiagramNodes.updateUIOverlay()` so `#nodeSelectionBox` and `#multiSelectionGroup` frame the selected Lite effect(s) accurately. |
| **4. Perimeter Tracer Geometry & Width/Height Bug** | Center-based `(x, y)` coordinates + independent `(width, height)` in `DiagramNodes` | In `lite-effects.js:274-288`: (1) `(x, y)` is treated as top-left for `rect`/`rounded`, shifting the center when `width`/`height` change; (2) for `circle`, `cx = x + w/2`, `cy = y + h/2`, and `crR = Math.min(w, h)/2`. Because default `w=140, h=80`, changing `Width` (`w > 80`) keeps `Math.min(w, 80)/2 = 40` constant and **only changes `cx = x + w/2`**, making the Width slider move the circle left/right! Also lacks an optional visible perimeter guide path. | **Width control moves the tracer horizontally instead of resizing it**, Height has inconsistent clamped behaviour, and there is no visible static boundary/guide option. | Convert `LiteEffect` `(x, y)` to **center coordinates** across all kinds (`perimeter-tracer`, `pulse`, `flash`). For `circle`, synchronize `width` and `height` (diameter `d = width = height`, `r = d/2`) around `(x, y)`. For `rect`, `rounded`, and `ellipse`, render centered at `(x - w/2, y - h/2)` with independent `width` and `height`. Add `showGuide` (default `true` subtle perimeter track in Studio/export, plus editor-only guide when hidden). |
| **5. Pulse Lite Dimensions, Aspect Lock & Pulse Range** | `ImageObjects` (`width`, `height`, `maintainAspectRatio`) + `Glows` | `Pulse Lite` only uses a single `radius` scalar and hardcoded `@keyframes sva-lite-pulse` (`scale(0.86)` to `scale(1.12)`). For `diamond`, polygon vertices `cx±r, cy±r` span `2r` without accounting for `width`/`height`. | Cannot author wide/narrow/rectangular pulses, lock/unlock aspect ratio, or configure pulse expansion/contraction range (`minScale` / `maxScale`), and diamond bounds/dimensions are rigid. | Support `width`, `height`, `maintainAspectRatio`, `minScale` (default `0.85`), and `maxScale` (default `1.15`) on `pulse`. Render `circle`/`ellipse`, `square`/`rect`, `diamond` (vertices at `cx ± w/2`, `cy ± h/2` so bounding box equals `w × h` exactly), `cross`, and `ring` using `width × height` and CSS variables `--sva-pulse-min` / `--sva-pulse-max`. |
| **6. Flash Lite Real-Time ms Duration & Editor Marker** | `scene-state.js` (`isExport` vs live preview rendering) | Flash `dur` is authored in seconds (`0.08–5.0s`, default `0.22s`), and when `repeat=false` the payload has `opacity: 0` after the one-shot flash finishes, leaving **nothing visible on the canvas**. | Between flashes, a Flash Lite object is **completely invisible** on the canvas and impossible to locate visually; duration control is in coarse seconds instead of 10 ms increments (`50–300 ms` normal range, max `500 ms`, default `100 ms`). | Author and display Flash duration in **milliseconds** (`durationMs`: default `100 ms`, step `10 ms`, slider `50–500 ms`, persisted in Go as `Duration: 0.10`s / `DurationMs: 100`). Render an **editor-only marker/outline & badge** (`.lite-editor-marker`) in Studio preview that is stripped/omitted in standalone SVG/widget export (`isExport`). Keep `✦ Trigger Flash` button in both drawer and Context Editor. |
| **7. Colour Picker & Eyedropper Regression** | `window.CustomColorPicker` (`color-picker.js:104-118`) | `color-picker.js:105` checks `if (!window.EyeDropper) { btnPipette.style.display = 'none'; }`. Because Chromium restricts `window.EyeDropper` to `[SecureContext]` (`https://` or `localhost`), accessing the Studio on Kiwiberry via `http://192.168.100.40:8088/` causes `window.EyeDropper` to be `undefined`, hiding `#btnColorPickerPipette` completely! | **The eyedropper button disappears completely** on `http://192.168.100.40:8088/` (and non-Chromium browsers). | Never hide `#btnColorPickerPipette`. When native `window.EyeDropper` is available, use it; when unavailable (such as plain HTTP LAN deployment `http://192.168.100.40:8088/`), provide an **interactive stage/canvas & DOM colour-sampling eyedropper fallback** that lets the user click any SVG element, canvas pixel, or UI swatch to sample its exact hex colour into `CustomColorPicker`. |
| **8. Context Editor & Multi-Instance Targeting** | `FloatingPropertiesEditor` (`context-editor.js`) + `AppCoordinator` | `context-editor.js` `_syncWidgetControls()` (`line 1694`) checks `if (!["node", "flow", "glow", "ripple", "image"].includes(this.activeType))` (omitting `"lite"`) and `syncFromActiveObject()` does not call `this._syncWidgetControls()` at the end of the `"lite"` branch. Also `#nodeSelectionBox` hit resolution (`_resolveTargetObject` line 1731) omits `"lite"`. | Context Editor **hides `#fctx-widgetSection`** for Lite effects, and clicking inside `#nodeSelectionBox` does not resolve to the selected Lite effect. | Add `"lite"` to `_syncWidgetControls()`, `_resolveTargetObject()`, and `syncFromActiveObject()`. Filter displayed controls by `le.kind` and `le.variant` so meaningless controls are never shown. |
| **9. Three-Tier Widget Property Exposure & Stage 16C Runtime** | `ObjectPropertyCatalog["lite"]` (`pkg/engine/scene/widget.go:449`), `app.js` (`getObjectPropValue`, `renderWidgetPropertyCards`, `renderWidgetParametersOverview`), `svg-widget.js` | `app.js` (`getDrawerWidgetPropDefs` fallback, `getObjectPropValue`, `renderWidgetPropertyCards`, `renderWidgetParametersOverview`) only handles `node`, `flow`, `glow`, `ripple`, `image`, and `group` — **omitting `lite`**! And `ObjectPropertyCatalog["lite"]` does not filter or describe kind-relevant properties (`trigger` & `duration` for Flash, `scale`/`intensity` for Pulse, `speed`/`dash` for Tracer). | **No usable way in the UI to choose which Lite properties are exposed to an exported widget**, and `getObjectPropValue('lite', ...)` returns `""`. | Wire `'lite'` into `app.js` (`getObjectPropValue`, `renderWidgetPropertyCards`, `renderWidgetParametersOverview`) and `context-editor.js` (`#fctx-widgetSection`). Expand `ObjectPropertyCatalog["lite"]` and `svg-widget.js` so author-selected properties (`color`, `opacity`, `visibility`, `speed`, `trigger`, `duration`, `scale`) are selectable in UI, validated, exported to `widget-manifest.json`, and controllable in `svg-widget.js` & Stage 16C consumer. |

---

## C. Proposed Implementation

### 1. Existing Components and APIs to Reuse
- **Go `Scene` & WASM Protocol (`pkg/engine/scene/`, `pkg/engine/protocol/`, `cmd/wasm/`):** Reuse `LiteEffect`, `CreateLiteEffect`, `UpdateLiteEffect`, `DeleteLiteEffect`, `LayerEntry{Type: SelectionLite}`, `ObjectPropertyCatalog`, `GenerateParameterName`, and `ValidateWidgetExport`.
- **Unified Layer System (`engine-bridge.js` `LayerManager` & `layers-ui.js`):** Reuse `LayerManager` methods and `#layersListContainer` / `syncInspectorLayerSections` rather than creating any Lite-specific layer list.
- **Stage Selection Box (`nodes.js` `updateUIOverlay` & `#nodeSelectionBox`):** Reuse `#nodeSelectionBox` and `#multiSelectionGroup` in `#trajectorySvg` for Lite selection outlines and multi-selection bounds.
- **Custom Colour Picker (`color-picker.js`):** Reuse `#customColorPickerModal` and `#btnColorPickerPipette` for all `<input type="color">` fields across Studio and Lite effects, adding a built-in stage/DOM eyedropper fallback when `window.EyeDropper` is blocked by non-HTTPS context.
- **Stage 16B Widget Exposure UI (`context-editor.js` `#fctx-widgetSection` & `app.js` `renderWidgetPropertyCards`):** Reuse the exact same checkbox cards, auto-generated `<objectName>-lite-<property>` badges, default editors, and `#acc-widget` overview list used by Nodes, Flows, Glows, and Ripples.

### 2. Minimal Extensions Required
- **`pkg/engine/scene/model.go` & `lite_effect.go`:**
  - Treat `X, Y` as center coordinates (`default X: 640, Y: 360` or `240, 180` in canvas pixels).
  - Add `MaintainAspectRatio bool` (`json:"maintainAspectRatio,omitempty"`), `MinScale float64` (`json:"minScale,omitempty"`, default `0.85`), `MaxScale float64` (`json:"maxScale,omitempty"`, default `1.15`), and `ShowGuide *bool` (`json:"showGuide,omitempty"`, default `true`) to `LiteEffect`.
  - For `flash`, set default `Duration = 0.10` (`100 ms`), valid range `0.01..1.50` seconds (`10..1500 ms`, UI slider `50..500 ms` in `10 ms` steps).
- **`pkg/engine/scene/widget.go` & `internal/web/static/js/svg-widget.js`:**
  - Support eligible `lite` runtime properties in `ObjectPropertyCatalog["lite"]`:
    - Common: `color` (`color`), `opacity` (`number`, `0..1`), `visibility` (`boolean`)
    - Tracer/Pulse timing: `speed` (`speed`, `0.1..10.0`)
    - Pulse range: `scale` (`number`, `0.5..2.5`, scales `--sva-pulse-max`)
    - Flash control: `trigger` (`number`, `0..1000000`, increments nonce to fire one-shot flash), `duration` (`number`, `20..1000` ms, updates `animation-duration` in ms)
  - Update `GetEligibleProperties` (or UI filtering in `renderWidgetPropertyCards`) so only properties applicable to the selected Lite effect's `kind` are offered in the UI (e.g., `trigger` and `duration` only for `flash`; `scale` only for `pulse`; `speed` for `perimeter-tracer` and `pulse`).

### 3. Files Expected to Change (After Approval)
1. `pkg/engine/scene/model.go` — `LiteEffect` struct fields (`MaintainAspectRatio`, `MinScale`, `MaxScale`, `ShowGuide`).
2. `pkg/engine/scene/lite_effect.go` — Defaults (`Duration = 0.10`, `MinScale = 0.85`, `MaxScale = 1.15`, center `X, Y`), patch handling, and validation.
3. `pkg/engine/scene/widget.go` — `ObjectPropertyCatalog["lite"]` refinements (`scale`, `duration`) and default extraction.
4. `pkg/engine/scene/lite_effect_test.go` — Go unit tests for new fields, center geometry, Flash ms duration, Pulse scale range, and widget catalog validation.
5. `internal/web/static/index.html` — `#acc-lite-effects` and `#fctx-liteSection` controls (Flash ms slider `50–500 ms` step `10 ms`, Pulse Width/Height/Lock Aspect + Min/Max Scale sliders, Tracer `Show Guide` toggle + Corner Radius, `#liteLayerSection` z-order/vis/lock bar).
6. `internal/web/static/js/lite-effects.js` — Center-based `(x, y)` geometry for all variants, circle `width=height` sync, diamond `width × height` vertex math, Pulse `--sva-pulse-min`/`--sva-pulse-max`, Flash editor-only marker (`.lite-editor-marker` omitted when `isExport` is true), `getBoundingBox(le)`, and Go/LayerManager sync.
7. `internal/web/static/js/engine-bridge.js` — `LayerManager` (`'lite'` support in `ensureAllObjectsRepresented`, `setVisibility`, `setLocked`, `getEffective`).
8. `internal/web/static/js/layers-ui.js` — `'lite'` support in `updateLayersToolbarState`, `renderLayersList` (icon/name by kind), and `syncInspectorLayerSections` (`Lite` section).
9. `internal/web/static/js/coordinator.js` & `internal/web/static/js/nodes.js` & `internal/web/static/js/stage.js` — `#nodeSelectionBox` and `#multiSelectionGroup` overlay rendering for `'lite'`, interactive resize handle support on `#nodeSelectionBox` for `'lite'`, and `coordinatePostRestore` selection restoration.
10. `internal/web/static/js/color-picker.js` — Keep `#btnColorPickerPipette` visible and implement interactive stage/SVG/DOM colour sampling fallback when `window.EyeDropper` is unavailable on plain HTTP (`http://192.168.100.40:8088/`).
11. `internal/web/static/js/context-editor.js` & `internal/web/static/js/app.js` — `#fctx-widgetSection` & `renderWidgetPropertyCards` / `getObjectPropValue` / `renderWidgetParametersOverview` integration for `'lite'`, filtered by `le.kind`.
12. `internal/web/static/js/svg-widget.js` — Runtime handlers for `scale` and `duration` (ms) on `lite-*` targets alongside `color`, `opacity`, `visibility`, `speed`, and `trigger`.
13. `scripts/regression-harness.js` & `scripts/cmd/verify-stage16d-a/main.go` — Automated regression and 20-scenario real-browser CDP verification suite.

---

## D. Work Packages and Dependencies

### WP1 — Go Engine Model, Geometry Math & Widget Catalog Extensions
- **Dependencies:** None.
- **Deliverables:**
  - Extend `scene.LiteEffect` (`MaintainAspectRatio`, `MinScale`, `MaxScale`, `ShowGuide`, center `X, Y`, Flash `Duration` default `0.10`s = `100 ms`).
  - Update `ObjectPropertyCatalog["lite"]` in `pkg/engine/scene/widget.go`.
  - Rebuild `internal/web/static/wasm/engine.wasm`.
- **Acceptance Criteria & Tests:** `go test ./pkg/engine/...` passes 100% with unit tests covering new fields, bounds, and widget catalog eligibility.

### WP2 — Unified Layers, Selection Outlines & Resize Integration
- **Dependencies:** WP1.
- **Deliverables:**
  - Add `'lite'` to `LayerManager` (`engine-bridge.js`) and `layers-ui.js` (`renderLayersList`, `updateLayersToolbarState`, `syncInspectorLayerSections`, `#liteLayerSection`).
  - Implement `LiteEffects.getBoundingBox(le)` and wire `'lite'` into `AppCoordinator.updateActiveOverlays()`, `DiagramNodes.updateUIOverlay()`, and `Stage` resize/drag handlers so selecting a Lite effect on canvas or in Layers shows the purple `#nodeSelectionBox` with accurate bounds and updates Layers selection bidirectionally.
- **Acceptance Criteria & Tests:** Creating, selecting (canvas or Layers), reordering, hiding, locking, renaming, and deleting Lite effects works identically to Nodes/Glows/Ripples.

### WP3 — Geometry & Authoring Fixes (Perimeter Tracer, Pulse Lite, Flash Lite)
- **Dependencies:** WP1, WP2.
- **Deliverables:**
  - **Perimeter Tracer Lite:** Center `(x, y)` coordinates; fix Width/Height bug (`circle` links `width = height = diameter` centered at `(x, y)`; `rect`, `rounded`, `ellipse` use `x - w/2, y - h/2` with independent `width`, `height`); add visible perimeter guide track (`showGuide`) + moving segment preview.
  - **Pulse Lite:** Editable `width`, `height`, `maintainAspectRatio` checkbox, `minScale` (`50%–100%`, default `85%`) and `maxScale` (`100%–200%`, default `115%`) sliders; fix `diamond` polygon vertices (`cx ± w/2, cy ± h/2`) so visual bounds match `width × height` and `#nodeSelectionBox`.
  - **Flash Lite:** Real-time duration in **milliseconds** (`50–500 ms` slider, `10 ms` steps, `100 ms` default); persistent editor-only marker/outline (`.lite-editor-marker`) when inactive so Flash is always visible and clickable in the editor but excluded from exported widgets; `✦ Trigger Flash` preview button in drawer and Context Editor.
- **Acceptance Criteria & Tests:** Changing Tracer Width/Height resizes in place without translating `x`/`y`; Pulse aspect ratio and min/max scale work across all 5 variants; Flash is visible/selectable when inactive and exports without the editor marker.

### WP4 — Custom Colour Picker & Eyedropper Restoration
- **Dependencies:** None (can run after WP3).
- **Deliverables:**
  - Fix `color-picker.js` so `#btnColorPickerPipette` is never hidden when `window.EyeDropper` is absent on plain HTTP (`http://192.168.100.40:8088/`).
  - Add interactive stage/SVG/DOM colour-sampling mode when native `window.EyeDropper` is unavailable, while preserving native `window.EyeDropper` on HTTPS/localhost.
- **Acceptance Criteria & Tests:** Clicking any colour input on Nodes, Flows, Glows, Ripples, or Lite effects opens `#customColorPickerModal` with a visible, working `#btnColorPickerPipette` eyedropper button.

### WP5 — Three-Tier Widget Property Exposure & Stage 16C Consumer Integration
- **Dependencies:** WP1, WP3.
- **Deliverables:**
  - Enable `#fctx-widgetSection` in `context-editor.js` and `renderWidgetPropertyCards` / `getObjectPropValue` / `renderWidgetParametersOverview` in `app.js` for `'lite'`, filtering eligible properties by `le.kind`.
  - Update `svg-widget.js` to support `scale` and `duration` alongside `color`, `opacity`, `visibility`, `speed`, and `trigger`.
- **Acceptance Criteria & Tests:** Author can expose Lite properties via checkboxes in Context Editor / Right Drawer, export a widget package, and independently control multiple Lite instances in `demo.html` and the Stage 16C consumer (`8091`).

### WP6 — 20-Scenario Real-Browser CDP Acceptance Suite, Deployment & Documentation
- **Dependencies:** WP1–WP5.
- **Deliverables:**
  - Extend `scripts/cmd/verify-stage16d-a/main.go` to execute all 20 required real-browser user interaction scenarios (real mouse clicks, drags, Layers selection, multi-instance isolation, Flash inactive selection + ms duration, Pulse min/max + aspect ratio, Tracer width/height without translation, colour picker + eyedropper, UI property exposure, save/reload, Stage 16C consumer control, visibility-aware scroll suspension, and existing Node/Flow/Glow/Ripple regression checks) and capture verification screenshots.
  - Deploy to Kiwiberry (`192.168.100.40:8088`) using non-hanging `ssh -f` / `setsid` and update BookStack page `466`.

---

## E. Regression Risks & Mitigations

| Risk Area | Potential Failure Mode | Mitigation |
|---|---|---|
| **Existing Flow, Glow, Ripple & Node editing** | Modifying `#nodeSelectionBox` or `LayerManager` could alter node/image selection overlays or layer ordering. | Keep `node` and `image` branches in `DiagramNodes.updateUIOverlay` untouched; add `lite` as an explicit branch when `activeSelection.type === 'lite'`. Run all 88 scenarios in `scripts/regression-harness.js`. |
| **Global Play Scene vs Independent Animations** | CSS `@keyframes` or Flash trigger could interfere with timeline scrubbing or Play Scene. | Keep Lite animations on scoped CSS classes (`.sva-lite-tracer-anim`, `.sva-lite-pulse-anim`, `.sva-lite-flash-anim`) without calling blanket `svg.pauseAnimations()`. |
| **Existing Saved Projects (`.sva` & SQLite)** | Center-coordinate migration or new `LiteEffect` fields could break older snapshots. | All new JSON fields use `omitempty` and safe defaults in `applyLiteEffectDefaults` (`MinScale: 0.85`, `MaxScale: 1.15`, `Duration: 0.10`, `ShowGuide: true`). |
| **Colour Picker Event Delegation** | Eyedropper sampling click on stage could accidentally trigger stage object selection or close the colour picker popover prematurely. | Use capture-phase `pointerdown`/`click` listener with `e.preventDefault(); e.stopPropagation();` while `CustomColorPicker.isSamplingPipette` is active, and ignore outside-click cancel during sampling. |
| **Stage 16B/16C Widget Export Validation** | Exposing a property on `lite-1` could fail `ValidateWidgetExport` if target resolution or catalog lookup mismatches. | Keep `ObjectPropertyCatalog["lite"]` in `pkg/engine/scene/widget.go` as the single source of truth for both UI property cards (`EngineBridge.getEligibleWidgetProperties('lite')`) and `ValidateWidgetExport`. |

---

## F. Progress and Verification Log

| Work Package | Status | Commit | Automated Tests | Browser Verified | Notes |
|---|---|---|---|---|---|
| **Phase 1–2: Architecture Audit & Plan** | **Implemented (Awaiting Approval)** | *(this commit)* | N/A (Planning only) | N/A | Audited all Stage 16B/16C/16D-A files; documented root causes & 6 work packages. |
| **WP1: Go Engine Model & Catalog** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |
| **WP2: Layers, Selection & Outlines** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |
| **WP3: Tracer, Pulse & Flash Fixes** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |
| **WP4: Colour Picker & Eyedropper** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |
| **WP5: Widget Property Exposure** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |
| **WP6: 20-Scenario CDP Verification** | Not started | — | Pending | Pending | Blocked on Phase 2 approval gate. |

---

## G. Session Recovery Instructions

If a new Antigravity session resumes this task:
1. **Current Stage:** Stage 16D-A (Corrective First-Class Object Integration). Do **NOT** start Stage 16D-B.
2. **Last Completed Action:** Phase 1 & Phase 2 (Architecture Audit and Persistent Plan committed in `docs/stage16d-a-integration-plan.md`).
3. **Approval Gate:** Check whether the project owner has explicitly approved this plan. If approved, execute **WP1 → WP2 → WP3 → WP4 → WP5 → WP6** in order, updating Section F after each work package.
4. **Tests to Run on Every Package:**
   - `go test ./...`
   - `node scripts/regression-harness.js`
   - `go run ./scripts/cmd/verify-stage16d-a/main.go` (against `http://192.168.100.40:8088/`, `8091`, and `8092`)
5. **Deployment Command (Non-Hanging SSH):**
   - Build: `GOOS=js GOARCH=wasm go build -o internal/web/static/wasm/engine.wasm ./cmd/wasm && go build -o /tmp/svg-animator ./cmd/svg-animator`
   - Deploy: `ssh nigel@192.168.100.40 "pkill -x svg-animator || true" && scp /tmp/svg-animator nigel@192.168.100.40:/home/nigel/svg-animator && ssh -f nigel@192.168.100.40 "setsid /home/nigel/svg-animator -serve :8088 </dev/null >/home/nigel/svg-animator.log 2>&1 &"`
   - Verify: `curl -sI http://192.168.100.40:8088/ | head -n 1`
   - Protected services: Do not touch ports `8091` (Stage 16C consumer), `8092` (Stage 16D benchmark), `studio_library.db`, or `release/`.
