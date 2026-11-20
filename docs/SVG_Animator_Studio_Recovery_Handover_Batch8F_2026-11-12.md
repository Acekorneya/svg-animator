# SVG Animator Studio — Recovery & Continuation Handover

**Checkpoint:** End of Batch 8F  
**Prepared:** 12 November 2026  
**Purpose:** Preserve the current state of the SVG Animator Studio project so its owner, ChatGPT, Gemini in Antigravity, or Claude can resume after model-usage quotas reset without repeating the architectural work or confusing the next priorities.  
**Priority:** Finish *real-browser acceptance of Batch 8F* before authorising further implementation. Stage 16/widgets are a later topic, not the focus of this handover.

> **Evidence status:** This document consolidates project delivery reports and the user's direct browser observations in the conversation. The assistant has **not independently checked the live repository, Git remotes, browser or BookStack**. Claims described as “reported” are based on the implementers' handovers. The most authoritative source for exact current implementation details remains the checked-out repository and the committed evidence files listed below.

---

## 1. Immediate recovery summary — read this first

- **Project:** SVG Animator Studio, Go/WASM-backed SVG scene editor and animation/export application.
- **Repository:** `/root/projects/svg-animator`; branch `main`.
- **Last reported implementation commit:** **`54c4488b02131aa3b246a47a1ec63ca492e76f9d`** (`54c4488`) — `fix(ui): restore snip extract action and crop overlay interaction`.
- **Last reported documentation milestone:** Batch **8F**, BookStack **Book 353 / Page 451**.
- **Development URL:** `http://127.0.0.1:8089/`; most recent handover reports HTTP 200 and PID `1113063`. Do not assume the same PID will remain valid after a restart.
- **Production safeguards:** Preserve `v1.2.0`, `release/v1.2.0`, root `./svg-animator` binary, databases, guides, deployment scripts and ports `8088`/`8090`. **No production deployment or SSH** to kiwiberry/elderberry without new authorisation.
- **Completed work as reported:** Batches 8A–8F, following remediation Batches 0–7. Batch 8D modularised `app.js`; Batch 8E corrected misleading evidence and duplicate image operations; Batch 8F repaired Snip/Crop interaction paths.
- **Last regression count:** **75/75 mocked JavaScript scenarios reported passing**. Go vet/tests/race tests reported passing. Architecture **9 PASS + 1 WARNING (ARCH-05)**; runtime **6 PASS + 1 NOT VERIFIED (RUN-06)**.
- **Critical open check:** The user originally experienced an inert floating **Extract Image** button and a **Crop** bounding box that could not be manipulated. Batch 8F reports code fixes, **but real interactive browser acceptance has not yet been reported**. Node DOM simulations and HTTP 200 are not substitutes.
- **Model quotas at handover:** Gemini in Antigravity reported **3.72% remaining**, with about **35 hours until reset**. Claude was on a free rolling-window/monthly allowance. The user may pause; **do not assume a task must be started to use remaining quota**.

### First action after resumption

1. Verify checkout on `main` at or descended from `54c4488`, with a clean or understood working tree.
2. Open development preview in a real browser, hard-refresh if needed, check the console and Go/WASM ready state.
3. Manually test both Snip/Extract entry points and Crop drag/move/resize/apply (details in §7).
4. Record any remaining issue with exact click/drag steps, selected tool, expected vs actual outcome and screenshot/console errors if available.
5. **Only then** decide whether a narrowly scoped follow-up fix is needed or whether to move on.

---

## 2. Non-negotiable architectural intent

The original and still-important design principle:

```text
Go authoritative scene engine
       ↓ typed commands and validated replies
Go/WASM bridge
       ↓ snapshots and results
JavaScript browser adapter / UI
       ↓
Browser DOM, SVG, Canvas and export rendering
```

**Go/WASM owns durable state and business rules:** scene objects and identities; persistent geometry, crop/clip metadata and asset references; background; Groups and Layers; authoritative selection; history/Undo/Redo; Canvas/Camera/Optics/Frame configuration; scene validation; timeline evaluation; project snapshots and persistence.

**JavaScript legitimately owns:** DOM and form controls; event handling; temporary pointer-selection rectangles and drag previews; browser SVG rendering; Canvas pixel extraction/baking preparation; image decoding; temporary Blob URLs; visual overlays; export downloads; browser-specific measurement; and lightweight standalone playback when a widget/export must operate without the editor.

**Important behavioural exception:** **Flow, Glow and Ripple animations remain independently animated in the live editor even when global Play Scene is paused**. Global Play Scene governs Camera and applicable Optical effects. Never apply a blanket `svg.pauseAnimations()` as a supposed architectural improvement.

For any new change, preserve the sequence **user interaction → validated Go command → successful Go result → read-only JavaScript hydration**. Browser-only temporary previews may precede the commit, but must not become phantom persistent state if Go rejects an operation.

### Existing product features to preserve

Stage 15 established non-destructive **Background Crop** directly on the main photo (Original/Crop 1/Crop 2 etc.), active crop version switching, **Match Canvas** matching intrinsic crop dimensions, image fit modes (`contain`, `cover`, `stretch`, and match semantics), Image Object crop/clip, Snip/Extract, Bake and Keep/Remove, Groups, Layers, Go history, `.sva` embedded assets, SQLite project library, SVG/HTML export, still image export and animated exports. Do not simplify an implementation by removing any of these.

---

## 3. Implementation history / checkpoints

### Remediation Batches 0–7 — before authority audit

The previous work addressed security, Go protocol, scene authority, Go history and lifecycle, Group geometry/locks, crop/snip/bake, export alpha/timing/concurrency, and reliability/clipboard. The last reported Batch 7 commit was **`61ca201`**, with **70/70** JavaScript regression scenarios reported passing. BookStack Batch 7 was Page **445**.

The v1.2.0 protected baseline has previously been identified as commit `81b2277d33d97cb24b42674e1d701a35560b73b2`; treat actual annotated tag verification in the repository as authoritative.

### Batch 8A — read-only Go/WASM authority audit

- **Starting commit:** `61ca201`.
- **Audit documentation commit:** `94bf398`.
- **BookStack:** Page **446**.
- **Result reported:** 437 JavaScript functions examined, 117 Go protocol commands inventoried, **12 authority findings** (A8A-001 to A8A-012). Audit output persisted to Markdown, CSV, and an evidence log. This identified the critical difference between JavaScript presentation and competing persistent authority.
- **Caution:** The audit was initially accepted as an investigation; later Claude review found inaccuracies in *Batch 8D's* documentation. Source and current command mappings should always be checked rather than unquestioningly inheriting report claims.

### Batch 8B — authoritative state and transaction contracts

- **Starting:** `94bf398` → **commit `db3d45a`**.
- **BookStack:** Page **447**.
- **Reported focus:** A8A-001 (background hydration writing into Go), A8A-004 (false-success movement), A8A-005 (unsafe whole-scene sync), A8A-009 (selection updated before Go acceptance), A8A-010 (non-atomic image bake/crop).
- **Implementation reported:** read-only background hydration, checked coordinator results, explicit validated project restore, Go-authoritative selection sequencing and `Image.Bake` transactional command.
- **Regression:** **71/71**.

### Batch 8C — remove JavaScript shadow engine

- **Starting:** `db3d45a` → **commit `3ea4d5c`**.
- **BookStack:** Page **448**.
- **Reported focus:** A8A-002 timeline `_sceneSynced` bypass, A8A-003 persistent JS fallback editing, A8A-006 Canvas/Camera/Optics/Frame, A8A-007 unlayered SVG visibility, A8A-008 photo-fit default, A8A-011 JS Layer reorder fallback, A8A-012 JS object ID counters.
- **Reported implementation:** Go timeline evaluation without fallback JS scene math, failed-engine editing restrictions, `Canvas.Get`/`Canvas.Update`, effective visibility checks, fit default `match`, Go layer operation delegation and Go-generated persistent IDs.
- **Regression:** **72/72**.
- **Caution:** Later Batch 8D's authority documentation contained **invented Go command names**; Batch 8E corrected the record. Verify any specific Go commands against real source before citing them.

### Batch 8D — presentation-only modularisation

- **Starting:** `3ea4d5c` → **commit `b607832`**.
- **BookStack:** Page **449** (later amended with a visible errata notice).
- **Reported structural result:** `app.js` shrank from **7,839 to 3,747 lines** (−4,092 lines; **52.2%**), with **4,139 lines** extracted into eight new presentation modules.

| New module | Reported lines | Responsibility |
|---|---:|---|
| `shape-dropdown.js` | 85 | Shape selector/dropdown presentation |
| `export-adapter.js` | 59 | SVG XML/export browser adapter |
| `workspace.js` | 541 | Workspace drawers/rails/accordion/maximise |
| `notifications.js` | 75 | Toast/status notifications |
| `color-picker.js` | 461 | Color picker UI, HSV/RGB math |
| `timeline.js` | 276 | Playback transport and timecode UI |
| `layers-ui.js` | 861 | Layers drawer and inspector UI |
| `context-editor.js` | 1,781 | Floating contextual properties editor |

**The measured reduction is in `app.js`, not necessarily in total JS lines.** The intention was separation of responsibilities, not elimination of necessary browser code. `coordinator.js` was approximately **1,400 lines** at a user checkpoint and should remain a coordinator rather than absorbing UI implementation. `ImageToolsController` was deliberately **left in `app.js`** to protect image crop/snip/bake behaviour; do not extract it merely to improve line counts.

- **Regression:** **73/73** reported passing.
- **Key subsequent discovery:** Claimed test/architecture/script evidence was partly unreliable even though the extracted code move itself appeared nearly verbatim.

### Claude's independent post-8D audit

- **Audit commit:** **`ae35a6a`**.
- **User-visible review outcome:** **0 P0, 1 P1, 2 P2, 4 P3**. Claude found **no confirmed runtime bug caused by moving the code**; the P1 and both P2 concerned false or inaccurate 8D documentation.
- **Notable findings:** ARCH-05 was a warning, not an all-clear; RUN-06 browser verification not performed; wrong test names; fabricated script files, order and command names in docs; pre-existing duplicate `Image.Update`/`Image.Delete`; `CustomColorPicker.init()` non-idempotent; stale `AI_GUIDE.md`; test harness script order differed from browser.
- **Audit was read-only for application source**, and the review was saved to persistent files.

### Batch 8E — evidence corrections and focused repairs

- **Starting:** `ae35a6a` → **commit `26bceb5`**.
- **BookStack:** Page **450**; Page 449 updated with a visible errata reference.
- **Reported fixes:** duplicate image update and delete dispatches; lock checks; idempotency guard for color-picker initialization; correct actual 25-script HTML loading order and test-harness parity; `AI_GUIDE.md` updated; 15 documentation discrepancies recorded; 44 genuine operations mapped in audit CSV.
- **Correct classifications:** **74/74** JS scenarios PASS; `go vet`, Go tests and race tests PASS; architecture **9 PASS, 1 WARNING (ARCH-05)**; runtime **6 PASS, 1 NOT VERIFIED (RUN-06)**.
- **Critical lesson:** Do not infer real interactive browser success from HTTP 200, syntax checks or a mocked DOM test.

### Batch 8F — Snip / Extract and Crop interaction repair (current)

- **Starting:** `26bceb51318c788dd28d20f726532a6e5d13b23c`.
- **Final implementation commit:** **`54c4488b02131aa3b246a47a1ec63ca492e76f9d`**.
- **BookStack:** Page **451**.
- **Reported source changes:** `index.html`, `style.css`, `stage.js`, `app.js`, `regression-harness.js`.
- **Reports:** all the Batch 8F files in §5.
- **Reported tests:** **75/75** JavaScript regression scenarios; Go vet, Go tests, Go race all PASS; **9 PASS / 1 WARNING** architecture; **6 PASS / 1 NOT VERIFIED** runtime.
- **Git remotes:** origin, gitea and github all reported synchronised with `54c4488`.
- **Remaining critical work:** **user's actual browser acceptance**. Do not call the interactive symptoms definitively resolved until observed.

---

## 4. The precise Batch 8F user symptoms and reported fixes

These details are especially important after context loss. They came from the user's direct report and the implementer's subsequent handover.

### A. Background Snip / Extract — floating button was inert

**Observed by user:** After selecting Background Snip, dragging a rectangle on the loaded image caused a floating **“Extract Image”** button to appear at the top, but clicking it did not perform extraction. Ignoring it and using **“Capture Selected Area”** in the drawer **did work**.

**Reported root causes:**

1. `#btnConfirmExtract` received a pointerdown in `#previewBox` which `stage.js` interpreted as the beginning of a fresh extraction drag. Since `.stage-banner-bar` clicks were not excluded, it reset `bgExtractBox` to **0×0** and captured the pointer.
2. `confirmBgExtractMode()` previously routed `confirmBgSnip(box)` to a hidden drawer preview without also calling `executeBackgroundExtraction(box)`.

**Reported fix:** Exclude banner/button pointer events from stage dragging and route **both** `#btnConfirmExtract` and `#btnCaptureSelectedArea` through a unified extraction operation, leading to one Go-backed Image Object creation; avoid duplicate toasts.

**Acceptance:** Draw the same selection through either workflow. The floating button should produce **one** Image Object and the same extracted content as the working drawer action. Neither control should clear the selection before execution or produce duplicates.

### B. Background Crop — bounding rectangle visible but not draggable

**Observed by user:** Crop displays a bounding box and the cursor becomes a crosshair, but the box cannot be selected or manipulated even though applying the crop still creates versions.

**Reported root causes:**

1. The initial rectangle filled the entire image (`100% × 100%`), preventing movement because there was no free translation area.
2. Pointerdown outside handles or the box did not start the expected new drag selection.
3. `#cropBoxRect` used `fill="rgba(88, 166, 255, 0.001)"`, effectively unpainted for SVG hit-testing at some precisions.
4. `#stageOverlayLayer` had `pointer-events: none` and controls lacked reliable explicit SVG pointer-event attributes.

**Reported fix:** Add `pointer-events="all"`, translucent hit-testable fills and dynamic overlay event handling, active mode cursor rules, drag-to-select, rectangle move, and eight resize handles (NW, N, NE, E, SE, S, SW, W), with coordinates constrained to image bounds.

**Acceptance:** With Crop active, drag out a **smaller** box, move it by its interior, resize it by a corner or side handle, Apply Crop, switch between Original and saved crop versions, and confirm exact expected result.

### C. Were Snip and Crop sharing state?

**Reported analysis:** They **did not** share the same conflicting coordinate state. Both were affected by the stage overlay/pointer event setup and missing banner exclusion. The correct fix should maintain distinct Snip temporary selection and Background Crop temporary editing state.

### D. Important related risks

- Clicking the floating Extract control must not be intercepted by stage pointer capture.
- Switching **Snip → Crop → Snip** must leave no invisible active overlay intercepting the next tool.
- Background crops must remain non-destructive and anchored to the correct source-image coordinates.
- Snip from a selected cropped background must use the correct visible region and crop/fit mapping.
- Zoom/pan, portrait/landscape dimensions, `contain`/`cover`/`stretch`/`match` must not shift selection coordinates.
- Pointer previews may be handled in JS; committed crop metadata and created Image Objects still belong to Go.

---

## 5. Canonical repository documentation to consult on resumption

All paths below are relative to `/root/projects/svg-animator`, unless otherwise noted. Check actual presence rather than inventing replacements.

### Project policy and architecture

- `AGENTS.md`
- `AI_GUIDE.md`
- `docs/architecture/wasm-migration-roadmap.md`
- `docs/architecture/sva-format-specification.md`

### Batch 8A audit

- `docs/architecture/batch8a-go-wasm-authority-audit.md`
- `docs/architecture/batch8a-authority-violation-register.csv`
- `docs/architecture/batch8a-js-function-inventory.csv`
- `docs/architecture/batch8a-go-command-coverage.md`
- `docs/architecture/batch8a-migration-plan.md`
- `docs/architecture/batch8a-audit-evidence.log`

### Batch 8B authority remediation

- `docs/architecture/batch8b-authority-remediation-report.md`
- `docs/architecture/batch8b-command-contract-matrix.csv`
- `docs/architecture/batch8b-failure-atomicity-tests.md`
- `docs/architecture/batch8b-implementation-evidence.log`

### Batch 8C shadow-engine remediation

- `docs/architecture/batch8c-shadow-engine-remediation-report.md`
- `docs/architecture/batch8c-fallback-removal-register.csv`
- `docs/architecture/batch8c-go-authority-command-matrix.csv`
- `docs/architecture/batch8c-timeline-parity-tests.md`
- `docs/architecture/batch8c-implementation-evidence.log`

### Batch 8D modularisation (read together with later errata)

- `docs/architecture/batch8d-modularisation-report.md`
- `docs/architecture/batch8d-appjs-function-map.csv`
- `docs/architecture/batch8d-module-dependency-map.md`
- `docs/architecture/batch8d-authority-boundary-verification.md`
- `docs/architecture/batch8d-regression-evidence.md`
- `docs/architecture/batch8d-implementation-evidence.log`

### Claude review + Batch 8E correction (important for reliability)

- `docs/architecture/claude-post-batch8d-independent-review.md`
- `docs/architecture/claude-post-batch8d-evidence.log`
- `docs/architecture/batch8e-correction-and-remediation-report.md`
- `docs/architecture/batch8e-batch8d-errata.md`
- `docs/architecture/batch8e-command-dispatch-audit.csv`
- `docs/architecture/batch8e-browser-verification.md`
- `docs/architecture/batch8e-implementation-evidence.log`

**Rule:** The original Batch 8D module dependency and authority documents contained source-inaccurate claims. Read **Batch 8E errata** and actual current source before relying on them. Do not resurrect invented command names or obsolete script lists.

### Batch 8F — current implementation reports

- **`docs/architecture/batch8f-snip-crop-interaction-report.md`**
- **`docs/architecture/batch8f-interaction-event-map.md`**
- **`docs/architecture/batch8f-regression-tests.md`**
- **`docs/architecture/batch8f-implementation-evidence.log`**

### BookStack trail

Book ID **353**, SVG Animator Project Log:

| Milestone | Page ID |
|---|---:|
| Batch 7 | 445 |
| Batch 8A | 446 |
| Batch 8B | 447 |
| Batch 8C | 448 |
| Batch 8D | 449 — corrected/annotated by Batch 8E |
| Batch 8E | 450 |
| Batch 8F | **451** |

Reported Batch 8F page:
`https://notes.seaoffate.net/books/svg-animator-project-log/page/post-carbon-review-batch-8f-snip-extract-and-background-crop-interaction-repair`

---

## 6. Important actual source and scripts

Frontend core (current root paths expected):

```text
internal/web/static/index.html
internal/web/static/style.css     # Verify path if renamed
internal/web/static/js/app.js
internal/web/static/js/coordinator.js
internal/web/static/js/engine-bridge.js
internal/web/static/js/scene-state.js
internal/web/static/js/stage.js
internal/web/static/js/nodes.js
internal/web/static/js/flows.js
internal/web/static/js/glows.js
internal/web/static/js/ripples.js
internal/web/static/js/history.js
internal/web/static/js/projects.js
internal/web/static/js/shape-dropdown.js
internal/web/static/js/export-adapter.js
internal/web/static/js/workspace.js
internal/web/static/js/notifications.js
internal/web/static/js/color-picker.js
internal/web/static/js/timeline.js
internal/web/static/js/layers-ui.js
internal/web/static/js/context-editor.js
```

Go side: `cmd/wasm/main.go`, `pkg/engine/protocol/commands.go`, `pkg/engine/scene/` and other actual scene, timeline and export packages.

**Actual production script order from Batch 8E (25 scripts, sourced from `index.html`):**

```text
 1 wasm_exec.js             2 engine-bridge.js
 3 effects.js               4 scene-state.js
 5 coordinator.js           6 stage.js
 7 shape-registry.js        8 presets.js
 9 nodes.js                10 flows.js
11 glows.js                12 ripples.js
13 history.js              14 projects.js
15 shape-dropdown.js       16 export-adapter.js
17 workspace.js            18 notifications.js
19 color-picker.js         20 timeline.js
21 layers-ui.js            22 context-editor.js
23 app.js                  24 still-export-modal.js
25 export-modal.js
```

Do **not** use the earlier Batch 8D handover's incorrect script order placing `coordinator.js` at position 24.

### Current test commands

Run from repository root, with exit codes accurately recorded:

```bash
node scripts/regression-harness.js
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./pkg/engine/scene/...
bash scripts/verify-architecture.sh
bash scripts/verify-runtime.sh
git diff --check
```

`verify-architecture.sh`: latest reported **9 PASS, 1 WARNING, 0 FAIL**. The warning **ARCH-05** is the known still-export offscreen DOM sandbox. It measures SVG path bounds using browser DOM geometry, a justified/accepted browser-specific exception unless it causes actual export errors or a new design requirement explicitly removes it. **Not a reason to launch another Go migration now.**

`verify-runtime.sh`: latest reported **6 PASS, 1 NOT VERIFIED**. RUN-06 requires an actual Chromium/Chrome browser, unavailable in the reported LXC environment. A missing browser is **not a pass**. Mocked tests cannot certify actual pointer hit-testing.

---

## 7. Outstanding Batch 8F manual browser acceptance — highest priority

Perform these checks **before any further implementation**, ideally in a new private/test project with no irreplaceable user data. Check DevTools console for errors.

### Checklist A — Snip and floating Extract Image

- [ ] Load a background photo via Canvas & Background.
- [ ] Open **Crop, Clip & Snip** (reported Rail button 8), then Background Snip.
- [ ] Drag a clearly smaller selection rectangle over the actual photo.
- [ ] Confirm floating **✓ Extract Image** is visible and enabled.
- [ ] Click it; verify **exactly one** Image Object is created, layer appears, stage updates, correct source pixels and dimensions are present, and no spurious error is logged.
- [ ] Repeat with a comparable rectangle via drawer **Capture Selected Area**. Check equivalent result.
- [ ] Click the floating action with invalid/empty selection; check controlled no-op or clear error, not phantom image creation.

### Checklist B — Background Crop

- [ ] Switch to **Crop Background**.
- [ ] Confirm bounding box appears with visible handles and crosshair on stage background.
- [ ] Drag on the photo to create a new, smaller crop selection.
- [ ] Hover over box interior; check move cursor and drag to translate within bounds.
- [ ] Hover over bottom-right and side handles; check resize cursor and drag to resize.
- [ ] Click floating **✓ Apply Crop**; verify cropped result and a new non-destructive crop version.
- [ ] Switch to Original and back to the new crop; verify both correct.
- [ ] Test Match Canvas to ensure intrinsic cropped dimensions are used.
- [ ] Cancel/Escape and re-enter Crop; ensure no stale overlay.

### Checklist C — Interference and preservation

- [ ] Switch **Snip → Crop → Snip → Crop**, repeat short pointer interactions each time.
- [ ] Confirm neither tool steals a click meant for a floating toolbar.
- [ ] Try a zoomed/panned image and one of `contain`, `cover`, `stretch` or `match` as applicable.
- [ ] Test Image Object Clip/Crop separately from Background Crop.
- [ ] Save/reopen a project with cropped background and extracted image; verify persistent state.
- [ ] Confirm Go Undo/Redo restores expected result.
- [ ] Run brief animation smoke: Flow/Glow/Ripple continue independently while Camera Play Scene is paused.

**Exit criteria:** The two original interaction defects are **observably** fixed in the real browser and the remaining Stage 15 flows above are still healthy. If not, make a **new, tightly scoped repair prompt** using exact reproduction details. Do not give a model an unlimited rewrite brief.

### Quick manual test record template

```text
Date / browser / version:
Checked commit and WASM readiness:
Snip floating Extract: PASS / FAIL — details:
Drawer Capture Selected Area: PASS / FAIL — details:
Crop drag-to-select: PASS / FAIL — details:
Crop move: PASS / FAIL — details:
Crop corner/side resize: PASS / FAIL — details:
Crop Apply + Original/version switch: PASS / FAIL — details:
Snip/Crop switching: PASS / FAIL — details:
Save/reopen and Undo/Redo: PASS / FAIL — details:
Console errors and screenshots:
```

---

## 8. Current operational restrictions and safe model workflow

At the time of handover, the user reported Gemini in Antigravity at **3.72% remaining with ~35 hours to reset**. Claude Opus/Sonnet were available through a limited free rolling and monthly allowance. The user is comfortable **pausing for a day or more**, then resuming.

Model-independent workflow:

1. Finish and preserve one controlled batch at a time.
2. Start from exact Git SHA and inspect `git status`.
3. Read relevant reports and **verify actual source**; do not inherit fabricated evidence or outdated script references.
4. Make the smallest correction consistent with Go authority.
5. Run specified Go and JS tests, plus the strongest real-browser check available.
6. Persist evidence to dedicated Markdown/CSV/log files in `docs/architecture/` as work progresses.
7. Distinguish PASS, FAIL, WARNING, SKIPPED and NOT VERIFIED; never equate HTTP 200 with browser/WASM interactive success.
8. Commit and push only intended files, directly to `origin`, `gitea` and `github` as the user has requested. No force pushes, reset/rebase/stash, blanket `git add -A`, experimental pushes or artificial delays.
9. Do not overlap model agents modifying the same files simultaneously. An independent Claude review can follow a Gemini implementation at a committed checkpoint.
10. Pause without penalty if quota is low. Nothing about the project requires immediate continuation.

### Protected deployment boundaries

Do not change protected `v1.2.0` or `release/v1.2.0`, repository-root production executable `./svg-animator`, deployment guides or `build_and_run.sh`/`.bat`, user SQLite files (including `studio_library.db` with possible WAL/SHM), or production services on `8088` and `8090`. Only use dedicated development binary/server at `127.0.0.1:8089` when authorised by the current batch. Do not SSH or deploy to kiwiberry/elderberry.

### BookStack and evidence discipline

All work must produce **local permanent documentation files**, not just a terminal handover or BookStack page. Publish a BookStack milestone if the batch requests it, and include its page ID in the final report. When correcting false past claims, issue a visible erratum rather than silently presenting historical reports as if always accurate.

---

## 9. What's left — and what is explicitly *not* left

### Still outstanding

1. **Real-browser verification of Batch 8F** (primary gate).
2. Any newly observed Snip/Crop interaction defects found in that test, if any.
3. The accepted **ARCH-05** browser-only still-export bounds-measurement exception: documented, not an urgent defect.
4. Optional future isolation of `ImageToolsController` from `app.js` if there is a concrete maintainability benefit and strong regression coverage; **not required** to finish Batch 8F.
5. Future product work (Stage 16), only after current UI interactions are accepted.

### No need to redo now

- Batch 8A audit from scratch.
- Batch 8B/8C Go authority migration wholesale.
- Batch 8D extraction of the eight UI modules.
- Batch 8E correction of duplicate Image commands, color picker guard or script ordering, unless a new regression shows one failed.
- Broad architectural refactor to eliminate the legitimate offscreen DOM measurement solely to get 10/10 green checkmarks.

---

## 10. Later roadmap — Stage 16 (brief reminder only)

The project has discussed Stage 16 as **Digital Twin Presenter and Live Telemetry**, with a practical incremental sequence:

- **Stage 16A — Reusable SVG widgets:** Embeddable self-contained scenes, isolation of IDs/filters/styles/animation state so multiple widgets can coexist.
- **Stage 16B — External parameter controls:** Defined host/widget API for colours, visibility, speed, state etc., first with simulated inputs.
- **Stage 16C — Digital twins/live telemetry:** Connect widget inputs to external MQTT/WebSocket/HTTP data for live equipment status, measurements and alarms.

**This staging is the conversation's proposed implementation sequence, not independently verified text from the project roadmap.** Before producing Stage 16 prompts, read `docs/architecture/wasm-migration-roadmap.md` and any dedicated Stage 16 design documents. The user explicitly wants **widgets first**, then external controls and digital twin connectivity. Go remains the editor's authoring authority; standalone widgets should not require the full editor engine unless the approved architecture explicitly says so.

**Do not start 16A yet unless the user explicitly approves it after manual 8F verification.**

---

## 11. Ready-to-paste continuation request for a later session

> We are continuing SVG Animator Studio at `/root/projects/svg-animator`, `main`, last reported commit `54c4488` (Batch 8F). Read the handover `SVG_Animator_Studio_Recovery_Handover_Batch8F_2026-11-12.md`, `AGENTS.md`, `AI_GUIDE.md`, the Batch 8F reports under `docs/architecture/`, and the Batch 8E errata. The current priority is real-browser verification of Snip floating Extract Image and Crop bounding-box selection/move/resize, not Stage 16 or another broad refactor. Preserve the protected v1.2.0 release, production DBs/ports and Go/WASM authority. Check `git status`, actual source paths and test evidence. Do not claim interactive browser success from mock tests. Once I provide the manual test results, help me decide whether we need a targeted repair batch or can proceed to the next stage.

---

## 12. Last known status, in one sentence

**SVG Animator Studio is reported clean, committed and pushed at Batch 8F (`54c4488`), with a significantly modularised UI, Go/WASM authority corrections, 75 passing JS regression scenarios and both Snip/Crop interaction fixes implemented — but the user's real-browser confirmation of those two controls is the remaining acceptance gate before deciding on future work.**
