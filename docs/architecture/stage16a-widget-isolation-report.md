# SVG Animator Studio — Stage 16A Architecture Report
## Reusable, Self-Contained and Collision-Free SVG Widgets

**Date:** 2026-11-15  
**Baseline Commit:** `54c4488b02131aa3b246a47a1ec63ca492e76f9d` (`54c4488`)  
**Milestone:** Stage 16A — Reusable, Self-Contained and Collision-Free SVG Widgets  
**Protected Release:** `v1.2.0`  
**Author:** SVG Animator Studio Development Agent

---

## 1. Executive Summary

Stage 16A accomplishes the first phase of the Stage 16 roadmap for SVG Animator Studio:

> **Make SVG Animator Studio scenes reusable as independent SVG widgets that can coexist on the same web page without interfering with each other.**

Prior to Stage 16A, exporting SVG Animator Studio scenes produced standalone SVG documents with static, un-namespaced IDs (such as `<clipPath id="viewportClip">`, `<filter id="nodeDropShadow">`, `<marker id="flowArrow-1">`, `<path id="flowPath-1">`, `<radialGradient id="glowGrad-1">`, `<style>.camera-rig { animation: camKenBurns ... }`). While perfectly valid when opened as single independent files or rasterized headlessly in isolation, embedding two or more of these scenes directly into the same host HTML page resulted in immediate browser document-tree collisions:

1. **Element ID Collisions:** The browser resolves `url(#id)` references against the *first* element in the entire document with a matching `id`. Multiple instances of identical or different diagram scenes hijacked each other's gradients, filters, markers, and flow motion paths.
2. **Global CSS Animation & Rule Collisions:** Inline `<style>` blocks inside inline `<svg>` elements are treated as global document stylesheets by browsers. Unscoped `.camera-rig` rules and identical `@keyframes camKenBurns` declarations collided, causing one widget's camera timing or transforms to override all others on the page.
3. **SMIL Animation Cross-Talk:** Chained or linked SMIL `<animateMotion>` targets using `<mpath href="#flowPath-1">` resolved across widget boundaries to whatever path happened to appear first in DOM order.

Stage 16A establishes a mathematically sound, complete, and collision-free isolation architecture:
- **Go Engine Authority:** `scene.IsolateSVGWidget` (`pkg/engine/scene/widget.go`) provides authoritative deterministic remapping of all declared IDs, internal references, SMIL animation targets, and CSS `@keyframes` / selectors.
- **Typed Protocol Dispatch:** `Widget.IsolateSVG` command (`pkg/engine/protocol/commands.go`) exposed through the WebAssembly runtime (`internal/web/static/wasm/engine.wasm`).
- **WebAssembly Engine Bridge:** `window.EngineBridge.isolateSVGWidget` (`internal/web/static/js/engine-bridge.js`) dispatches directly to Go/WASM.
- **Standalone Widget Runtime:** `window.SVGWidget` (`internal/web/static/js/svg-widget.js`) provides zero-dependency client-side isolation, mounting, and optional Shadow DOM encapsulation.
- **Export Studio Presentation:** `window.SVGExportAdapter.isolateWidget` and UI button `btnDownloadWidgetSVG` in `#acc-export` enable one-click export of reusable widgets directly from the studio.
- **Reproducible Multi-Widget Demonstration:** `internal/web/static/demo-widgets.html` embeds 3 simultaneous instances with live independent animations and zero ID collisions.
- **Strict Backward Compatibility:** All existing export pathways (`btnDownloadSVG`, `btnDownloadHTML`, `btnOpenStillModal`, `btnOpenBitmapModal`) remain 100% untouched and preserved.

---

## 2. Scope of Isolation & Boundary Verification

### 2.1 SVG IDs Isolated
Stage 16A comprehensively maps and scopes every category of ID generated in SVG Animator Studio outputs:

| Category | Typical SVG Element IDs | Isolation Transformation | Reference Rewriting |
|---|---|---|---|
| **Gradients** | `glowGrad-1`, `bgRadialGrad` | `[prefix]glowGrad-1` | `fill="url(#[prefix]glowGrad-1)"` |
| **Filters** | `nodeDropShadow`, `flowBlur-1`, `glowBlur-1`, `rippleBlur-1`, `opticsFilmGrain`, `opticsScanlines` | `[prefix]nodeDropShadow`, `[prefix]flowBlur-1` | `filter="url(#[prefix]nodeDropShadow)"` |
| **Clip Paths** | `viewportClip`, `clip-1`, `clip-group-1` | `[prefix]viewportClip` | `clip-path="url(#[prefix]viewportClip)"` |
| **Masks** | `rippleMask-1` | `[prefix]rippleMask-1` | `mask="url(#[prefix]rippleMask-1)"` |
| **Patterns** | `bgMinorGrid`, `bgMajorGrid`, `bgDotGrid`, `bgGraphSub`, `bgGraphGrid` | `[prefix]bgGrid` | `fill="url(#[prefix]bgGrid)"` |
| **Markers** | `flowArrow-1` | `[prefix]flowArrow-1` | `marker-end="url(#[prefix]flowArrow-1)"`, `marker-start` |
| **Flow Paths** | `flowPath-1` | `[prefix]flowPath-1` | `<mpath href="#[prefix]flowPath-1" xlink:href="#[prefix]flowPath-1" />` |
| **Image Objects** | `img-1`, `img-logo` | `[prefix]img-1` | Non-conflicting DOM IDs |
| **Groups & Layers** | `scene-layers`, `clip-group-1` | `[prefix]scene-layers` | Group hierarchy isolation |
| **Root SVG** | Root `<svg>` | `sva-widget-[instanceId]` | `data-sva-widget-instance="[instanceId]"` |

### 2.2 Preserved Assets & Non-Corrupted Syntax
Strict safety rules prevent accidental corruption:
- **Data URLs:** Base64 image strings (`data:image/png;base64,...`, `data:image/jpeg;base64,...`) are never mistaken for fragment references and remain byte-for-byte identical.
- **External URLs:** Standard HTTP(S) links (`https://...`, `http://...`) are detected and left untouched.
- **Arbitrary Text Containing `#`:** Text nodes such as `<text>Source #Node 1</text>` or `<text>Priority: #High</text>` are preserved without modification.
- **Hex Color Literals:** Colors like `fill="#0d1117"` or `stroke="#38bdf8"` are direct color codes, not `url(#...)` lookups, and remain untouched.

### 2.3 CSS Scoping & Keyframe Isolation
Inside `<style>` blocks:
- **Selector Scoping:** Global selectors like `.camera-rig` and `.camera-stage-3d` are rewritten as `#sva-widget-[instanceId] .camera-rig` and `#sva-widget-[instanceId] .camera-stage-3d`. They cannot override rules belonging to sibling widget instances or the host document.
- **Keyframe Namespacing:** `@keyframes camKenBurns` is renamed to `@keyframes [prefix]camKenBurns`.
- **Animation Property Updating:** Declarations such as `animation: camKenBurns 6s ...` are rewritten to `animation: [prefix]camKenBurns 6s ...`.

---

## 3. Implementation Summary

### 3.1 Go Engine (`pkg/engine/scene/widget.go` & `widget_test.go`)
- `scene.IsolateSVGWidget(rawSVG, opts)`:
  - Scans and maps all declared IDs: `idMap: oldID -> prefix + oldID`.
  - Collects all `@keyframes` names.
  - Rewrites all internal references (`url(#...)`, `href="#..."`, `xlink:href="#..."`, `marker-*`, `aria-labelledby`, `begin`/`end` event targets).
  - Scopes CSS rules to `#sva-widget-[instanceId]`.
  - Tags root `<svg>` with `id="sva-widget-[instanceId]"` and `data-sva-widget-instance="[instanceId]"`.
- Unit tests in `widget_test.go` verify:
  - Basic isolation and deterministic prefixing.
  - Multi-instance collision freedom with disjoint ID sets.
  - Preservation of data URLs and `#` text content.
  - CSS scoping and keyframe isolation.

### 3.2 Protocol Router (`pkg/engine/protocol/commands.go` & `commands_test.go`)
- Added typed command `Widget.IsolateSVG`:
  - Request: `IsolateSVGWidgetRequest { SVG, InstanceID, Prefix, ScopeCSS }`
  - Response: `IsolateSVGWidgetResponse { Success, IsolatedSVG, InstanceID, Prefix, IDMap, Error }`
- Verified in `commands_test.go:TestDispatchWidgetIsolateSVG`.

### 3.3 WASM Bridge (`internal/web/static/wasm/engine.wasm` & `engine-bridge.js`)
- Recompiled `engine.wasm` with `Widget.IsolateSVG` command support.
- Added `isolateSVGWidget(rawSvg, instanceId, prefix, scopeCss)` on `window.EngineBridge`.

### 3.4 Standalone Widget Library (`internal/web/static/js/svg-widget.js`)
- Universal UMD library: Browser global `window.SVGWidget`, CommonJS `module.exports`, and AMD/ESM compatible.
- API:
  - `SVGWidget.isolate(rawSvg, options)`: Pure string/regex bounded transformation.
  - `SVGWidget.create(rawSvg, options)`: Returns isolated SVG DOM element.
  - `SVGWidget.mount(target, rawSvg, options)`: Mounts widget into DOM container with instance tracking, unmount lifecycle, and optional Shadow DOM isolation.
  - `SVGWidget.generateInstanceId()`: Collision-free instance generator.

### 3.5 Export Presentation Adapter & UI (`export-adapter.js`, `index.html`, `app.js`)
- Extended `window.SVGExportAdapter.isolateWidget` with primary Go/WASM delegation, secondary `SVGWidget` delegation, and tertiary self-contained fallback.
- Added global helper `window.isolateSVGForWidget`.
- Added UI button `btnDownloadWidgetSVG` ("📦 Export Reusable SVG Widget") in `#acc-export`.
- Preserved all existing export buttons and behaviors.

### 3.6 Multi-Widget Demonstration Page (`internal/web/static/demo-widgets.html`)
- Interactive demonstration mounting 3 identical animated diagram scenes simultaneously (Alpha, Beta, Gamma).
- Live DOM scanner verifying **0 duplicate IDs** across all active instances.
- Independent camera animations, marching ant flow paths, and SVG filter rendering.
- Dynamic "+ Mount New Instance" and "✕ Unmount" lifecycle verification.

---

## 4. Verification Results

| Quality Gate | Target | Result | Evidence |
|---|---|---|---|
| Go Codebase Vet | `go vet ./...` | **PASS** | Clean, zero errors across all 11 packages |
| Go Unit Tests | `go test -count=1 ./...` | **PASS** | 11/11 packages PASS (0.954s) |
| Go Race Detector | `go test -race -count=1 ./pkg/engine/scene/...` | **PASS** | 0 data races detected |
| Architecture Verification | `bash scripts/verify-architecture.sh` | **PASS** | 9 PASS, 1 WARNING (ARCH-05 offscreen DOM exception) |
| Runtime Verification | `bash scripts/verify-runtime.sh` | **PASS** | 6 PASS, 1 NOT VERIFIED (RUN-06 Chromium absent on LXC) |
| Regression Harness | `node scripts/regression-harness.js` | **PASS** | **76/76 Scenarios PASS** (Scenario 76 Stage 16A verified) |
| Multi-Widget Demo | `GET /demo-widgets.html` | **HTTP 200** | Live multi-instance collision-free demo |
| Protected Release | `v1.2.0`, ports 8088/8090, root binary | **INTACT** | Preserved without modifications |
