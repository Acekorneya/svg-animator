# SVG Animation Test Lab — Milestone 6 Final Comparative Validation Report

**Measurement Spec Version:** `2.0.0` (M1–M5 validated methodology)  
**Test Host & Browser:** `HeadlessChrome/155.0.8059.84` on Kiwiberry (`192.168.100.40`, `Linux 7.0.0-31-generic x86_64`, `hardwareConcurrency: 2`, `SwiftShader` software WebGL/Vulkan renderer, `CLK_TCK=100`).  
**Compositor Presented FPS:** `N/A` (unavailable in headless/SwiftShader CDP; `rAF` callback Hz is reported separately and is **not** presented FPS).  
**Studies Completed:** 7 experiments (`m6-01` through `m6-07`), **124 scenarios**, **330 browser trials** (with 2–3 repetitions per scenario in alternating execution order).  
**Historical Archives:** Pre-correction runs in `testlab/results/` remain preserved and labelled `HISTORICAL — UNVALIDATED METHODOLOGY`; none are combined into M6 findings.

---

## 1. Executive Findings — Assumptions Challenged

| Assumption Tested | Outcome | Measured Evidence |
|---|---|---|
| *"CSS is always cheaper than JS RAF"* | **Contradicted** on tracer/pulse/flash/flow-line | At `N=500` tracers (`m6-02`), `javascript-raf` (`shared`) measured `1.687s` CPU / `488.3ms` task vs `css` `1.757s` CPU / `571.6ms` task; at `N=100`, both are within noise (`~1.53–1.63s`). |
| *"JS RAF is always cheapest"* | **Contradicted** on `flow-particle` | In `m6-01` (`N=100`), `flow-particle` JS RAF (`getPointAtLength` per frame) cost **`2.153s` CPU / `850.4ms` task** vs CSS `offset-path` (`1.563s` / `368.9ms`) and SMIL `<animateMotion>` (`1.567s` / `384.5ms`). |
| *"SMIL is inherently smoother or lighter"* | **Contradicted** | SMIL consistently incurred **`1.5×–3×` higher main-thread `TaskDuration`** (`386–890ms`) than CSS/JS-RAF (`122–572ms`), and per-object off-screen suspension is **unsupported** inside a shared SVG (`UNSUPPORTED_OR_NON_EQUIVALENT`). |
| *"Shared RAF always strongly outperforms per-instance RAF"* | **Small difference only** | In `m6-02` (`N=50–500`), shared RAF was only `0.03–0.09s` lower CPU than per-instance RAF (`1.687s` vs `1.717s` at `N=500`). |
| *"Off-screen suspension saves equal work across methods"* | **Contradicted** | In `m6-05` (`shared-dt-scene`, `N=250`, 20 visible), suspending 230 off-screen tracers cut CPU by **`46%` (`2.11s -> 1.14s`)** for CSS and **`44%` (`2.05s -> 1.15s`)** for JS-RAF, while SMIL cannot suspend per-`<g>` without pausing the whole `<svg>`. |
| *"100 Stage 16C widgets behave like 100 bare SVG roots"* | **Contradicted** | `100` bare tracers = `637` DOM nodes (`~220–274ms` task); `100` `host-monitor-v1` Stage 16C widgets = `9,542` DOM nodes (`392ms` task); `250` active widgets = `22,529` DOM nodes and **`rAF` drops to `52.5 Hz`** (`994ms` task), recovering to `58.3 Hz` with off-screen suspension. |
| *"4× display scaling causes detectable rAF drops in headless Chromium"* | **Inconclusive / Not Visually Verified** | All `0.5×`–`4.0×` scenarios (`viewBox`, `css-transform`, `authored-size`) maintained `59.9–60.0 Hz` rAF in headless SwiftShader; visual judder at `4×` on real displays is **NOT VISUALLY VERIFIED**. |

---

## 2. Effect-Family Recommendations & Practical Operating Limits

### 2.1 `perimeter-tracer`
- **Recommended Method:** **CSS `@keyframes` (`stroke-dashoffset`)** (or **Shared `javascript-raf`** when dynamic runtime control/suspension in mixed SVG/HTML is required).
- **Validated Operating Range:** `50–500` active instances (`m6-02`), `rectangle`, `rounded-rectangle`, `circle`, `ellipse` (`40×30` to `160×100`), `0.8s–4.0s` periods (`m6-03`), `1–4` segments, `1:3` and `1:1` ratios, `0.5×–4.0×` scale in headless SwiftShader (`m6-04`), and `100–250` overlays over `2560×1440` `shared-dt-scene` (`m6-05`).
- **Known Limitations:** `clip-reveal` is non-equivalent (`UNSUPPORTED_OR_NON_EQUIVALENT`); SMIL per-object off-screen suspension is unsupported in `shared-scene` / `shared-dt-scene`; visual smoothness at `4×` magnification on headed displays is **NOT VISUALLY VERIFIED**.
- **Practical Restrictions:** Avoid `clip-reveal` as a tracer substitute; do not use SMIL in `shared-dt-scene` if off-screen suspension is required.
- **Further Development:** **No further action** required within validated bounds.

### 2.2 `pulse`
- **Recommended Method:** **CSS `@keyframes`** or **Shared `javascript-raf`** (indistinguishable at `N=100`: `1.697s` vs `1.523s` CPU, `59.9 Hz` rAF).
- **Validated Operating Range:** `opacity-only`, `scale-only`, and `scale-opacity` modes (`m6-03`), `circle` and `diamond` (`36×36` to `50×50`), `100` active instances.
- **Known Limitations:** SMIL requires separate `<animate>` + `<animateTransform>` elements (`824` vs `524` DOM nodes at `N=100`) and fails per-`<g>` suspension in shared SVGs.
- **Further Development:** **No further action** required.

### 2.3 `flash`
- **Recommended Method:** **CSS `@keyframes`** or **Shared `javascript-raf`** (`1.237s` / `1.190s` CPU at `N=100`, `173.6ms` / `136.7ms` task).
- **Validated Operating Range:** `140ms` repeating bursts at `1.0s` interval (`N=100`), `star` 8-vertex polygon.
- **Known Limitations:** In `m6-07` (`mixed-dashboard`), SMIL `vis-12-suspended` failed `RESUMPTION_FAILED` upon `scrollIntoView` (short burst windows can miss a `350ms` post-resume probe if paused mid-off-phase).
- **Further Development:** **No further action** required.

### 2.4 `flow-line`
- **Recommended Method:** **CSS `@keyframes` (`stroke-dashoffset`)** or **Shared `javascript-raf`** (`1.613s` / `1.650s` CPU, `239.7ms` / `228.2ms` task at `N=100`; SMIL cost `1.957s` CPU / `697.3ms` task).
- **Validated Operating Range:** `straight` and `curved` paths (`100×40`), `1.6s` period, `100` active instances.
- **Further Development:** **No further action** required.

### 2.5 `flow-particle`
- **Recommended Method:** **CSS `offset-path`** (or **SMIL `<animateMotion>`** where standalone zero-JS SVG compatibility is mandatory).
- **Rejected Alternative:** **`javascript-raf` (`getPointAtLength` per frame)** is **not recommended** for large particle populations (`2.153s` CPU / `850.4ms` task at `N=100` vs `1.563s` / `368.9ms` for CSS `offset-path`).
- **Validated Operating Range:** `curved` paths (`100×40`), `1.6s` period, `100` active instances.
- **Further Development:** **No further action** required; keep `flow-particle` on CSS `offset-path` or SMIL `<animateMotion>`.

### 2.6 Complete Stage 16C Exported Widgets (`host-monitor-v1`)
- **Recommended Arrangement:** `stage16c-widgets` with **`suspend_offscreen: true`** when deploying `>= 100` complete widgets.
- **Validated Operating Range:** `50–100` active widgets (`59.9 Hz` rAF, `4,529–9,542` DOM nodes). At **`250` active widgets**, `rAF` drops to `52.5 Hz` (`22,529` DOM nodes, `994ms` task); enabling off-screen suspension (`vis-12-suspended`) recovers `rAF` to `58.3 Hz` and cuts task work by `41%`.
- **Known Limitations:** `evaluateStage16CWidgetsCorrectness` verifies mounting, unique identity, `setParameter()` isolation, suspension, and lifecycle stability, **not** M1 per-frame inner-element motion inside exported widgets.
- **Practical Restrictions:** Keep active visible complete widgets `<= 100` per view, or enable `IntersectionObserver` suspension above `100` widgets.
- **Further Development:** **No further action** required on the Test Lab.

---

## 3. Viewer Access (`http://192.168.100.40:8093/`)

- `m6-01-baseline-all-families`: `/m6-01-baseline-all-families/report.html`
- `m6-02-load-scaling-and-raf-sched`: `/m6-02-load-scaling-and-raf-sched/report.html`
- `m6-03-geometry-size-speed`: `/m6-03-geometry-size-speed/report.html`
- `m6-04-display-scaling`: `/m6-04-display-scaling/report.html`
- `m6-05-dt-large-bg-and-visibility`: `/m6-05-dt-large-bg-and-visibility/report.html`
- `m6-06-stage16c-widgets`: `/m6-06-stage16c-widgets/report.html`
- `m6-07-mixed-workloads`: `/m6-07-mixed-workloads/report.html`
