#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16B Milestone — Property Eligibility & Unique Binding Contract"

PAGE_MARKDOWN = """# Stage 16B Milestone — Property Eligibility & Unique Binding Contract

## Executive Summary
This milestone finalizes the property authoring contract for Stage 16B before entering Digital Twin integration. It delivers an authoritative **Object Property Eligibility Catalog**, enforces the **Unique Binding Contract** across Go engine and UI adapters, provides non-destructive legacy project migration with conflict resolution, and implements the **Global vs Local Playback Speed Coexistence** formula.

---

## 1. Authoritative Object Property Eligibility Catalog

Property exposure is strictly governed by the Go engine (`pkg/engine/scene/widget.go:ObjectPropertyCatalog`). Unsupported attributes (such as `speed`, `height`, or `width` on nodes) are rejected at the Go engine boundary:

| Target Type | Supported Properties | Type | Default Value | Notes / Description |
| :--- | :--- | :--- | :--- | :--- |
| **`node`** | `fill` | `color` | Current shape fill | Background fill colour |
| | `stroke` | `color` | Current shape stroke | Border outline colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Overall node opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| | `text` | `text` | Current node text label | Primary text label |
| **`flow`** | `stroke` | `color` | Current path colour | Connector path and particle colour |
| | `speed` | `speed` | `1.0` (range: `0.1`–`10.0`) | Flow animation rate multiplier |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Flow track opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`glow`** | `color` | `color` | Current glow colour | Radial glow accent colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Glow intensity opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`ripple`** | `color` | `color` | Current wave colour | Ripple wave ring colour |
| | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Wave pulse opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`image`** | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Image layer opacity |
| | `visibility` | `boolean` | `true` | Visibility toggle |
| **`group`** | `opacity` | `number` | `1.0` (range: `0.0`–`1.0`) | Composite group opacity |
| | `visibility` | `boolean` | `true` | Composite visibility toggle |
| **`root`** | `playbackRate` | `speed` | `1.0` (range: `0.1`–`5.0`) | Master widget playback speed |
| | `visibility` | `boolean` | `true` | Master widget container visibility |
| | `operatingState` | `enum` | `"running"` | High-level lifecycle state |

---

## 2. Unique Binding Contract

To prevent ambiguous states and duplicate UI controls, SVG Animator Studio enforces:

$$\\text{One Object ID} + \\text{One Controllable Property} + \\text{One Scope} \\le 1\\text{ External Parameter Binding}$$

- **Collision Rejection:** Adding a parameter for an already-bound property (e.g. adding `flow1c` when `flow-1.stroke` is bound to `flow1color`) fails immediately with `property "stroke" on target "flow-1" is already bound to parameter "flow1color"`.
- **Multi-Property Exposure:** A single object can expose multiple *different* eligible properties simultaneously (e.g. `flow-1` exposes `stroke` and `speed`).
- **Explicit Renaming (`RenameParameter`):** Renaming preserves target, property, limits, and runtime identity.

---

## 3. Global vs Local Playback Speed Coexistence

Global widget playback rate and local flow animation speed independently coexist according to the authoritative formula:

$$\\text{Effective Speed} = \\text{Master Playback Rate} \\times \\text{Targeted Flow Speed}$$

Both Web Animations API (`playbackRate`) and SMIL declarative animations (`dur = baseDur / effectiveSpeed`) scale proportionally without drift or crosstalk.

---

## 4. Migration & Conflict Resolution

- **Non-Destructive Ingestion:** Existing projects with duplicate bindings load safely without silent deletion.
- **In-Editor Conflict UI:** The Widget Properties drawer displays a warning banner with one-click resolution cards: `Keep "[name]"`.
- **Export Safety Blocking:** `BuildWidgetPackageZip` explicitly halts export if unresolved duplicate bindings exist, preventing broken packages.

---

## 5. Acceptance Verification Summary

### Real-Device Kiwiberry Chrome CDP Acceptance (`192.168.100.40:8088`):
- `[PASS] StudioLoad`: SVG Animator Studio loaded successfully
- `[PASS] WASMReady`: Go WebAssembly engine mounted and ready
- `[PASS] NodeEligibility`: Node allows fill/stroke/opacity/visibility/text and rejects speed/height
- `[PASS] FlowEligibility`: Flow allows speed and stroke
- `[PASS] RootEligibility`: Root allows playbackRate and operatingState
- `[PASS] RejectIneligible`: Rejected speed on node: `property "speed" is not eligible for target "node-1" of type "node"`
- `[PASS] RejectBadRange`: Rejected out-of-range default: `speed default value (5) cannot be greater than maximum (2)`
- `[PASS] InitialBinding`: `flow-1.stroke` bound to `flow1Stroke`
- `[PASS] RejectDuplicate`: Duplicate rejected: `property "stroke" on target "flow-1" is already bound to parameter "flow1Stroke"`
- `[PASS] MultiPropertyExposure`: Different property (`speed`) on `flow-1` added successfully
- `[PASS] RenameSuccess`: Renamed `flow1Speed` -> `flow1Rate` preserving target and property
- `[PASS] DetectConflicts`: Detected 1 duplicate binding conflict on `flow-1.stroke`
- `[PASS] BlockExportOnConflict`: `BuildWidgetPackage` blocked when unresolved conflicts exist
- `[PASS] ResolveConflict`: `resolveWidgetConflict` kept `flow1color`, 0 conflicts remaining
- `[PASS] PackageExport`: Exported `mail-service-package.zip` (18,713 bytes)
- `[PASS] MultiInstanceMount`: All 3 instances (`unit-a`, `unit-b`, `unit-c`) mounted offline via `file://`
- `[PASS] SpeedCoexistence`: Master `playbackRate` (1.5) and local flow `speed` (2.0) independently set and coexist

### Automated Test Suite:
- **Go Unit Tests:** 100% pass across all engine packages (`pkg/engine/...`, `internal/...`).
- **Regression Harness:** 100% pass across all 84 test scenarios (`node scripts/regression-harness.js`).
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
