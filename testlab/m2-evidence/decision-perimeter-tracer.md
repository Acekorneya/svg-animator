# Decision Report — `perimeter-tracer`

## 1. Tested Methods & Correctness

| Method | Trials | Correctness Passed | Mean rAF (Hz) | Mean Main-Thread Task (ms) | Mean Owned Browser CPU (s) | Limitations |
|---|---:|---:|---:|---:|---:|---|
| `css` | 1 | 1/1 | 60.0 | 70.94 | 0.583 | - |
| `javascript-raf` | 1 | 1/1 | 60.0 | 61.72 | 0.550 | - |
| `smil` | 1 | 1/1 | 60.0 | 217.81 | 0.753 | - |

## 2. Assessment & Recommendation

- **Selected Recommendation:** `javascript-raf` (passed 1/1 correctness checks; 60.0 mean rAF Hz; 61.72 ms main-thread task; 0.550 s owned browser CPU).
- **Alternatives Considered:**
  - `css`: 60.0 mean rAF Hz, 70.94 ms task, 0.583 s CPU.
  - `smil`: 60.0 mean rAF Hz, 217.81 ms task, 0.753 s CPU.

- **Export & Browser Compatibility:** CSS `@keyframes` and SMIL `<animate>` work in standalone SVG documents without JavaScript; `javascript-raf` requires a host script runtime. Notice this report does not automatically modify Studio.
