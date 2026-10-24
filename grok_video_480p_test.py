#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""grok-imagine-video 480p 视频生成测试（走 OpenAI /v1/videos 协议，不用 Grok 原生路径）

创建 -> 轮询 -> 下载，全流程串起来。纯标准库，无第三方依赖。

用法:
    python grok_video_480p_test.py
    python grok_video_480p_test.py --prompt "一只猫在海边奔跑" --seconds 6 --aspect-ratio 16:9
    python grok_video_480p_test.py --base-url http://HOST:PORT --key sk-xxx --model grok-imagine-video-1.5

环境变量覆盖: VIDEO_BASE_URL / VIDEO_API_KEY / VIDEO_MODEL
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

DEFAULT_BASE_URL = "http://43.248.3.158:20039"
DEFAULT_API_KEY = "sk-aawqQxcrfmNAnLYJMi1C2LVTVcY2QDSkypfqMDksWbSoiNkj"
DEFAULT_MODEL = "grok-imagine-video"

# 上游对分辨率枚举大小写敏感: 只认小写 "480p" / "720p" / "1080p"。
# 传 "480P" 或 "854x480" 会被网关直接拒绝: plugin_usage_invalid。
RESOLUTION = "480p"

TERMINAL = {"completed", "failed", "cancelled"}


def request(method, url, key, payload=None, timeout=60):
    """返回 (status, body)。body 是 dict；非 JSON 时是 {'raw': ...}。"""
    data = None
    headers = {"Authorization": "Bearer " + key}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            try:
                return resp.status, json.loads(raw.decode("utf-8") or "{}")
            except ValueError:
                return resp.status, {"raw": raw.decode("utf-8", "replace")}
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        try:
            return exc.code, json.loads(raw.decode("utf-8") or "{}")
        except ValueError:
            return exc.code, {"raw": raw.decode("utf-8", "replace")}


def explain(status, body):
    """把网关/上游的报错翻译成人话，返回诊断行列表。"""
    lines = []
    err = body.get("error") if isinstance(body.get("error"), dict) else {}
    message = body.get("message") or err.get("message") or ""
    message = message or json.dumps(body, ensure_ascii=False)

    # 上游错误常被网关原样塞进 message 字符串里，再解一层
    inner = None
    if isinstance(message, str) and message.strip().startswith("{"):
        try:
            inner = json.loads(message)
        except ValueError:
            inner = None

    code = body.get("code") or (inner or {}).get("code") or ""
    inner_msg = (inner or {}).get("message", "")

    if code == "API_KEY_REQUIRED" or "API_KEY_REQUIRED" in str(message):
        lines.append("请求已通过 new-api 认证并提交到上游，是站点的 grok 渠道没配 xAI 密钥。")
        lines.append("跟你这个 sk- 令牌无关：同一个令牌打 /v1/chat/completions 是正常的。")
        lines.append("要出片得让站方在「渠道」里给 grok 渠道填上可用的 xAI API key。")
    elif code == "plugin_usage_invalid":
        lines.append("参数枚举被网关拒绝：resolution 只认小写 480p / 720p / 1080p，")
        lines.append("不接受 480P、854x480、640x480 这类写法。")
    elif status == 401:
        lines.append("网关认证失败：令牌无效、被禁用或额度耗尽。")
    elif status == 400 and "Invalid task protocol request" in str(message):
        lines.append("请求体没通过插件的 decodeRequest 校验（缺 model/prompt，或字段类型不对）。")
    elif "resolution must be one of" in str(message):
        lines.append("分辨率不合法：只支持 480p / 720p / 1080p。")
    elif code:
        lines.append("错误码: %s" % code)
    if inner_msg and inner_msg not in str(message):
        lines.append("上游原文: %s" % inner_msg)
    return lines


def build_payload(model, prompt, seconds, aspect_ratio, resolution):
    return {
        "model": model,
        "prompt": prompt,
        "seconds": seconds,
        "aspect_ratio": aspect_ratio,
        "resolution": resolution,
    }


def create_task(base, key, payload, timeout):
    url = base + "/v1/videos"
    print("[1/3] create task POST %s" % url)
    print("      body: %s" % json.dumps(payload, ensure_ascii=False))
    status, body = request("POST", url, key, payload, timeout)
    print("      HTTP %d -> %s" % (status, json.dumps(body, ensure_ascii=False)[:400]))
    return status, body


def poll_task(base, key, task_id, interval, max_wait):
    url = "%s/v1/videos/%s" % (base, task_id)
    print("[2/3] poll task GET %s" % url)
    started = time.time()
    last = None
    while time.time() - started < max_wait:
        status, body = request("GET", url, key, None, 60)
        if status != 200:
            print("      HTTP %d -> %s" % (status, json.dumps(body, ensure_ascii=False)[:300]))
            return status, body
        state = body.get("status")
        progress = body.get("progress")
        line = "      status=%s progress=%s" % (state, progress)
        if line != last:
            print(line + "  (%.0fs)" % (time.time() - started))
            last = line
        if state in TERMINAL:
            return 200, body
        time.sleep(interval)
    print("      timeout: not finished within %.0fs" % max_wait)
    return 408, {"code": "client_timeout", "message": "polling timed out"}


def download_content(base, key, task_id, out_path):
    url = "%s/v1/videos/%s/content" % (base, task_id)
    print("[3/3] download video GET %s" % url)
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + key}, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=300) as resp:
            blob = resp.read()
            ctype = resp.headers.get("Content-Type", "")
    except urllib.error.HTTPError as exc:
        print("      HTTP %d -> %s" % (exc.code, exc.read().decode("utf-8", "replace")[:300]))
        return False
    if not blob:
        print("      empty response, no video bytes")
        return False
    with open(out_path, "wb") as fh:
        fh.write(blob)
    print("      saved %s (%d bytes, Content-Type=%s)" % (out_path, len(blob), ctype))
    return True


def main():
    parser = argparse.ArgumentParser(description="grok-imagine-video 480p 生成测试 (OpenAI /v1/videos)")
    parser.add_argument("--base-url", default=os.environ.get("VIDEO_BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--key", default=os.environ.get("VIDEO_API_KEY", DEFAULT_API_KEY))
    parser.add_argument("--model", default=os.environ.get("VIDEO_MODEL", DEFAULT_MODEL))
    parser.add_argument("--prompt", default="A cat walking on a beach at sunset, cinematic, slow camera push-in")
    parser.add_argument("--seconds", type=int, default=6)
    parser.add_argument("--aspect-ratio", default="16:9")
    parser.add_argument("--resolution", default=RESOLUTION, help="只认小写 480p / 720p / 1080p")
    parser.add_argument("--interval", type=float, default=5.0, help="轮询间隔秒")
    parser.add_argument("--max-wait", type=float, default=300.0, help="最长等待秒")
    parser.add_argument("--out", default="grok_video_480p.mp4")
    parser.add_argument("--probe-only", action="store_true", help="只做连通性/参数自检，不轮询不下载")
    args = parser.parse_args()

    base = args.base_url.rstrip("/")
    print("target: %s   model: %s   resolution: %s" % (base, args.model, args.resolution))
    print("-" * 68)

    payload = build_payload(args.model, args.prompt, args.seconds, args.aspect_ratio, args.resolution)
    status, body = create_task(base, args.key, payload, 90)

    if status != 200:
        print("-" * 68)
        print("create failed, diagnosis:")
        for line in explain(status, body):
            print("  - " + line)
        return 1

    task_id = body.get("id") or body.get("task_id")
    if not task_id:
        print("no task id in response, raw: %s" % json.dumps(body, ensure_ascii=False))
        return 1
    print("      task id: %s" % task_id)

    if args.probe_only:
        print("-" * 68)
        print("probe ok: request accepted by gateway, task created")
        return 0

    status, body = poll_task(base, args.key, task_id, args.interval, args.max_wait)
    if status != 200:
        print("-" * 68)
        print("poll failed, diagnosis:")
        for line in explain(status, body):
            print("  - " + line)
        return 1

    if body.get("status") != "completed":
        print("-" * 68)
        print("task not successful: status=%s error=%s" % (body.get("status"), json.dumps(body.get("error"), ensure_ascii=False)))
        for line in explain(status, body):
            print("  - " + line)
        return 1

    print("      completed: %s" % json.dumps(body, ensure_ascii=False)[:400])
    ok = download_content(base, args.key, task_id, args.out)
    print("-" * 68)
    print("result: %s" % ("ok, saved to " + os.path.abspath(args.out) if ok else "task completed but content download failed"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
