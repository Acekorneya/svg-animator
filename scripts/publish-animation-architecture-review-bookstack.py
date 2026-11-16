#!/usr/bin/env python3
import urllib.request
import json
import os

BOOKSTACK_URL = "https://notes.seaoffate.net"
BOOK_ID = 353
TOKEN_ID = "dLfxypy15tqUfmnNy2Y36MXBdKK5RXEB"
TOKEN_SECRET = "56fltcsySDv73M68XMGuvXr9xbLoUuiz"

PAGE_TITLE = "Animation Architecture and Extensibility Review"

with open("docs/architecture/animation-architecture-and-extensibility-review.md", "r", encoding="utf-8") as f:
    PAGE_MARKDOWN = f.read()

def publish_to_bookstack():
    headers = {
        "Authorization": f"Token {TOKEN_ID}:{TOKEN_SECRET}",
        "Content-Type": "application/json"
    }

    req_pages = urllib.request.Request(f"{BOOKSTACK_URL}/api/books/{BOOK_ID}", headers=headers)
    try:
        with urllib.request.urlopen(req_pages) as resp:
            book_data = json.loads(resp.read().decode())
    except Exception as e:
        print(f"Error fetching book: {e}")
        return False

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
        print(f"Updating existing page ID {existing_page_id}...")
        url = f"{BOOKSTACK_URL}/api/pages/{existing_page_id}"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="PUT")
    else:
        print(f"Creating new page under book ID {BOOK_ID}...")
        payload["book_id"] = BOOK_ID
        url = f"{BOOKSTACK_URL}/api/pages"
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers, method="POST")

    try:
        with urllib.request.urlopen(req) as resp:
            res_data = json.loads(resp.read().decode())
            print(f"Successfully published: ID={res_data.get('id')}, Slug={res_data.get('slug')}")
            print(f"URL: {BOOKSTACK_URL}/books/{book_data.get('slug')}/page/{res_data.get('slug')}")
            return True
    except Exception as e:
        print(f"Error publishing page: {e}")
        return False

if __name__ == "__main__":
    publish_to_bookstack()
