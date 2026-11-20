# SVG Animation Test Lab Report — `m2-full-suite`

M2 Measurement Integrity verification

- **Timestamp:** `2026-11-19T17:18:07Z`
- **Browser:** `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/155.0.0.0 Safari/537.36` (`hardwareConcurrency: 2`, `isSoftwareRenderer: true`, `renderer: ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero) (0x0000C0DE)), SwiftShader driver)`)
- **Execution:** warmup=`0.3s`, duration=`1.2s`, repetitions=`3`, orderStrategy=`alternating`, seed=`2026`

## Results Matrix

| Run ID | Family | Method | Workload | Count | Valid | Correctness | rAF (Hz) | Presented FPS | Avg rAF (ms) | P95 rAF (ms) | Browser CPU (s) | Task (ms) | Heap (MB) | DOM Nodes | Long Tasks |
|---|---|---|---|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `perimeter-tracer__css__all-active__n40` | perimeter-tracer | `css` | all-active | 40 | 3/3 | PASS | 60.0 | N/A | 16.67 | 16.70 | 0.870 | 68.95 | 1.0 | 277 | 0 |
| `perimeter-tracer__smil__all-active__n40` | perimeter-tracer | `smil` | all-active | 40 | 3/3 | PASS | 60.0 | N/A | 16.67 | 16.73 | 1.053 | 256.33 | 1.0 | 317 | 0 |
| `perimeter-tracer__javascript-raf__all-active__n40` | perimeter-tracer | `javascript-raf` | all-active | 40 | 3/3 | PASS | 60.0 | N/A | 16.67 | 16.77 | 0.883 | 68.85 | 1.0 | 277 | 0 |

