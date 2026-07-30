"""HTTP 客户端封装层。

统一处理：鉴权头注入、per-client 源 IP（X-Forwarded-For，用于隔离限流桶）、
超时、请求/响应日志、Allure 步骤。用例只表达"业务意图"，不重复写样板。
"""
from __future__ import annotations

import logging
import uuid

import requests

from .config import settings

log = logging.getLogger("gateway.client")

try:  # Allure 可选：没装也能跑
    import allure
except ImportError:  # pragma: no cover
    allure = None


class GatewayClient:
    """针对被测网关的会话封装。

    :param source_ip: 该客户端对外呈现的源 IP。网关用 c.ClientIP() 做限流键，
        默认信任 X-Forwarded-For。给每个测试独立 IP，就能把限流桶隔离开，
        让用例可安全并行（也顺带验证 test-points 2.4：不同 IP 互不影响）。
    """

    def __init__(self, source_ip: str | None = None, token: str | None = None):
        self.base_url = settings.base_url.rstrip("/")
        self.source_ip = source_ip or _random_ip()
        self._token = token
        self._session = requests.Session()

    # ---- 鉴权 ----
    def with_token(self, token: str | None) -> "GatewayClient":
        """设置 Bearer token（None 表示不带鉴权头，用于负向用例）。"""
        self._token = token
        return self

    def _headers(self, extra: dict | None) -> dict:
        headers = {"X-Forwarded-For": self.source_ip}
        if self._token is not None:
            headers["Authorization"] = f"Bearer {self._token}"
        if extra:
            headers.update(extra)
        return headers

    # ---- 核心请求 ----
    def request(self, method: str, path: str, *, headers=None, **kwargs) -> requests.Response:
        url = f"{self.base_url}{path}"
        kwargs.setdefault("timeout", settings.http_timeout)
        merged = self._headers(headers)

        log.info("→ %s %s (src=%s)", method, path, self.source_ip)
        resp = self._session.request(method, url, headers=merged, **kwargs)
        log.info("← %s %s [%s] served-by=%s",
                 method, path, resp.status_code, resp.headers.get("X-Served-By", "-"))

        if allure is not None:
            _attach(method, url, resp)
        return resp

    def get(self, path, **kw):
        return self.request("GET", path, **kw)

    def post(self, path, **kw):
        return self.request("POST", path, **kw)

    def put(self, path, **kw):
        return self.request("PUT", path, **kw)

    def delete(self, path, **kw):
        return self.request("DELETE", path, **kw)


def _random_ip() -> str:
    """生成一个稳定唯一的私网 IP（测试专用源，隔离限流桶）。"""
    n = uuid.uuid4().int
    return f"10.{n % 256}.{(n >> 8) % 256}.{(n >> 16) % 256 or 1}"


def _attach(method: str, url: str, resp: requests.Response) -> None:  # pragma: no cover
    body = resp.text[:2000]
    allure.attach(
        f"{method} {url}\n\nstatus: {resp.status_code}\n"
        f"headers: {dict(resp.headers)}\n\nbody:\n{body}",
        name=f"{method} {url} -> {resp.status_code}",
        attachment_type=allure.attachment_type.TEXT,
    )
