#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16B Completion — Final Property Contract, Auto-Derived Naming & Effect Targeting Isolation"

PAGE_MARKDOWN = """# Stage 16B Completion — Final Property Contract, Auto-Derived Naming & Effect Targeting Isolation

## Executive Summary
This milestone completes Stage 16B property authoring, export packaging, and runtime presentation for SVG Animator Studio. Based on real-world widget inspection (`gensaves-package.zip`), this release establishes authoritative **Stable Internal Identity vs. Unique User Object Names**, automatic **Derived External Parameter Naming**, strict **Effect Targeting Isolation in Runtime (`svg-widget.js`)**, and a simplified **Compact Property Cards UI**.

All 10 acceptance criteria have been verified 100% passing in Chrome over CDP on the Kiwiberry acceptance host (`192.168.100.40:8088`).

---

## 1. Architectural Contract & Object Identity

### A. Stable Internal Object Identity
- Every object in the scene maintains an immutable Go-owned ID (`StringID`) used for scene graph topology, connection routing, selection state, undo/redo history, and persistence.
- Internal object IDs are never exposed directly to external systems as arbitrary configuration keys.

### B. Unique User-Editable Object Names with Fixed Prefixes
- Objects enforce unique names within their category (case-insensitive) using fixed type prefixes:
  - `node-` (e.g. `node-gateway`, `node-database`)
  - `flow-` (e.g. `flow-border`, `flow-telemetry`)
  - `glow-` (e.g. `glow-status`, `glow-alarm`)
  - `ripple-` (e.g. `ripple-sensor`, `ripple-pulse`)
  - `image-` (e.g. `image-backdrop`, `image-overlay`)
- Uniqueness is validated by the authoritative Go engine via `ValidateObjectName(type, name, excludeID)` across object creation, duplication, import, and inline renaming. Empty strings `""` are permitted to clear explicit names and revert to default ID references.

### C. Automatic External Parameter Name Generation
- External parameter names are derived deterministically:
  $$\\text{Parameter Name} = \\langle\\text{objectName}\\rangle\\text{-}\\langle\\text{propertySuffix}\\rangle$$
  Examples: `node-gateway-fill`, `flow-border-color`, `flow-border-speed`, `glow-status-color`, `ripple-sensor-color`.
- Users never have to manually invent arbitrary parameter identifiers or pick variable types; types, limits, and defaults are derived authoritatively from the Go scene model.
- Renaming an object automatically updates all bound public parameter names across the scene and synchronizes the UI, exported manifest, and documentation.

---

## 2. Effect Targeting Isolation in Runtime (`svg-widget.js`)

To guarantee that external parameter adjustments never degrade or alter the authored SVG artwork:

1. **Flow Targeting:**
   - Recolouring `color` or `stroke` changes only the moving payload (`.flow-payload-group` particles, circles, or dashes).
   - Motion guide track paths (`.flow-track-path` authored with `stroke="none"`) are strictly preserved as `stroke="none"` and never painted.
   - Individual flow speed controls scale connector animation duration without affecting master transport.
2. **Glow Targeting:**
   - Recolouring `color` targets radial gradient stops (`<radialGradient>`) and illuminated core geometry.
   - Transparent hit-test rectangles (`.glow-hit-target` with `fill="transparent"`) remain untouched and strictly transparent.
3. **Ripple Targeting:**
   - Recolouring `color` and `opacity` targets visible wave crests (`.ripple-wave`).
   - Transparent hit-test rectangles (`.ripple-hit-target`) and structural shadow rings (`.ripple-shadow-ring` with `stroke="#000000"`) are strictly preserved.
4. **Node & Image Targeting:**
   - Targets only attributes that can be faithfully altered at runtime (`fill`, `stroke`, `opacity`, `visibility`, `text`).

---

## 3. Simplified Compact Cards Authoring UI

When an object is selected in SVG Animator Studio, the Widget drawer presents a compact card for each eligible property:
- **Checkbox:** `[ ] Expose` toggles the parameter binding immediately.
- **Badge:** Displays the auto-derived public parameter name (e.g. `node-gateway-fill`).
- **Default Value Editor:** Allows fine-tuning the initial authored default.
- **Duplicate Prevention:** If a property is already bound, the card offers an edit input and `✕ Disable` button, ensuring zero duplicate bindings.

---

## 4. Real-Device Acceptance Verification Results

Executed on Kiwiberry (`192.168.100.40:8088`) in real Headless Chrome over CDP:

```
==================================================================
 STAGE 16B FINAL ACCEPTANCE VERIFICATION SUMMARY
==================================================================
[PASS] Criteria 10: Undo/Redo Verification             : Parameter addition correctly recorded and restored across undo/redo stack
[PASS] Criteria  9: Studio UI Zero Console Errors      : Studio produced 0 console errors during authoring ()
[PASS] Criteria  8: Manifest & Docs Accuracy           : Manifest contains 7 valid parameters with strict types and zero invalid limits
[PASS] Criteria  1: Direct file:// Operation           : All 3 instances rendered directly from filesystem without a web server
[PASS] Criteria  2: Multi-Instance Isolation           : Zero ID collisions between instance unit-a (10 IDs) and unit-b (10 IDs), overlap: []
[PASS] Criteria  3: Node Recolour & Text Control       : Node fill changed to #10b981 and text updated to 'Core Switch'
[PASS] Criteria  4: Flow Payload vs Guide Isolation    : Payload recoloured to #f43f5e while .flow-track-path motion guide remains stroke=none
[PASS] Criteria  5: Flow Speed vs Master Coexistence   : Flow speed (2.0) changed independently while preserving master playbackRate (1.5)
[PASS] Criteria  6: Glow Hit-Target Preservation       : Glow hit-target transparent=true, stops=[#fbbf24 #fbbf24], updated=true
[PASS] Criteria  7: Ripple Wave vs Hit/Shadow Isolation : Wave crest recoloured to #ec4899; hit-target remains transparent; shadow ring remains #000000

>>> ALL 10 STAGE 16B ACCEPTANCE CRITERIA PASS 100% ON KIWIBERRY <<<
```

---

## 5. Automated Test Suite Status

- **Go Unit Tests:** 100% pass across all engine packages (`pkg/engine/...`, `internal/...`).
- **Regression Harness:** 100% pass across all 85 scenarios (`node scripts/regression-harness.js`).
- **Status:** Stage 16B is 100% complete, verified, and ready for Stage 16C (Digital Twin Presenter & Telemetry Bindings).
"""

def publish():
    url = f"{BOOKSTACK_URL}/api/pages"
    headers = {
        "Authorization": f"Token {TOKEN_ID}:{TOKEN_SECRET}",
        "Content-Type": "application/json"
    }
    payload = {
        "book_id": BOOK_ID,
        "name": PAGE_TITLE,
        "markdown": PAGE_MARKDOWN
    }
    req = urllib.request.Request(url, data=json.dumps(payload).encode("utf-8"), headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req) as resp:
            data = json.loads(resp.read().decode())
            print(f"Successfully created BookStack page: ID {data.get('id')}, Slug: {data.get('slug')}")
            return data
    except Exception as e:
        print("BookStack publish error:", e)
        raise

if __name__ == "__main__":
    publish()
