# Stage 16D-A — Foundation, Visibility-Aware Animation & Low-Load Effects

## 1. Overview

Stage 16D-A establishes the scalability foundation and first three **Low-Load Effects** (`LiteEffects`) for SVG Animator Studio:
1. **Automated Browser Performance Benchmark** (`scripts/cmd/benchmark-stage16d/main.go`, `demos/stage16d-benchmark/index.html`) measuring 30, 100, and 250 widget instances in real Chromium over CDP.
2. **Visibility-Aware Animation & Reduced Motion** (`internal/web/static/js/svg-widget.js`) using `IntersectionObserver`, CSS `.sva-suspended` (`animation-play-state: paused !important`), and per-element SMIL `begin="indefinite"` / `endElement()` without calling blanket `svg.pauseAnimations()`.
3. **Low-Load Effects Category (`LiteEffects`):**
   - **Perimeter Tracer Lite** (`perimeter-tracer`: `rect`, `rounded`, `circle`, `ellipse`)
   - **Pulse Lite** (`pulse`: `circle`, `square`, `diamond`, `cross`, `ring`)
   - **Flash Lite** (`flash`: `circle`, `square`, `diamond`, `cross`, `star`)

---

## 2. Automated Benchmark Methodology & Results

### Environment Configuration
- **Browser:** `HeadlessChrome/155.0.0.0` (Linux x86_64), viewport `1366×900`, `devicePixelRatio: 1`
- **Graphics:** `ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero)), SwiftShader driver)`
- **Host:** `Linux 7.0.12-1-pve`, `2` vCPUs (`hardwareConcurrency: 2`)
- **Unavailable Metrics Note:** Direct per-process OS CPU % and raw GPU hardware utilization % are not exposed by HeadlessChrome CDP; main-thread `TaskDuration`, `ScriptDuration`, `LayoutDuration`, `RecalcStyleDuration`, `JSHeapUsedSize`, `Nodes`, and `requestAnimationFrame` frame timing are directly measured (`docs/benchmarks/stage16d-a-baseline.json` & `docs/benchmarks/stage16d-a-visibility-aware.json`).

### Baseline vs. Visibility-Aware Comparison (Legacy SMIL `host-monitor-v1`)

| Scenario | Mounted | Visible (Baseline → VisAware) | Suspended (VisAware) | Baseline FPS / P95 (ms) | VisAware FPS / P95 (ms) | Baseline TaskMs (per 1s) | VisAware TaskMs (per 1s) | TaskMs Delta |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `30-static-dashboard` | 30 | 9 → 9 | 21 | 60.0 / 16.70 | 60.0 / 16.80 | 18.21 | 13.47 | -26.0% |
| `30-animated-dashboard` | 30 | 9 → 9 | 21 | 60.0 / 16.80 | 60.0 / 16.70 | 150.70 | 144.54 | -4.1% |
| `30-animated-all-hidden` | 30 | 0 → 0 | 30 | 60.0 / 16.80 | 60.0 / 16.80 | 164.46 | 8.45 | **-94.9%** |
| `100-static-dashboard` | 100 | 9 → 9 | 91 | 60.0 / 16.80 | 60.0 / 16.70 | 13.00 | 15.11 | — |
| `100-animated-dashboard` | 100 | 9 → 9 | 91 | 58.0 / 16.80 | **60.0 / 16.80** | 632.16 | 98.44 | **-84.4%** |
| `100-mixed-dashboard` | 100 | 9 → 9 | 91 | 59.0 / 16.80 | **60.0 / 16.70** | 296.91 | 101.80 | **-65.7%** |
| `100-animated-scroll` | 100 | 12 → 15 | 85 | 60.0 / 16.70 | 60.0 / 16.80 | 545.30 | 174.43 | **-68.0%** |
| `100-animated-hidden-updates` | 100 | 0 → 0 | 100 | 60.0 / 16.80 | 60.0 / 16.80 | 538.71 | 62.52 | **-88.4%** |
| `250-animated-dashboard` | 250 | 9 → 9 | 241 | 55.0 / **33.40** | **60.0 / 16.80** | 1019.85 | 175.89 | **-82.8%** |

### Key Finding
Chromium **does not** automatically stop ticking off-screen SMIL animations (`30-animated-all-hidden` consumed `164.46 ms/s` and `100-animated-hidden-updates` consumed `538.71 ms/s` with zero visible widgets). With `IntersectionObserver`-driven per-instance suspension, main-thread `TaskDuration` at 250 instances dropped from **1019.85 ms** (`55 FPS`) to **175.89 ms** (`60 FPS`), while hidden widgets stayed mounted and continued accepting live parameter updates.

---

## 3. Visibility-Aware Animation Design (`svg-widget.js`)

- **Mechanism:** `IntersectionObserver` observes each `.sva-widget-wrapper` against `options.visibilityRoot || null` (`rootMargin: '50px'`, `threshold: 0.01`).
- **No Blanket `svg.pauseAnimations()`:**
  - CSS `@keyframes` animations pause via `.sva-widget-wrapper.sva-suspended { animation-play-state: paused !important; }`.
  - SMIL elements (`animate`, `animateMotion`, `animateTransform`) pause per-element by storing `dataset.svaOrigBegin`, setting `begin="indefinite"`, and calling `endElement()`, then restoring `begin` and calling `beginElement()` upon becoming visible.
- **Configuration & Accessibility:**
  - `visibilityAware: false` opts out cleanly.
  - `reducedMotion: 'auto' | 'reduce' | 'ignore'` respects `prefers-reduced-motion: reduce`.
  - `unmount()` / `destroy()` disconnects the observer and removes the `matchMedia` listener.

---

## 4. Low-Load Effects Architecture (`LiteEffect`)

### Go Authority (`pkg/engine/scene/`)
- `LiteEffect` struct (`pkg/engine/scene/model.go`) and CRUD/validation (`pkg/engine/scene/lite_effect.go`).
- Snapshot & `.sva` persistence (`Snapshot.LiteEffects`, `LiteEffectsCounter`), Layer support (`SelectionLite`), Naming (`lite-`), and `ObjectPropertyCatalog["lite"]` (`color`, `opacity`, `visibility`, `speed`, `trigger`).
- `ComputePerimeterDash(segments, lineGapRatio)` calculates repeating `stroke-dasharray` in `pathLength="1000"` units:
  $$\text{unit} = \frac{1000}{N}, \quad \text{dashLen} = \text{unit} \cdot \frac{L}{L + G}, \quad \text{gapLen} = \text{unit} \cdot \frac{G}{L + G}$$
  *(e.g. 2 segments at `1:3` yields `125` dash, `375` gap).*

### Rendering & Runtime (`lite-effects.js` & `svg-widget.js`)
- Uses 1 simple SVG primitive per effect + 1 transparent `.lite-hit-target`, zero `<filter>` blur, and shared CSS `@keyframes` (`sva-lite-trace-cw`, `sva-lite-trace-ccw`, `sva-lite-pulse`, `sva-lite-flash`).
- **Flash Lite** defaults to single-shot (`repeat: false`, `dur: 0.22s`, `intensity: 0.75`) and supports external event triggering via `setParameter('<name>-lite-trigger', N)`.

---

## 5. Tests & Real-Browser Verification

- **Go Tests (`go test ./...`):** 100% PASS (including `TestComputePerimeterDash` and `TestLiteEffectCRUDAndValidation`).
- **JS Regression Harness (`node scripts/regression-harness.js`):** 88/88 scenarios PASS (including Scenario 88).
- **Real-Browser CDP (`scripts/cmd/verify-stage16d-a/main.go`):** 4/4 criteria PASS on Kiwiberry (`192.168.100.40:8088` & `8092`, plus unpacked `file:///home/nigel/acceptance-demo/stage16d-a-widget/lite-effects-demo/demo.html`).
