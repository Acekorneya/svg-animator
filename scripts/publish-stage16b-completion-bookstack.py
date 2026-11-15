#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16B Completion — Visual Widget Property Bindings Inside Editor"

with open("docs/architecture/stage16b-visual-authoring-implementation.md", "r") as f:
    impl_md = f.read()

with open("docs/architecture/stage16b-visual-authoring-acceptance.md", "r") as f:
    acc_md = f.read()

PAGE_MARKDOWN = f"""# Stage 16B Completion — Visual Widget Property Bindings Inside Editor

## Overview
This milestone marks the complete delivery of **Visual Widget Property Bindings** inside SVG Animator Studio. Users can now author, select, expose, configure, preview, and export interactive SVG widgets directly through Studio's visual interface without ever manually writing JSON manifests or editing SVG source code.

---

{impl_md}

---

{acc_md}
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
        # If page already exists or error, try listing or update
        raise

if __name__ == "__main__":
    publish()
