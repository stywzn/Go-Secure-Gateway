---
name: ensure-test-tools
description: Check whether the Allure CLI and k6 load-tester are available for the e2e-py test framework, and install whichever is missing using China-friendly sources (Aliyun Maven mirror for Allure, Docker image for k6). Use whenever the user asks to check, verify, fix, or install allure or k6 — e.g. "看看 allure/k6 装好没", "把没装成功的装上", "ensure test tools".
---

# Ensure test tools: Allure + k6

The user is on Windows with a slow/unreliable connection to overseas hosts (GitHub, npm's binary CDN, proxy.golang.org). Java 21 and npm are installed. Docker works **with a registry mirror**. Prefer the China-friendly install paths below; do NOT use `npm install -g allure-commandline` (its post-install fetches from GitHub and hangs) or `winget install k6.k6` (wrong id, fails exit 20).

Run the checks first, then only install what is missing. Report a short status table at the end.

## 1. Allure CLI

**Check:** run `allure --version`. If it prints a version, it's done — skip install.

Also check if a previous download already extracted it:
`& "$env:USERPROFILE\.allure\allure-2.29.0\bin\allure.bat" --version` (if this works, just ensure PATH — see below).

**Install (only if missing)** — download the zip from Aliyun Maven (works from China), extract, add to the user PATH. Run in PowerShell:

```powershell
$dir = "$env:USERPROFILE\.allure"
New-Item -ItemType Directory -Force $dir | Out-Null
curl.exe -L -o "$dir\allure.zip" "https://maven.aliyun.com/repository/public/io/qameta/allure/allure-commandline/2.29.0/allure-commandline-2.29.0.zip"
Expand-Archive -Force "$dir\allure.zip" $dir
$bin = "$dir\allure-2.29.0\bin"
$p = [Environment]::GetEnvironmentVariable("PATH","User")
if ($p -notlike "*$bin*") { [Environment]::SetEnvironmentVariable("PATH", "$p;$bin", "User") }
& "$bin\allure.bat" --version
```

Notes:
- The zip is ~37 MB. If the connection is crawling (<1 MB/min), stop and tell the user to retry when the network is better — do not keep retrying. The Aliyun URL is stable.
- The download can be run in the background so it doesn't block; check the output file size against 37 MB to gauge progress.
- After editing the user PATH, a **new terminal** is required for the bare `allure` command; the just-installed session can use the full `...\bin\allure.bat` path.
- Fallback versions on the same mirror if 2.29.0 is unavailable: `2.27.0`, `2.24.1` (same URL pattern).

## 2. k6

**Check:** run `k6 version`. If it prints a version, it's done — skip install.

**Preferred: no install — run k6 via Docker** (Docker already works with the registry mirror). Verify the image pulls:

```bash
docker run --rm grafana/k6 version
```

If that works, k6 is "available" via Docker. To run a script, the invocation is:

```bash
docker run --rm -i --add-host=host.docker.internal:host-gateway grafana/k6 run - < e2e-py/perf/ratelimit.js
```

(Inside the container the gateway is reached at `http://host.docker.internal:8080`, not `127.0.0.1`.)

**Fallback: native install** — the correct winget id is **`GrafanaLabs.k6`** (verified via `winget search k6`; NOT `k6.k6`, which fails exit 20):
`winget install --id GrafanaLabs.k6 -e --accept-source-agreements --accept-package-agreements`
Or `choco install k6`, or download the Windows zip from k6 GitHub releases and add to PATH (may be slow from China).

## 3. Report

Finish with a short table:

| tool | status | how |
|------|--------|-----|
| allure | installed / just installed / FAILED (retry later) | aliyun maven zip |
| k6 | available via docker / installed / FAILED | docker grafana/k6 |

If anything failed due to network, say so plainly and tell the user it's safe to retry later — the tools are not needed to *write* tests, only to render the Allure report / run the k6 load test, and both have no-local-install fallbacks (CI renders Allure; Docker runs k6).
