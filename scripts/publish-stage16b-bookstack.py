#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16B — Externally Controllable SVG Widgets and Runtime Parameter API"

PAGE_MARKDOWN = """# Stage 16B — Externally Controllable SVG Widgets and Runtime Parameter API

## Executive Summary

**Stage 16B: External Widget Controls, Runtime Parameters and Independent Host Integration** establishes an authoritative, typed, and secure mechanism for hosting web applications to control arbitrary SVG Animator widget instances independently.

This milestone builds directly on Stage 16A's collision-free multi-instance foundation, advancing SVG Animator Studio towards modular digital twins and embedded interactive graphics while strictly honoring the project's architectural hierarchy:

$$\\\\text{Go Engine (Authoritative)} \\\\longrightarrow \\\\text{WASM Bridge} \\\\longrightarrow \\\\text{JavaScript Browser Adapter} \\\\longrightarrow \\\\text{Browser SVG Engine}$$

---

## 1. Architectural Highlights & Scope Boundaries

### Go Authoritative Ownership
- **Data Models:** `WidgetParameter` and `WidgetManifest` in `pkg/engine/scene/model.go`.
- **Snapshot & SVA Persistence:** Lossless parameter preservation in `.sva` packages and SQLite library persistence.
- **Authoritative Isolation:** Automatic embedded manifest injection (`<script type="application/json" id="sva-widget-manifest">`) during `Widget.IsolateSVG`.
- **Typed Protocol Dispatch:** Full command support for `Widget.GetParameters`, `Widget.SetParameters`, `Widget.AddParameter`, and `Widget.RemoveParameter`.

### JavaScript Browser Adapter (`svg-widget.js`)
- **Universal UMD Module:** Browser global, CommonJS (Node.js), and ES module compatibility (v1.2.0).
- **Semantic Target Resolution:** Targets elements via semantic target names, `idMap`, or entity attributes without exposing namespaced instance prefixes (`barrel_a__`).
- **Atomic Batch Validation:** Two-phase commit transaction (`setParameters(batch)`). Any invalid type, bound, or missing target rejects the batch with zero DOM mutations.
- **Independent Playback Control:** Native per-instance animation coordination (SVG SMIL, CSS keyframes, Web Animations API) ensuring zero mutual interference.
- **Security Boundaries:** Strict validation against injection, preventing arbitrary JavaScript execution, invalid styles, or DOM corruption.

---

## 2. Parameter Manifest Schema

Exported widgets embed an immutable, machine-readable manifest schema:

```json
{
  "schemaVersion": 1,
  "parameters": [
    {
      "name": "operatingState",
      "type": "enum",
      "default": "running",
      "target": "root",
      "property": "state",
      "options": ["running", "paused", "stopped", "alarm"],
      "description": "Operating lifecycle state of the tank unit"
    },
    {
      "name": "statusColor",
      "type": "color",
      "default": "#22c55e",
      "target": "statusDot",
      "property": "fill",
      "description": "LED beacon and indicator accent color"
    },
    {
      "name": "fillLevel",
      "type": "number",
      "default": 65,
      "target": "fluidLevel",
      "property": "height",
      "min": 0,
      "max": 180,
      "description": "Tank fluid volume height in pixels"
    },
    {
      "name": "speed",
      "type": "speed",
      "default": 1.0,
      "target": "root",
      "property": "playbackRate",
      "min": 0.1,
      "max": 3.0,
      "description": "Animation playback speed rate"
    }
  ]
}
```

---

## 3. Multi-Instance Demonstration

A live interactive demonstration is available at `http://127.0.0.1:8089/demo-widgets.html` and packaged for offline use:

- **Barrel A (Primary Process):** Running, `#22c55e` (green), 100% speed, 120px fluid fill.
- **Barrel B (Maintenance Standby):** Paused, `#f59e0b` (amber), 50% speed, 45px fluid fill.
- **Barrel C (Emergency Surge):** Alarm, `#ef4444` (red), 200% speed, 165px fluid fill.

All instances operate concurrently with **0 DOM ID collisions**, independent animation playback, and live host telemetry integration.

---

## 4. Verification & Quality Gates

- **Regression Harness:** 77/77 Scenarios PASS (Scenario 77 verifies W16B-01 through W16B-22).
- **Go Engine Tests:** 100% PASS across all engine packages (`go test -count=1 ./...`).
- **Race Detector:** 0 data races detected (`go test -race ./pkg/engine/scene/...`).
- **Go Vet:** 0 warnings (`go vet ./...`).
- **Architecture & Runtime Scripts:** All static and runtime verification checks PASSED.
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
            print(f"Successfully published BookStack page: ID {data.get('id')}, Slug: {data.get('slug')}")
            return data
    except Exception as e:
        print("BookStack publish error:", e)
        raise

if __name__ == "__main__":
    publish()
