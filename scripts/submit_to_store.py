#!/usr/bin/env python3
"""
Microsoft Store Submission API - 自动提交 MSIX 包到商店
用法: python3 submit_to_store.py <msix_file_path>

需要环境变量:
  AZURE_TENANT_ID   - Partner Center 中的 Tenant ID
  AZURE_CLIENT_ID    - Partner Center 中创建的 Azure AD 应用 Client ID
  AZURE_CLIENT_SECRET- Partner Center 中生成的 Key

流程:
  1. 获取 Azure AD access token
  2. 获取应用信息 (确认 app ID)
  3. 创建新 submission (clone 上一次的)
  4. 上传 MSIX 包到 Azure blob
  5. 更新 submission (替换 package)
  6. 提交 (commit)
"""

import os
import sys
import json
import time
import requests
from urllib.parse import quote

# === 配置 ===
STORE_ID = "9N0LNL3JM3GG"  # bili-FM 的 Store ID
RESOURCE = "https://manage.devcenter.microsoft.com"
API_BASE = "https://manage.devcenter.microsoft.com/v1.0/my/applications"

def get_env_or_die(name):
    val = os.environ.get(name, "").strip()
    if not val:
        print(f"ERROR: 环境变量 {name} 未设置")
        print("请在 Partner Center → Account settings → Users → Azure AD application 中获取")
        sys.exit(1)
    return val

def get_access_token(tenant_id, client_id, client_secret):
    """Step 1: 获取 Azure AD access token"""
    url = f"https://login.microsoftonline.com/{tenant_id}/oauth2/token"
    data = {
        "grant_type": "client_credentials",
        "client_id": client_id,
        "client_secret": client_secret,
        "resource": RESOURCE,
    }
    print("▶ 获取 access token...")
    resp = requests.post(url, data=data, timeout=30)
    resp.raise_for_status()
    token = resp.json()["access_token"]
    print(f"  ✓ Token 获取成功 (前20字符: {token[:20]}...)")
    return token

def get_headers(token):
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }

def get_app_info(token, store_id):
    """Step 2: 获取应用信息"""
    url = f"{API_BASE}/{store_id}"
    print(f"▶ 获取应用信息 ({store_id})...")
    resp = requests.get(url, headers=get_headers(token), timeout=30)
    if resp.status_code == 404:
        print(f"  ✗ 应用 {store_id} 未找到，请检查 Store ID")
        sys.exit(1)
    resp.raise_for_status()
    app = resp.json()
    print(f"  ✓ 应用名称: {app.get('primaryName', 'N/A')}")
    print(f"  ✓ 应用 ID: {app.get('id', 'N/A')}")
    return app

def create_submission(token, store_id):
    """Step 3: 创建新 submission (clone 上一次的配置)"""
    url = f"{API_BASE}/{store_id}/submissions"
    print("▶ 创建新 submission...")
    resp = requests.post(url, headers=get_headers(token), timeout=30)
    if resp.status_code == 409:
        print("  ✗ 已有 pending submission，需先删除或提交")
        print(f"  详情: {resp.text}")
        sys.exit(1)
    resp.raise_for_status()
    sub = resp.json()
    sub_id = sub["id"]
    upload_url = sub["fileUploadUrl"]
    print(f"  ✓ Submission ID: {sub_id}")
    print(f"  ✓ Upload URL: {upload_url[:80]}...")
    return sub_id, upload_url

def upload_package(msix_path, upload_url):
    """Step 4: 上传 MSIX 到 Azure blob"""
    print(f"▶ 上传 MSIX 包 ({os.path.getsize(msix_path)} bytes)...")
    with open(msix_path, "rb") as f:
        resp = requests.put(upload_url, data=f, headers={
            "x-ms-blob-type": "BlockBlob",
            "Content-Type": "application/octet-stream",
        }, timeout=300)
    resp.raise_for_status()
    print("  ✓ 上传完成")

def update_submission(token, store_id, sub_id, msix_filename):
    """Step 5: 更新 submission - 替换 package，保留其他配置"""
    url = f"{API_BASE}/{store_id}/submissions/{sub_id}"
    print("▶ 获取当前 submission 配置...")
    resp = requests.get(url, headers=get_headers(token), timeout=30)
    resp.raise_for_status()
    sub = resp.json()

    # 替换 package 信息
    if "applicationPackages" in sub and sub["applicationPackages"]:
        old_pkg = sub["applicationPackages"][0]
        sub["applicationPackages"] = [{
            "fileName": msix_filename,
            "fileStatus": "PendingUpload",
            "minimumDirectXVersion": old_pkg.get("minimumDirectXVersion", "None"),
            "minimumSystemRam": old_pkg.get("minimumSystemRam", "None"),
        }]
        print(f"  ✓ 替换包: {old_pkg.get('fileName', 'N/A')} → {msix_filename}")
    else:
        sub["applicationPackages"] = [{
            "fileName": msix_filename,
            "fileStatus": "PendingUpload",
            "minimumDirectXVersion": "None",
            "minimumSystemRam": "None",
        }]
        print(f"  ✓ 新增包: {msix_filename}")

    # 确保保留原有 listings 等信息（只改 package）
    print("▶ 更新 submission...")
    resp = requests.put(url, json=sub, headers=get_headers(token), timeout=30)
    resp.raise_for_status()
    print("  ✓ Submission 更新成功")

def commit_submission(token, store_id, sub_id):
    """Step 6: 提交 (commit)"""
    url = f"{API_BASE}/{store_id}/submissions/{sub_id}/commit"
    print("▶ 提交 submission...")
    resp = requests.post(url, headers=get_headers(token), timeout=30)
    resp.raise_for_status()
    result = resp.json()
    print(f"  ✓ 提交成功! 状态: {result.get('status', 'N/A')}")
    return result

def check_submission_status(token, store_id, sub_id):
    """检查提交状态"""
    url = f"{API_BASE}/{store_id}/submissions/{sub_id}"
    resp = requests.get(url, headers=get_headers(token), timeout=30)
    resp.raise_for_status()
    return resp.json()

def main():
    msix_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/bili-fm-store-pkg/bili-FM-windows-amd64.msix"

    if not os.path.exists(msix_path):
        print(f"ERROR: MSIX 文件不存在: {msix_path}")
        sys.exit(1)

    msix_filename = os.path.basename(msix_path)
    print(f"=== Microsoft Store 自动提交 ===")
    print(f"应用 Store ID: {STORE_ID}")
    print(f"MSIX 文件: {msix_path} ({os.path.getsize(msix_path)} bytes)")
    print()

    # 获取凭据
    tenant_id = get_env_or_die("AZURE_TENANT_ID")
    client_id = get_env_or_die("AZURE_CLIENT_ID")
    client_secret = get_env_or_die("AZURE_CLIENT_SECRET")

    # Step 1: 获取 token
    token = get_access_token(tenant_id, client_id, client_secret)

    # Step 2: 获取应用信息
    get_app_info(token, STORE_ID)

    # Step 3: 创建 submission
    sub_id, upload_url = create_submission(token, STORE_ID)

    try:
        # Step 4: 上传包
        upload_package(msix_path, upload_url)

        # Step 5: 更新 submission
        update_submission(token, STORE_ID, sub_id, msix_filename)

        # Step 6: 提交
        result = commit_submission(token, STORE_ID, sub_id)

        print()
        print("=== 提交完成 ===")
        print(f"Submission ID: {sub_id}")
        print(f"状态: {result.get('status', 'N/A')}")
        print(f"目标: {result.get('targetPublishMode', 'N/A')}")
        print()
        print("提交后微软会进行认证审核，通常需要几小时到几天。")
        print(f"可在 Partner Center 查看进度:")
        print(f"https://partner.microsoft.com/dashboard/products/{STORE_ID}/submissions")

    except Exception as e:
        print(f"\n✗ 提交过程中出错: {e}")
        print(f"Submission ID: {sub_id} (可能需要在 Partner Center 手动处理)")
        raise

if __name__ == "__main__":
    main()
