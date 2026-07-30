"""领域断言层：把重复的校验逻辑收敛成语义化断言，失败信息可读。"""
from __future__ import annotations

from collections import Counter

import requests


def assert_status(resp: requests.Response, expected: int) -> None:
    assert resp.status_code == expected, (
        f"期望状态码 {expected}，实际 {resp.status_code}；body={resp.text[:300]}"
    )


def assert_unauthorized(resp: requests.Response) -> None:
    """401 且响应体是 JSON 错误结构。"""
    assert_status(resp, 401)
    assert "error" in resp.json(), f"401 响应缺少 error 字段：{resp.text[:200]}"


def served_by(resp: requests.Response) -> str:
    """取后端标识（mock backend 在 X-Served-By 回填 SERVICE_NAME）。"""
    return resp.headers.get("X-Served-By", "")


def assert_round_robin(served: list[str], backends: tuple[str, ...], tolerance: float = 0.25) -> None:
    """校验轮询分布：每个后端都被命中，且分布大致均匀。

    :param tolerance: 允许的相对偏差（默认 ±25%），吸收并发下的轻微抖动。
    """
    counts = Counter(served)
    missing = set(backends) - set(counts)
    assert not missing, f"这些后端从未被命中（轮询异常/坏节点被跳过?）：{missing}；实际分布={dict(counts)}"

    expected = len(served) / len(backends)
    for backend in backends:
        deviation = abs(counts[backend] - expected) / expected
        assert deviation <= tolerance, (
            f"后端 {backend} 命中 {counts[backend]} 次，期望约 {expected:.1f} "
            f"(偏差 {deviation:.0%} > {tolerance:.0%})；完整分布={dict(counts)}"
        )
