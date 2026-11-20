# SVG Animator Studio v1.2.0 (Linux x86_64)

## Overview
SVG Animator Studio is a self-hosted vector diagram animation studio and CLI tool. This release compiles the authoritative Go engine, embedded WebAssembly engine, and modern responsive frontend directly into a single portable binary.

## Release Metadata
- **Version:** v1.2.0
- **Architecture:** linux/amd64 (statically linked ELF)
- **Engine Protocol:** 1.0.0
- **Native Project Format (.sva):** 1.0.0
- **Project Schema:** 1.2.0

## Quick Start
```bash
# 1. Run with default config (listening on http://0.0.0.0:8088):
./svg-animator

# 2. Run on custom port:
./svg-animator -serve :8090

# 3. Print version information:
./svg-animator -version
```

## Runtime Dependencies
- **Core Studio & SVA Interchange:** Fully self-contained. No external database or node runtime needed (SQLite and frontend embedded in binary).
- **Video Exports (WebP / APNG / MP4):** Requires `ffmpeg` and headless `google-chrome` or native `chromium` on PATH.
