# SVG Animation Engineering Guidance

**Status:** Active project guidance (based on M1–M6 validated SVG Animation Test Lab evidence)  
**Supporting M6 Report:** [testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md](../../testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md)  
**Authoritative BookStack Plan:** `https://notes.seaoffate.net/books/the-svg-animation-test-lab` (Book `468`, Page `470`)  
**Live Lab Viewer:** `http://192.168.100.40:8093/`

---

## 1. Evidence-Favoured Methods by Effect Family

| Effect Family | Favoured Method(s) | Notes & Dependencies |
|---|---|---|
| **`perimeter-tracer`** | **CSS `@keyframes` (`stroke-dashoffset`)** *(preferred existing implementation)* or **Shared `javascript-raf`** | CSS and shared JS RAF are competitive across `50–500` active instances. Keep CSS unless dynamic runtime control/suspension in mixed environments justifies JS RAF. `clip-reveal` is non-equivalent (`UNSUPPORTED_OR_NON_EQUIVALENT`). |
| **`pulse`** | **CSS `@keyframes`** or **Shared `javascript-raf`** | Indistinguishable within measurement noise at `N=100` (`opacity-only`, `scale-only`, `scale-opacity`). SMIL adds extra `<animate>` + `<animateTransform>` DOM nodes. |
| **`flash`** | **CSS `@keyframes`** or **Shared `javascript-raf`** | Verified for `140ms` repeating bursts at `1.0s` interval (`N=100`). |
| **`flow-line`** | **CSS `@keyframes` (`stroke-dashoffset`)** or **Shared `javascript-raf`** | Lower main-thread task work than SMIL at `N=100`. |
| **`flow-particle`** | **CSS `offset-path`** or **SMIL `<animateMotion>`** | Both substantially outperform `javascript-raf` (`getPointAtLength` per frame) on CPU and main-thread task duration. Use SMIL `<animateMotion>` where zero-JS standalone SVG compatibility is required. |
| **Static / Telemetry Gauges** | **Static SVG** + targeted **`data-driven` DOM updates** | Keep decorative animations separate from telemetry updates so off-screen suspension does not block data updates. |

---

## 2. What Recommendations Depend On

- **Environment tested:** Headless Chromium (`HeadlessChrome/155.0.8059.84`) on Linux (`Kiwiberry`, 2 cores) using the `SwiftShader` software renderer (`measurementSpecVersion: 2.0.0`).
- **RAF Hz vs. Presented FPS:** `rafHz` measures `requestAnimationFrame` callback cadence, **not** visually presented compositor FPS (`presentedFps` is `N/A` in headless SwiftShader CDP).
- **Hardware & Package Specificity:** Measured CPU, task time, and DOM-node saturation (such as `host-monitor-v1` dropping rAF at `250` active widgets) are specific to the tested widget package, geometry, and host environment—not universal fixed limits.

---

## 3. Known Limitations & Unverified Conditions

- **High-magnification visual smoothness (`NOT VISUALLY VERIFIED`):** Headless SwiftShader `60 Hz` rAF timing does not prove visual smoothness at `4×` display scaling on real headed displays.
- **Shared-SVG SMIL off-screen suspension (`UNSUPPORTED_OR_NON_EQUIVALENT`):** Individual `<g>` objects inside a single shared `<svg>` share one SVG document timeline and cannot be individually paused via SMIL without pausing sibling elements.
- **Observed mixed-dashboard SMIL resumption failure:** In `m6-07` (`m6-mixed-dashboard__smil__vis-12-suspended__n100`), SMIL failed `RESUMPTION_FAILED` after `scrollIntoView`. Treat this as an observed failure under that test configuration rather than claiming a proven fundamental browser limitation; avoid relying on per-element SMIL pause/resume in mixed dashboards without targeted verification.
- **Stage 16C widget motion coverage:** M4/M6 Stage 16C tests verify mounting, unique identity, `setParameter()` independence, suspension, and lifecycle stability; they do not run M1 per-frame inner-element motion sampling inside exported widgets.
- **Do not turn known limitations into automatic development tasks:** If a technique operates reliably within a documented range, stay within that range unless new product requirements justify engineering work.

---

## 4. Stability Over Marginal Gains

- A method with marginally lower CPU or task time in a synthetic test does **not** automatically justify replacing a working production Studio implementation.
- Preserve existing working CSS/SMIL/widget implementations unless a material requirement (correctness defect, scaling target, suspension need, or new effect family) justifies a change.

---

## 5. Deployment Models: Digital Twin vs. Independent Widgets

- **Shared Digital Twin Scene (`shared-dt-scene`):** One static flattened raster background (`<img>`) plus dynamic SVG `<g>` overlays in one shared `<svg>`. Minimises DOM node count (`~340–790` nodes for `100–250` overlays) and supports per-`<g>` CSS/JS-RAF off-screen suspension.
- **Independent SVG Roots (`independent-svg-roots`):** Separate `<svg>` per object (`~637` DOM nodes for `100` tracers); useful baseline when each object must be self-contained.
- **Complete Stage 16C Exported Widgets (`stage16c-widgets`):** Full Studio-exported packages (`widget-manifest.json`, `<widget>.svg`, `js/svg-widget.js`) with isolated runtime state and `setParameter()` APIs. DOM node count scales with widget complexity (`~9,542` nodes at `100` `host-monitor-v1` widgets; `~22,529` at `250`).

---

## 6. Visibility-Aware Suspension

- In `m6-05` (`shared-dt-scene`, `N=250`, 20 visible) and `m6-06` (`stage16c-widgets`, `N=250`, 12 visible), suspending off-screen decorative animations substantially reduced CPU (`44%–46%` in DT) and main-thread task work (`41%` in Stage 16C widgets, recovering rAF from `52.5 Hz` to `58.3 Hz`).
- Always keep `data-driven` / `setParameter()` telemetry updates active even when decorative animations are suspended off-screen.

---

## 7. Running the Test Lab for Future Decisions

**Prerequisites:** Go toolchain, Chromium CDP reachable (e.g. `127.0.0.1:9222`), and `/proc` access on `process_cpu_host` (if attributing browser process-tree CPU).

```bash
# 1. Validate YAML and preview expanded scenarios (no recompilation needed for YAML edits)
go run ./cmd/anim-test-lab validate -c testlab/configs/m6-01-baseline-all-families.yaml

# 2. Generate standalone visual samples & side-by-side catalogue
go run ./cmd/anim-test-lab samples -c testlab/configs/m6-01-baseline-all-families.yaml -out testlab/results/my-samples

# 3. Run a benchmark experiment
go run ./cmd/anim-test-lab run -c testlab/configs/m6-01-baseline-all-families.yaml -out testlab/results/my-run

# 4. Rerun an earlier experiment from its resolved-config.yaml
go run ./cmd/anim-test-lab rerun -dir testlab/results/my-run -out testlab/results/my-rerun

# 5. Inspect results in the persistent viewer (or start locally)
# Persistent Kiwiberry viewer: http://192.168.100.40:8093/
go run ./cmd/anim-test-lab serve -dir testlab/results -addr :8093
```

**Interpreting results:** Check `correctnessPassed` first (failing/non-equivalent/unsupported runs are excluded from rankings), inspect `processCpuSec` (`Mean ± StdDev`), `rafHz` (distinct from `presentedFps: N/A`), `taskDeltaMs`, `domNodes`, and `measurementSpecVersion` (`2.0.0` vs. pre-correction `HISTORICAL — UNVALIDATED METHODOLOGY`).
