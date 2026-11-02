#!/usr/bin/env python3
"""删除 Gitee 上误同步的 release（可选同时删 tag）。

用法：
    GITEE_TOKEN=... TAG=mygo-v3.1.15 python3 delete_gitee_release.py [--keep-tag]
"""

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

TOKEN = os.environ["GITEE_TOKEN"]
TAG = os.environ["TAG"]
BASE = "https://gitee.com/api/v5/repos/vst93/bili-fm"


def call(method, path):
    url = BASE + path + "?" + urllib.parse.urlencode({"access_token": TOKEN})
    req = urllib.request.Request(url, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return resp.status, resp.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace")


def list_releases():
    status, body = call("GET", "/releases?per_page=100")
    if status >= 300:
        raise SystemExit(f"list releases failed: {status} {body[:300]}")
    return json.loads(body)


def main():
    keep_tag = "--keep-tag" in sys.argv[1:]
    hits = [r for r in list_releases() if r.get("tag_name") == TAG]
    if not hits:
        print(f"no release with tag {TAG}")
    for r in hits:
        status, body = call("DELETE", f"/releases/{r['id']}")
        print(f"delete release {r['id']}: {status} {body[:200]}")

    if not keep_tag:
        status, body = call("DELETE", f"/tags/{urllib.parse.quote(TAG)}")
        print(f"delete tag {TAG}: {status} {body[:200]}")

    remaining = [
        r["tag_name"] for r in list_releases() if r.get("tag_name", "").startswith("mygo-v")
    ]
    print("remaining mygo releases:", remaining)


if __name__ == "__main__":
    main()
