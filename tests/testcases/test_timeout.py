"""超时 / 延迟注入 —— 故障注入的另一面，并记录一个真实风险点。

用后端可控参数 ?delay=<dur> 注入延迟：
  - 中等延迟：网关如实等待并透传（验证延迟注入生效、连接不被过早切断）。
  - 超长延迟（opt-in，slow）：暴露"网关对上游调用没有独立超时"这一设计风险
    —— 慢后端会一直拖到网关 WriteTimeout(15s) 才断开（test-points 记录的风险 1）。

后一条不是"功能 bug"，而是**有依据的风险点**：面试里"我测出并评估了这个风险、
给了改进建议（给上游加 per-request timeout）"比"功能都正常"更有分量。
"""
from __future__ import annotations

import time

import pytest

from framework.assertions import assert_status
from framework.config import settings

pytestmark = pytest.mark.routing


def test_moderate_delay_is_forwarded(auth_client):
    """注入 2s 延迟：仍返回 200，且端到端耗时 >= 2s（证明延迟真实生效）。"""
    start = time.monotonic()
    resp = auth_client.get("/compute/run", params={"delay": "2s"})
    elapsed = time.monotonic() - start

    assert_status(resp, 200)
    assert elapsed >= 2.0, f"延迟未生效？耗时仅 {elapsed:.2f}s"


@pytest.mark.slow
def test_upstream_has_no_independent_timeout(auth_client):
    """记录风险：上游无独立超时，慢后端会拖到网关 WriteTimeout 才断。

    delay 设为略大于 WriteTimeout(15s)。当前实现下网关不会主动对上游超时，
    连接会在 ~15s 左右被 WriteTimeout 切断（客户端侧表现为读到不完整响应/连接错误）。
    用 xfail 记录"已知风险"，避免它被误当成回归失败，同时在报告里可见。
    """
    delay = int(settings.http_timeout)  # 略大于网关 15s WriteTimeout 的场景
    try:
        resp = auth_client.get("/compute/run", params={"delay": f"{delay}s"})
    except Exception as e:  # noqa: BLE001 —— 连接被切断也是"命中风险"的一种表现
        pytest.xfail(f"上游无独立超时：连接在网关 WriteTimeout 处被切断（{type(e).__name__}）")
    else:
        # 若返回了，说明是被网关的 WriteTimeout 兜底切断（非上游级精细超时）
        if resp.status_code >= 500:
            pytest.xfail("上游无独立超时：仅由服务级 WriteTimeout 兜底")
        assert_status(resp, 200)
