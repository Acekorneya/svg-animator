# Stage 16B UI Regression Fix — Export Controls Layout & Drawer Containment

**Date:** 2026-11-15  
**Component:** Frontend Workspace Layout (`internal/web/static/index.html`)  
**Acceptance Host:** Kiwiberry (`192.168.100.40`) running Google Chrome 155  
**Service Endpoint:** `http://192.168.100.40:8088`  
**Status:** Resolved & Verified  

---

## 1. Problem Description

Following recent Stage 16B widget parameter visual authoring additions, the **Export / Production Render & Downloads** accordion (`#acc-export`) broke out of its containing drawer (`#rightDrawer` / `#settingsPanel`) and expanded across the entire display width.

This layout displacement caused the export controls to render horizontally across the canvas and workspace body, obstructing interaction with other panels and obscuring the central diagram canvas.

---

## 2. Root Cause Investigation

Direct browser DOM inspection via the Chrome DevTools Protocol (CDP) on Kiwiberry revealed:

```json
{
  "Export Parent": "<BODY id=\"\" class=\"\">",
  "Is Inside #settingsPanel": false,
  "Is Inside #rightDrawer": false,
  "Export Rect": { "x": 0, "y": 101.65, "width": 780, "height": 36.18 },
  "Drawer Rect": { "x": 712.28, "y": 56, "width": 19.73, "height": 45.65 }
}
```

### Analysis
1. In `internal/web/static/index.html`, during the reorganization of `#acc-widget-params`, two surplus closing tags (`</div></div>`) were introduced at lines 2563–2564:
   ```html
               <div id="widgetParamsList" class="widget-params-list" ...>...</div>
             </div>
           </div>
         </div>
           </div>  <!-- Line 2563: UNINTENTIONAL - prematurely closed #settingsPanel -->
         </div>    <!-- Line 2564: UNINTENTIONAL - prematurely closed #rightDrawer -->

         <!-- 5. ACCORDION: EXPORT & RENDER -->
         <div class="accordion-section open" id="acc-export">
   ```
2. Because of these two premature closing tags:
   - `<div class="panel right-panel" id="settingsPanel">` was closed before `#acc-export`.
   - `<aside class="drawer right-drawer" id="rightDrawer">` was closed before `#acc-export`.
   - The main workspace layout container was closed before `#acc-export`.
3. Consequently, `#acc-export` was interpreted by browser HTML parsers as an immediate child of `<body>`.
4. As a root child of `<body>`, `#acc-export` inherited full body width (`width: 100%` / flex row expansion), escaping the right-hand panel column and spanning horizontally across the canvas.
5. In addition, orphan closing tags (`</div></aside>`) existed at lines 2584–2585 after `#acc-export`.

---

## 3. Solution

1. Removed the redundant duplicate closing tags at lines 2563–2564 in [internal/web/static/index.html](file:///root/projects/svg-animator/internal/web/static/index.html).
2. Verified exact HTML tag balance from `<html>` to `</html>` using a complete HTML syntax parser:
   - All 3,191 lines of `index.html` now feature 100% matched start/end tags.
   - `#acc-export` is restored as a direct child of `#settingsPanel` inside `#rightDrawer`.
3. Recompiled Go engine and WebAssembly binary (`engine.wasm`) and Linux server executable (`bin/svg-animator-linux`).
4. Re-deployed the updated binary to Kiwiberry (`/home/nigel/svg-animator`) and restarted the service on `:8088`.
5. Added automated regression test coverage in `scripts/regression-harness.js` (**Scenario 82**) asserting:
   - `#rightDrawer.contains(#acc-export) === true`
   - `#settingsPanel.contains(#acc-export) === true`
   - `#acc-export.parentElement === #settingsPanel`
   - All 7 export buttons remain strictly contained within `#acc-export`.

---

## 4. Real-Browser Layout Verification Results

Automated layout measurements and visual inspections were performed via CDP against Google Chrome 155 on Kiwiberry across all required viewports:

| Viewport | Purpose | Drawer Width | Export Width | Canvas Right Edge | Export Left Edge | Overlap Status | Result |
|---|---|---|---|---|---|---|---|
| **1920 × 1080** | Desktop | 340.0 px | 311.8 px | 0.0 px | 1546.6 px | None (Separated) | **PASS** |
| **1366 × 768** | Laptop | 310.0 px | 281.8 px | 0.0 px | 1022.6 px | None (Separated) | **PASS** |
| **1024 × 768** | Narrow / Tablet | 310.0 px | 281.8 px | 0.0 px | 680.6 px | None (Separated) | **PASS** |

### Button Containment & Dimensions
All 7 export action buttons fit neatly inside the panel without horizontal overflow or label clipping:
1. `btnDownloadSVG`: "Download Vector SVG (Locked Ratio)" — width 308.6 px (1920x1080) / 278.6 px (1366x768)
2. `btnDownloadHTML`: "Download Interactive HTML" — width 308.6 px / 278.6 px
3. `btnDownloadWidgetSVG`: "📦 Export Reusable SVG Widget" — width 308.6 px / 278.6 px
4. `btnDownloadWidgetGuide`: "📋 Download Widget Properties Guide" — width 308.6 px / 278.6 px
5. `btnDownloadWidgetPackage`: "🎁 Export Widget Package (ZIP)" — width 308.6 px / 278.6 px
6. `btnOpenStillModal`: "🖼️ Export Still / Print Image (PNG/WebP/JPG)" — width 308.6 px / 278.6 px
7. `btnOpenBitmapModal`: "🎬 Export Video / Animated (MP4/WebP/APNG)" — width 308.6 px / 278.6 px

### Panel Transitions & Usability
- Clicking `#railBtn-widget-params` cleanly focuses the Widget Parameters accordion.
- Clicking `#railBtn-export` cleanly focuses the Export & Render accordion.
- The diagram canvas remains fully visible, responsive, and clickable.
- Zero JavaScript console errors or warnings were logged.

---

## 5. Verification Suite Status

- **Go Unit Tests:** `go test -count=1 ./...` — **PASS (100%)**
- **JS Regression Harness:** `node scripts/regression-harness.js` — **PASS (82 Scenarios)**
- **Architecture Integrity:** `bash scripts/verify-architecture.sh` — **PASS**
- **Multi-Viewport Browser CDP:** `go run scripts/cmd/verify-export-layout/main.go` — **PASS**
- **Birdscroll End-to-End Package:** `go run scripts/cmd/verify-birdscroll/main.go` — **PASS**

---

## 6. Deployment Details

- **Host:** Kiwiberry (`192.168.100.40`)
- **Binary Path:** `/home/nigel/svg-animator` (Linux x86_64, ~27 MB)
- **Service Command:** `/home/nigel/svg-animator -serve :8088`
- **Application URL:** `http://192.168.100.40:8088/`
