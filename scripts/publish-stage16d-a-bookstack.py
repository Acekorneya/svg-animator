#!/usr/bin/env python3
import urllib.request
import json

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Stage 16D-A — Foundation, Visibility-Aware Animation & Low-Load Effects"

with open("docs/architecture/stage16d-a-foundation-and-lite-effects.md", "r", encoding="utf-8") as f:
    PAGE_MARKDOWN = f.read()

def publish_to_bookstack():
    headers = {
        "Authorization": f"Token {TOKEN_ID}:{TOKEN_SECRET}",
        "Content-Type": "application/json"
    }

    req_pages = urllib.request.Request(f"{BOOKSTACK_URL}/api/books/{BOOK_ID}", headers=headers)
    with urllib.request.urlopen(req_pages) as resp:
        book_data = json.loads(resp.read().decode())

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
        url = f"{BOOKSTACK_URL}/api/pages/{existing_page_id}"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="PUT")
    else:
        payload["book_id"] = BOOK_ID
        url = f"{BOOKSTACK_URL}/api/pages"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="POST")

    with urllib.request.urlopen(req) as resp:
        res_data = json.loads(resp.read().decode())
        print(f"Successfully published: ID={res_data.get('id')}, Slug={res_data.get('slug')}")
        print(f"URL: {BOOKSTACK_URL}/books/{book_data.get('slug')}/page/{res_data.get('slug')}")

if __name__ == "__main__":
    publish_to_bookstack()
