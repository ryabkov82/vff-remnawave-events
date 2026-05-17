#!/usr/bin/env python3
import argparse
import datetime as dt
import hashlib
import hmac
import json
import pathlib
import subprocess
import sys
import urllib.request


def utc_now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def main() -> int:
    parser = argparse.ArgumentParser(description="Send a signed Remnawave test webhook.")
    parser.add_argument("--url", default="http://127.0.0.1:8080/remnawave")
    parser.add_argument("--secret", default="change-me")
    parser.add_argument("--payload", default="testdata/torrent_blocker_report.json")
    parser.add_argument("--use-curl", action="store_true", help="Print and execute an equivalent curl command")
    args = parser.parse_args()

    payload_path = pathlib.Path(args.payload)
    payload = json.loads(payload_path.read_text(encoding="utf-8"))

    now = utc_now_iso()
    payload["timestamp"] = now
    payload["data"]["report"]["actionReport"]["processedAt"] = now

    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    signature = hmac.new(args.secret.encode("utf-8"), body, hashlib.sha256).hexdigest()

    if args.use_curl:
        curl_cmd = [
            "curl", "-sS", "-i", "-X", "POST", args.url,
            "-H", "Content-Type: application/json",
            "-H", f"X-Remnawave-Signature: {signature}",
            "-H", f"X-Remnawave-Timestamp: {now}",
            "--data-binary", "@-",
        ]
        print(" ".join(curl_cmd))
        proc = subprocess.run(curl_cmd, input=body)
        return proc.returncode

    req = urllib.request.Request(
        args.url,
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "X-Remnawave-Signature": signature,
            "X-Remnawave-Timestamp": now,
            "User-Agent": "Remnawave-test",
        },
    )

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            print(f"HTTP {resp.status}")
            print(resp.read().decode("utf-8"))
            return 0
    except urllib.error.HTTPError as exc:
        print(f"HTTP {exc.code}", file=sys.stderr)
        print(exc.read().decode("utf-8"), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
