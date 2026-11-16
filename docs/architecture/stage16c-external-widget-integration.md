# Stage 16C — External Data-Driven Widget Integration

## 1. Overview

Stage 16C proves that **SVG Animator Studio** exported widget packages (`<slug>.svg`, `widget-manifest.json`, `js/svg-widget.js`) can be consumed by an independent host application and driven by changing external data without relying on Studio editor internals, Go/WASM runtime, or undocumented DOM structures.

The reference consumer (`demos/stage16c-widget-consumer/`) mounts three isolated widget instances (`host-a`, `host-b`, `host-c`) from complete Studio-authored packages (`host-monitor-v1` and `host-monitor-v2`) and binds simulated host telemetry (`hostname`, `online`, `cpu`) to the exported Stage 16B parameters.

---

## 2. Independent Consumer Architecture

The reference consumer (`demos/stage16c-widget-consumer/consumer.js` and `index.html`) is strictly separated from SVG Animator Studio:

- **Zero Studio Dependencies:** Does not load `engine-bridge.js`, `coordinator.js`, `nodes.js`, `scene-state.js`, `engine.wasm`, or Studio DOM elements.
- **Public Contract Only:** Interacts exclusively with:
  - `window.SVGWidget` (`js/svg-widget.js` from the exported package)
  - `widget-manifest.json` (parameter schema and package identity)
  - `<slug>.svg` (isolated SVG artwork with embedded manifest)
- **Component Breakdown:**
  1. `validateWidgetPackageConsistency(pkg)` — Verifies manifest schema, SVG integrity, runtime availability, and `packageId` / parameter-list parity between `widget-manifest.json` and the SVG's embedded `<script id="...sva-widget-manifest">`.
  2. `validateMappingsAgainstManifest(mappingSpec, manifest)` — Ensures all consumer-configured parameter names exist in the active package manifest before mounting.
  3. `mapHostRecordToWidgetParameters(host, mappingSpec, manifest, options)` — Pure application-level transform mapping external host metrics onto exposed widget parameters.
  4. `SimulatedHostDataSource` — Emits three independent host records (`pve01`, `pve02`, `pve03`) with periodic updates, online/offline transitions, zero values, and missing values.
  5. `WidgetInstanceManager` — Manages mount, live in-place `setParameters()` updates (without remounting), error isolation, and clean `destroy()` / `unmount()` lifecycle per host instance.

---

## 3. Exported Widget Runtime API & Instance Lifecycle

Each mounted host instance uses the standard Stage 16B `SVGWidget` handle:

```javascript
// 1. Mount isolated instance into a host container
const handle = widgetRuntime.mount(containerEl, svgText, {
  instanceId: 'consumer-host-a-1',
  manifest: manifest,
  parameters: initialParams
});

// 2. Live parameter updates without remounting
handle.setParameters({
  'hostname-node-text': 'pve01',
  'traffic-flow-speed': 2.5,
  'status-glow-color': '#22c55e'
});

// 3. Read current parameter state
const current = handle.getParameters();

// 4. Clean teardown
handle.destroy();
```

### Multi-Instance Isolation & Intentional Playback Semantics
- Every mounted instance receives a unique `instanceId` prefix, ensuring zero ID collisions across SVG `<defs>`, `<radialGradient>`, `<path id="flowPath-1">`, and `<mpath>` references.
- Flow, Glow, and Ripple SMIL animations continue running independently per instance. Global `svg.pauseAnimations()` is never invoked across unrelated effects; individual visibility/opacity/speed changes are applied strictly via each instance's exposed parameters.

---

## 4. Host Data-to-Parameter Mappings

The reference consumer uses the `host-monitor-v1` package generated directly by `scene.BuildWidgetPackageZip`, exposing 10 object-name-first parameters:

| External Host Field | Mapped Widget Parameter (`v1`) | Type | Mapping Behaviour |
| :--- | :--- | :--- | :--- |
| `host.hostname` + `online` | `hostname-node-text` | `text` | Displays `hostname` (or `hostname [OFF]` / `hostname [NO CPU]`) |
| `host.online` | `traffic-flow-visibility` | `boolean` | `true` when online, `false` when offline |
| `host.online` | `heartbeat-ripple-visibility` | `boolean` | `true` when online, `false` when offline |
| `host.online` + `cpu` | `hostname-node-fill` | `color` | Slate `#1e293b` (normal), `#3b1518` (CPU $\ge$ 80%), `#2a1215` (offline) |
| `host.online` + `cpu` | `hostname-node-stroke` | `color` | `#22c55e` (<50%), `#f59e0b` (50–79%), `#ef4444` ($\ge$80% or offline) |
| `host.cpu` | `traffic-flow-color` | `color` | Reflects load tier (`#38bdf8` idle, `#22c55e`, `#f59e0b`, `#ef4444`) |
| `host.cpu` | `traffic-flow-speed` | `speed` | Scales `0.5` to `4.0` (`min: 0.2`, `max: 5.0`) |
| `host.cpu` | `status-glow-color` | `color` | Reflects status/load or amber warning when CPU is missing |
| `host.cpu` | `status-glow-opacity` | `number` | Scales `0.35` to `1.0` |
| `host.cpu` | `heartbeat-ripple-opacity` | `number` | Scales `0.3` to `1.0` (`0.0` when offline) |

---

## 5. Missing Data vs. Valid Zero Behaviour

The consumer explicitly preserves the distinction between `cpu === 0` and `cpu === null | undefined`:
- **Valid Zero (`cpu === 0`, `online: true`):** Rendered as a healthy idle host (`#38bdf8` cyan glow, `traffic-flow-speed: 0.5`, `hostname-node-text: "pve01"`).
- **Missing Value (`cpu === null`, `online: true`):** Handled according to the consumer's selectable missing-data strategy without altering Studio's widget runtime:
  - **`warn` (default):** Displays amber `#f59e0b` indicator and `"pve01 [NO CPU]"`.
  - **`treat-as-zero`:** Maps missing CPU identically to `0`.
  - **`hide`:** Sets `traffic-flow-visibility: false` and `heartbeat-ripple-visibility: false`, dimming glow opacity to `0.15`.

---

## 6. Package Version & Consistency Contract

To prevent accidental mixing of files across widget exports, `WidgetManifest` (`pkg/engine/scene/model.go`) includes backward-compatible `omitempty` metadata populated during `BuildWidgetPackageZip`:
- `packageVersion` (e.g. `"1.0.0"`)
- `packageId` (deterministic hash `<slug>-<sha256[:6]>`, e.g. `"host-monitor-v1-76948473a9ef"`)
- `widgetName` (clean slug)

Before mounting, `validateWidgetPackageConsistency()` verifies that `widget-manifest.json`'s `packageId`, `widgetName`, and parameter list match the manifest embedded inside `<slug>.svg`. Switching to `host-monitor-v2` (which renames `hostname` $\rightarrow$ `server` and removes `heartbeat-ripple-visibility`) with stale `v1` mappings is explicitly rejected with a clear mapping error.

---

## 7. Integration Boundaries (Sentinel-PVE & Digital Twin)

- **Stage 16C Scope:** Uses only an in-memory `SimulatedHostDataSource` with no network calls to Sentinel-PVE or Digital Twin backends.
- **Future Integration Boundary:** A future Digital Twin presenter can replace `SimulatedHostDataSource` with a telemetry feed while reusing the exact same `validateWidgetPackageConsistency`, `validateMappingsAgainstManifest`, and `SVGWidget.mount` / `setParameters` lifecycle.

---

## 8. Test Evidence & Known Limitations

### Automated & Real-Browser Verification
- **Go Tests:** `go test ./...` — 100% PASS.
- **JS Regression Harness:** `node scripts/regression-harness.js` — 87/87 scenarios PASS (including Scenario 87 covering Stage 16C package consistency, stale mapping rejection, 3-instance isolation, zero vs. missing handling, invalid parameter error handling, unmount/remount, and v2 switching).
- **Real-Browser CDP Acceptance (`scripts/cmd/verify-stage16c-consumer/main.go`):** 9/9 criteria PASS on Kiwiberry (`http://192.168.100.40:8091/index.html`).

### Known Limitations
1. Offline `file://` execution of `index.html` is subject to browser `fetch()` CORS restrictions when loading sibling JSON/SVG files; serve over HTTP (e.g. `python3 -m http.server`) for the multi-package switcher, whereas each exported package's own `demo.html` remains directly runnable over `file://`.
2. Animations remain bounded by the Stage 16B catalog (no perimeter tracers or Node-attached Glows yet, as evaluated in the Animation Architecture Review).
