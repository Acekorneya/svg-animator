# Stage 16D-B — Proposed Implementation Plan

**Status:** **M1 ACCEPTED (`a3a7d24`); M2 ACCEPTED (`ad814bb`); M3 ACCEPTED (`a086170`, 4-threshold enhancement completed in M4); Stage 16D-B-M4 — IMPLEMENTED, AWAITING USER ACCEPTANCE**
**Project:** SVG Animator Studio
**Stage:** Stage 16D-B (Flows, Data Indicators and Waveforms)
**Implementation Authorisation:** **M1 (`16D-B-M1`: Flow Lite) ACCEPTED (`a3a7d24`); M2 (`16D-B-M2`: Radar Ping Lite) ACCEPTED (`ad814bb`); M3 (`16D-B-M3`: Gauge / Level Lite) ACCEPTED (`a086170`, with 4 independently enabled thresholds completed in M4); Stage 16D-B-M4 (`16D-B-M4`: Oscilloscope Lite) IMPLEMENTED — AWAITING USER ACCEPTANCE**
**Baseline Commits:** `bf7ee3e` (Test Lab M6 closeout) and `7e8d4f6` (Stage 16D-A standalone SVG export audit)  
**Engineering Guidance Reference:** [`docs/architecture/svg-animation-engineering-guidance.md`](architecture/svg-animation-engineering-guidance.md)  
**Test Lab Comparative Evidence:** [`testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md`](../testlab/m6-evidence/M6_FINAL_COMPARATIVE_REPORT.md) (BookStack Book 468, Page 470, revision 13; live viewer at `http://192.168.100.40:8093/`)

---

## A. Verified Current Baseline

### 1. Repository and Deployed Services
The repository is on branch main at commit `bf7ee3e`. The production Studio codebase under `pkg/engine/` and `internal/` remained untouched throughout the SVG Animation Test Lab programme and reflects the Stage 16D-A export audit baseline at commit `7e8d4f6`.

Four services are running on the Kiwiberry development host (192.168.100.40):
- Studio development server at `http://192.168.100.40:8088/`
- Stage 16C external widget consumer at `http://192.168.100.40:8091/index.html`
- Stage 16D automated browser benchmark harness at `http://192.168.100.40:8092/stage16d-benchmark/index.html`
- Persistent SVG Animation Test Lab viewer at `http://192.168.100.40:8093/`

### 2. Foundation Established in Stage 16D-A
Stage 16D-A (commits `830c866` through `7e8d4f6`) established the Low-Load Effects category (`LiteEffect`) and its first three effect families as first-class Studio objects:
- **Authoritative Go and WebAssembly Scene Model:** `LiteEffect` (`SelectionLite`) in `pkg/engine/scene/model.go` and `pkg/engine/scene/lite_effect.go` is owned by the Go `Scene` engine. It participates in `.sva` archive packaging, SQLite `schema_json` persistence, undo/redo history, the unified Layer manager (`LayerManager` in `engine-bridge.js` and `layers-ui.js`), standard Studio object grouping, and canvas selection bounding outlines (`#nodeSelectionBox`).
- **Initial Low-Load Effects (`internal/web/static/js/lite-effects.js`):**
  - **Perimeter Tracer Lite (perimeter-tracer):** Traces closed geometric boundaries (rounded rectangle, rectangle, circle, and ellipse) using normalized perimeter dashing (`ComputePerimeterDash`), CSS keyframe animation, continuous, ping-pong, and single-pass playback modes, and cycle-boundary burst/pause scheduling.
  - **Pulse Lite (pulse):** Provides geometric status pulses (circle, square, diamond, cross, and ring) with editable dimensions down to 5 SVG units, aspect-ratio locking, configurable minimum and maximum scale range, and independent arm thickness.
  - **Flash Lite (`flash`):** Produces brief geometric flashes (diamond, circle, square, cross, and eight-point starburst) with real-time millisecond duration, single-shot trigger support, optional repeat intervals, and an editor-only placement marker stripped automatically on export.
- **Visibility-Aware Animation and Three-Tier Property Exposure:** `svg-widget.js` uses `IntersectionObserver` to suspend decorative CSS animations, SMIL elements, and burst timers when a widget is scrolled off-screen without calling global `svg.pauseAnimations()`, while continuing to apply incoming `setParameter()` telemetry updates. `ObjectPropertyCatalog["lite"]` in `pkg/engine/scene/widget.go` defines runtime-eligible properties that authors can selectively expose in `widget-manifest.json`.

### 3. Documented Roadmap vs. Implemented State
Two documentation discrepancies were noted during the baseline audit:
- `AGENTS.md` (Section 2) and `AI_GUIDE.md` (Sections 3 and 4) still describe the baseline as of version 1.2.0 and Stage 14 (listing 25 scripts in `index.html`), omitting Stage 15 (image cropping and baking), Post-Carbon Review Batches 0 through 8F, Stages 16A through 16C (reusable externally controllable widgets), and Stage 16D-A (`js/lite-effects.js` and `js/svg-widget.js`, which bring `index.html` to 27 scripts).
- `docs/stage16d-a-integration-plan.md, docs/architecture/stage16d-a-foundation-and-lite-effects.md`, and BookStack Page 466 still record Stage 16D-A as awaiting final acceptance from commit `7e8d4f6` prior to the Test Lab programme. With the six-milestone Test Lab programme completed and accepted at `bf7ee3e`, the Stage 16D-A implementation at `7e8d4f6` serves as the verified baseline for Stage 16D-B.

---

## B. Original Roadmap Scope for Stage 16D-B

In the governing Stage 16D specification (Section 2 and Section 5), Stage 16D-B is assigned four Low-Load capabilities:
1. **Flow Lite (flow-lite):** A simpler, economical open-path alternative to the existing particle-based Flow connector, based on moving lines, dashes, or dots following a user-authored multi-point path with straight and curved sections, configurable start and end endpoint behaviour, and start and end delays.
2. **Radar Ping Lite (radar-lite):** An expanding-outline circular ping effect (distinct from the refractive Ripple effect and filter-blurred Glow), supporting single expanding rings, multiple staggered rings, and fading circular pings without background refraction or SVG blur filters.
3. **Gauge / Level Lite (gauge-lite):** A family of general-purpose, static data-driven numeric indicators (circular/ring gauge, horizontal bar, vertical bar, segmented bar, and filled-level indicator) that update their visual fill only when their numeric value changes, supporting both application-supplied colours and optional numeric threshold colour banding.
4. **Oscilloscope Lite (oscilloscope-lite):** A reusable waveform display supporting seven waveform presets (ECG/heartbeat, sine, square, triangle, sawtooth, angular polyline, and Bézier-smoothed curve) across three operating modes: static pattern, animated pattern, and sampled-data display.

---

## C. Scope Reconciliation

### 1. Feature Classification
- **Committed to Stage 16D-B:** Flow Lite, Radar Ping Lite, Gauge / Level Lite, and Oscilloscope Lite, including their authoritative Go models, Studio Low-Load drawer and Context Editor controls, Layers and grouping integration, standalone SVG export, Stage 16B three-tier widget property exposure, and Stage 16C consumer control.
- **Reserved for Stage 16D-C:** Shape text layout modes (auto, single-line centred, two-line major/minor), independent horizontal and vertical text alignment, curated browser-compatible font stacks, and Text Box Lite (traced borders, moving underlines, side and corner accents).
- **Reserved for Stage 16E and Later Stages:** Group-level data inputs, group property aliases, and custom composite objects (Stage 16E); canvas and background image fitting modes (future canvas stage); and Digital Twin flattened raster background workflows, shared animation definition aliases such as `cpuwindow`, and large-scale telemetry mapping services (future Digital Twin stages).
- **Explicitly Excluded:** Replacing existing Flow, Glow, or Ripple implementations; automatic orthogonal route-finding or node-topology attachment on Flow Lite; tunnel grow/shrink entry/exit on closed Perimeter Tracer Lite shapes (which remains deferred from Stage 16D-A); and building a full scientific oscilloscope or streaming telemetry server.

### 2. Overlap Between Test Lab Generators and Studio Features
The SVG Animation Test Lab (`pkg/animlab/effects.go`) contains benchmark generators named flow-line, flow-particle, and static-gauge. Those generators are synthetic benchmark fixtures that emit fixed-coordinate SVG markup to measure browser CPU and main-thread task costs; they lack Go `Scene` state, `.sva` and SQLite persistence, Studio UI inspectors, canvas hit-testing, waypoint manipulation, Layer integration, `ObjectPropertyCatalog` definitions, and `svg-widget.js` runtime handlers. The Test Lab's flow-particle generator corresponds to Studio's existing particle Flow (`scene.Flow`), whereas Stage 16D-B's Flow Lite builds a full multi-point authoring and runtime feature informed by the Test Lab's flow-line CSS findings.

---

## D. Proposed Technical Design

### 1. Shared Architectural Foundation
All four Stage 16D-B capabilities extend `scene.LiteEffect` (`SelectionLite`) in `pkg/engine/scene/model.go` and `pkg/engine/scene/lite_effect.go` using new `Kind` discriminators (flow-lite, radar-lite, gauge-lite, and oscilloscope-lite) and backward-compatible omitempty JSON fields. This reuses the established Go/WASM scene state, `.sva` and SQLite persistence, undo/redo history, Layer ordering, Studio grouping, Context Editor property cards, `<object-name>-lite-<property>` parameter naming, `ValidateWidgetExport, and svg-widget.js` off-screen suspension without introducing a parallel object framework.

---

### 2. Flow Lite (flow-lite) — Multi-Point Path, Endpoint Behaviour, Delays and Pattern Controls

#### 2.1 Audit of Existing Main Flow Infrastructure and Planned Reuse
Inspection of the existing main Flow implementation (`pkg/engine/scene/model.go, pkg/engine/scene/flow.go, pkg/engine/geometry/flowpath.go, pkg/engine/geometry/flow_geom.go, internal/web/static/js/flows.js, and internal/web/static/js/stage.js`) establishes a clear boundary between proven path-editing mechanics to reuse and legacy connector mechanics to avoid:

- **Main Flow Components to Reuse:**
  - **Authoritative Waypoint Storage:** Main Flow stores an ordered slice of percentage canvas coordinates (`[]geometry.Point`, where X and Y are 0 to 100 canvas percentages converted to pixels). Flow Lite will store its user-authored waypoints in `LiteEffect.Points` (`[]geometry.Point, json:"points,omitempty"`, minimum 2 points, defaulting to a 2-point horizontal path from (25%, 50%) to (75%, 50%)).
  - **Per-Section Bend Representation:** In `pkg/engine/geometry/flowpath.go` (lines 102–127), a curved segment offsets its quadratic Bézier control point perpendicular to the segment chord by 40% of the chord length at maximum curvature, emitting a quadratic Bézier command (Q) when curved and a straight line command (L) when curvature is zero. Flow Lite will store a per-section bend slice `LiteEffect.Bends` (`[]float64, json:"bends,omitempty"`, one value from -100 to +100 for each section between consecutive points, defaulting to 0 for straight segments) plus an optional whole-path `Curvature` (-100 to +100, default 0). The effective curvature for section i is the sum of its section bend and whole-path curvature, clamped to the range -100 to +100.
  - **Continuous Single-Path SVG Generation, Bounds, Length, and Hit-Testing:** New Go geometry helpers `GenerateFlowLitePath, ComputeFlowLiteBounds, ComputeFlowLiteLength, and DistanceToFlowLitePath` in `pkg/engine/geometry/flow_geom.go` will emit one continuous SVG <path> element combining straight and quadratic-Bézier sections. Because the entire multi-point route is a single <path> element, any dash or dot pattern travels smoothly across all waypoints and bends without restarting at intermediate points.
  - **Canvas Waypoint Reticle Overlay and Point Editing Workflow:** When a Flow Lite instance is selected, Studio will reuse the existing `#dynamicFlowReticles` and `#flowCurvePath` overlay layer in `stage.js` and `flows.js`. Draggable reticle handles (N0, N1, N2, and so on) appear at each point in the path. Clicking or dragging a reticle selects that point and its associated path section, updates coordinates via Go/WASM, and allows adding points (Add Point), removing points (Drop Point, down to a 2-point minimum), and adjusting the selected section's bend (-100 to +100). Dragging the Flow Lite path body or moving it within a Studio group translates all points together.

- **Main Flow Components Not Copied:**
  - Flow Lite will **not** copy automatic orthogonal routing (ortho-hv, ortho-vh, ortho-mid), node topology attachment (`FromNodeID`, `ToNodeID`), arrowhead markers, midpoint text badges, Gaussian blur filters, or multi-element SMIL <animateMotion> object payloads (orb, packet, envelope, database, server, or custom bitmap images).

#### 2.2 Pattern Controls (Dashed, Dotted, and Single Segment; Forward and Reverse)
Instead of a ratio-centred control, Flow Lite uses direct pixel-dimension pattern controls along the true pixel length L of the authored path (`ComputeFlowLiteLength`):

- **Pattern Mode (`variant`):**
  1. **Dashed (dashed — Repeating Dashed Line, Default):**
     - **Dash Length (`dashLength`):** Length of each illuminated dash in SVG pixels (2 to 300 px, default 18 px).
     - **Gap Spacing (`gapLength`):** Empty space between dashes in SVG pixels (2 to 300 px, default 14 px).
     - **Line Thickness (`strokeWidth`):** Stroke thickness in SVG pixels (0.5 to 24 px, default 3.0 px).
     - **Cap Style (`segmentStyle`):** Rounded (rounded, using round line caps, default) or Flat (solid, using butt line caps).
  2. **Dotted (dotted — Repeating Dotted Line):**
     - **Dot Size (`dotSize`):** Diameter of each circular dot in SVG pixels (1 to 32 px, default 5.0 px). Rendered using a near-zero dash length (0.01), round line caps, and stroke width equal to the dot size.
     - **Gap Spacing (`gapLength`):** Clear edge-to-edge space between dots in SVG pixels (2 to 300 px, default 14 px). In the SVG dash array, the gap value is emitted as the sum of dot size and gap spacing so the visible empty space between circular caps matches the configured gap spacing and dots never overlap.
     - *UI Simplification:* The Line Thickness control is hidden when Dotted is selected because Dot Size directly controls dot diameter.
  3. **Single Segment (single — Single Travelling Segment Preset):**
     - **Segment Length (`dashLength`):** Length of the single moving line segment in SVG pixels (4 to 400 px, default 36 px).
     - **Line Thickness (`strokeWidth`):** Stroke thickness in SVG pixels (0.5 to 24 px, default 3.0 px).
     - **Cap Style (`segmentStyle`):** Rounded or Flat.
     - *UI Simplification:* The Gap Spacing control is hidden when Single Segment is selected because the trailing clear gap is automatically computed from the path length L so only one segment occupies the path per cycle. A separate Multi-Segment mode is omitted because multiple dashes with explicit dash length and gap spacing are already represented directly by the Dashed mode without duplicate controls.
- **Direction (`direction`):**
  - Uses **Forward (forward)** (travelling from start point P0 to end point PK-1, default) and **Reverse (reverse)** (travelling from end point PK-1 to start point P0) in the Studio UI and widget parameter catalogue, while normalizing legacy clockwise/counterclockwise values on load.

#### 2.3 Independent Start and End Endpoint Behaviour (`startMode` and `endMode`)
Flow Lite provides two independent endpoint controls:
- **Start Behaviour (`startMode`):** Abrupt (abrupt) or Tunnel (tunnel, default).
- **End Behaviour (`endMode`):** Abrupt (abrupt) or Tunnel (tunnel, default).

All four combinations (Tunnel + Tunnel, Tunnel + Abrupt, Abrupt + Tunnel, and Abrupt + Abrupt) are supported using genuine geometric stroke clipping along the open SVG <path> of pixel length L (never simulated via opacity fading):

1. **How Geometric Growth and Shortening Work on an Open SVG Path:**
   - An open SVG <path> exists only on the arc-length interval from 0 to L. The browser natively clips any dashed stroke at the path origin (0) and path termination (L).
   - We construct a finite wave pattern of span W_pat in stroke-dasharray followed by a clearing gap G_clear of at least L + W_pat + d:
     - For Single Segment mode: the wave consists of 1 segment of length d, so W_pat = d.
     - For Dashed or Dotted mode: when either endpoint uses Tunnel mode (or when start/end delays separate passes), the wave consists of M = max(1, ceil(L / (d + g))) dashes or dots separated by gap g (total wave span W_pat approximately equal to L), followed by the clearing gap G_clear.
   - On the path from 0 to L, a dash offset value places the active wave along the path:
     - **Tunnel Entry:** The pass begins with the wave positioned on [-W_pat, 0] just before the start point (0 px visible on the path). As the dash offset advances, each dash crosses the path start and its visible length **grows linearly from 0 px to full dash length d** as it emerges from the start point.
     - **Abrupt Entry:** The pass begins with the wave positioned immediately on [0, W_pat] at full dash length d from the first frame of movement, without progressive emergence from behind the start point.
     - **Tunnel Exit:** The pass continues until the wave slides completely past the path endpoint onto [L, L + W_pat]. As each dash crosses L, the path endpoint clips it so its visible length **shortens linearly from full dash length d down to 0 px** until the final dash disappears into the endpoint.
     - **Abrupt Exit:**
       - For Single Segment mode: the pass ends as soon as the segment's leading edge reaches L at full length [L - d, L], and immediately clears to hidden upon completion without shortening into the endpoint.
       - For Dashed and Dotted stream modes: when combined with Tunnel Entry, the wave emerges from the start tunnel until the leading edge reaches L (with the full path populated at full dash lengths) and clears abruptly at the end of the cycle without draining into L. When both endpoints are Abrupt, the path remains populated across [0, L] throughout movement by shifting a repeating dash pattern by an integer multiple of the pattern period `d + g`.
   - **Cap-Protrusion Clipping for Round Dots and Caps:** Because round SVG line caps extend a semicircle of radius `strokeWidth / 2` past 0 and L, when Tunnel Entry or Tunnel Exit is active with round caps or dots, an endpoint clip boundary prevents the semicircular cap from popping in at full diameter before the stroke enters [0, L], ensuring dots and rounded dashes emerge progressively from zero width.
   - **Reverse Direction:** Reverses the offset trajectory so `startMode` governs emergence at the motion origin (PK-1) and `endMode` governs arrival at the motion destination (P0).

#### 2.4 Start Delay and End Delay Semantics (`delay` and `endDelay`)
Flow Lite provides two explicit delay controls and omits the closed-perimeter burst/pause controls from the Flow Lite inspector so there is only one unified timing model:
- **Start Delay (`delay`):** 0.0 to 60.0 seconds (default 0.0 s) — Pause before movement begins from the start of the path.
- **End Delay (`endDelay`):** 0.0 to 60.0 seconds (default 0.0 s) — Pause at the end of the path before restarting or reversing.
- **Transit Duration (speed):** 0.2 to 30.0 seconds (default 2.0 s) — Active movement duration for one pass along the path.

**Unified Declarative CSS Keyframe Encoding (Zero JavaScript Timers Required):**
For each Flow Lite instance, the total keyframe cycle duration is the sum of Start Delay, Transit Duration, and End Delay (in Ping-Pong mode, where CSS `animation-direction: alternate` traverses the keyframe hold twice at each turnaround, the keyframe hold windows at start and end are halved so the actual observed pause at each turnaround equals the configured Start Delay and End Delay):
- **During Start Delay:**
  - The dash offset holds stationary at the starting offset.
  - In Tunnel Entry mode, the wave lies before the start point, so the path is naturally empty (only the optional static guide track is visible).
  - In Abrupt Entry mode, the payload stroke is held hidden during the start delay window and steps to full authored opacity the instant movement begins.
- **During Active Transit:**
  - The dash offset interpolates linearly from the starting offset to the ending offset at full authored opacity.
- **During End Delay:**
  - The dash offset holds stationary at the ending offset.
  - In Tunnel Exit mode, the ending offset has already drained the pattern completely past the end point, so the path remains empty during the end delay.
  - In Abrupt Exit mode, upon reaching the end of the active transit, the pattern disappears abruptly during the end delay window before the next loop pass or reverse pass begins.
- **Behaviour by Playback Mode (`playbackMode`):**
  - **Loop (loop):** Repeats Start Delay → Forward Transit → End Delay indefinitely.
  - **Ping-Pong (pingpong):** Alternates Start Delay → Forward Transit → End Delay → Reverse Transit indefinitely.
  - **Once (once):** Runs Start Delay → Forward Transit → End State once and stops.
- **Why Declarative Keyframe Timing Is Superior:** Because Start Delay and End Delay are compiled directly into the scoped @keyframes percentages for the Flow Lite element, they require zero JavaScript `setTimeout` loops, work identically in Studio preview, standalone `.svg` files (including inside HTML <img> tags), and Stage 16C widget packages, and pause/resume without drift when `IntersectionObserver` toggles `.sva-suspended`.

#### 2.5 Updated Flow Lite Control List and Widget Property Eligibility
- **Studio Authoring Controls (`#acc-lite-effects` and `#fctx-liteSection`):**
  - **Name (`name`):** Optional display name.
  - **Waypoints and Section Bend Panel:** Add Point and Drop Point buttons (minimum 2 points); Selected Point/Section selector (synchronized with canvas reticles N0, N1, ...); Point X (%) and Point Y (%) numeric inputs; Section Bend slider and number input (-100 to +100, default 0 for straight); and Whole-Path Curvature slider and number input (-100 to +100, default 0).
  - **Pattern Type (`variant`):** Dashed (dashed), Dotted (dotted), or Single Segment (single).
  - **Pattern Dimensions (context-sensitive by Pattern Type):**
    - When Dashed: Dash Length in px (`dashLength`, 2 to 300, default 18), Gap Spacing in px (`gapLength`, 2 to 300, default 14), Line Thickness in px (`strokeWidth`, 0.5 to 24, default 3.0), and Cap Style (`segmentStyle`: Rounded or Flat).
    - When Dotted: Dot Size in px (`dotSize`, 1 to 32, default 5.0) and Gap Spacing in px (`gapLength`, 2 to 300, default 14).
    - When Single Segment: Segment Length in px (`dashLength`, 4 to 400, default 36), Line Thickness in px (`strokeWidth`, 0.5 to 24, default 3.0), and Cap Style (`segmentStyle`: Rounded or Flat).
  - **Appearance:** Colour (color, default #38bdf8), Opacity (`opacity`, 0.05 to 1.0, default 0.9), and Track Guide toggle (`showGuide`, default true).
  - **Endpoint Behaviour:** Start Mode (`startMode`: Tunnel or Abrupt) and End Mode (`endMode`: Tunnel or Abrupt).
  - **Motion and Timing:** Direction (`direction`: Forward or Reverse), Motion Mode (`playbackMode`: Loop, Ping-Pong, or Once), Transit Duration (speed, 0.2 to 30.0 s, default 2.0 s), Start Delay (`delay`, 0.0 to 60.0 s, default 0.0 s), and End Delay (`endDelay`, 0.0 to 60.0 s, default 0.0 s).
- **Tier 2 Runtime-Eligible Widget Properties for Flow Lite (`ObjectPropertyCatalog["lite"]`):**
  - color, `opacity, visibility`, speed, `strokeWidth, dashLength, gapLength, dotSize, direction, playbackMode, and showGuide`.
  - Waypoint coordinates (`points` and `bends`) and keyframe-structural endpoint/delay settings (`startMode, endMode, delay, and endDelay`) remain Tier-1 authoring-time properties baked into the exported SVG path and @keyframes definition.

---

### 3. Radar Ping Lite (radar-lite)

- **Visual Behaviour:** Expanding circular or elliptical outline rings radiating from center (x, y) to `(width, height)` and fading to transparent at the outer boundary, with an optional static center core dot (`showGuide`). Uses zero background refraction and zero <filter> blur elements.
- **Rendering Method (Backed by Test Lab `m6-01` and `m6-03` Evidence):**
  - Uses CSS `@keyframes sva-lite-radar` animating scale from `minScale` (default 0.15) to `maxScale` (default 1.0) and opacity from the authored base opacity down to 0 on 1 to 6 <ellipse> ring primitives with staggered negative animation delays.
  - Animating CSS `transform` and `opacity` with center transform origin avoids per-frame SVG radius layout recalculation, runs in standalone SVGs and widgets without JavaScript, and suspends automatically via `.sva-suspended`.
- **Variations (`variant`):**
  - Single Ring (single, 1 expanding outline ring)
  - Multi-Ring (multi, 2 to 6 staggered expanding outline rings, default 3)
  - Fading Ping (fading-ping, expanding outline ring with a subtle translucent interior fill disk that expands and fades out)
- **Authored Properties and Defaults:** `variant` (multi), `rings` (3, range 1 to 6), `minScale` (0.15), `maxScale` (1.0), `period` (2.4 s), `strokeWidth` (2.0 px), color (#22c55e), `opacity` (0.9), `showGuide` (true, center core dot), `width` (80 px), and `height` (80 px).
- **Tier 2 Runtime-Eligible Widget Properties:** color, `opacity, visibility`, speed, `strokeWidth, scale, period, and showGuide`. (`rings` is Tier-1 authoring-only because changing ring count adds or removes SVG <ellipse> elements.)

---

### 4. Gauge / Level Lite (gauge-lite) — Static, Data-Driven Operation and Colour Precedence Contract

#### 4.1 Static Between Telemetry Updates (Zero Continuous Animation Loop)
- Gauge / Level Lite is a **purely static, data-driven indicator**. It renders its SVG fill geometry from its current numeric `value` within [minValue, maxValue] and updates **only** when `value` (or `minValue, maxValue`, color, `bgColor`) changes via Studio controls or external `setParameter()` calls.
- It never runs a continuous CSS, SMIL, or JavaScript requestAnimationFrame loop. When mounted in a dashboard, 100 gauges consume zero idle animation CPU while reflecting incoming `setParameter()` telemetry changes immediately—even while decorative animations on the same widget are suspended off-screen.

#### 4.2 Variations (`variant`)
1. **Circular / Ring Gauge (ring):** Background track ellipse (`pathLength="100"`) paired with a foreground value arc ellipse (`pathLength="100"`).
2. **Horizontal Bar Gauge (hbar):** Rounded background track rectangle paired with a left-anchored proportional fill rectangle.
3. **Vertical Bar Gauge (vbar):** Rounded background track rectangle paired with a bottom-anchored proportional fill rectangle.
4. **Segmented Bar Gauge (segmented):** Discrete segment cells (`segments`, 4 to 24, default 10); cells up to the normalized active count display the active fill colour while remaining cells display the background track colour.
5. **Filled-Level Indicator (level):** Outer framed container rectangle paired with a rising interior fill rectangle.

#### 4.3 Numeric Domain, Valid Zero vs. Missing Data, and Colour Precedence Rule
- **Actual Numeric Domain Interpretation:**
  - The normalized fill fraction is `(value - minValue) / (maxValue - minValue)`, clamped to the range 0.0 to 1.0, where `maxValue > minValue`.
  - Optional warning and critical thresholds (`warnThreshold` and `critThreshold`) are authored and evaluated in the **gauge's actual numeric value domain** [minValue, maxValue]—never assumed to be 0 to 100 percentages. For example, if a gauge is authored with a range of 0 to 3600 RPM, a warning threshold of 2800 and critical threshold of 3300 are compared directly against the incoming numeric value in RPM.
- **Valid Zero vs. Missing Data:**
  - Zero (`value === 0`) is a valid finite number. When `minValue` is 0 and `value` is 0, the normalized fill fraction is 0.0: the gauge remains visible with its background track intact, active fill extent at zero, and normal healthy base colour.
  - Missing, null, undefined, or NaN values are rejected by `SVGWidget` parameter validation (`validateParameterValue`). As established in Stage 16C, the consuming host application remains responsible for applying its missing-data strategy (warn, treat-as-zero, or hide).
- **Colour Selection and Explicit Precedence Rule:**
  - Gauge / Level Lite supports two colour control patterns:
    1. **Application-Controlled Colour:** The consuming application's JavaScript computes its own semantic status colour and supplies it via `setParameter('<name>-lite-color', '#...')`.
    2. **Author-Configured Threshold Banding:** The Studio author enables `useThresholds` and configures `warnThreshold, warnColor` (default #f59e0b), `critThreshold, and critColor` (default #ef4444). These threshold settings are serialized as `data-gauge-*` attributes on the gauge element in the exported SVG.
  - **Precedence Rule:** **An explicitly supplied application colour (`setParameter('<name>-lite-color', ...)`) takes precedence over automatic threshold banding.**
    - When threshold banding is enabled on a gauge, updating `<name>-lite-value` automatically computes the active fill colour from the authored threshold bands **unless** the consuming application has explicitly set `<name>-lite-color` on that widget instance (which marks `data-gauge-color-override="true"` on the gauge element, or supplies `<name>-lite-color` in the same `setParameters()` batch). Calling `resetParameters()` clears the override flag and restores automatic threshold banding.

#### 4.4 Authoring-Time Settings vs. Consumer-Exposed Widget Parameters for Gauge Lite
- **Tier 1 Authoring-Time Settings (Persisted in Go `Scene` and baked into exported SVG `data-gauge-*` attributes):**
  - `variant` (ring, hbar, vbar, segmented, level), x, y, `width, height, segments` (for segmented), `useThresholds, warnThreshold, warnColor, critThreshold, and critColor`.
- **Tier 2 Runtime-Eligible Widget Properties (Selectable by the author for Tier 3 manifest exposure in `widget-manifest.json`):**
  - `value` (number: updates gauge fill geometry and evaluates threshold colour when no explicit colour override is active)
  - `minValue` (number: updates minimum domain bound and recalculates fill)
  - `maxValue` (number: updates maximum domain bound and recalculates fill)
  - color (color: sets active gauge fill colour and establishes explicit application colour precedence)
  - `bgColor` (color: recolours the background track and unlit segments)
  - `strokeWidth` (number: ring arc or frame stroke width)
  - `opacity` (number: 0.0 to 1.0)
  - `visibility` (boolean)

---

### 5. Oscilloscope Lite (oscilloscope-lite)

- **Visual Behaviour:** Rectangular waveform display (default 160 × 72 px) with optional screen background (`bgColor`), optional graticule grid lines (`showGuide`), and a waveform trace supporting seven presets across three operating modes.
- **Waveform Presets (`variant`):**
  - ECG/Heartbeat (ecg, default), Sine (sine), Square (square), Triangle (triangle), Sawtooth (sawtooth), Angular Polyline (polyline), and Bézier-Smoothed Curve (bezier).
- **Three Operating Modes (`playbackMode`):**
  1. **Static Pattern (static):** Generates a single-width waveform <path> once at authoring/render time with zero continuous animation.
  2. **Animated Pattern (animated):** Pre-generates a 2-tile periodic waveform <path> spanning twice the viewport width once at authoring/render time, clipped to the viewport width by `<clipPath id="lite-osc-clip-${id}">`, and scrolled horizontally via CSS `@keyframes sva-lite-osc-scroll` (`transform: translateX`). This avoids per-frame JavaScript path recalculation, works in standalone SVGs and widgets, and suspends automatically via `.sva-suspended`.
  3. **Sampled-Data Display (sampled):** Renders a waveform <path> from a bounded numeric sequence (`samples`: comma-separated numeric string or JSON numeric array of 2 to 64 samples). The path geometry is recalculated **only** when `samples` (or `intensity` amplitude) changes in Studio or via `setParameter('<name>-lite-samples', '...')`.
- **Tier 2 Runtime-Eligible Widget Properties:** color, `bgColor, opacity, visibility`, speed (sweep period in animated mode), `strokeWidth, direction` (forward or reverse), `showGuide` (graticule grid visibility), `intensity` (vertical amplitude from 0.1 to 1.0), and `samples` (text: bounded numeric sample list for sampled mode).

---

## E. Staged Milestone Breakdown

> **Governance Rule:** Only **one** implementation milestone may be authorised at a time. Hydrogen must stop at the completion of each milestone for user verification and approval before beginning the next milestone.

### Milestone 1 (`16D-B-M1`): Flow Lite (flow-lite) — Multi-Point Path, Endpoint Modes, Delays and Pattern Controls
- **Scope:**
  - **Reused Infrastructure:** Reuses `geometry.Point`, perpendicular quadratic-Bézier midpoint bend math from `pkg/engine/geometry/flowpath.go, #dynamicFlowReticles` and `#flowCurvePath` waypoint overlay mechanics in `stage.js` and `flows.js, LayerManager`, Studio grouping, and `ObjectPropertyCatalog["lite"]`.
  - **New Go Engine Work (`pkg/engine/scene/model.go, lite_effect.go, pkg/engine/geometry/flow_geom.go, pkg/engine/scene/widget.go`):**
    - Add `Points, Bends, Curvature, DashLength, GapLength, DotSize, StartMode, EndMode, Delay, and EndDelay` fields to `scene.LiteEffect`.
    - Implement `GenerateFlowLitePath, ComputeFlowLiteBounds, ComputeFlowLiteLength, and DistanceToFlowLitePath` in `pkg/engine/geometry/` supporting multi-point paths with per-section bends (-100 to +100) and whole-path curvature (-100 to +100).
    - Add `dashLength, gapLength, and dotSize` to `ObjectPropertyCatalog["lite"]` and update `direction` options to support Forward and Reverse (while preserving clockwise/counterclockwise compatibility for Perimeter Tracer Lite).
  - **New Studio UI and Runtime Work (`index.html, lite-effects.js, stage.js, context-editor.js, app.js, layers-ui.js, svg-widget.js`):**
    - Add the Flow Lite creation button (`#btnAddLiteFlow`), multi-point waypoint reticle editing (Add Point, Drop Point, point/section selector, Section Bend slider from -100 to +100, Whole-Path Curvature slider from -100 to +100), pattern controls (Dashed, Dotted, and Single Segment with dash length, gap spacing, dot size, and line thickness), independent Start Mode and End Mode selectors, Forward/Reverse direction selector, and Start Delay/End Delay controls.
    - Implement single-path SVG and scoped CSS @keyframes generation in `lite-effects.js` and runtime handlers (`dashLength, gapLength, dotSize, direction`) in `svg-widget.js`.
- **Dependencies:** Stage 16D-A baseline (`bf7ee3e` and `7e8d4f6`).
- **Stop Condition:** Stop after `go test ./...`, `node scripts/regression-harness.js`, and real-browser CDP verification of Flow Lite on Kiwiberry (ports 8088 and 8091) for user review and approval.

### Milestone 2 (`16D-B-M2`): Radar Ping Lite (radar-lite) — Expanding-Outline Circular Ping Effect
- **Scope:**
  - Extend `scene.LiteEffect` with `Rings` (1 to 6) and radar-lite validation (Single Ring, Multi-Ring, and Fading Ping variants).
  - Add `@keyframes sva-lite-radar` and staggered <ellipse> ring generation with optional center core dot (`showGuide`) to `lite-effects.js`.
  - Add the Radar Ping creation button and inspector controls (`index.html, context-editor.js, app.js, svg-widget.js`).
- **Dependencies:** Milestone 1 (`16D-B-M1`) accepted.
- **Stop Condition:** Stop after Go, JS regression, standalone export, and real-browser CDP verification of Radar Ping Lite on Kiwiberry for user review and approval.

### Milestone 3 (`16D-B-M3`): Gauge / Level Lite (gauge-lite) — Static Data-Driven Numeric Indicators
- **Scope:**
  - Extend `scene.LiteEffect` with `Value, MinValue, MaxValue, BgColor, UseThresholds, WarnThreshold, WarnColor, CritThreshold, and CritColor` fields and gauge-lite validation across all five variants.
  - Add `value, minValue, maxValue, and bgColor` to `ObjectPropertyCatalog["lite"]` (`pkg/engine/scene/widget.go`) and implement static data-driven DOM update handlers and application-colour-precedence logic in `svg-widget.js`.
  - Add the Gauge / Level creation button, five variant SVG builders (`lite-effects.js`), and Drawer / Context Editor controls (`index.html, context-editor.js, app.js`).
- **Dependencies:** Milestone 2 (`16D-B-M2`) accepted.
- **Stop Condition:** Stop after verifying all five gauge variants, actual numeric threshold evaluation, application colour override precedence, valid zero vs. missing data behaviour, and off-screen `setParameter()` updates on Kiwiberry for user review and approval.

### Milestone 4 (`16D-B-M4`): Oscilloscope Lite (oscilloscope-lite) and Stage 16D-B Closeout Verification
- **Scope:**
  - Extend `scene.LiteEffect` with `Samples` and oscilloscope-lite validation across seven waveform presets and three operating modes (Static, Animated 2-tile clipped CSS scroll, and Sampled bounded numeric sequence).
  - Add `samples` to `ObjectPropertyCatalog["lite"]` and implement bounded sample parsing (2 to 64 finite numbers) and in-place <path> updates in `svg-widget.js`.
  - Add the Oscilloscope creation button and inspector controls (`index.html, lite-effects.js, context-editor.js, app.js`).
  - Run complete Stage 16D-B regression and real-browser CDP verification across all seven Low-Load effect families and update project documentation and BookStack.
- **Dependencies:** Milestone 3 (`16D-B-M3`) accepted.
- **Stop Condition:** Stop after complete Stage 16D-B verification for final Stage 16D-B human visual acceptance.

---

## F. Acceptance Criteria and Testing Strategy

### 1. Revised Milestone 1 (`16D-B-M1`: Flow Lite) Acceptance Criteria
Milestone 1 will be verified across Go unit tests (`go test ./...`), the Node regression harness (`node scripts/regression-harness.js`), and real-browser Chromium CDP automation on Kiwiberry (ports 8088 and 8091) against the following 16 criteria:
1. **Multi-Point Path Authoring:** Creating a Flow Lite object initializes a 2-point path; clicking Add Point adds points (N2, N3, ...) and Drop Point removes points down to the 2-point minimum; dragging reticles on the stage moves individual points accurately.
2. **Straight and Bent Sections:** Setting section bend to 0 renders a straight line segment; setting a non-zero section bend (-100 to +100) bends that specific section into a smooth quadratic Bézier curve.
3. **Independent Bend Adjustments:** Modifying the bend of Section 1 on a 4-point path curves only Section 1 (N1 to N2) while leaving Section 0 (N0 to N1) and Section 2 (N2 to N3) unchanged; optional whole-path Curvature offsets all sections predictably.
4. **Pattern Continuity Across Complete Path:** The entire multi-point path is emitted as a single continuous <path> element so moving dashes and dots travel seamlessly across intermediate waypoints and bends without resetting or jumping at vertices.
5. **Forward and Reverse Direction:** Selecting Forward moves the pattern from start (P0) to end (PK-1); selecting Reverse moves the pattern from end (PK-1) to start (P0).
6. **Dashed and Dotted Spacing Controls:** In Dashed mode, `dashLength` and `gapLength` produce exact pixel dash and gap lengths along the path; in Dotted mode, `dotSize` and `gapLength` produce circular dots separated by `gapLength` clear pixels without overlapping; in Single Segment mode, a single segment of length `dashLength` traverses the path per cycle.
7. **Stroke Thickness:** Changing `strokeWidth` updates the rendered line thickness in Dashed and Single Segment modes (while `dotSize` controls dot diameter in Dotted mode).
8. **Both Start Endpoint Modes (`startMode`):**
   - Tunnel: incoming segments emerge progressively from the start point, growing geometrically from 0 px to full `dashLength` (verified by sampling rendered visible stroke length at the start of the cycle, not opacity fading).
   - Abrupt: the pattern starts immediately visible at full segment length at the start of movement.
9. **Both End Endpoint Modes (`endMode`):**
   - Tunnel: outgoing segments shorten geometrically from full `dashLength` down to 0 px as they enter the end point.
   - Abrupt: the pattern travels at full segment length to the end point and disappears abruptly upon completion without shortening into the end point.
10. **Mixed Start and End Modes:** Tunnel entry combined with Abrupt exit, and Abrupt entry combined with Tunnel exit, both operate independently and pass temporal frame-sampling checks.
11. **Start and End Delays:** Setting Start Delay (`delay` > 0) holds the flow before starting at P0 for the configured duration; setting End Delay (`endDelay` > 0) holds after arrival at PK-1 for the configured duration before restarting or reversing.
12. **Loop, Ping-Pong, and Once Modes:** Loop repeats continuously with start and end delays; Ping-Pong alternates forward and reverse passes with exact turnaround delays; Once completes a single pass and stops in its end state.
13. **Studio Preview and Layer/Group Integration:** Live canvas preview, `#nodeSelectionBox` bounds, Layers panel ordering/visibility/locking, and Ctrl+G group movement work identically to established Studio objects.
14. **Save/Reload and Undo/Redo:** `.sva` project export/import, SQLite project save/load, and `HistoryManager` undo/redo preserve all points, per-section bends, pattern dimensions, endpoint modes, and delays.
15. **Standalone SVG Export:** Exported standalone `.svg` files validate as well-formed XML, strip editor-only markers, and animate all Flow Lite modes and delays natively in the browser without external scripts.
16. **Complete Stage 16C Widget Export and Independent Instances:** Exporting a widget package and mounting multiple instances in the Stage 16C consumer (port 8091) confirms independent runtime control of exposed Flow Lite properties (color, `opacity, visibility`, speed, `strokeWidth, dashLength, gapLength, dotSize, direction, playbackMode, showGuide`) and clean off-screen `IntersectionObserver` suspension and resumption.

### 2. Milestones 2–4 Acceptance Summary
- **M2 (Radar Ping Lite):** Verifies single, multi-ring (2 to 6), and fading-ping expansion from `minScale` to `maxScale` with zero <filter> blur, keyframe scoping in widget packages, off-screen suspension, and independent Stage 16C consumer control.
- **M3 (Gauge / Level Lite):** Verifies all five gauge variants, zero continuous animation work between updates, threshold banding in the gauge's actual [minValue, maxValue] numeric domain, explicit application-supplied `<name>-lite-color` precedence over threshold banding, valid zero vs. missing data handling, and off-screen telemetry updates while suspended.
- **M4 (Oscilloscope Lite):** Verifies all seven waveform presets across Static, Animated (2-tile <clipPath> CSS scroll with collision-free scoped clip IDs), and Sampled (2 to 64 bounded numeric samples updated only on `setParameter('<name>-lite-samples', ...)`) modes, plus full Stage 16D-B regression verification.

---

## G. Architectural Trade-Offs, Risks, and Exclusions

### 1. Architectural Trade-Off Analysis: Tunnel Entry/Exit and Delays on a Multi-Point Path
During design review, two implementation approaches were evaluated for open-path tunnel entry/exit and start/end delays on a multi-point path of length L:
- **Alternative 1 (Rejected) — Per-Frame JavaScript RAF Path Slicing (`getPointAtLength` / `setAttribute`):** Running a 60 Hz JavaScript loop to recompute sub-paths or mutate dash offsets and delay timers every frame. The Test Lab (`m6-01`) proved that per-frame geometry queries cost significantly more CPU and main-thread task time, and require inline <script> execution that fails when an SVG is embedded via <img>.
- **Alternative 2 (Recommended) — Single Continuous <path> with True Pixel-Length Wave Dashing and Scoped CSS @keyframes:** Computing the multi-point path's exact pixel length L in Go (`ComputeFlowLiteLength`), emitting a wave of length W_pat followed by a clearing gap of at least L + W_pat + d in real SVG pixel units, and encoding `startMode, endMode, delay, and endDelay` directly into the element's scoped CSS @keyframes.
  - **Why Alternative 2 is recommended:** It produces genuine geometric length growth at the start point (0 to d) and genuine geometric shortening at the end point (d to 0) via native browser path-endpoint stroke clipping, works across arbitrary straight and quadratic-Bézier multi-point paths, requires zero JavaScript timers, runs inside <img> tags as well as standalone <svg> and Stage 16C widgets, and pauses cleanly via `.sva-suspended`.
  - **Runtime caveat handled cleanly:** If a consumer dynamically changes `dashLength, gapLength, or dotSize` at runtime via `setParameter()` on an exported widget, `svg-widget.js` reads the path length stored in `data-path-length` on the <path> element and recomputes stroke-dasharray and the CSS custom properties `--sva-flow-from` and `--sva-flow-to` in one lightweight arithmetic update without recalculating path geometry.

### 2. Explicit Exclusions
- **No automatic route-finding or node attachment on Flow Lite:** Users control path geometry directly through points and section bends.
- **No closed-shape tunnel grow/shrink:** Closed Perimeter Tracer Lite remains unchanged from Stage 16D-A.
- **No replacement of existing Flow, Glow, or Ripple systems.**
- **No Stage 16D-C text/font features, Stage 16E group-property aliases, canvas/background fitting changes, or Digital Twin alias/telemetry services.**

---

## H. Decisions Requiring User Approval

1. **Approve the revised Stage 16D-B plan** incorporating the multi-point path and per-section bend model, independent start/end Abrupt and Tunnel modes, declarative Start Delay and End Delay timing, pixel-based Dashed/Dotted/Single Segment pattern controls, and the static Gauge Lite colour precedence contract.
2. **Authorise Milestone 1 (`16D-B-M1`: Flow Lite) only** for implementation. (Approved; M1 implemented in commit `a3a7d24` and **ACCEPTED** by user.)
3. **Authorise Milestone 2 (`16D-B-M2`: Radar Ping Lite) and the two minor M1 Flow Lite inspector corrections.** (Approved; M2 implemented in commit `ad814bb` and **ACCEPTED** by user.)
4. **Authorise Milestone 3 (`16D-B-M3`: Gauge / Level Lite) only** for implementation. (Approved; M3 implemented and awaiting user acceptance.)

---

## I. Milestone 1 (`16D-B-M1`: Flow Lite) Implementation Evidence & Status

**Milestone Status:** **ACCEPTED (`a3a7d24`, with minor M1 inspector corrections completed in M2)**
**Live Studio URL:** `http://192.168.100.40:8088/`  
**Live 10-Example Visual Demonstration URL:** `http://192.168.100.40:8088/flow-lite-demo.html`  
**Live Stage 16C Widget Consumer URL:** `http://192.168.100.40:8091/index.html`

### 1. Rendering Feasibility Gate (24 / 24 Combinations Passed)
Before integrating Flow Lite into Studio, an automated Chromium CDP feasibility gate (`scripts/cmd/verify-flow-lite-feasibility/main.go`) verified per-segment **Tunnel** and **Abrupt** start and end behaviour across all 24 combinations of:
- **3 Pattern Modes:** Single segment, repeating dashed stream, and circular dotted stream
- **4 Endpoint Combinations:** Tunnel entry + Tunnel exit, Tunnel entry + Abrupt exit, Abrupt entry + Tunnel exit, and Abrupt entry + Abrupt exit
- **2 Travel Directions:** Forward (start to end) and Reverse (end to start)
- **4-Waypoint Multi-Bend Path:** Straight section (bend 0) followed by upward and downward quadratic Bézier curved sections (+50 and -45)

**Chosen Rendering Mechanism:**
- A single shared multi-point `<path id="lite-flow-track-{id}">` in `<defs>` defines the continuous geometry across all waypoints and per-section quadratic Bézier bends.
- Each travelling dash, dot, or single segment is rendered as a lightweight `<use href="#lite-flow-track-{id}" class="lite-payload lite-flow-seg">` carrying a single active segment (`stroke-dasharray="{dashLen} {clearGap}"`) with shared scoped CSS `@keyframes` and staggered phase delay.
- **Why a single uniform `stroke-dasharray="d g"` path was rejected for mixed/abrupt modes:** On a single multi-dash `<path>`, native stroke clipping at the path endpoints naturally produces progressive tunnel growth and shortening, but cannot make an individual outgoing dash disappear instantaneously at full length `[L - d, L]` while trailing dashes continue moving along the same path, nor make an incoming dash appear instantaneously at full length `[0, d]` without emerging from `[-d, 0]`.
- With per-segment staggered `<use>` instances and an even-odd `<clipPath>` masking only the tiny round-cap overhang boxes (`2r × 2r`) at tunnel endpoints:
  - **Tunnel Entry** starts at offset `+d` and grows progressively from `0 px` to full length `d` across `P0`.
  - **Abrupt Entry** starts at offset `0` (`-2r` on lead-extended round-cap paths) so each segment enters immediately at full length `d`.
  - **Tunnel Exit** travels to offset `-(L + 2r)` so each segment shortens progressively from `d` down to `0 px` across `P_end`.
  - **Abrupt Exit** travels to offset `-(L - d)` where the segment is still 100% inside the path at full length `d`, and then steps to `opacity: 0` while trailing segments continue uninterrupted.

### 2. Implemented Production Files
- **Go Authoritative Engine & Geometry:**
  - `pkg/engine/geometry/flow_geom.go`: Added `GenerateFlowLitePath`, `SampleFlowLitePath`, `ComputeFlowLiteLength`, `ComputeFlowLiteBounds`, and `DistanceToFlowLitePath`.
  - `pkg/engine/scene/model.go` & `pkg/engine/scene/lite_effect.go`: Extended `LiteEffect` with `Points`, `Bends`, `Curvature`, `DashLength`, `GapLength`, `DotSize`, `StartMode`, `EndMode`, `Delay`, and `EndDelay`, full operational validation (`ValidateLiteEffect`), defaults (`applyLiteEffectDefaults`), and patch updates (`UpdateLiteEffect`).
  - `pkg/engine/scene/deepcopy.go`, `pkg/engine/scene/scene.go`, `pkg/engine/scene/history.go`, `pkg/engine/scene/group.go`: Added `DeepCopyLiteEffect` (deep-copying `Points` and `Bends` slices for history snapshots), group translation and bounds computation for `flow-lite`.
  - `pkg/engine/scene/widget.go`: Added `dashLength`, `gapLength`, and `dotSize` to `ObjectPropertyCatalog["lite"]` and extended `direction` options to include `forward` and `reverse`.
  - `pkg/engine/scene/lite_effect_test.go`: Added unit test `TestFlowLiteMultiPointBendsEndpointsAndWidgetScoping`.
- **Frontend Studio Adapter & Widget Runtime:**
  - `internal/web/static/index.html`: Added `+ Flow` button (`#btnAddLiteFlow`) in the Low-Load Effects drawer, full Flow Lite property controls in both `#liteFlowGroup` (left drawer) and `#fctx-liteFlowGroup` (floating Context Editor), and included `js/svg-widget.js`.
  - `internal/web/static/js/lite-effects.js`: Implemented Flow Lite creation, multi-point waypoint CRUD (`addWaypointToActive`, `dropWaypointFromActive`, `selectWaypoint`, `updateActiveWaypointCoord`, `updateActiveSectionBend`, `translateFlowLitePoints`), canvas waypoint reticle overlay (`updateUIOverlay`), bounding box calculation, and declarative SVG generation (`buildFlowLiteContent`).
  - `internal/web/static/js/stage.js`, `coordinator.js`, `context-editor.js`, `app.js`, `layers-ui.js`, `history.js`, `engine-bridge.js`: Wired waypoint reticle dragging, whole-path canvas dragging, group dragging, History undo/redo snapshot synchronization, and Three-Tier widget property exposure cards.
  - `internal/web/static/js/svg-widget.js`: Added runtime parameter handlers for `dashLength`, `gapLength`, `dotSize`, `strokeWidth`, `direction` (`forward` / `reverse`), `playbackMode` (`loop` / `pingpong` / `once`), `speed` (preserving phase stagger across segments), `color`, `opacity`, `visibility`, and `showGuide`.
  - `internal/web/static/flow-lite-demo.html`: Created the 10-example side-by-side visual demonstration page.

### 3. Verification Results Summary
- **Feasibility Gate (`go run ./scripts/cmd/verify-flow-lite-feasibility/main.go`):** **24 / 24 PASS**
- **Go Unit & Package Tests (`go vet ./... && go test ./...`):** **PASS (100%)**
- **Node Regression Harness (`node scripts/regression-harness.js`):** **89 / 89 Scenarios PASS** (including Scenario 89 testing Flow Lite multi-point authoring, bends, endpoints, and serialization)
- **Real-Browser Chromium CDP Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m1/main.go`):** **16 / 16 Gates PASS (`AT-M1-01` through `AT-M1-16`)**
- **Minor M1 Flow Lite Adjustments Completed in M2:**
  - Renamed the Flow Lite speed inspector label to **Cycle Duration** in both the Low-Load Effects drawer (`#liteFlowGroup`) and the floating Context Editor (`#fctx-liteFlowGroup`) while preserving the `speed` property key for backward compatibility.
  - Extended Flow Lite Cycle Duration range to **`0.2 s` – `30.0 s`** across HTML sliders, numeric inputs, Go validation (`ValidateLiteEffect`, `applyLiteEffectDefaults`, `UpdateLiteEffect`), serialization/restoration, undo/redo, standalone SVG export, and the Stage 16C widget runtime.

---

## J. Milestone 2 (`16D-B-M2`: Radar Ping Lite) Implementation Evidence & Status

**Milestone Status:** **ACCEPTED (`ad814bb`)**
**Live Studio URL:** `http://192.168.100.40:8088/`
**Live 9-Example Radar Ping Lite Visual Showcase URL:** `http://192.168.100.40:8088/radar-lite-demo.html`
**Live 10-Example Flow Lite Visual Showcase URL:** `http://192.168.100.40:8088/flow-lite-demo.html`
**Live Stage 16C Widget Consumer URL:** `http://192.168.100.40:8091/index.html`

### 1. Architecture & Rendering Model
- **Authoritative Go Ownership (`pkg/engine/scene/model.go`, `pkg/engine/scene/lite_effect.go`, `pkg/engine/scene/widget.go`):**
  - Added `"radar-lite"` (`kind: "radar-lite"`) and `Rings int` (`json:"rings,omitempty"`, `1..6`) to `scene.LiteEffect`.
  - Enforced strict validation in `ValidateLiteEffect`, defaults in `applyLiteEffectDefaults`, and patch clamping in `UpdateLiteEffect` for `variant` (`"single"`, `"multi"`, `"fading-ping"`), `rings` (`1..6`, normalized to `1` when `variant == "single"`), `minScale` (`0.05..2.0`), `maxScale` (`0.1..3.0`, strictly `> minScale`), `period` (`0.2..30.0 s`), `strokeWidth` (`0.5..20.0 px`), `showGuide` (centre marker dot), and `maintainAspectRatio` (circular vs. elliptical geometry).
  - Updated `ObjectPropertyCatalog["lite"]` in `pkg/engine/scene/widget.go` so `color`, `opacity`, `visibility`, `speed`, `strokeWidth`, `scale`, `period`, and `showGuide` are eligible for three-tier Stage 16B/16C widget parameter exposure.
- **Filter-Free Declarative CSS SVG Rendering (`internal/web/static/js/lite-effects.js`):**
  - Renders `1..6` concentric `<ellipse class="lite-payload lite-radar-ring sva-lite-radar-anim" vector-effect="non-scaling-stroke">` rings plus an optional `<circle class="lite-guide-track lite-radar-center">` centre anchor dot.
  - Uses zero SVG filter primitives (`<filter>`, `<feGaussianBlur>`, `<feDisplacementMap>`, `<feColorMatrix>`).
  - Shared `@keyframes sva-lite-radar` expands each ring monotonically from `scale(var(--sva-radar-min))` to `scale(var(--sva-radar-max))` while fading opacity from `1` to `0`, with `transform-box: fill-box; transform-origin: center` and `vector-effect="non-scaling-stroke"` so ring stroke thickness remains visually constant throughout expansion.
  - Multi-ring staggering assigns each ring `k` (`0 .. ringCount-1`) a negative `animation-delay: -(k / ringCount) * period` and stores `--sva-phase-ratio: k / ringCount`. When `period` or `speed` is updated at runtime via `SVGWidget.setParameter()`, `svg-widget.js` recomputes both `animationDuration` and staggered `animationDelay` so rings never collapse into synchronized phase.

### 2. Verification Results Summary
- **Go Unit & Package Tests (`go vet ./... && go test ./...`):** **PASS (100%)** (including `TestRadarPingLiteVariantsRingsScaleAndPersistence` and `30.0 s` Flow Lite Cycle Duration tests in `pkg/engine/scene/lite_effect_test.go`).
- **Node Regression Harness (`node scripts/regression-harness.js`):** **90 / 90 Scenarios PASS** (including Scenario 90 verifying Flow Lite `30.0 s` Cycle Duration and Radar Ping Lite `single`, `multi` `1..6` rings, `fading-ping`, elliptical geometry, centre marker, and `.sva` / project round-trip).
- **Stage 16D-B-M1 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m1/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M2 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m2/main.go`):** **16 / 16 Gates PASS (`AT-M2-01` through `AT-M2-16`)**, including temporal browser sampling of monotonic outward ring expansion, opacity fade-out, multi-ring staggered phase separation, negative controls for stationary/synchronized rings, standalone SVG export, Stage 16C multi-instance widget isolation, and off-screen suspension/resumption.

---

## K. Milestone 3 (`16D-B-M3`: Gauge / Level Lite) Implementation Evidence & Status

**Milestone Status:** **ACCEPTED (`a086170`), WITH FOUR-THRESHOLD ENHANCEMENT COMPLETED IN M4**
**Live Studio URL:** `http://192.168.100.40:8088/`
**Live Gauge / Level Lite Visual Showcase URL:** `http://192.168.100.40:8088/gauge-lite-demo.html`
**Live Radar Ping Lite Visual Showcase URL:** `http://192.168.100.40:8088/radar-lite-demo.html`
**Live Flow Lite Visual Showcase URL:** `http://192.168.100.40:8088/flow-lite-demo.html`
**Live Stage 16C Widget Consumer URL:** `http://192.168.100.40:8091/index.html`

### 1. Five Planned Gauge / Level Lite Variants Implemented
All five planned indicator styles from Section D.4.2 are implemented under `kind: "gauge-lite"` with zero idle CSS, SMIL, or `requestAnimationFrame` loops:
1. **Circular / Ring Gauge (`variant: "ring"`):** Background track `<ellipse pathLength="100">` paired with a foreground value arc `<ellipse pathLength="100" stroke-dasharray="{frac*100} 200">` starting at 12 o'clock (`rotate(-90)`), supporting both clockwise (`forward`) and counter-clockwise (`reverse`) fill directions.
2. **Horizontal Bar Gauge (`variant: "hbar"`):** Rounded background track `<rect>` paired with a proportional horizontal fill `<rect>` anchored left-to-right (`forward`) or right-to-left (`reverse`).
3. **Vertical Bar Gauge (`variant: "vbar"`):** Rounded background track `<rect>` paired with a proportional vertical fill `<rect>` anchored bottom-to-top (`forward`) or top-to-bottom (`reverse`).
4. **Segmented Bar Gauge (`variant: "segmented"`):** Discrete cell array (`segments`, `2..32`, default `10`), lighting `round(frac * segments)` cells in the active fill colour and remaining cells in `bgColor`.
5. **Filled-Level Indicator (`variant: "level"`):** Framed vessel container `<rect>` stroked in the active status colour with a translucent interior background and a rising (or top-down) interior fill `<rect>`.

### 2. Range, Valid Zero, Four Independently Enabled Thresholds & Colour Precedence Contract
- **Actual Numeric Domain & Valid Zero:**
  - `Value`, `MinValue`, `MaxValue`, `WarnThreshold`, and `CritThreshold` are stored as `*float64` in Go (`pkg/engine/scene/model.go`) so `0.0` is preserved as an explicit valid number and never confused with missing data.
  - Visual fill fraction `frac = clamp((value - minValue) / (maxValue - minValue), 0.0, 1.0)` clamps only the graphical indicator to `[0.0, 1.0]` while preserving the exact incoming semantic value in `data-gauge-value` and the optional value readout text.
- **Four Independently Enabled Thresholds (`Thresholds []GaugeThresholdBand`):**
  - Each Gauge Lite instance carries four threshold bands (`Band 1` through `Band 4`), each having its own `Enabled bool`, `Value float64`, and `Color string`.
  - Any combination of 0, 1, 2, 3, or 4 thresholds can be enabled simultaneously, including non-contiguous combinations (e.g., Threshold 1 enabled at `40` Green, Threshold 2 disabled at `60` Yellow, Threshold 3 enabled at `80` Orange, Threshold 4 enabled at `95` Red).
  - Disabled thresholds are completely ignored during evaluation and do not constrain the numeric ordering of enabled thresholds.
  - Enabled thresholds are sorted in ascending numeric order and evaluated with inclusive `>=` boundary semantics (`value >= threshold.value`), applying the colour of the highest active threshold reached and falling back to the gauge's base `color` when `value` is below all enabled thresholds (or when all thresholds are disabled).
  - Existing two-threshold configurations (`warnThreshold` / `critThreshold`) migrate deterministically into Bands 3 and 4 (with Bands 1 and 2 disabled by default).
- **Colour Precedence Rule:**
  1. **Explicit Application Colour Override (`setParameter('<name>-lite-color', '#...')`):** Sets `data-gauge-color-override="true"` and `data-gauge-override-color` on the gauge instance, taking precedence over automatic threshold banding. Calling `resetParameters()` clears the override and restores automatic threshold evaluation.
  2. **Four-Band Threshold Evaluation:** Evaluates only enabled threshold bands in the gauge's actual `[minValue, maxValue]` numeric domain.
  3. **Base Authored Colour (`color`):** Used when neither an application override nor any enabled threshold is reached.

### 3. Verification Results Summary
- **Go Unit & Package Tests (`go vet ./... && go test ./...`):** **PASS (100%)** (including `TestGaugeLiteVariantsRangesThresholdsAndPersistence` in `pkg/engine/scene/lite_effect_test.go`).
- **Node Regression Harness (`node scripts/regression-harness.js`):** **92 / 92 Scenarios PASS** (including Scenarios 91 and 92 verifying all 5 Gauge Lite variants, 4 independently enabled thresholds, non-contiguous bands, actual-domain RPM thresholds, valid zero, and project serialization round-trip).
- **Stage 16D-B-M1 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m1/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M2 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m2/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M3 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m3/main.go`):** **16 / 16 Gates PASS (`AT-M3-01` through `AT-M3-16`)**.

---

## L. Milestone 4 (`16D-B-M4`: Oscilloscope Lite & Part A Four-Threshold Gauge Enhancement) Implementation Evidence & Status

**Milestone Status:** **Stage 16D-B-M4 — IMPLEMENTED, AWAITING USER ACCEPTANCE**
**Live Studio URL:** `http://192.168.100.40:8088/`
**Live Oscilloscope Lite Visual Showcase URL:** `http://192.168.100.40:8088/oscilloscope-lite-demo.html`
**Live Updated 4-Threshold Gauge Lite Visual Showcase URL:** `http://192.168.100.40:8088/gauge-lite-demo.html`
**Live Radar Ping Lite Visual Showcase URL:** `http://192.168.100.40:8088/radar-lite-demo.html`
**Live Flow Lite Visual Showcase URL:** `http://192.168.100.40:8088/flow-lite-demo.html`
**Live Stage 16C Widget Consumer URL:** `http://192.168.100.40:8091/index.html`

### 1. Seven Approved Oscilloscope Waveform Presets (`kind: "oscilloscope-lite"`)
All seven planned waveform styles from Section D.5 are implemented with exact periodic boundary continuity (`y(0) = y(1) = yBase`):
1. **ECG / Pulse (`variant: "ecg"`):** Clinical P-QRS-T heartbeat complex returning cleanly to the baseline at both period boundaries.
2. **Sine Wave (`variant: "sine"`):** Smooth 16-segment harmonic sinusoid per period.
3. **Square Wave (`variant: "square"`):** Crisp digital high/low pulse train with exact baseline transitions at period boundaries.
4. **Triangle Wave (`variant: "triangle"`):** Linear constant-slope peak-and-trough triangular waveform.
5. **Sawtooth Wave (`variant: "sawtooth"`):** Linear ramp waveform with vertical mid-cycle reset and exact baseline continuity at period boundaries.
6. **Segmented Line / Polyline (`variant: "polyline"`):** Multi-vertex angular telemetry trace.
7. **Smooth Curve / Bézier (`variant: "bezier"`):** Cubic Bézier (`C`) smoothed waveform path.

### 2. Three Operating Modes & Seamless Periodic Scrolling
- **Animated Pattern Mode (`playbackMode: "animated"`):**
  - Renders `2 * cycles` identical waveform periods spanning `[x, x + 2*w]` inside an instance-scoped `<clipPath id="lite-osc-clip-{id}">` viewport of width `w`.
  - Scrolls horizontally via compositor-friendly CSS `@keyframes sva-lite-osc-scroll` (`transform: translateX(0px)` to `translateX(var(--sva-osc-shift, -w))`), supporting both `forward` and `reverse` scroll directions and Cycle Duration (`speed`) from `0.2 s` to `30.0 s`.
  - Because Tile 2 (`[x + w, x + 2*w]`) is mathematically identical to Tile 1 (`[x, x + w]`) and every preset satisfies `y(0) = y(1) = yBase`, the loop wrap at `100% -> 0%` has zero visual jump or seam.
- **Static Pattern Mode (`playbackMode: "static"`):**
  - Renders `1 * cycles` waveform periods inside the viewport with zero running CSS, SMIL, or JavaScript animations (`getAnimations().length === 0`).
- **Sampled / Data-Driven Waveform Mode (`playbackMode: "sampled"`):**
  - Renders a bounded numeric sample sequence (`samples`, 2 to 64 finite numbers parsed via `ParseOscilloscopeSamples` in Go and validated in `svg-widget.js`) across `[x, x + w]` as either a polyline or cubic Bézier trace, with zero continuous animation loop while samples remain unchanged.
  - Invalid sample sequences (`< 2` samples, `> 64` samples, or non-finite tokens) are rejected atomically by both the Go engine (`ValidateLiteEffect` / `UpdateLiteEffect`) and `SVGWidget.setParameter()`, preserving the last valid waveform state.

### 3. Verification Results Summary
- **Go Unit & Package Tests (`go vet ./... && go test ./...`):** **PASS (100%)** (including `TestOscilloscopeLitePresetsModesSamplesAndPersistence` and 4-threshold assertions in `pkg/engine/scene/lite_effect_test.go`).
- **Node Regression Harness (`node scripts/regression-harness.js`):** **92 / 92 Scenarios PASS** (including Scenario 92 testing 4-threshold Gauge Lite and Oscilloscope Lite presets, modes, sample validation, and project serialization round-trip).
- **Stage 16D-B-M1 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m1/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M2 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m2/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M3 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m3/main.go`):** **16 / 16 Gates PASS**.
- **Stage 16D-B-M4 Real-Browser Acceptance Suite (`go run ./scripts/cmd/verify-stage16d-b-m4/main.go`):** **15 / 15 Gates PASS (`AT-M4-01` through `AT-M4-15`)**.
