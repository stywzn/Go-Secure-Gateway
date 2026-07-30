"""模块 1 —— 鉴权（JWT Auth）。

覆盖：正向放行、缺头/格式错、以及数据驱动的负向&安全用例
（过期/无 exp/错误密钥/算法混淆/篡改）。对应 test-points.md 模块 1 + 8。
"""
from __future__ import annotations

from pathlib import Path

import pytest
import yaml

from framework import jwt_utils
from framework.assertions import assert_status, assert_unauthorized

pytestmark = pytest.mark.auth

_CASES = yaml.safe_load((Path(__file__).parent.parent / "data" / "auth_cases.yaml").read_text(encoding="utf-8"))


# ---- 正向 ----

def test_valid_token_is_allowed(auth_client):
    """1.1 携带合法 token 访问受保护路由 → 200，且透传到后端。"""
    resp = auth_client.get("/interaction/ping")
    assert_status(resp, 200)
    assert resp.json()["service"] == "interaction"


def test_user_id_propagated_downstream(auth_client):
    """1.10 合法 token 的 user_id 被网关注入下游 X-User-Id。"""
    resp = auth_client.get("/interaction/ping")
    assert resp.json()["user_id"] == "9527"


# ---- 负向：缺头 / 格式 ----

def test_missing_authorization_header(client):
    """1.2 不带 Authorization → 401。"""
    assert_unauthorized(client.with_token(None).get("/interaction/ping"))


@pytest.mark.parametrize("bad_header", [
    "Basic abc123",              # 错误 scheme
    "token-without-bearer",      # 无 Bearer 前缀
    "Bearer",                    # 只有 Bearer 无 token
])
def test_malformed_authorization_header(client, bad_header):
    """1.3 / 1.9 头格式错误 → 401。"""
    resp = client.with_token(None).get("/interaction/ping", headers={"Authorization": bad_header})
    assert_unauthorized(resp)


# ---- 数据驱动的负向 & 安全 ----
# 在收集期把 yaml 里的每条 case 转成 pytest.param，安全类附加 security 标记，
# 这样 `pytest -m security` 能精确挑出算法混淆/篡改这类高价值安全用例。
def _build_params():
    params = []
    for case in _CASES["negative_cases"]:
        marks = [pytest.mark.security] if case.get("security") else []
        params.append(pytest.param(case, id=case["id"], marks=marks))
    return params


@pytest.mark.parametrize("case", _build_params())
def test_invalid_tokens_are_rejected(client, case):
    """1.4~1.8 / 8.1 数据驱动：各类非法 token 均应 401。"""
    token = getattr(jwt_utils, case["token_factory"])()
    resp = client.with_token(token).get("/interaction/ping")
    assert_status(resp, case["expected_status"])
