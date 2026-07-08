#!/usr/bin/env python3
"""Sync GitHub Releases to Gitee with parallel upload."""

import os
import sys
import json
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
from requests_toolbelt import MultipartEncoder

GITHUB_API = "https://api.github.com/repos"
GITEE_API = "https://gitee.com/api/v5/repos"

GH_OWNER = os.environ["GITHUB_OWNER"]
GH_REPO = os.environ["GITHUB_REPO"]
GITEE_OWNER = os.environ["GITEE_OWNER"]
GITEE_REPO = os.environ["GITEE_REPO"]
GITEE_TOKEN = os.environ["GITEE_TOKEN"]
MAX_WORKERS = int(os.environ.get("MAX_WORKERS", "4"))
DOWNLOAD_DIR = os.environ.get("DOWNLOAD_DIR", "downloads")

os.makedirs(DOWNLOAD_DIR, exist_ok=True)


def gh_headers():
    return {"Accept": "application/vnd.github+json"}


def get_github_releases():
    """Fetch all releases from GitHub with pagination."""
    all_releases = []
    page = 1
    while True:
        resp = requests.get(
            f"{GITHUB_API}/{GH_OWNER}/{GH_REPO}/releases",
            headers=gh_headers(),
            params={"per_page": 100, "page": page},
        )
        resp.raise_for_status()
        data = resp.json()
        if not data:
            break
        all_releases.extend(data)
        page += 1
    return all_releases


def get_gitee_releases():
    """Fetch all releases from Gitee, return dict keyed by tag_name."""
    resp = requests.get(
        f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases",
        params={"access_token": GITEE_TOKEN, "per_page": 100},
    )
    resp.raise_for_status()
    return {r["tag_name"]: r for r in resp.json()}


def create_gitee_release(tag_name, name, body, target_commitish):
    """Create a release on Gitee, return release info dict."""
    resp = requests.post(
        f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases",
        data={
            "access_token": GITEE_TOKEN,
            "tag_name": tag_name,
            "name": name,
            "body": body or "-",
            "target_commitish": target_commitish,
        },
    )
    if resp.status_code >= 300:
        raise Exception(f"Create release failed: {resp.status_code} {resp.text}")
    return resp.json()


def get_gitee_release_assets(release_id):
    """Get existing asset names on a Gitee release."""
    resp = requests.get(
        f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases/{release_id}",
        params={"access_token": GITEE_TOKEN},
    )
    resp.raise_for_status()
    data = resp.json()
    return {a["name"] for a in data.get("assets", [])}


def download_asset(url, filename):
    """Download a file from GitHub to local, return local path."""
    local_path = os.path.join(DOWNLOAD_DIR, filename)
    resp = requests.get(url, stream=True, headers=gh_headers())
    resp.raise_for_status()
    with open(local_path, "wb") as f:
        for chunk in resp.iter_content(chunk_size=1024 * 1024):
            f.write(chunk)
    return local_path


def upload_asset(release_id, local_path, filename):
    """Upload a single file to a Gitee release."""
    with open(local_path, "rb") as f:
        encoder = MultipartEncoder(
            fields={
                "access_token": GITEE_TOKEN,
                "file": (filename, f, "application/octet-stream"),
            }
        )
        resp = requests.post(
            f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases/{release_id}/attach_files",
            data=encoder,
            headers={"Content-Type": encoder.content_type},
        )
    if resp.status_code >= 300:
        raise Exception(f"Upload {filename} failed: {resp.status_code} {resp.text}")
    return resp.json()


def sync_release(github_release, gitee_release_info):
    """Download assets from GitHub and upload to Gitee for one release."""
    tag = github_release["tag_name"]
    gitee_release_id = gitee_release_info["id"]

    # Get existing assets on Gitee
    existing = get_gitee_release_assets(gitee_release_id)

    # Filter assets that need syncing
    to_sync = [
        a for a in github_release.get("assets", []) if a["name"] not in existing
    ]

    if not to_sync:
        print(f"  [{tag}] All assets already synced, skipping")
        return

    print(f"  [{tag}] {len(to_sync)} assets to sync ({MAX_WORKERS} parallel)")

    def process_asset(asset):
        """Download then upload a single asset."""
        name = asset["name"]
        url = asset["browser_download_url"]
        local_path = download_asset(url, name)
        upload_asset(gitee_release_id, local_path, name)
        os.remove(local_path)
        return name

    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as pool:
        futures = {pool.submit(process_asset, a): a["name"] for a in to_sync}
        for future in as_completed(futures):
            name = futures[future]
            try:
                future.result()
                print(f"    ✅ {name}")
            except Exception as e:
                print(f"    ❌ {name}: {e}", file=sys.stderr)
                raise


def main():
    print("Fetching GitHub releases...")
    github_releases = get_github_releases()
    print(f"  Found {len(github_releases)} releases")

    print("Fetching Gitee releases...")
    gitee_releases = get_gitee_releases()
    print(f"  Found {len(gitee_releases)} releases")

    # Sort by created_at (old to new)
    github_releases.sort(key=lambda x: x.get("created_at", ""))

    for gh_release in github_releases:
        tag = gh_release["tag_name"]

        # Create release on Gitee if not exists
        if tag in gitee_releases:
            gitee_release_info = gitee_releases[tag]
        else:
            print(f"  [{tag}] Creating Gitee release...")
            # Get commit message for body if missing
            body = gh_release.get("body", "")
            if not body:
                commitish = gh_release.get("target_commitish", "")
                if commitish:
                    try:
                        cr = requests.get(
                            f"{GITHUB_API}/{GH_OWNER}/{GH_REPO}/commits/{commitish}",
                            headers=gh_headers(),
                        )
                        cr.raise_for_status()
                        body = cr.json().get("commit", {}).get("message", "-")
                    except Exception:
                        body = "-"
            gitee_release_info = create_gitee_release(
                tag,
                gh_release.get("name", tag),
                body,
                gh_release.get("target_commitish", "master"),
            )
            gitee_releases[tag] = gitee_release_info

        # Sync assets
        sync_release(gh_release, gitee_release_info)

    print("Done!")


if __name__ == "__main__":
    main()
