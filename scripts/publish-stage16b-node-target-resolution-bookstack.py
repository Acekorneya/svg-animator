#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16B: Node Target Resolution, Standardised Naming & Pre-Export Validation"

PAGE_MARKDOWN = """# Stage 16B: Node Target Resolution, Standardised Naming & Pre-Export Validation

## Executive Summary
This update stabilizes widget export and multi-instance runtime execution for Node objects following real-world acceptance testing of `magnifyingmap-package(1).zip`. 

Key deliverables:
1. **Node Target Resolution:** Explicit `id="node-${id}"`, `data-semantic-target="node-${id}"`, and `data-semantic-name="${name}"` markup emission in `nodes.js`.
2. **Standardised Object-Name-First Parameter Naming:** Standardized `<object-name>-<object-type>-<property>` pattern (`mytext-node-text`, `mytext-node-fill`, `border-flow-speed`, `alarm-glow-color`, `warning-ripple-opacity`).
3. **Pre-Export Target Validation:** Immediate build-time rejection in Go engine (`ValidateWidgetExport`) if parameter targets cannot resolve in scene or SVG markup.
4. **Automated Live CDP Verification:** 9 of 9 criteria passing 100% on Kiwiberry (`192.168.100.40:8088`) with zero console errors on mount.

---

## 1. Problem Resolution & Root Cause

### A. Missing Node Semantic Markup
Previously, `renderNodeElement` in `nodes.js` rendered SVG groups without `data-semantic-target` or `id` attributes. When widgets were mounted, `resolveTargetElement` in `svg-widget.js` could not find `node-1`, causing:
```text
[SVGWidget] Failed applying initial parameters on mount:
Error: Target element 'node-1' not found in widget for parameter 'node-mytext-stroke'.
```
**Fix:** Guaranteed `id="node-${n.id}" class="diagram-node-instance" data-node-id="${n.id}" data-semantic-target="node-${n.id}" data-semantic-name="${escapeXML(n.name)}"` on every rendered node `<g>`.

### B. Standardised Object-Name-First Parameter Naming
The naming formula was updated in both Go engine (`pkg/engine/scene/widget.go`) and JS bridge (`internal/web/static/js/engine-bridge.js`) to:
$$\\text{Parameter Name} = \\langle\\text{clean-object-name}\\rangle\\text{-}\\langle\\text{object-type}\\rangle\\text{-}\\langle\\text{property}\\rangle$$
- `mytext` (type `node`, prop `text`) → `mytext-node-text`
- `mytext` (type `node`, prop `fill`) → `mytext-node-fill`
- `border` (type `flow`, prop `speed`) → `border-flow-speed`
- `flow-border` (type `flow`, prop `color`) → `border-flow-color` (strips duplicate type prefix)
- `node-1` (type `node`, prop `text`) → `node-1-text` (default unrenamed pattern)

### C. Pre-Export Target Validation Gatekeeper
`ValidateWidgetExport(manifest, isolatedSVG, sc)` in Go engine now enforces:
- Parameter name structure & uniqueness.
- Entity existence in scene models.
- Target element resolution in exported SVG before ZIP generation.
- Rejection of orphan/unresolvable bindings with descriptive errors.

---

## 2. Acceptance Verification on Kiwiberry (`192.168.100.40`)

Executed via automated headless Chrome over CDP:

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
"""

def publish_to_bookstack():
    headers = {
        "Authorization": f"Token {TOKEN_ID}:{TOKEN_SECRET}",
        "Content-Type": "application/json"
    }

    req_pages = urllib.request.Request(f"{BOOKSTACK_URL}/api/books/{BOOK_ID}", headers=headers)
    try:
        with urllib.request.urlopen(req_pages) as resp:
            book_data = json.loads(resp.read().decode())
    except Exception as e:
        print(f"Error fetching book: {e}")
        return False

    existing_page_id = None
    for content in book_data.get("contents", []):
        if content.get("type") == "page" and content.get("name") == PAGE_TITLE:
            existing_page_id = content.get("id")
            break

    payload = {
        "name": PAGE_TITLE,
        "markdown": PAGE_MARKDOWN
    }

    if existing_page_id:
        print(f"Updating existing page ID {existing_page_id}...")
        url = f"{BOOKSTACK_URL}/api/pages/{existing_page_id}"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="PUT")
    else:
        print(f"Creating new page under book ID {BOOK_ID}...")
        payload["book_id"] = BOOK_ID
        url = f"{BOOKSTACK_URL}/api/pages"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="POST")

    try:
        with urllib.request.urlopen(req) as resp:
            res_data = json.loads(resp.read().decode())
            print(f"Successfully published: ID={res_data.get('id')}, Slug={res_data.get('slug')}")
            print(f"URL: {BOOKSTACK_URL}/books/{book_data.get('slug')}/page/{res_data.get('slug')}")
            return True
    except Exception as e:
        print(f"Error publishing page: {e}")
        return False

if __name__ == "__main__":
    publish_to_bookstack()
