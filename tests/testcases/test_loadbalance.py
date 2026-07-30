"""模块 4 —— 负载均衡（RoundRobin）。

/storage 路由配了两个副本（storage-a / storage-b）。连续请求应在两者间
均匀轮转，可通过响应头 X-Served-By 观测。对应 test-points.md 模块 4。
"""
from __future__ import annotations

import pytest

from framework.assertions import assert_round_robin, served_by
from framework.config import settings

pytestmark = pytest.mark.loadbalance


def test_round_robin_distribution(auth_client):
    """4.1 连发 N 次（副本数的整数倍）→ 两副本均匀轮转，无遗漏。

    注意：所有请求走同一个 client（同一源 IP），保证是同一个轮询计数器序列。
    """
    n = len(settings.storage_backends) * 10  # 20 次
    served = [served_by(auth_client.get("/storage/ping")) for _ in range(n)]
    assert_round_robin(served, settings.storage_backends)


def test_both_replicas_hit(auth_client):
    """4.2 请求数不是整数倍时，两副本仍都被命中（不偏斜到一台）。"""
    served = {served_by(auth_client.get("/storage/ping")) for _ in range(7)}
    assert set(settings.storage_backends).issubset(served), (
        f"存在副本从未被命中：{set(settings.storage_backends) - served}"
    )


def test_single_backend_route_is_sticky(auth_client):
    """4.4 单后端路由（/compute）始终命中同一后端。"""
    served = {served_by(auth_client.get("/compute/run")) for _ in range(5)}
    assert served == {"compute"}, f"单后端路由不应出现多个后端：{served}"
