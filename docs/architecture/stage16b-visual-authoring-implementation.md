# SVG Animator Studio — Stage 16B Completion: Visual Widget Property Bindings Inside Editor

**Starting Git Commit:** `b2de129`  
**Completion Date:** 2026-11-15  
**Subsystem Owners:** `pkg/engine/scene/`, `pkg/engine/protocol/`, `internal/web/static/js/`  
**Target Environment:** Kiwiberry (`192.168.100.40`), binary `/home/nigel/svg-animator`

---

## 1. Architectural Summary

Stage 16B established the external widget control protocol and client-side runtime API. This completion phase implements full visual authoring of widget parameter bindings directly inside SVG Animator Studio, eliminating any need for manual SVG editing or JSON manifest crafting.

### Three-Layer Responsibility

1. **Go Authoritative Engine (`pkg/engine/scene/widget.go`, `scene.go`):**
   - Owns parameter schemas, target associations, default values, min/max limits, and validation.
   - Enforces strict security validation: rejects script injection (`onclick`, `onload`, `href`, `javascript:`), verifies alphanumeric naming (`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`), validates numeric bounds (`min <= max`), and checks type-property compatibility.
   - Automatically prunes bound parameters when target objects (nodes, flows, glows, ripples, images) are deleted from the scene.
   - Auto-embeds the Stage 16B parameter manifest `<script type="application/json" id="{prefix}sva-widget-manifest">` during widget isolation if parameters exist.

2. **WebAssembly Protocol Boundary (`pkg/engine/protocol/commands.go`, `engine-bridge.js`):**
   - Typed commands: `Widget.GetParameters`, `Widget.SetParameters`, `Widget.AddParameter`, `Widget.RemoveParameter`, `Widget.IsolateSVG`.
   - Atomic error reporting and rollback contracts.

3. **Presentation & UI Adapter (`context-editor.js`, `app.js`, `workspace.js`, `svg-widget.js`):**
   - **Floating Properties Editor Section:** Context-sensitive `#fctx-widgetSection` inside `#floatingContextEditor` displayed when any object is selected. Enables external exposure, property selection, parameter naming, default specification, limits/enum configuration, and validation status feedback.
   - **Right Drawer Accordion Overview:** `#acc-widget-params` showing all exposed parameters in the current scene, live interactive preview controls (colors, range sliders, checkboxes, dropdowns, text inputs) with instant on-canvas feedback, default resets (`↺`), and binding deletion.
   - **Namespace Isolation & Scoping:** Multi-instance ID rewriting, CSS scoping, and group target property propagation in `svg-widget.js`.

---

## 2. Supported Properties & Parameter Types

| Parameter Type | Permitted Properties | Supported Targets | Description & Limits |
|---|---|---|---|
| `color` | `fill`, `stroke`, `color` | Node, Flow, Glow, Ripple | Hex color string (`#rrggbb`) |
| `number` | `height`, `width`, `opacity`, `stroke-width`, `radius` | Node, Flow, Glow, Ripple | Bounded numeric range with configurable `min`, `max`, `step` |
| `boolean` | `visibility` | Node, Flow, Glow, Ripple, Image | Boolean toggle (`true` / `false` -> `visible` / `hidden`) |
| `text` | `text`, `textContent` | Textbox, Node labels | UTF-8 text string |
| `enum` | `state`, `filter` | Root, Node | Restricted set of permitted string options |
| `speed` | `playbackRate` | Root, Flow, Glow, Ripple | Animation playback speed multiplier (`0.1` to `5.0`) |

### Prohibited / Blocked Properties (Security Validation)
- Any script handler: `onclick`, `onmouseover`, `onload`, `onerror`, `onfocus`
- URI targets: `href`, `xlink:href`, `action`, `src`
- Inline script / style injection: `<script>`, `javascript:`, `expression()`

---

## 3. Authoring Workflow

1. Select any Node, Flow, Glow, Ripple, or Image on the canvas.
2. In the Floating Properties Editor (`#floatingContextEditor`), locate **Widget / External Controls**.
3. Toggle **Expose as Widget Parameter**.
4. Choose the target property from the dropdown.
5. Provide a public external parameter name (e.g. `tankColor`, `liquidLevel`, `statusText`).
6. Configure default value, numeric range (`min`/`max`/`step`), or enum options as appropriate.
7. Click **💾 Save Binding**.
8. View the live listing in the **Widget Parameters** drawer accordion (`#acc-widget-params`).
9. Use the preview controls to verify visual responsiveness without altering authored defaults.
10. Click **🚀 Production Render & Downloads -> Download Reusable SVG Widget**.
11. The exported SVG automatically embeds the complete isolated JSON manifest.

---

## 4. SQLite Persistence & Project Round-Trip

Widget bindings are stored in the project schema JSON (v1.2.0) under `parameters: [...]`.
- Saving via `/api/export/projects/save` persists all bindings to `studio_library.db`.
- Reopening via `/api/export/projects/get?id=...` hydrates `parameters` into the Go/WASM engine authoritative state.
- SVA package export/import preserves parameters in `manifest.json` and `scene.json`.

---

## 5. Automatic Widget Properties Documentation & No-Code Widget Package

### Automatic Properties Guide Generation (`pkg/engine/scene/widget_docs.go`)
- **Authoritative Data Source:** Directly queries `sc.nodes`, `sc.glows`, `sc.flows`, `sc.ripples`, `sc.images`, `sc.canvas`, and `sc.parameters` from the Go scene model.
- **Generated File:** `<widgetName>-properties.md`
- **Contents:**
  1. Title, versioning (Scene Schema `1.2.0`, Protocol `1.0.0`), and metadata.
  2. Overview & Architecture (canvas dimensions, viewBox, scoping model).
  3. Visual Object Inventory Table (target IDs, names, types, coordinates, layers, bound parameters).
  4. Host-Controllable Parameter Specification (name, type, target, property, default, range, units, description).
  5. Animation & Transport capabilities.
  6. Ready-to-Run No-Code Controls instructions.
  7. Developer Integration & JavaScript API reference with real code examples using the exported parameter names.

### Ready-to-Run No-Code Control Page (`demo.html`)
- **Generated File:** `demo.html`
- **Operation:** Completely independent of SVG Animator Studio once unzipped; relies only on bundled `js/svg-widget.js`.
- **UI Architecture:** Dynamic UI generator reading parameter types to render:
  - Range sliders with live unit labels for bounded numbers.
  - Color pickers for hex colors.
  - Checkboxes for booleans.
  - Dropdown selectors for enumerations.
  - Text inputs for labels and textual content.
- **Multi-Instance Isolation:** Pre-mounts 3 independent instances (`Unit A: Optimal`, `Unit B: Warning`, `Unit C: Critical Alarm`) side-by-side with distinct parameter values and live visual feedback on the actual mounted SVGs.
- **Transport Controls:** Play, Pause, and Reset to Authored Defaults.

### Portable Widget Package Export (ZIP)
- **Archive Contents:**
  ```text
  <widgetName>/
  ├── <widgetName>.svg
  ├── <widgetName>-properties.md
  ├── widget-manifest.json
  ├── js/svg-widget.js
  ├── README.md
  └── demo.html
  ```
- **Integrity:** Standard PKZIP archive structure verified with zero external dependencies.

