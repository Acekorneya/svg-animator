# host-monitor-v1 — Widget Properties & Integration Guide

**Widget File:** `host-monitor-v1.svg`  
**Format:** SVG Widget (Stage 16B Isolated Multi-Instance)  
**Scene Schema Version:** `1.2.0`  
**Protocol Version:** `1.0.0`  
**Generated At:** 2026-11-16 09:14:16 UTC  

---

## 1. Overview & Architecture

`host-monitor-v1` is a self-contained, collision-free SVG widget exported from **SVG Animator Studio**.
It encapsulates vector graphics, embedded animations, filter definitions, and an external parameter manifest.

- **Native Canvas Dimensions:** 360 × 220 px (viewBox `0 0 360 220`)
- **Isolation Model:** Element IDs, gradients, filters, and animation keyframes are automatically scoped per mounted instance.
- **Multi-Instance Capability:** Multiple copies of this widget can coexist inside the same HTML document or DOM tree with zero ID collisions or styling crosstalk.

## 2. Controllable Objects & Scene Inventory

The following inventory lists the visual objects authored within this widget and their corresponding external parameter bindings:

| Target Identifier | Object Name | Object Type | Geometry / Position | Layer | Bound Parameters |
|---|---|---|---|---|---|
| `node-1` | hostname | Node (rounded) | (40.0, 70.0) 150×80 | Base | `hostname-node-text`, `hostname-node-fill`, `hostname-node-stroke` |
| `glow-1` | status | Glow Hotspot | (290.0, 60.0) size=28 | Optical | `status-glow-color`, `status-glow-opacity` |
| `flow-1` | traffic | Connector Flow | Nodes: 2 | Base | `traffic-flow-color`, `traffic-flow-speed`, `traffic-flow-visibility` |
| `ripple-1` | heartbeat | Ripple Wave | (290.0, 155.0) maxR=32 | Optical | `heartbeat-ripple-opacity`, `heartbeat-ripple-visibility` |

---

## 3. Parameter Specification & API Reference

| Parameter Name | Type | Target | Property | Default | Permitted Limits / Values | Unit | Description |
|---|---|---|---|---|---|---|---|
| **`hostname-node-text`** | `text` | `node-1` | `text` | host-01 | - | - | Controls text property on node-1 |
| **`hostname-node-fill`** | `color` | `node-1` | `fill` | `#1e293b` | - | - | Controls fill property on node-1 |
| **`hostname-node-stroke`** | `color` | `node-1` | `stroke` | `#38bdf8` | - | - | Controls stroke property on node-1 |
| **`traffic-flow-color`** | `color` | `flow-1` | `color` | `#22c55e` | - | - | Controls color property on flow-1 |
| **`traffic-flow-speed`** | `speed` | `flow-1` | `speed` | 1 | 0.2 to 5.0 | - | Controls speed property on flow-1 |
| **`traffic-flow-visibility`** | `boolean` | `flow-1` | `visibility` | true | `true`, `false` | - | Controls visibility property on flow-1 |
| **`status-glow-color`** | `color` | `glow-1` | `color` | `#22c55e` | - | - | Controls color property on glow-1 |
| **`status-glow-opacity`** | `number` | `glow-1` | `opacity` | 0.8 | 0.0 to 1.0 | - | Controls opacity property on glow-1 |
| **`heartbeat-ripple-opacity`** | `number` | `ripple-1` | `opacity` | 0.7 | 0.0 to 1.0 | - | Controls opacity property on ripple-1 |
| **`heartbeat-ripple-visibility`** | `boolean` | `ripple-1` | `visibility` | true | `true`, `false` | - | Controls visibility property on ripple-1 |

---

## 4. Animation & Transport Capabilities

SVG Animator Studio distinguishes between authored animation loops and external host controls:

- **Connector Flows, Optical Glows & Ripples:** Run continuously via browser-native SVG animation engines.
- **Instance Animation Playback Control:**
  - `widget.play()`: Resumes/plays all animations within the instance.
  - `widget.pause()`: Freezes SMIL and CSS animations for the instance only.
  - `widget.setSpeed(multiplier)`: Scales playback speed (`0.1` to `5.0`) on supported animations.
  - `widget.resetParameters()`: Restores all bound properties to authored scene defaults.
  - Individual instance playback does **not** interfere with other instances on the same page.

---

## 5. No-Code Usage Guide (For Non-Programmers)

This widget package includes a pre-built interactive control studio:

1. Locate `demo.html` in this package directory.
2. Double-click `demo.html` to open it in any modern browser (Chrome, Edge, Firefox, Safari).
3. The page automatically mounts **three independent instances** (Unit A, Unit B, Unit C).
4. Adjust the sliders, color pickers, and toggles in each panel. Each instance responds instantly.
5. Use the **Reset** button to restore defaults, or **Pause/Play** to freeze/resume animations.
6. No web server, command line, or code editing is required.

---

## 6. Developer Integration & JavaScript API Reference

### Mounting in a Web Application

```html
<!-- 1. Include the lightweight Stage 16B widget runtime -->
<script src="js/svg-widget.js"></script>

<!-- 2. Target container element -->
<div id="host-monitor-v1-container" style="width: 400px; height: 320px;"></div>

<script>
  // 3. Mount instance (offline file:// supported via inline template/markup or HTTP URL)
  const svgSource = document.getElementById('host-monitor-v1-template') ? document.getElementById('host-monitor-v1-template').innerHTML : 'host-monitor-v1.svg';
  const myWidget = SVGWidget.mount('#host-monitor-v1-container', svgSource, {
    instanceId: 'host-monitor-v1-1'
  });
</script>
```

### Controlling Instance Parameters

```javascript
// Single parameter update (using real exported parameter 'hostname-node-text'):
myWidget.setParameter('hostname-node-text', 'host-01');

// Atomic batch update:
myWidget.setParameters({
  hostname-node-text: 'host-01',
  hostname-node-fill: '#1e293b',
  hostname-node-stroke: '#38bdf8',
  traffic-flow-color: '#22c55e',
});

// Read current value:
const val = myWidget.getParameter('hostname-node-text');

// Reset to authored defaults:
myWidget.resetParameters();

// Listen for property changes:
myWidget.on('change', (evt) => {
  console.log(`[${evt.instanceId}] ${evt.name} changed to:`, evt.value);
});
```

### Multi-Instance Independence Example

```javascript
const svgSource = document.getElementById('host-monitor-v1-template') ? document.getElementById('host-monitor-v1-template').innerHTML : 'host-monitor-v1.svg';

const instA = SVGWidget.mount('#unitA', svgSource, { instanceId: 'unit-a' });
const instB = SVGWidget.mount('#unitB', svgSource, { instanceId: 'unit-b' });
const instC = SVGWidget.mount('#unitC', svgSource, { instanceId: 'unit-c' });

instA.setParameter('hostname-node-text', 'Unit 1 Active');
instB.setParameter('hostname-node-text', 'Unit 2 Standby');
instC.setParameter('hostname-node-text', 'Unit 3 Offline');
```

---

*(End of Guide — Generated automatically by SVG Animator Studio)*
