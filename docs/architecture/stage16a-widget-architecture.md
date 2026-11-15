# SVG Animator Studio — Stage 16A Architecture Specification
## Reusable SVG Widget Architecture, Data Flow & Scoping Contracts

**Date:** 2026-11-15  
**Document ID:** `ARCH-STAGE-16A`  
**Milestone:** Stage 16A — Reusable, Self-Contained and Collision-Free SVG Widgets  
**Protected Release:** `v1.2.0`  

---

## 1. Architectural Model & Layers

Stage 16A strictly adheres to the project's permanent architectural principle:

```text
Authoring UI (HTML/CSS)
         │
         ▼
Presentation Adapters (JavaScript: app.js, export-adapter.js, svg-widget.js)
         │
         ▼
Authoritative Scene & Geometry Engine (Go / WASM: pkg/engine/scene/widget.go)
         │
         ▼
Validated Isolated SVG Widget Artifact
         │
         ▼
Independent Browser Presentation (Light DOM or Shadow DOM)
```

### Layer Separation
1. **Layer 1 (UI Presentation):**
   - Studio Export UI in `#acc-export` with `#btnDownloadWidgetSVG`.
   - Demonstration page `internal/web/static/demo-widgets.html` providing live multi-instance host page integration.
2. **Layer 2 (Coordination & Adapters):**
   - `internal/web/static/js/export-adapter.js`: Orchestrates export packaging and delegates widget isolation to the Go/WASM engine with fallback.
   - `internal/web/static/js/engine-bridge.js`: Bridges JavaScript calls across the WebAssembly boundary to Go engine protocol command `Widget.IsolateSVG`.
   - `internal/web/static/js/svg-widget.js`: Zero-dependency UMD client library providing runtime embedding, parsing, mounting, and unmounting on arbitrary host documents.
3. **Layer 3 (Authoritative Operational Logic):**
   - `pkg/engine/scene/widget.go`: Go authoritative transformer implementing deterministic namespace allocation, ID remapping, reference rewriting, CSS rule scoping, and keyframe isolation.
   - `pkg/engine/protocol/commands.go`: Versioned `Widget.IsolateSVG` command dispatch.

---

## 2. Namespace & Reference Rewriting Contract

### 2.1 Instance ID & Prefix Allocation
- Each widget instance is assigned a unique `instanceId` (e.g. `inst-alpha`, `widget-123456`, or auto-generated `sva-w-[timestamp]-[random]-[seq]`).
- Prefix `P` defaults to `${instanceId}__`.
- Root widget container ID is defined as `sva-widget-${instanceId}`.
- Root `<svg>` element receives attributes:
  ```xml
  <svg id="sva-widget-[instanceId]" data-sva-widget-instance="[instanceId]" ...>
  ```

### 2.2 ID Mapping Table Construction
Before modifying markup, the transformer performs an initial inspection pass collecting all declared IDs:
```text
idMap = { [originalId]: `${prefix}${originalId}` }
```
Elements declaring IDs include:
- `<clipPath id="...">`
- `<radialGradient id="...">` / `<linearGradient id="...">`
- `<filter id="...">`
- `<mask id="...">`
- `<pattern id="...">`
- `<marker id="...">`
- `<path id="...">`
- `<g id="...">`
- `<image id="...">`
- `<text id="...">`
- Non-root `<svg id="...">`

### 2.3 Reference Rewriting Rules
During the transformation pass, attributes referencing element IDs are rewritten:

1. **`url(#...)` References:**
   - Any attribute (`fill`, `stroke`, `filter`, `clip-path`, `mask`, `marker-start`, `marker-mid`, `marker-end`, `style`) containing `url(#oldId)` or `url('#oldId')` is rewritten to `url(#[newId])` if `oldId` is present in `idMap`.
   - Direct color values (`#0d1117`, `#38bdf8`) do not match `url(#...)` syntax and remain untouched.
2. **`href` and `xlink:href` Fragment References:**
   - On `<use>`, `<mpath>`, `<textPath>`, `<animate>`, and `<set>` elements, values starting with `#` are resolved against `idMap`:
     ```xml
     <mpath href="#flowPath-1" xlink:href="#flowPath-1" />
     ──►
     <mpath href="#inst1__flowPath-1" xlink:href="#inst1__flowPath-1" />
     ```
   - Values NOT starting with `#` (e.g. `data:image/png;base64,...` or `https://example.com/asset.png`) are preserved verbatim.
3. **SMIL Event Timing References (`begin`, `end`):**
   - References like `begin="anim1.begin"` or `end="flowPath1.end+2s"` are rewritten to `begin="inst1__anim1.begin"`.
4. **Accessibility Attributes (`aria-labelledby`, `aria-describedby`):**
   - Space-separated ID tokens are remapped according to `idMap`.
5. **Arbitrary Text Preservation:**
   - Text node contents (e.g. `<text>Node #1 Title</text>`) are never parsed as attribute references and remain unmodified.

---

## 3. CSS Scoping & Keyframe Isolation Contract

Browsers treat `<style>` blocks in inline SVG as document-level styles. Stage 16A solves stylesheet collisions using a two-tier mechanism:

### 3.1 Keyframe Namespacing
1. All `@keyframes` declarations are identified:
   ```text
   keyframeMap = { [oldKeyframeName]: `${prefix}${oldKeyframeName}` }
   ```
2. Definitions are updated:
   ```css
   @keyframes camKenBurns { ... }
   ──►
   @keyframes inst1__camKenBurns { ... }
   ```
3. References in `animation` and `animation-name` properties are updated:
   ```css
   animation: camKenBurns 6s ease-in-out infinite;
   ──►
   animation: inst1__camKenBurns 6s ease-in-out infinite;
   ```

### 3.2 Rule Selector Scoping
1. Rules are scoped to the root SVG element `#sva-widget-[instanceId]`:
   ```css
   .camera-rig { transform: ...; }
   ──►
   #sva-widget-inst1 .camera-rig { transform: ...; }
   ```
2. ID selectors in CSS are updated with their namespaced IDs:
   ```css
   #flowPath-1 { stroke: ...; }
   ──►
   #sva-widget-inst1 #inst1__flowPath-1 { stroke: ...; }
   ```
3. `url(#oldId)` inside CSS property bodies (e.g. `fill: url(#glowGrad-1);`) are remapped to `url(#inst1__glowGrad-1)`.

---

## 4. Standalone Embedding & Lifecycle Contract (`svg-widget.js`)

The client library provides a clean, dependency-free API for third-party host documents:

### API Specification
```javascript
// 1. Pure isolation of SVG string
const isolatedSvgString = window.SVGWidget.isolate(rawSvg, {
  instanceId: 'widget-sensor-alpha', // Optional, auto-generated if omitted
  prefix: 'alpha__',                 // Optional, defaults to `${instanceId}__`
  scopeCss: true                     // Optional, defaults to true
});

// 2. Direct DOM mount into container element
const handle = window.SVGWidget.mount('#widget-container-1', rawSvg, {
  instanceId: 'widget-sensor-alpha',
  shadowDOM: false,                  // Set true for Shadow DOM encapsulation
  width: '100%',
  height: 'auto'
});

// 3. Widget handle properties
handle.instanceId; // 'widget-sensor-alpha'
handle.prefix;     // 'alpha__'
handle.rootId;     // 'sva-widget-widget-sensor-alpha'
handle.idMap;      // { 'viewportClip': 'alpha__viewportClip', ... }
handle.svgElement; // SVGElement DOM node
handle.container;  // HTMLElement container

// 4. Lifecycle cleanup
handle.unmount();  // Removes widget wrapper and elements from DOM
```

---

## 5. Scope Boundaries

- **Stage 16A (Implemented):** Reusable SVG widgets, multi-instance co-existence, element ID isolation, reference remapping, CSS scoping, keyframe namespacing, independent playback, and reproducible demonstration.
- **Stage 16B (Deferred):** External parameter APIs, host-driven speed/color controls, public command protocols.
- **Stage 16C (Deferred):** Digital twins, live MQTT/WebSocket telemetry ingestion, sensor alarms.
