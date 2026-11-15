#!/usr/bin/env python3
import os
import shutil
import zipfile
import tarfile

ROOT_DIR = "/root/projects/svg-animator"
DEMO_DIR = os.path.join(ROOT_DIR, "docs/demonstrations/widget-demo")
JS_SRC = os.path.join(ROOT_DIR, "internal/web/static/js/svg-widget.js")
JS_DEST = os.path.join(DEMO_DIR, "js/svg-widget.js")
INDEX_SRC = os.path.join(ROOT_DIR, "internal/web/static/demo-widgets.html")
INDEX_DEST = os.path.join(DEMO_DIR, "index.html")
STANDALONE_DEST = os.path.join(DEMO_DIR, "standalone-demo.html")
ZIP_DEST = os.path.join(ROOT_DIR, "docs/demonstrations/svg-widget-offline-demo.zip")
TAR_DEST = os.path.join(ROOT_DIR, "docs/demonstrations/svg-widget-offline-demo.tar.gz")
ARTIFACT_DIR = "/root/.gemini/antigravity-cli/brain/66e3e607-0141-40d2-90e5-05fe86002a21"

os.makedirs(os.path.join(DEMO_DIR, "js"), exist_ok=True)

# 1. Copy js and index
shutil.copyfile(JS_SRC, JS_DEST)
shutil.copyfile(INDEX_SRC, INDEX_DEST)

# 2. Build standalone-demo.html by inlining svg-widget.js
with open(JS_SRC, "r", encoding="utf-8") as f:
    js_content = f.read()

with open(INDEX_SRC, "r", encoding="utf-8") as f:
    html_content = f.read()

# Replace <script src="js/svg-widget.js"></script> with inline script
inline_script = f"<script>\n{js_content}\n</script>"
standalone_html = html_content.replace('<script src="js/svg-widget.js"></script>', inline_script)

with open(STANDALONE_DEST, "w", encoding="utf-8") as f:
    f.write(standalone_html)

# Update README.md
readme_content = """# SVG Animator Studio — Stage 16B Interactive Widget Demonstration

This offline package demonstrates **Stage 16B: External Widget Controls, Runtime Parameters and Independent Host Integration**.

## Contents

- `standalone-demo.html`: Single-file zero-dependency self-contained runner with inlined `SVGWidget` library and 3 interactive process barrels. Open directly in Chrome, Firefox, Safari, or Edge (`file://` or HTTP).
- `index.html`: Multi-file demonstration linking to `./js/svg-widget.js`.
- `js/svg-widget.js`: Standalone universal UMD library (`SVGWidget.mount`, `isolate`, `setParameter`, `setSpeed`, etc.).

## Key Capabilities Demonstrated

1. **Independent Instance Control:** Three Barrel instances (A, B, C) operate concurrently with individual states (`running`, `paused`, `alarm`), distinct fill levels, custom status colors, and individual speed playback rates.
2. **Zero DOM ID Collisions:** Automatic ID namespacing (`barrel_a__*`, `barrel_b__*`, `barrel_c__*`) ensures zero mutual interference in gradients, filters, or SMIL animations.
3. **Atomic Parameter Validation:** Strict runtime validation against the declared manifest schema, preventing invalid types, out-of-range numbers, or malicious injection.
4. **Independent Playback Control:** Pausing or changing the playback speed of Barrel B has zero effect on Barrel A or Barrel C.
5. **Real-time Host Telemetry:** Click '⚡ Auto Telemetry: ON' to simulate live host-driven process updates across all mounted widgets.
"""

with open(os.path.join(DEMO_DIR, "README.md"), "w", encoding="utf-8") as f:
    f.write(readme_content)

# 3. Create zip archive
with zipfile.ZipFile(ZIP_DEST, "w", zipfile.ZIP_DEFLATED) as z:
    for root, dirs, files in os.walk(DEMO_DIR):
        for file in files:
            full_path = os.path.join(root, file)
            rel_path = os.path.relpath(full_path, DEMO_DIR)
            z.write(full_path, arcname=os.path.join("widget-demo", rel_path))

# 4. Create tar.gz archive
with tarfile.open(TAR_DEST, "w:gz") as t:
    t.add(DEMO_DIR, arcname="widget-demo")

# 5. Copy to artifact directory for easy download
if os.path.exists(ARTIFACT_DIR):
    shutil.copyfile(STANDALONE_DEST, os.path.join(ARTIFACT_DIR, "standalone-demo.html"))
    shutil.copyfile(ZIP_DEST, os.path.join(ARTIFACT_DIR, "svg-widget-offline-demo.zip"))
    shutil.copyfile(TAR_DEST, os.path.join(ARTIFACT_DIR, "svg-widget-offline-demo.tar.gz"))

print("Stage 16B Demo Package built successfully!")
