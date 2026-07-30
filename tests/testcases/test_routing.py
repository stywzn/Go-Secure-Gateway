"""模块 3 —— 路由与反向代理。

覆盖：前缀剥离 / 保留、方法与查询透传、身份注入、身份伪造防护、未知路由。
对应 test-points.md 模块 3（含安全点 3.8）。
"""
from __future__ import annotations

import pytest

from framework.assertions import assert_status

pytestmark = pytest.mark.routing


def test_prefix_preserved_when_strip_false(auth_client):
    """3.1 /interaction 配置 strip=false → 后端收到的 path 保留前缀。"""
    resp = auth_client.get("/interaction/ping")
    assert_status(resp, 200)
    assert resp.json()["path"] == "/interaction/ping"


def test_prefix_stripped_when_strip_true(auth_client):
    """3.2 /storage 配置 strip=true → 前缀被剥离，后端 echo 出的 path == /ping。"""
    resp = auth_client.get("/storage/ping")
    assert_status(resp, 200)
    assert resp.json()["path"] == "/ping", resp.text


def test_stripped_subpath(auth_client):
    """3.2 细化：/compute/files/1（strip=true）→ 后端 path == /files/1。"""
    resp = auth_client.get("/compute/files/1")
    assert resp.json()["path"] == "/files/1", resp.text


@pytest.mark.parametrize("method", ["GET", "POST", "PUT", "DELETE"])
def test_all_methods_forwarded(auth_client, method):
    """3.5 各 HTTP 方法与 body 完整透传（用 echo 的 method 字段验证）。"""
    # 用 compute 路由（echo，无 CRUD 语义）避免方法被 CRUD 处理器拦截
    resp = auth_client.request(method, "/compute/run", json={"k": "v"})
    assert_status(resp, 200)
    assert resp.json()["method"] == method


def test_query_string_forwarded(auth_client):
    """3.4 查询参数完整透传。"""
    resp = auth_client.get("/compute/run", params={"n": "42", "x": "y"})
    assert "n=42" in resp.json()["query"]


def test_user_id_injected(auth_client):
    """3.7 鉴权后 X-User-Id 注入下游。"""
    resp = auth_client.get("/compute/run")
    assert resp.json()["user_id"] == "9527"


@pytest.mark.security
def test_client_cannot_spoof_user_id(auth_client):
    """3.8 🔒 客户端伪造 X-User-Id → 被网关剥离并重设为真实身份。"""
    resp = auth_client.get("/compute/run", headers={"X-User-Id": "admin-spoofed"})
    assert resp.json()["user_id"] == "9527", "身份伪造未被拦截，存在越权风险！"


def test_unknown_prefix_returns_404(auth_client):
    """3.6 未配置的前缀 → 404。"""
    assert_status(auth_client.get("/unknown/whatever"), 404)
