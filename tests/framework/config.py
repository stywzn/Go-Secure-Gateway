"""环境配置层：所有可变项集中在此，通过环境变量覆盖，切换环境零改用例代码。

默认值对齐 docker-compose（gateway 直连 8080，JWT_SECRET=compose-dev-secret）。
本地直连:   BASE_URL=http://localhost:8080
经 nginx:   BASE_URL=http://localhost:8088/api
CI 环境:    在 workflow 里 export 覆盖即可。
"""
from __future__ import annotations

import os
from dataclasses import dataclass


def _get(name: str, default: str) -> str:
    return os.environ.get(name, default)


@dataclass(frozen=True)
class Settings:
    # 被测网关地址
    base_url: str = _get("GATEWAY_BASE_URL", "http://localhost:8080")

    # 与 docker-compose 注入的 JWT_SECRET 保持一致。
    # 负向用例（错误密钥/算法混淆）不需要它；但"过期""无 exp"这类
    # 要被"因为过期而非因为签名"拒绝的精确用例，需要用真实密钥签名。
    jwt_secret: str = _get("GATEWAY_JWT_SECRET", "compose-dev-secret")

    # 请求超时（秒）。故意略大于网关 WriteTimeout(15s)，
    # 这样"注入 delay 触发超时"的用例能观测到网关侧断开而非客户端先超时。
    http_timeout: float = float(_get("GATEWAY_HTTP_TIMEOUT", "20"))

    # 熔断器参数（与 proxy.go 中 NewCircuitBreaker(5, 10s) 对齐）。
    breaker_threshold: int = int(_get("GATEWAY_BREAKER_THRESHOLD", "5"))
    breaker_cooldown_s: float = float(_get("GATEWAY_BREAKER_COOLDOWN", "10"))

    # 限流参数（与 config.docker.yaml 对齐：rps=50, burst=100）。
    rate_rps: float = float(_get("GATEWAY_RATE_RPS", "50"))
    rate_burst: int = int(_get("GATEWAY_RATE_BURST", "100"))

    @property
    def storage_backends(self) -> tuple[str, ...]:
        """/storage 路由后端的 X-Served-By 取值（轮询断言用）。"""
        return ("storage-a", "storage-b")


settings = Settings()
