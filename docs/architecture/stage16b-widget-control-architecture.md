# Stage 16B: External Widget Controls and Runtime Parameter Architecture

## 1. Architectural Overview

Stage 16B builds on the reusable SVG widget foundation established in Stage 16A, transforming isolated SVG documents from static graphical assets into **programmable, interactive, host-controllable components**.

The central architectural rule of SVG Animator Studio remains paramount:

$$\text{Go Engine (Authoritative)} \longrightarrow \text{WASM Bridge} \longrightarrow \text{JavaScript Browser Adapter} \longrightarrow \text{Browser SVG Engine}$$

Stage 16B strictly honors this contract:
1. **Go Engine (`pkg/engine/scene/`, `pkg/engine/sva/`, `pkg/engine/protocol/`) owns:**
   - The authoritative `WidgetParameter` data model and `WidgetManifest` schema.
   - SVA project packaging and lossless schema deserialization.
   - Authoritative SVG isolation and automatic manifest injection in `IsolateSVGWidget`.
   - Protocol command dispatch (`Widget.GetParameters`, `Widget.SetParameters`, `Widget.AddParameter`, `Widget.RemoveParameter`).
2. **JavaScript Adapter (`internal/web/static/js/svg-widget.js`, `engine-bridge.js`) owns:**
   - Light DOM and Shadow DOM mounting and lifecycle coordination.
   - Semantic target resolution (`target` -> namespaced DOM node).
   - Atomic batch parameter validation and DOM property application.
   - Per-instance independent animation control (SMIL, CSS, Web Animations API).
   - Host event emission (`change`, `parametersChanged`, `play`, `pause`, `stop`, `destroy`).

---

## 2. Multi-Instance Control Topology

```
+-----------------------------------------------------------------------------------+
| Host Web Application / Webpage                                                    |
|                                                                                   |
|  const barrelA = SVGWidget.mount('#tankA', rawSvg, { instanceId: 'barrel-a' });  |
|  const barrelB = SVGWidget.mount('#tankB', rawSvg, { instanceId: 'barrel-b' });  |
|  const barrelC = SVGWidget.mount('#tankC', rawSvg, { instanceId: 'barrel-c' });  |
+-----------------------------------------------------------------------------------+
        |                                 |                                 |
        | setParameter('state', 'run')    | pause()                         | setSpeed(2.0)
        v                                 v                                 v
+-----------------------+         +-----------------------+         +-----------------------+
| Instance A (barrel_a) |         | Instance B (barrel_b) |         | Instance C (barrel_c) |
| State: RUNNING        |         | State: PAUSED         |         | State: ALARM          |
| Fill: 120px           |         | Fill: 45px            |         | Fill: 165px           |
| Color: #22c55e (Green)|         | Color: #f59e0b (Amber)|         | Color: #ef4444 (Red)  |
| Speed: 1.0x           |         | Speed: 0.0x           |         | Speed: 2.0x           |
| Prefix: barrel_a__    |         | Prefix: barrel_b__    |         | Prefix: barrel_c__    |
+-----------------------+         +-----------------------+         +-----------------------+
        |                                 |                                 |
        +---------------------------------+---------------------------------+
                                          |
                              (ZERO MUTUAL COUPLING)
```

---

## 3. Semantic Target Resolution

To prevent host applications from needing internal knowledge of namespaced ID prefixes (e.g. `barrel_a__fluidLevel` vs `barrel_b__fluidLevel`), parameter targets are declared semantically as `fluidLevel` or `statusDot`.

When an update is dispatched, `SVGWidget` resolves targets using a 5-step fallback chain:

1. **Explicit Semantic Target:** `[data-semantic-target="<target>"]`
2. **ID Mapping Translation:** Lookup `target` in `instance.idMap` -> query `#<mappedId>`
3. **Engine Entity ID:** `[data-node-id="<target>"]`, `[data-flow-id="<target>"]`, `[data-glow-id="<target>"]`, `[data-image-id="<target>"]`
4. **Scoped Direct ID Query:** `#<target>` within widget root
5. **CSS Selector Query:** Query target as standard CSS selector within widget root
6. **Root Target:** `target === 'root'` or `target === 'svg'` targets root SVG element itself

---

## 4. Atomic Batch Validation

To maintain scene integrity in mission-critical applications, `instance.setParameters(batch)` executes as a two-phase transaction:

1. **Validation Phase:**
   - Every key must be declared in the widget's manifest schema.
   - Value must conform to declared parameter `type` (`color`, `number`, `boolean`, `enum`, `speed`, `text`).
   - Numbers must be finite and within `min` / `max` bounds.
   - Enums must be present in declared `options` allowlist.
   - Semantic target must exist in the mounted DOM.
2. **Commit Phase:**
   - If *any* check fails, the transaction aborts with a descriptive Error. Zero DOM mutations occur and internal state remains untouched.
   - If all checks pass, updates are applied synchronously in a single repaint pass, internal values update, and `'change'` events are emitted.

---

## 5. Per-Instance Animation Control Mechanics

SVG animations operate across three distinct browser subsystems. `SVGWidget` coordinates all three independently:

- **SVG SMIL Animations (`<animate>`, `<animateMotion>`, `<animateTransform>`):**
  Controlled per-instance using native SVG DOM methods `svgElement.pauseAnimations()`, `svgElement.unpauseAnimations()`, and `svgElement.setCurrentTime(0)`. Crucially, invoking these methods on `svgA` does not affect `svgB`.
- **CSS Keyframe Animations (`@keyframes`):**
  Paused per-instance by applying class `.sva-paused` to the widget wrapper container, scoped via `.sva-widget-wrapper.sva-paused * { animation-play-state: paused !important; }`.
- **Web Animations API:**
  If supported by the browser, `svgElement.getAnimations({ subtree: true })` applies `.play()`, `.pause()`, and `.playbackRate = speed` on the instance subtree.
