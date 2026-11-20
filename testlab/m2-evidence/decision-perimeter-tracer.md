# Decision Report — `perimeter-tracer`

## 1. Tested Methods & Correctness

| Method | Trials | Correctness Passed | Mean rAF (Hz) | Mean Main-Thread Task (ms) | Mean Owned Browser CPU (s) | Limitations |
|---|---:|---:|---:|---:|---:|---|
| `css` | 1 | 1/1 | 60.0 | 68.95 | 0.870 | - |
| `javascript-raf` | 1 | 1/1 | 60.0 | 68.85 | 0.883 | - |
| `smil` | 1 | 1/1 | 60.0 | 256.33 | 1.053 | - |

## 2. Assessment & Recommendation

- **Selected Recommendation:** Indistinguishable within measurement margin between `css` and `javascript-raf` (both passed correctness; `css`: 60.0 Hz rAF, 68.95 ms main-thread task, 0.870 s owned browser CPU).
- **Alternatives Considered:**
  - `javascript-raf`: 60.0 mean rAF Hz, 68.85 ms task, 0.883 s CPU.
  - `smil`: 60.0 mean rAF Hz, 256.33 ms task, 1.053 s CPU.

- **Export & Browser Compatibility:** CSS `@keyframes` and SMIL `<animate>` work in standalone SVG documents without JavaScript; `javascript-raf` requires a host script runtime. Notice this report does not automatically modify Studio.
