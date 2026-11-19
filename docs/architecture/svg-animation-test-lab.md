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
