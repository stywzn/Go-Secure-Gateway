"""分布式全局限流(模块7,Ultra)。

被测:docker compose --profile distributed 起两个网关副本(:8082/:8083),
它们通过 Redis 共享一个全局限流计数。验证核心:
  - 单 IP 跨两副本猛打,放行数受【全局】阈值约束,而不是每副本各放一份(~2×);
  - 在一个副本上耗尽配额,另一个副本立刻也拒(证明状态是共享的)。

前置:先跑 `docker compose --profile distributed up -d --build`。
默认不随常规套件跑(需 `-m distributed`),不影响 e2e-py / 其它模块。
"""
from __future__ import annotations

import itertools
import os

import pytest
import requests

from framework import jwt_utils

# 两个副本地址(可用环境变量覆盖);全局阈值与 config.distributed.yaml 的 burst 对齐。
REPLICAS = os.environ.get(
    "GATEWAY_REPLICAS", "http://127.0.0.1:8082,http://127.0.0.1:8083"
).split(",")
GLOBAL_LIMIT = int(os.environ.get("GATEWAY_GLOBAL_LIMIT", "20"))

pytestmark = pytest.mark.distributed


def _headers(ip: str) -> dict:
    return {"Authorization": f"Bearer {jwt_utils.valid_token()}", "X-Forwarded-For": ip}


def test_global_limit_holds_across_replicas():
    """单 IP 跨两副本猛打 → 放行数 ≈ 全局阈值,远小于每实例独立时的 ~2×。"""
    headers = _headers("10.222.222.222")  # 固定同一 IP → 命中同一个 Redis key
    allowed = 0
    rr = itertools.cycle(REPLICAS)  # 轮流打两个副本
    for _ in range(100):
        resp = requests.get(f"{next(rr)}/interaction/ping", headers=headers, timeout=5)
        if resp.status_code == 200:
            allowed += 1

    assert allowed >= 1, "限流不应把请求全拒"
    assert allowed <= GLOBAL_LIMIT + 5, (
        f"放行了 {allowed} 个,超出全局阈值 {GLOBAL_LIMIT}(+容差)。"
        f"若限流是每实例独立的,这里会接近 {GLOBAL_LIMIT * 2}——正是 Redis 全局限流要解决的。"
    )


def test_quota_used_on_one_replica_is_seen_by_the_other():
    """在副本 A 上耗尽全局配额,副本 B 立刻也被限流 → 证明计数是跨副本共享的。"""
    headers = _headers("10.223.223.223")
    a, b = REPLICAS[0], REPLICAS[1]

    # 在 A 上打满全局阈值
    for _ in range(GLOBAL_LIMIT + 2):
        requests.get(f"{a}/interaction/ping", headers=headers, timeout=5)

    # 立刻换 B —— 共享 Redis 计数,应已被限流
    resp = requests.get(f"{b}/interaction/ping", headers=headers, timeout=5)
    assert resp.status_code == 429, "副本 B 应看到副本 A 已耗尽的全局配额(共享 Redis 计数)"
