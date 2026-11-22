# SVG Animator Studio v1.2.0 (Build: 2026-11-22 18:10:00 UTC)

## Overview
SVG Animator Studio is a self-hosted vector diagram animation studio, widget maker, and CLI tool. This release compiles the authoritative Go engine, embedded WebAssembly engine, and responsive studio frontend directly into standalone portable binaries for Linux (x86_64) and Windows (x86_64).

## Release Metadata
- **Version:** v1.2.0
- **Build Date & Time:** 2026-11-22 18:10:00 UTC (2026-11-22T18:10:00Z)
- **Source Commit:** 18902db
- **Architectures:** linux/amd64 (statically linked ELF) & windows/amd64 (.exe)
- **Engine Protocol:** 1.0.0
- **Native Project Format (.sva):** 1.0.0
- **Project Schema:** 1.2.0

## Included Highlights
- Independent canvas width and height down to **20 × 20 pixels** (up to 3840 × 3840 working canvas and 12000 × 12000 still print export) with **2500%** manual stage zoom.
- All 8 **Low-Load Effects**: Perimeter Tracer Lite, Pulse Lite, Flash Lite, Flow Lite, Radar Ping Lite, 4-Threshold Gauge / Level Lite, 7-Preset Oscilloscope Lite, and Text Box Lite (10 font stacks, curved `<textPath>` arcs, horizontal tickers, multiline and stacked vertical scrolls, 4 typewriter sequences, 3-phase fade/blink, scramble, and cycle-boundary pending-text handover).
- **Groups, Semantic Group Property Mappings & Reusable Composite Templates** (`<group>-group-<alias>` multi-target fan-out, atomic validation, sequential placed instances).
- Universal **`play` ("Play Animation")** boolean widget property across all object categories (`lite`, `node`, `flow`, `glow`, `ripple`, and `group`), independent of `playbackMode`.

## Quick Start (Linux)
```bash
chmod +x ./svg-animator-linux
./svg-animator-linux -serve :8088
```

## Quick Start (Windows)
```cmd
svg-animator-windows.exe -serve :8088
```
