"""契约测试 —— 用 docs/openapi.yaml 作为"真相源"校验响应结构。

思路：不手写"响应长什么样"的断言，而是直接拿接口契约（OpenAPI schema）去
校验真实响应。契约变了、实现漂移了，用例自动发现 —— 这是接口测试从"面向
实现"升级到"面向契约"的关键，面试可讲。

这里做两类校验：
  1) 响应体符合 OpenAPI 里声明的 schema（用 jsonschema 校验）。
  2) 声明的状态码在实现中确实可达（正/负向都打一遍）。
"""
from __future__ import annotations

from pathlib import Path

import pytest
import yaml
from jsonschema import Draft7Validator

from framework.client import GatewayClient
from framework import jwt_utils

pytestmark = pytest.mark.contract

_SPEC_PATH = Path(__file__).parent.parent.parent / "docs" / "openapi.yaml"
_SPEC = yaml.safe_load(_SPEC_PATH.read_text(encoding="utf-8"))


def _schema_for(path: str, method: str, status: str) -> dict | None:
    """从 spec 里取某 path/method/status 的 application/json schema（解 $ref）。"""
    op = _SPEC["paths"][path][method]
    resp = op["responses"][status]
    resp = _resolve_ref(resp)
    content = resp.get("content", {}).get("application/json")
    return content.get("schema") if content else None


def _resolve_ref(node: dict) -> dict:
    ref = node.get("$ref")
    if not ref:
        return node
    # 形如 #/components/responses/Unauthorized
    cur = _SPEC
    for part in ref.lstrip("#/").split("/"):
        cur = cur[part]
    return cur


def test_list_items_matches_schema(reset_data):
    """GET /data/items 200 响应体符合 OpenAPI 声明的 {items[], count} schema。"""
    schema = _schema_for("/data/items", "get", "200")
    resp = reset_data.get("/data/items")
    assert resp.status_code == 200
    errors = sorted(Draft7Validator(schema).iter_errors(resp.json()), key=str)
    assert not errors, "响应不符合契约：\n" + "\n".join(e.message for e in errors)


def test_unauthorized_matches_schema():
    """401 响应体符合 Unauthorized 契约（含 error 字段）。"""
    schema = _schema_for("/data/items", "get", "401")
    resp = GatewayClient().with_token(None).get("/data/items")
    assert resp.status_code == 401
    errors = list(Draft7Validator(schema).iter_errors(resp.json()))
    assert not errors, "401 响应不符合契约"


def test_create_item_contract(reset_data):
    """POST /data/items 声明的 201 在实现中可达，且返回带 id 的对象。"""
    resp = reset_data.post("/data/items", json={"name": "widget"})
    assert resp.status_code == 201, resp.text
    assert "id" in resp.json(), "创建响应应包含分配的 id"


def test_declared_debug_token_shape():
    """/debug/token 契约声明返回 {token}；校验实际结构一致。"""
    schema = _schema_for("/debug/token", "get", "200")
    resp = GatewayClient().with_token(None).get("/debug/token")
    assert resp.status_code == 200
    errors = list(Draft7Validator(schema).iter_errors(resp.json()))
    assert not errors, "debug/token 响应不符合契约"
