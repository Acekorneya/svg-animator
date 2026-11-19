# Decision Report — `perimeter-tracer`

## 1. Tested Methods & Correctness

| Method | Trials | Correctness Passed | Mean rAF (Hz) | Mean Main-Thread Task (ms) | Mean Owned Browser CPU (s) | Limitations |
|---|---:|---:|---:|---:|---:|---|
| `css` | 1 | 1/1 | 60.0 | 82.33 | 0.610 | - |
| `javascript-raf` | 1 | 1/1 | 60.0 | 79.77 | 0.557 | - |
| `smil` | 1 | 1/1 | 60.0 | 177.33 | 0.627 | - |

## 2. Assessment & Recommendation

- **Selected Recommendation:** `javascript-raf` (passed 1/1 correctness checks; 60.0 mean rAF Hz; 79.77 ms main-thread task; 0.557 s owned browser CPU).
- **Alternatives Considered:**
  - `css`: 60.0 mean rAF Hz, 82.33 ms task, 0.610 s CPU.
  - `smil`: 60.0 mean rAF Hz, 177.33 ms task, 0.627 s CPU.

- **Export & Browser Compatibility:** CSS `@keyframes` and SMIL `<animate>` work in standalone SVG documents without JavaScript; `javascript-raf` requires a host script runtime. Notice this report does not automatically modify Studio.
