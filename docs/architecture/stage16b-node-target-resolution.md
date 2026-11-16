# Stage 16B: Node Target Resolution, Standardised Naming & Pre-Export Validation

## 1. Context and Problem Definition

During user acceptance testing on Kiwiberry (`magnifyingmap-package(1).zip`), widget packages containing Node property bindings failed on mount with runtime exceptions:

```text
[SVGWidget] Failed applying initial parameters on mount:
Error: Target element 'node-1' not found in widget for parameter 'node-mytext-stroke'.
Error: Target element 'node-1' not found in widget for parameter 'node-mytext-text'.
Error: Target element 'node-1' not found in widget for parameter 'node-mytext-fill'.
```

### Root Cause Analysis

1. **Missing Node Semantic Attributes:**
   In `internal/web/static/js/nodes.js`, `renderNodeElement` created `<g>` container elements without `id="node-${id}"`, `data-semantic-target="node-${id}"`, or `data-semantic-name="${escapeXML(name)}"`. While Flow, Glow, and Ripple objects emitted explicit semantic data attributes, Nodes relied on ad-hoc shape rendering, causing `resolveTargetElement` in `svg-widget.js` to fail to locate `node-1`.

2. **Inconsistent Parameter Naming Contract:**
   Widget parameters were named using type-first tokens (`node-mytext-fill`, `flow-border-speed`), whereas the project specification requires object-name-first tokens: `<object-name>-<object-type>-<property>` (e.g. `mytext-node-fill`, `border-flow-speed`, `alarm-glow-color`, `warning-ripple-opacity`).

3. **Absence of Pre-Export Validation:**
   Widget packages were compiled even when parameter targets did not exist in the scene or could not be resolved in the exported SVG. This deferred failures to the host runtime rather than catching invalid configurations at authoring time.

---

## 2. Architectural Design & Implementation

Following **Section 4 of `AGENTS.md`**, Go serves as the authoritative engine and owner of scene models and export validation; JavaScript serves as the browser and DOM presentation adapter.

### A. Authoritative Node Semantic Markup (`internal/web/static/js/nodes.js`)

`renderNodeElement` was updated to guarantee complete semantic tagging on every node `<g>` element:

```javascript
`<g id="node-${n.id}" class="diagram-node-instance" data-node-id="${n.id}" data-semantic-target="node-${n.id}" data-semantic-name="${escapeXML(n.name)}" ...>`
```

Similarly, image objects in `internal/web/static/js/engine-bridge.js` were updated with `data-semantic-target="image-${img.id}"` and `data-semantic-name`.

### B. Standardised Object-Name-First Parameter Naming

Implemented in Go engine (`pkg/engine/scene/widget.go`) and mirrored in JS bridge (`internal/web/static/js/engine-bridge.js`):

Formula: `<object-name>-<object-type>-<property>`

- Clean object name stripping redundant type prefixes:
  - `mytext` (type `node`, prop `text`) → `mytext-node-text`
  - `mytext` (type `node`, prop `fill`) → `mytext-node-fill`
  - `border` (type `flow`, prop `speed`) → `border-flow-speed`
  - `flow-border` (type `flow`, prop `color`) → `border-flow-color` (strips duplicate `flow-` prefix)
  - `alarm` (type `glow`, prop `color`) → `alarm-glow-color`
  - `warning` (type `ripple`, prop `opacity`) → `warning-ripple-opacity`
- Default unrenamed objects:
  - `node-1` (type `node`, prop `text`) → `node-1-text` (no redundant `node-1-node-text`)
  - `flow-2` (type `flow`, prop `speed`) → `flow-2-speed`

### C. Authoritative Pre-Export Validation (`pkg/engine/scene/widget.go`)

`ValidateWidgetExport(manifest, isolatedSVG, sc)` validates:
1. **Schema & Types:** Strict parameter type validation (`color`, `number`, `boolean`, `text`, `enum`, `speed`).
2. **Name Uniqueness:** No duplicate parameter names permitted.
3. **Scene Object Existence:** If the scene contains objects, verifies the target entity exists in `sc.GetNode()`, `sc.GetFlow()`, `sc.GetGlow()`, `sc.GetRipple()`, or `sc.GetImage()`.
4. **SVG Target Resolution:** Verifies that target ID resolves in `isolatedSVG` across `data-semantic-target`, `data-semantic-name`, `id`, `data-*-id`, or class selectors.

If validation fails, package generation is aborted immediately with an explanatory error, preventing broken ZIP exports.

### D. Safe Property Application in Widget Runtime (`internal/web/static/js/svg-widget.js`)

Updated `applyPropertyToElement`:
- **Text:** Safely updates `<text>` label inside node while preserving shape styling.
- **Fill:** Recolours non-transparent, non-none shapes without altering hit-test elements or borders.
- **Stroke:** Recolours outline strokes without affecting transparent bounds.
- **Resolution:** Robust 7-stage fallback resolution across `data-semantic-target`, `data-semantic-name`, `idMap`, `data-*-id`, stripped prefixes, and CSS selectors.

---

## 3. Verification & Acceptance Results

### A. Automated Go Tests & Regression Suite
- `go test ./...` passed 100% across all packages.
- `node scripts/regression-harness.js` passed all 86 scenarios with 0 errors, including new Scenario 86 testing Node target resolution, parameter naming, and DOM mutation.

### B. Kiwiberry Chrome CDP Live Acceptance (`scripts/cmd/verify-node-target-resolution/main.go`)

Executed directly against Kiwiberry (`192.168.100.40:8088`) with headless Chrome over CDP:

```text
==================================================================
 Stage 16B: Node Target Resolution & Standardised Naming Verification
 Target: Kiwiberry (192.168.100.40:8088) with Chrome CDP
==================================================================

[PHASE 1] Navigating to SVG Animator Studio on Kiwiberry...
 > Go WebAssembly engine mounted and ready.

[PHASE 2] Verifying Object-Name-First Parameter Naming Generation...
[PASS] Criteria  1: Standardised Parameter Naming      - Generated names follow <object-name>-<object-type>-<property> contract

[PHASE 3] Testing Pre-Export Target Validation...
[PASS] Criteria  2: Pre-Export Target Validation       - Export rejected with descriptive error when target object is missing from scene or SVG

[PHASE 4] Building Multi-Node Production Scene & Exporting Package...
 > Exported widget package ZIP (21905 bytes).

[PHASE 5] Inspecting Exported ZIP Manifest & Documentation...
[PASS] Criteria  3: Standalone 6-File ZIP Package      - Export contains all 6 required files including widget-manifest.json, demo.html and js/svg-widget.js
[PASS] Criteria  4: Manifest Naming Compliance         - Manifest contains 7 parameters strictly formatted as <object-name>-<object-type>-<property>

[PHASE 6] Unpacking Package on Kiwiberry for file:// Browser Verification...
 > Unpacked package to Kiwiberry filesystem.

[PHASE 7] Opening file:///home/nigel/acceptance-demo/node-target-resolution/node-target-widget/demo.html in Headless Chrome via file://...
[PASS] Criteria  5: Zero Mount Target Errors           - No 'Target element not found' errors on widget mount (captured errors: [])
[PASS] Criteria  6: Multi-Instance Isolation           - Zero ID collisions between instance unit-a (11 IDs) and unit-b (11 IDs)
[PASS] Criteria  7: Node Text Control & Isolation      - Unit-A node-1 updated to 'Production Gateway', Unit-B remained 'Primary Server', Node-2 remained 'Backup Client'
[PASS] Criteria  8: Node Fill & Stroke Isolation       - Unit-A node fill=#b91c1c/stroke=#fca5a5, Unit-B fill=#1e293b/stroke=#3b82f6, Node-2 fill=#0f172a/stroke=#10b981
[PASS] Criteria  9: Optical & Flow Parameter Coexistence - Flow speed=2.5, glow color=#10b981 (stop=#10b981), ripple opacity=0.4 verified

>>> ALL 9 STAGE 16B NODE TARGET RESOLUTION CRITERIA PASS 100% ON KIWIBERRY <<<
```

In addition, the comprehensive `verify-final-contract` suite passed all 10 criteria on Kiwiberry.
