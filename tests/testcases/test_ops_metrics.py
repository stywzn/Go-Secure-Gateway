"""模块 6 —— 运维探针 / 可观测（含"监控断言"这一亮点）。

覆盖：健康/就绪探针、debug 后门开关、以及"发一个请求后 Prometheus 计数器
确实 +1"的监控断言 —— 把"功能对不对"和"监控采得准不准"一起测。
对应 test-points.md 模块 6（含安全点 6.7）。
"""
from __future__ import annotations

import pytest

from framework.assertions import assert_status
from framework.client import GatewayClient
from framework.config import settings
from framework.prometheus import counter_value
from framework import jwt_utils

pytestmark = pytest.mark.ops

METRIC = "gateway_http_requests_total"


def test_healthz_public(client):
    """6.1 /healthz 无需鉴权 → 200。"""
    assert_status(client.with_token(None).get("/healthz"), 200)


def test_readyz_ready(client):
    """6.2 正常运行时 /readyz → 200。"""
    assert_status(client.with_token(None).get("/readyz"), 200)


def test_metrics_exposed(client):
    """6.4 /metrics 公开可读，且含网关自定义指标。"""
    resp = client.with_token(None).get("/metrics")
    assert_status(resp, 200)
    assert METRIC in resp.text, "缺少 gateway_http_requests_total 指标"


@pytest.mark.security
def test_debug_token_backdoor_state():
    """6.7/6.8 🔒 debug 后门：debug=true 时可用；生产应关闭返回 404。

    当前被测栈 debug=true（demo 需要），故断言可用；README 说明如何在
    debug=false 环境下改判为 404，把它作为一条安全回归项。
    """
    resp = GatewayClient().with_token(None).get("/debug/token")
    assert resp.status_code == 200 and "token" in resp.json(), (
        "debug=true 时后门应返回 token；若已在生产配置下运行，请改断言为 404"
    )


def test_metrics_counter_increments():
    """6.6 监控断言：发一个成功请求后，对应 route 的成功计数器应 +1。

    读取请求前后的 gateway_http_requests_total{route="/interaction",status="200"}，
    校验增量 >= 1 —— 证明可观测数据与真实流量一致（测试即验证监控）。
    """
    probe = GatewayClient().with_token(None)
    client = GatewayClient(token=jwt_utils.valid_token())

    before = counter_value(
        probe.get("/metrics").text, METRIC, route="/interaction", status="200"
    )
    assert_status(client.get("/interaction/ping"), 200)
    after = counter_value(
        probe.get("/metrics").text, METRIC, route="/interaction", status="200"
    )

    assert after >= before + 1, (
        f"计数器未按预期增长：before={before} after={after}（监控与实际流量不一致）"
    )


def test_metric_label_uses_route_prefix_not_raw_path():
    """6.5 指标 label 用稳定的路由前缀而非原始 path（避免高基数爆炸）。"""
    probe = GatewayClient().with_token(None)
    client = GatewayClient(token=jwt_utils.valid_token())

    # 打几个不同的子路径；label 应始终归并到 route="/interaction"
    for sub in ("/interaction/a", "/interaction/b/c", "/interaction/d"):
        client.get(sub)

    text = probe.get("/metrics").text
    assert 'route="/interaction"' in text
    # 原始子路径不应作为独立 label 出现
    assert 'route="/interaction/a"' not in text, "指标基数未收敛，存在高基数风险"
