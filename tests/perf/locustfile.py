"""Locust 压测脚本（k6 的 Python 替代，便于本地快速看曲线）。

运行：
    TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
    locust -f tests/perf/locustfile.py --host http://localhost:8080 \
           --headless -u 100 -r 50 -t 30s

关注指标：
    - RPS / 失败率（其中 429 是"预期内的限流"，非故障）
    - p95 / p99 延迟
用不同源 IP 可对比"单 IP 限流"与"多 IP 独立计数"两种画像。
"""
from __future__ import annotations

import os

from locust import HttpUser, task, between

TOKEN = os.environ.get("TOKEN", "")


class GatewayUser(HttpUser):
    wait_time = between(0, 0.05)  # 高频，制造限流压力

    @task
    def ping(self):
        with self.client.get(
            "/interaction/ping",
            headers={
                "Authorization": f"Bearer {TOKEN}",
                "X-Forwarded-For": "198.51.100.7",  # 共用一个限流桶
            },
            name="/interaction/ping",
            catch_response=True,
        ) as resp:
            # 把 429 标记为"预期内"，不计入失败率——限流生效是正确行为。
            if resp.status_code in (200, 429):
                resp.success()
            else:
                resp.failure(f"unexpected status {resp.status_code}")
