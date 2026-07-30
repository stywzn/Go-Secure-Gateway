"""全局 fixtures 与前置检查。

设计要点：
  - session 级做一次"网关可达 + debug 可用"探活，环境没起直接给出清晰指引，
    而不是让每条用例各自超时报一堆红。
  - client / auth_client 为函数级，每条用例独立源 IP → 限流桶隔离 → 可并行。
  - reset_data 在 CRUD 用例前清空 /data 后端内存，保证用例幂等、无脏数据串扰。
"""
from __future__ import annotations

import pytest
import requests

from framework.client import GatewayClient
from framework.config import settings
from framework import jwt_utils


@pytest.fixture(scope="session", autouse=True)
def _preflight():
    """会话开始前确认被测环境就绪，否则给出可操作的提示。"""
    try:
        r = requests.get(f"{settings.base_url}/healthz", timeout=5)
        r.raise_for_status()
    except Exception as e:  # noqa: BLE001
        pytest.exit(
            f"\n无法连通网关 {settings.base_url}/healthz：{e}\n"
            f"请先在项目根目录启动被测栈：docker compose up --build -d\n"
            f"或用 GATEWAY_BASE_URL 指向已运行的实例。\n",
            returncode=3,
        )


@pytest.fixture
def client() -> GatewayClient:
    """未鉴权客户端（独立源 IP）。负向用例 / 需要自定义 token 时用。"""
    return GatewayClient()


@pytest.fixture
def auth_client() -> GatewayClient:
    """已携带合法 token 的客户端（独立源 IP）。正向业务用例用。"""
    return GatewayClient(token=jwt_utils.valid_token())


@pytest.fixture
def reset_data(auth_client: GatewayClient):
    """清空 /data 后端内存存储，保证 CRUD 类用例从干净状态开始。

    路由分工（见 configs/config.docker.yaml）：CRUD 在单后端的 /data 路由上，
    与负载均衡路由 /storage、故障注入路由 /compute 分开，互不干扰。
    """
    auth_client.post("/data/_reset")
    return auth_client
