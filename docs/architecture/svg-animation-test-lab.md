# SVG Animation Test Lab (`cmd/anim-test-lab`, `pkg/animlab`)

## 1. Purpose & Isolation

The **SVG Animation Test Lab** is a standalone, YAML-driven benchmarking, correctness-verification, and visual-catalogue tool for comparing SVG animation techniques (`css`, `smil`, `javascript-raf`, `clip-reveal`, `static`, `data-driven`) across configurable instance counts (`50`, `100`, `250`, `500`, or any positive integer), geometries, speeds, and mixed workloads.

It is strictly isolated from the production Studio editor (`internal/`, `pkg/engine/`) and does not modify Studio or interfere with Studio (`:8088`), Stage 16C consumer (`:8091`), or Stage 16D benchmark (`:8092`).

---

## 2. CLI Commands

Build once:
```bash
go build -o /tmp/anim-test-lab ./cmd/anim-test-lab
```

Once built, **all ordinary experiments are defined and modified via YAML without recompilation**:
- `anim-test-lab validate -c <config.yaml>` — Validate YAML schema, `inherits`, and `presets`.
- `anim-test-lab list` — List registered effect families, supported methods, and measurements.
- `anim-test-lab samples -c <config.yaml> [-out <dir>]` — Generate standalone SVG samples (`samples/*.svg`) and browsable catalogue (`catalogue/index.html`).
- `anim-test-lab run -c <config.yaml> [-test <filter>] [-out <dir>]` — Run all or filtered scenarios in real Chromium CDP and generate reports.
- `anim-test-lab report -in <raw-results.json> [-c <resolved-config.yaml>] [-out <dir>]` — Regenerate HTML, Markdown, and Decision reports from saved raw results.
- `anim-test-lab rerun -dir <previous-run-dir> [-out <dir>]` — Re-run an experiment from its preserved `resolved-config.yaml`.
- `anim-test-lab serve -dir <out-dir> [-addr :8093]` — Serve visual catalogue and HTML reports.

---

## 3. Adding Future Effect Families or Rendering Methods

1. Implement `EffectGenerator` in `pkg/animlab/effects.go`:
   - `Family() string`
   - `Description() string`
   - `SupportedMethods() []string`
   - `SupportsMethod(method string) bool`
   - `Render(p InstanceParams, viewW, viewH int) RenderedEffect`
2. Register the new generator in `DefaultRegistry()` in `pkg/animlab/effects.go`.
3. Add any family-specific default shapes/options in `applyDefaults()` (`pkg/animlab/config.go`).
4. Any new family or method immediately becomes selectable in YAML (`effects:` and `mixture:`) and appears in `anim-test-lab list`, sample catalogues, and decision reports.

---

## 4. Outputs Per Run

Every run directory preserves:
- `resolved-config.yaml` — Exact resolved YAML configuration (after `inherits` & `presets` expansion).
- `raw-results.json` — Machine-readable trial & aggregate metrics.
- `report.md` & `report.html` — Human-readable comparison tables.
- `decision-<family>.md` — Evidence-based per-effect assessment and recommendation (methods failing correctness checks, or marked `NonEquivalent` / `Unsupported`, are never ranked as winners).
- `catalogue/index.html` & `samples/*.svg` — Interactive visual sample catalogue (standalone `javascript-raf` and `data-driven` `.svg` samples embed an SVG-safe `<script type="application/ecmascript">` block so they play when opened directly).
- `screenshots/*.png` — Real Chromium screenshots of each scenario.

---

## 5. Milestone 1 — Correctness & Comparison Fairness

- **Effect Families:** `flow` is split into `flow-line` (stroke-dashoffset line flow) and `flow-particle` (distinct moving particle via SMIL `<animateMotion>`, CSS `offset-path`, or JS RAF `getPointAtLength`). Legacy `flow` in YAML is rejected with a migration error.
- **RAF Scheduling:** `effects[].raf_scheduling` supports `"shared"` (default) and `"per-instance"`, recorded in `ScenarioReport.SchedulingStrategy`.
- **Two-Level Verification (`pkg/animlab/correctness.go`):**
  - **Level A (Whole-Population):** Inspects all `N` `.lab-payload` elements for count, unique IDs, animation attachment, offscreen suspension, and per-instance motion (`PARTIAL_POPULATION_FAILURE`, `STATIONARY`, `SUSPENSION_LEAK`).
  - **Level B (Detailed Temporal Sampling):** Samples deterministic subset `[0, N/4, N/2, 3*N/4, N-1]` at `30ms`–`60ms` cadence to verify tracer direction/speed/loop continuity (`WRONG_DIRECTION`, `WRONG_SPEED`, `LOOP_DISCONTINUITY`), pulse mode exclusivity (`MODE_MISMATCH`, `WRONG_AMPLITUDE`), flash pulse detection (`MISSED_FLASH`, `STUCK_ACTIVE`), `data-driven` gauge updates, and scroll-into-view resumption (`RESUMPTION_FAILED`).

---

## 6. Milestone 2 — Measurement Integrity

- **Owned Browser Process-Tree CPU (`pkg/animlab/cpu_linux.go`):** Identifies the specific attached Chromium root PID (via CDP `SystemInfo.getProcessInfo` + port/cmdline verification), walks its recursive descendant PID tree on `ProcessCPUHost`, and divides `/proc/<pid>/stat` ticks by `getconf CLK_TCK`. Unrelated Chromium instances or processes are excluded; unavailable ownership yields `null` / `N/A` and marks the trial `partial`/`failed` rather than inventing `0.0`.
- **Viewport & Per-Trial Isolation (`pkg/animlab/runner.go`):** Each trial runs in a fresh isolated tab target (`chromedp.NewContext` + `defer cancel()`), applies `viewport_width`/`viewport_height`/`device_scale_factor` via `emulation.SetDeviceMetricsOverride`, and verifies actual `window.innerWidth`/`innerHeight`/`devicePixelRatio`.
- **Metric Semantics & Uncertainty (`pkg/animlab/metrics.go`, `pkg/animlab/report.go`):**
  - `rafHz` (`requestAnimationFrame` callback cadence) is strictly separated from `presentedFps` (`null` / `N/A` when compositor presentation counters are unavailable).
  - `longFramesOver16ms` (`> 16.67 ms`) is separated from `longFramesOver20ms` (`> 20.0 ms`) and `longFramesOver50ms` (`> 50.0 ms`).
  - `MetricDefinitions` and `Environment` (WebGL renderer, `isSoftwareRenderer`, OS/CPU/RAM, `clkTck`, `browserOwnership`) are persisted in `raw-results.json`.
  - Decision reports rank equivalent, valid, correctness-passing methods by **means** (never sums), report `N/A` for unavailable metrics, and flag results within measurement margin as indistinguishable.

---

## 7. Milestone 3 — YAML Flexibility, Variation Strategies & Shared-Scene Visibility

- **Strict YAML Schema & Inheritance (`pkg/animlab/config.go`):**
  - Unknown YAML keys are rejected via `yaml.Decoder.KnownFields(true)`.
  - Inheritance (`inherits`) detects cycles immediately and supports `false`/`0` child overrides (`headless`, `warmup_seconds`, `alternate_method_order`, `output.*`).
  - Named presets (`presets`) copy all effect fields (`directions`, `pulse_modes`, `pulse_amplitudes`, `flash_modes`, `flash_duration_ms`, `flash_interval_s`, `raf_scheduling`, `variation_strategy`).
- **Variation Strategies (`pkg/animlab/workload.go`):**
  - `representative` (default): single scenario with de-correlated per-factor shuffled index tables and per-scenario deterministic seeding.
  - `isolated`: holds baseline constant and varies one factor at a time (`shape_*`, `size_*`, `speed_*`, `segs_*`, `ratio_*`, `pmode_*`, `fmode_*`).
  - `factorial`: expands the full Cartesian product across configured dimensions.
  - `validate -c <config.yaml>` previews expanded scenarios, strategies, counts, and mixture allocations.
- **Mixed Workloads & Shared-Scene Visibility (`pkg/animlab/workload.go`):**
  - Hamilton largest-remainder rounding (`allocateMixtureCounts`) ensures exact instance counts summing to `N` with per-family de-correlated variation. Unsupported dynamic methods in mixtures are rejected at validation time.
  - `shared-scene` `<g class="lab-instance">` visibility is evaluated against `#labGrid` via `getBoundingClientRect()`, `scroll`, and `ResizeObserver` (no `.lab-card` dependency), suspending decorative CSS/JS-RAF off-screen without pausing `data-driven` telemetry updates.

---

## 8. Milestone 4 — Realistic Digital Twin & Stage 16C Exported-Widget Workloads

- **Deployment Arrangements (`instances.isolation`):**
  - `shared-scene` / `shared-svg-scene`: many animated `<g>` objects inside one `<svg>` root.
  - `shared-dt-scene`: one static flattened raster `<img>` background (`scene.background.file`, `fit: contain|cover|fill`, `static: true`) paired with one shared `<svg>` containing `N` independently positioned `<g class="lab-instance">` overlays. Substitute any larger architectural PNG/WebP/JPEG via `scene.background.file` in YAML without recompilation.
  - `independent-svg-roots`: `N` separate `<svg>` root elements.
  - `stage16c-widgets`: `N` independent instances of a genuine Studio-exported Stage 16C widget package (`widget_runtime.package_dir`, e.g. `demos/stage16c-widget-consumer/packages/host-monitor-v1`), validated for manifest/SVG consistency (`pkg/animlab/stage16c.go`), mounted via the unmodified `js/svg-widget.js` (`SVGWidget.mount`), and driven via real `setParameter()` calls from `widget_runtime.updates`.
- **Demonstrations & Fixtures:**
  - Lightweight architectural fixture: `testlab/fixtures/dt-building-bg.png` (`1280x720`, 12.6 KB).
  - Demonstration A (`testlab/configs/dt-building-window-tracers.yaml`): 100 animated window tracers over flattened building background.
  - Demonstration B (`testlab/configs/stage16c-widget-dashboard.yaml`): 100 independent `host-monitor-v1` Stage 16C widgets with per-instance `setParameter` updates.
