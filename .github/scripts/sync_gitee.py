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
# CORE_ONLY: 只同步应用内更新必需的资产（各平台 update-*.json + 原生归档/
# delta）。Gitee 的上传带宽是整套镜像里唯一的慢点：全量约 170MB 要 30-40
# 分钟；先完成这 ~42MB，Gitee 的检查/下载链路就能用了，安装包/迁移包可以
# 后补。workflow_dispatch 的 core_only 输入会设置本变量。
CORE_ONLY = os.environ.get("CORE_ONLY", "").strip().lower() in ("1", "true", "yes", "on")

os.makedirs(DOWNLOAD_DIR, exist_ok=True)


def gh_headers():
    return {"Accept": "application/vnd.github+json"}


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
    return {a["name"] for a in get_gitee_release_assets_detail(release_id)}


def get_gitee_release_assets_detail(release_id):
    """Get assets (name/id) on a Gitee release."""
    resp = requests.get(
        f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases/{release_id}",
        params={"access_token": GITEE_TOKEN},
    )
    resp.raise_for_status()
    return resp.json().get("assets", [])


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

    # latest.json 是给旧 Tauri updater 的清单：GitHub 版里的 URL 指向
    # GitHub；同步到 Gitee 时要用 latest-gitee.json（URL 指向 Gitee），由
    # sync_gitee_latest 改名上传。其余资产正常同步。
    skip = {"latest.json", "latest-gitee.json"}
    to_sync = [
        a for a in github_release.get("assets", [])
        if a["name"] not in existing and a["name"] not in skip
    ]

    if not to_sync:
        print(f"  [{tag}] All assets already synced, skipping")
        return

    def is_core(name):
        # 应用内更新必需：清单 + 清单指向的原生归档 / delta。注意原生归档是
        # 小写 bili-fm-*；大写 bili-FM-*.app.tar.gz 是旧 Tauri 迁移包。
        if name.startswith("update-") and name.endswith(".json"):
            return True
        return name.startswith("bili-fm-") and (name.endswith(".tar.gz") or name.endswith(".delta"))

    def upload_batch(batch, label):
        if not batch:
            return
        print(f"  [{tag}] {label}: {len(batch)} assets ({MAX_WORKERS} parallel)")

        def process_asset(asset):
            """Download then upload a single asset."""
            name = asset["name"]
            url = asset["browser_download_url"]
            local_path = download_asset(url, name)
            upload_asset(gitee_release_id, local_path, name)
            os.remove(local_path)
            return name

        with ThreadPoolExecutor(max_workers=MAX_WORKERS) as pool:
            futures = {pool.submit(process_asset, a): a["name"] for a in batch}
            for future in as_completed(futures):
                name = futures[future]
                try:
                    future.result()
                    print(f"    ✅ {name}")
                except Exception as e:
                    print(f"    ❌ {name}: {e}", file=sys.stderr)
                    raise

    core_archives = [a for a in to_sync if is_core(a["name"]) and not a["name"].startswith("update-")]
    manifests = [a for a in to_sync if a["name"].startswith("update-")]
    rest = [a for a in to_sync if a not in core_archives and a not in manifests]

    # 先归档、后清单：清单一旦在 Gitee 可见，应用会立刻按它去下载归档；
    # 归档先就位可以避免「刚同步到一半，Gitee 清单已存在但 404」的窗口。
    upload_batch(core_archives, "core archives")
    upload_batch(manifests, "core manifests")
    if CORE_ONLY:
        print(f"  [{tag}] core_only=true，跳过 {len(rest)} 个安装包/迁移包")
    else:
        upload_batch(rest, "installers / migration packages")


def sync_gitee_latest(github_release, gitee_release_info):
    """把 GitHub 上的 latest-gitee.json 作为 Gitee 的 latest.json 上传.

    旧 Tauri updater 的第一个 endpoint 是 Gitee；如果 Gitee 的 latest.json
    里还是 GitHub 资产 URL，中国用户会卡在下载。桥接 release 会同时生成
    latest-gitee.json，这里替换掉普通同步的 latest.json。
    """
    source = next((a for a in github_release.get("assets", []) if a["name"] == "latest-gitee.json"), None)
    if source is None:
        return
    print("  Syncing Gitee-specific latest.json...")
    local_path = download_asset(source["browser_download_url"], "latest-gitee.json")
    try:
        release_id = gitee_release_info["id"]
        for asset in get_gitee_release_assets_detail(release_id):
            if asset.get("name") != "latest.json":
                continue
            # Gitee 的 attach file id；删除失败也不阻断（例如 API 版本差异）。
            resp = requests.delete(
                f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases/{release_id}/attach_files/{asset.get('id')}",
                params={"access_token": GITEE_TOKEN},
            )
            if resp.status_code >= 300 and resp.status_code != 404:
                print(f"    ⚠ 删除旧 latest.json 失败: {resp.status_code} {resp.text}", file=sys.stderr)
        upload_asset(release_id, local_path, "latest.json")
        print("    ✅ latest.json (Gitee URLs)")
    finally:
        os.remove(local_path)


def main():
    print("Fetching GitHub latest release...")
    resp = requests.get(
        f"{GITHUB_API}/{GH_OWNER}/{GH_REPO}/releases/latest",
        headers=gh_headers(),
    )
    resp.raise_for_status()
    gh_release = resp.json()
    tag = gh_release["tag_name"]
    print(f"  Latest release: {tag}")

    print("Fetching Gitee releases...")
    gitee_releases = get_gitee_releases()
    print(f"  Found {len(gitee_releases)} releases")

    # Create release on Gitee if not exists
    if tag in gitee_releases:
        gitee_release_info = gitee_releases[tag]
    else:
        print(f"  [{tag}] Creating Gitee release...")
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

    # Keep release name/body/prerelease in sync with GitHub
    gh_body = gh_release.get("body", "")
    gh_name = gh_release.get("name", tag)
    gh_prerelease = "true" if gh_release.get("prerelease") else "false"
    gitee_body = (gitee_release_info.get("body") or "").replace("\r\n", "\n")
    if gitee_body != gh_body or gitee_release_info.get("name") != gh_name:
        print(f"  [{tag}] Updating release name/body on Gitee...")
        resp = requests.patch(
            f"{GITEE_API}/{GITEE_OWNER}/{GITEE_REPO}/releases/{gitee_release_info['id']}",
            data={
                "access_token": GITEE_TOKEN,
                "tag_name": tag,
                "name": gh_name,
                "body": gh_body or "-",
                "prerelease": gh_prerelease,
                "target_commitish": gh_release.get("target_commitish", "master"),
            },
        )
        if resp.status_code >= 300:
            raise Exception(f"Update release failed: {resp.status_code} {resp.text}")

    # Sync assets
    sync_release(gh_release, gitee_release_info)
    if not CORE_ONLY:
        sync_gitee_latest(gh_release, gitee_release_info)

    print("Done!")


if __name__ == "__main__":
    main()
