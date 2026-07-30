"""模块 5 —— 熔断器状态机（本框架最具"故障注入"含金量的部分）。

熔断器是 **按路由的服务端共享状态**（见 proxy.go / breaker.go），与客户端 IP 无关：
  closed --(连续 5 次 5xx)--> open --(冷却 10s)--> half-open
  half-open --(探测成功)--> closed ；half-open --(探测失败)--> open

因此这些用例：
  1) 必须**串行且隔离**运行（不能和其它打同一路由的用例并行，否则互相干扰）；
     → 用 breaker 标记，CI 里单独一阶段串行跑。
  2) 独占 /interaction 路由，并在每个用例前后把熔断器复位到 closed，保证幂等。

判定技巧：熔断打开时网关直接短路返回 503，**请求不会到达后端**，因此响应头
里没有 X-Served-By；而后端强制的 5xx 一定带 X-Served-By。以此区分"网关熔断"
与"后端错误"。
"""
from __future__ import annotations

import time

import pytest

from framework import jwt_utils
from framework.assertions import assert_status
from framework.client import GatewayClient
from framework.config import settings

pytestmark = [pytest.mark.breaker, pytest.mark.slow]

# 用 /compute 路由做熔断测试：它是配置里专门留给故障注入的单后端路由，
# 与 CRUD(/data)、负载均衡(/storage)、鉴权(/interaction)分开，
# 打开熔断也不会波及其它模块的用例。
ROUTE = "/compute/run"


def _trip_open(client: GatewayClient) -> None:
    """连续打满阈值的 5xx，把熔断器打开。"""
    for _ in range(settings.breaker_threshold):
        client.get(ROUTE, params={"status": 503})


def _is_short_circuited(resp) -> bool:
    """网关熔断短路：503 且没有 X-Served-By（未到达后端）。"""
    return resp.status_code == 503 and "X-Served-By" not in resp.headers


@pytest.fixture(autouse=True)
def _reset_breaker():
    """用例前后把熔断器恢复到 closed，避免相互污染。"""
    yield
    # 冷却 → 半开 → 用一次成功请求关闭熔断
    time.sleep(settings.breaker_cooldown_s + 1)
    GatewayClient(token=jwt_utils.valid_token()).get(ROUTE)


@pytest.fixture
def bc():
    return GatewayClient(token=jwt_utils.valid_token())


def test_4xx_does_not_trip_breaker(bc):
    """5.7 只有 5xx 触发熔断；连续 4xx 不应打开熔断。"""
    for _ in range(settings.breaker_threshold + 2):
        resp = bc.get(ROUTE, params={"status": 404})
        assert resp.status_code == 404
        assert "X-Served-By" in resp.headers  # 始终到达后端，未被短路


def test_breaker_opens_then_recovers(bc):
    """5.1~5.5 完整状态机：closed → open → half-open(探测成功) → closed。"""
    # 1) closed：正常请求成功
    assert_status(bc.get(ROUTE), 200)

    # 2) 连续 5 次 5xx → 打开
    _trip_open(bc)

    # 3) open：后续请求被网关直接短路 503（未到后端）
    resp = bc.get(ROUTE)
    assert _is_short_circuited(resp), (
        f"熔断未打开：status={resp.status_code} served-by={resp.headers.get('X-Served-By')}"
    )

    # 4) 冷却后进入 half-open，放行一个探测；让探测成功（正常请求）→ 关闭
    time.sleep(settings.breaker_cooldown_s + 1)
    probe = bc.get(ROUTE)
    assert_status(probe, 200)
    assert "X-Served-By" in probe.headers  # 探测确实打到了后端

    # 5) 已恢复 closed：正常流量放行
    assert_status(bc.get(ROUTE), 200)


def test_half_open_probe_failure_reopens(bc):
    """5.6 半开探测失败（后端仍 5xx）→ 立即重新打开。"""
    _trip_open(bc)
    assert _is_short_circuited(bc.get(ROUTE))

    time.sleep(settings.breaker_cooldown_s + 1)
    # 半开放行的这一个探测让它失败
    failed_probe = bc.get(ROUTE, params={"status": 503})
    assert failed_probe.status_code in (503,)  # 探测本身返回后端 503
    # 探测失败 → 立即重新打开：下一个请求又被短路
    assert _is_short_circuited(bc.get(ROUTE))
