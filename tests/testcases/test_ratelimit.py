"""模块 2 —— 限流（按 IP 令牌桶）。

限流键是 c.ClientIP()，网关默认信任 X-Forwarded-For。框架给每个 client 独立
源 IP，因此：
  - 各用例限流桶天然隔离 → 可与其它模块并行（无需串行）。
  - 顺带验证 test-points 2.4：不同 IP 互不影响。

参数（config.docker.yaml）：rps=50, burst=100。
"""
from __future__ import annotations

import time

import pytest

from framework.client import GatewayClient
from framework.config import settings
from framework import jwt_utils

pytestmark = pytest.mark.ratelimit

ROUTE = "/interaction/ping"


def _burst(client: GatewayClient, n: int) -> list[int]:
    """尽量快地连发 n 次，返回状态码序列。"""
    return [client.get(ROUTE).status_code for _ in range(n)]


def test_within_burst_all_pass():
    """2.1 突发在 burst 容量内 → 首个请求必过（令牌桶初始为满）。"""
    client = GatewayClient(token=jwt_utils.valid_token())
    assert client.get(ROUTE).status_code == 200


def test_exceeding_burst_triggers_429():
    """2.2 瞬间远超 burst → 出现 429，且首个请求仍是 200。"""
    client = GatewayClient(token=jwt_utils.valid_token())
    codes = _burst(client, settings.rate_burst * 3)  # 300 次
    assert codes[0] == 200, "令牌桶初始应为满，首个请求必过"
    assert 429 in codes, f"未观测到限流 429；状态分布={_dist(codes)}"


def test_recovers_after_wait():
    """2.3 触发限流后等待令牌恢复 → 重新放行。"""
    client = GatewayClient(token=jwt_utils.valid_token())
    _burst(client, settings.rate_burst * 3)  # 打到限流
    time.sleep(1.0)  # rps=50，1s 恢复约 50 个令牌
    assert client.get(ROUTE).status_code == 200, "等待后应恢复放行"


def test_different_ips_are_independent():
    """2.4 不同 IP 各自独立计数：打爆 A 不影响 B。"""
    ip_a = GatewayClient(source_ip="203.0.113.10", token=jwt_utils.valid_token())
    ip_b = GatewayClient(source_ip="203.0.113.20", token=jwt_utils.valid_token())

    _burst(ip_a, settings.rate_burst * 3)  # 打爆 A
    assert ip_b.get(ROUTE).status_code == 200, "另一 IP 不应被 A 的限流波及"


@pytest.mark.security
def test_rate_limit_runs_before_auth():
    """2.6 🔒 中间件顺序：限流在鉴权之前。

    over-limit 且**不带 token**的请求应返回 429（限流先命中），而非 401。
    证明 DoS 防护在鉴权前生效——攻击者无需有效 token 也会被限流拦下。
    """
    client = GatewayClient()  # 不带 token
    codes = _burst(client.with_token(None), settings.rate_burst * 3)
    assert 429 in codes, f"未观测到 429；分布={_dist(codes)}"
    # 顺序正确的话，超限那部分是 429 而不是 401
    assert codes.count(429) > 0


def _dist(codes: list[int]) -> dict:
    from collections import Counter
    return dict(Counter(codes))
