# SVG Animator Studio — Stage 16B Interactive Widget Demonstration

This offline package demonstrates **Stage 16B: External Widget Controls, Runtime Parameters and Independent Host Integration**.

## Contents

- `standalone-demo.html`: Single-file zero-dependency self-contained runner with inlined `SVGWidget` library and 3 interactive process barrels. Open directly in Chrome, Firefox, Safari, or Edge (`file://` or HTTP).
- `index.html`: Multi-file demonstration linking to `./js/svg-widget.js`.
- `js/svg-widget.js`: Standalone universal UMD library (`SVGWidget.mount`, `isolate`, `setParameter`, `setSpeed`, etc.).

## Key Capabilities Demonstrated

1. **Independent Instance Control:** Three Barrel instances (A, B, C) operate concurrently with individual states (`running`, `paused`, `alarm`), distinct fill levels, custom status colors, and individual speed playback rates.
2. **Zero DOM ID Collisions:** Automatic ID namespacing (`barrel_a__*`, `barrel_b__*`, `barrel_c__*`) ensures zero mutual interference in gradients, filters, or SMIL animations.
3. **Atomic Parameter Validation:** Strict runtime validation against the declared manifest schema, preventing invalid types, out-of-range numbers, or malicious injection.
4. **Independent Playback Control:** Pausing or changing the playback speed of Barrel B has zero effect on Barrel A or Barrel C.
5. **Real-time Host Telemetry:** Click '⚡ Auto Telemetry: ON' to simulate live host-driven process updates across all mounted widgets.
