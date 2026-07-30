"""JWT 构造工厂 —— 负向 / 安全测试的核心。

网关鉴权规则（见 internal/middleware/jwt.go）：
  - 只接受 HS256（jwt.WithValidMethods(["HS256"])）
  - 强制要求 exp（jwt.WithExpirationRequired()）
  - 用 user_id / sub 提取身份，注入下游 X-User-Id

每个工厂函数对应一类攻击面 / 边界，命名即测试意图。
"""
from __future__ import annotations

import time

import jwt as pyjwt
from cryptography.hazmat.primitives.asymmetric import rsa

from .config import settings


def valid_token(user_id: int | str = 9527, ttl_seconds: int = 3600) -> str:
    """合法 token：HS256 + 正确密钥 + 未过期 + 带 exp。"""
    payload = {"user_id": user_id, "exp": int(time.time()) + ttl_seconds}
    return pyjwt.encode(payload, settings.jwt_secret, algorithm="HS256")


def expired_token(user_id: int | str = 9527) -> str:
    """已过期：用真实密钥签名，仅 exp 过期 —— 应因"过期"被拒（而非签名）。"""
    payload = {"user_id": user_id, "exp": int(time.time()) - 60}
    return pyjwt.encode(payload, settings.jwt_secret, algorithm="HS256")


def no_exp_token(user_id: int | str = 9527) -> str:
    """无 exp 声明：签名合法但缺 exp —— 网关强制要求 exp，应拒。"""
    payload = {"user_id": user_id}
    return pyjwt.encode(payload, settings.jwt_secret, algorithm="HS256")


def wrong_secret_token(user_id: int | str = 9527) -> str:
    """错误密钥签名 —— 签名校验失败，应拒。"""
    payload = {"user_id": user_id, "exp": int(time.time()) + 3600}
    return pyjwt.encode(payload, "definitely-not-the-secret", algorithm="HS256")


def alg_none_token(user_id: int | str = 9527) -> str:
    """算法混淆攻击：alg=none 无签名伪造 —— 经典 JWT 漏洞，必须被拒。🔒"""
    payload = {"user_id": user_id, "exp": int(time.time()) + 3600}
    # PyJWT 需显式允许 none 才能生成
    return pyjwt.encode(payload, key="", algorithm="none")


def alg_rs256_token(user_id: int | str = 9527) -> str:
    """算法混淆攻击：改用 RS256 伪造 —— 网关只认 HS256，应被 WithValidMethods 拒绝。🔒"""
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    payload = {"user_id": user_id, "exp": int(time.time()) + 3600}
    return pyjwt.encode(payload, private_key, algorithm="RS256")


def tampered_token(user_id: int | str = 9527) -> str:
    """篡改 payload 不改签名 —— 改动后签名不匹配，应拒。"""
    token = valid_token(user_id)
    header, payload, signature = token.split(".")
    # 把 payload 换成另一个合法 base64（改 user_id），签名保持原样
    forged_payload = pyjwt.utils.base64url_encode(
        b'{"user_id":"admin","exp":9999999999}'
    ).decode()
    return f"{header}.{forged_payload}.{signature}"
