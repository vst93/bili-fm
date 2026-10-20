#!/usr/bin/env python3
"""Generate a Tauri v2 latest.json for the mygo -> Tauri migration release.

Usage:
  make_tauri_latest.py --version 3.1.0 --base-url https://github.com/... \
      --assets-dir bridge --out bridge/latest.json

The JSON schema is the one the Tauri updater plugin reads. Every platform
entry points at a migration artifact signed with the *old* Tauri minisign
key (see sign-tauri.sh); the mygo side uses its own update-*.json files.
"""

import argparse
import datetime
import json
from pathlib import Path

PLATFORMS = {
    "windows-x86_64": "bili-FM-{version}-windows-amd64-migrate.exe",
    "windows-aarch64": "bili-FM-{version}-windows-arm64-migrate.exe",
    "darwin-aarch64": "bili-FM-{version}-macos-apple-silicon.app.tar.gz",
    "darwin-x86_64": "bili-FM-{version}-macos-intel.app.tar.gz",
    "bili-fm-linux-x86_64-deb-v2": "bili-FM-{version}-linux-x86_64.deb",
    "bili-fm-linux-x86_64-rpm-v2": "bili-FM-{version}-linux-x86_64.rpm",
    "bili-fm-linux-aarch64-deb-v2": "bili-FM-{version}-linux-arm64.deb",
    "bili-fm-linux-aarch64-rpm-v2": "bili-FM-{version}-linux-arm64.rpm",
}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--version", required=True)
    ap.add_argument("--base-url", required=True)
    ap.add_argument("--assets-dir", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    assets = Path(args.assets_dir)
    base = args.base_url.rstrip("/")
    platforms = {}
    for target, name_t in PLATFORMS.items():
        name = name_t.format(version=args.version)
        sig = assets / (name + ".sig")
        if not sig.exists():
            raise SystemExit(f"missing Tauri signature: {sig}")
        url = f"{base}/{name}"
        platforms[target] = {
            "signature": sig.read_text(encoding="utf-8").strip(),
            "url": url,
        }

    manifest = {
        "version": args.version,
        "notes": f"bili-FM {args.version} (native version; migrating from Tauri)",
        "pub_date": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
        "platforms": platforms,
    }
    Path(args.out).write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(manifest, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
