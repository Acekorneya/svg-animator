# SVG Animator Studio — Stage 16A Regression Test Matrix
## Automated Verification Suites & Quality Gates

**Date:** 2026-11-15  
**Document ID:** `TEST-STAGE-16A`  
**Milestone:** Stage 16A — Reusable, Self-Contained and Collision-Free SVG Widgets  
**Protected Release:** `v1.2.0`  

---

## 1. Quality Gate Summary

All primary quality gates pass with zero regressions across the codebase:

```bash
# Gate 1: Go Codebase Vet
go vet ./...
# Result: PASS (0 errors across 11 packages)

# Gate 2: Go Unit Test Suite
go test -count=1 ./...
# Result: PASS (11/11 packages PASS, 0.954s)

# Gate 3: Go Race Detector
go test -race -count=1 ./pkg/engine/scene/...
# Result: PASS (0 data races detected)

# Gate 4: Phase 1 Static Architecture Verification
bash scripts/verify-architecture.sh
# Result: PASS (9 PASS, 1 WARNING [ARCH-05 offscreen DOM exception])

# Gate 5: Phase 1 Runtime Verification
bash scripts/verify-runtime.sh
# Result: PASS (6 PASS, 1 NOT VERIFIED [RUN-06 Chromium absent on LXC])

# Gate 6: Complete Regression Test Harness
node scripts/regression-harness.js
# Result: PASS (76/76 Scenarios PASS, including new Scenario 76)
```

---

## 2. Go Unit Tests (`pkg/engine/scene/widget_test.go` & `protocol/commands_test.go`)

| Test Function | Package | Description | Status |
|---|---|---|---|
| `TestIsolateSVGWidgetBasic` | `pkg/engine/scene` | Verifies root SVG tagging, ID remapping, internal reference updates, data URL preservation, text `#` preservation, CSS scoping, and keyframe prefixing | **PASS** |
| `TestIsolateSVGWidgetMultiInstanceCollisionFree` | `pkg/engine/scene` | Mounts two instances (A and B) of the same SVG; verifies ID intersection is completely disjoint | **PASS** |
| `TestIsolateSVGWidgetValidation` | `pkg/engine/scene` | Verifies empty SVG rejection and automatic unique instance ID generation | **PASS** |
| `TestDispatchWidgetIsolateSVG` | `pkg/engine/protocol` | Verifies `Widget.IsolateSVG` command dispatch, JSON marshaling, and response structure | **PASS** |
| `TestBuildStandaloneWidgetHTML` | `internal/generator` | Verifies Go CLI standalone widget HTML container generation | **PASS** |

---

## 3. Regression Harness Scenario 76 (`scripts/regression-harness.js`)

Scenario 76 was added to the authoritative test harness to enforce end-to-end multi-instance isolation across both Go/WASM and JavaScript runtime layers:

### Test 1: Go Engine Protocol Command `Widget.IsolateSVG`
- **Execution:** Dispatches `Widget.IsolateSVG` via `window.EngineBridge.dispatch` with complex diagram topology payload (defs, filters, gradients, markers, flows, data URLs, text with `#`, CSS camera keyframes).
- **Assertions:**
  - `goRes.success === true`
  - `goRes.instanceId === 'go-inst1'`
  - Root SVG has `id="sva-widget-go-inst1"` and `data-sva-widget-instance="go-inst1"`
  - IDs remapped: `go_inst1__flowPath-1`, `go_inst1__flowArrow-1`, `go_inst1__glowGrad-1`, `go_inst1__viewportClip`, `go_inst1__nodeDropShadow`, `go_inst1__flowBlur-1`
  - References remapped: `mpath href`, `xlink:href`, `marker-end`, `fill`, `clip-path`, `filter`
  - Data URLs untouched: `data:image/png;base64,...` byte-for-byte identical
  - Text node `#` untouched: `Source #Node 1`
  - Keyframes renamed: `@keyframes go_inst1__camKenBurns`
  - Animation properties remapped: `animation: go_inst1__camKenBurns ...`
  - CSS selectors scoped: `#sva-widget-go-inst1 .camera-rig`

### Test 2: Standalone `svg-widget.js` CommonJS & Library Isolation
- **Execution:** Loads `internal/web/static/js/svg-widget.js` via Node.js `require`.
- **Assertions:**
  - `SVGWidgetLib.version === '1.0.0'`
  - Isolates Instance Alpha (`alpha__`) and Instance Beta (`beta__`)
  - Constructs `Set(idsAlpha)` and `Set(idsBeta)`; asserts zero overlap (`idsBeta.has(id) === false` for all `id` in Alpha)
  - Asserts Instance Alpha markup contains zero references to `beta__` and vice versa

### Test 3: Presentation Adapter & Global Delegator
- **Execution:** Tests `window.SVGExportAdapter.isolateWidget` and `window.isolateSVGForWidget`.
- **Assertions:**
  - `typeof window.SVGExportAdapter.isolateWidget === 'function'`
  - `typeof window.isolateSVGForWidget === 'function'`
  - Generates isolated markup for Instance Gamma with root ID and instance attributes

### Test 4: Multi-Instance DOM Mounting Simulation
- **Execution:** Simulates DOM mount of two independent containers using `SVGWidget.mount`.
- **Assertions:**
  - Container 1 and Container 2 each contain 1 mounted widget wrapper
  - Instance IDs match handles
  - Calling `handle1.unmount()` leaves Container 1 empty while Container 2 remains intact

### Test 5: Reusable SVG Widget Export UI Button
- **Execution:** Validates DOM presence of `#btnDownloadWidgetSVG` in `index.html`.
- **Assertions:**
  - `#btnDownloadWidgetSVG` exists and is bound to export handler in `app.js`

---

## 4. Protected Asset Invariance Verification

| Target | Verification Command | Expected State | Actual State |
|---|---|---|---|
| Release Tag `v1.2.0` | `git tag -v v1.2.0` / `git rev-parse v1.2.0` | Annotated tag intact | **INTACT** |
| Protected Binary | `./svg-animator -version` | Untouched root binary | **INTACT** |
| Release Directory | `ls release/v1.2.0` | Production distribution | **INTACT** |
| SQLite Databases | `studio_library.db` | Schema version 1.2.0 | **INTACT** |
| Protected Ports | Ports 8088 & 8090 | Not bound by development | **INTACT** |
| Dev Server Port | Port 8089 | Active development preview | **ACTIVE (HTTP 200)** |
| Multi-Remote Mirrors | `git remote -v` | `origin`, `gitea`, `github` | **CONFIGURED** |
