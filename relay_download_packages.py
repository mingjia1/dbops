#!/usr/bin/env python3
"""
中继服务器包下载脚本
用法: python relay_download_packages.py <agent_host> <agent_token> [mysql_version]

下载指定版本的 MySQL 安装包到中继服务器缓存。
默认下载 5.7 版本，也可以指定 5.7/8.0/8.4。
需提供 agent 的 Bearer token 进行认证。

示例:
  python relay_download_packages.py 10.1.81.21 your-token-here 5.7
  python relay_download_packages.py 10.1.81.21 your-token-here 8.0
"""

import requests
import json
import sys
import time

def prefetch_mysql_packages(host, token, mysql_version="5.7", distro="ubuntu", codename="jammy"):
    """调用 agent relay/prefetch 接口下载 MySQL 安装包到中继缓存"""
    url = f"http://{host}:9090/agent/relay/prefetch"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }
    payload = {
        "mysql_version": mysql_version,
        "distro": distro,
        "codename": codename
    }

    print(f"Requesting prefetch: MySQL {mysql_version} on {distro} {codename}")
    print(f"URL: {url}")
    print(f"Headers: Authorization: Bearer **** ({len(token)} chars)")
    print(f"Payload: {json.dumps(payload)}")
    print()

    try:
        start = time.time()
        r = requests.post(url, json=payload, headers=headers, timeout=600)
        elapsed = time.time() - start
        print(f"Response time: {elapsed:.1f}s")
        print(f"Status: {r.status_code}")
        
        if r.status_code == 200:
            data = r.json()
            print(json.dumps(data, indent=2, ensure_ascii=False))
            
            if data.get("code") == 200:
                results = data.get("data", [])
                success = sum(1 for res in results if res.get("status") == "success")
                failed = sum(1 for res in results if res.get("status") == "failed")
                print(f"\n结果: {success} 成功, {failed} 失败")
                return results
        elif r.status_code == 401:
            print("错误: 认证失败 (401) — 请检查 agent_token")
            print(f"响应: {r.text[:200]}")
        else:
            print(f"错误: HTTP {r.status_code}")
            print(f"响应: {r.text[:500]}")
    except requests.exceptions.Timeout:
        print("错误: 请求超时 (600s)")
    except requests.exceptions.ConnectionError as e:
        print(f"错误: 连接失败 — {e}")
        print(f"请确认 {host}:9090 可达且 agent 正在运行")
    except Exception as e:
        print(f"错误: {e}")

    return None


def check_relay_status(host, token):
    """查看中继服务器状态"""
    url = f"http://{host}:9090/agent/relay/status"
    headers = {"Authorization": f"Bearer {token}"}

    try:
        r = requests.get(url, headers=headers, timeout=10)
        if r.status_code == 200:
            data = r.json()
            print(json.dumps(data, indent=2, ensure_ascii=False))
            return data.get("data")
        else:
            print(f"Relay status error: HTTP {r.status_code} - {r.text[:200]}")
    except Exception as e:
        print(f"Relay status error: {e}")
    return None


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)

    host = sys.argv[1]
    token = sys.argv[2]
    version = sys.argv[3] if len(sys.argv) > 3 else "5.7"

    print(f"=== 中继包下载工具 ===")
    print(f"目标: {host}:9090")
    print(f"版本: MySQL {version}")
    print()

    # 先查看当前中继状态
    print("--- 当前中继状态 ---")
    status = check_relay_status(host, token)
    if status:
        print(f"包数量: {status.get('package_count', 0)}")
        print(f"缓存目录: {status.get('cache_dir', '')}")
        print(f"已用空间: {status.get('total_size_mb', 0)} MB")
    print()

    # 下载指定版本的包
    print(f"--- 下载 MySQL {version} 安装包 ---")
    results = prefetch_mysql_packages(host, token, mysql_version=version)

    if results:
        print(f"\n完成! 查看中继状态确认:")
        print(f"  python {sys.argv[0]} {host} {token}")
