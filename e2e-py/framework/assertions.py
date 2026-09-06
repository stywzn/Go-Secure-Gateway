"""领域断言层：把重复的校验逻辑收敛成语义化断言，失败信息可读。

裸 `assert resp.status_code == 200` 失败时 pytest 只能显示 `assert 401 == 200` ——
知道数字对不上，但不知道打的哪个 URL、被测系统说了什么。而后者往往直接写着原因。
"""
from collections import Counter
def assert_status(resp, expected):
    """断言状态码，失败时带出请求与响应现场。

    响应体截断到 200 字符：`/metrics` 这类端点有近 9000 字符，不截断一次失败就刷屏。
    """
    assert resp.status_code == expected, (
        f"期望状态码 {expected}，实际 {resp.status_code}\n"
        f"  请求: {resp.request.method} {resp.request.url}\n"
        f"  响应: {resp.text[:200]}"
    )

def assert_unauthorized(resp):
    """断言这是一个**格式规范的** 401 拒绝响应。

    只断状态码是不够的：401 只说明"被拒了"，没说明"拒得像不像话"。
    下面这些都会让"只断 401"的用例通过，但其实都是缺陷：
      - 401 但响应体是空的        → 调用方拿不到原因
      - 401 但响应体是 HTML 错误页 → 请求没走到鉴权中间件，被更外层拦了
      - 401 但 JSON 里没有 error   → 错误响应的契约变了
    """
    assert_status(resp, 401)

    try:
        body = resp.json()
    except ValueError:
        # from None 隐藏 json 库内部的 ValueError 堆栈——对使用者没有信息量。
        raise AssertionError(
            f"401 响应体不是合法 JSON\n"
            f"  请求: {resp.request.method} {resp.request.url}\n"
            f"  响应: {resp.text[:200]}"
        ) from None

    assert "error" in body, (
        f"401 响应缺少 error 字段\n"
        f"  请求: {resp.request.method} {resp.request.url}\n"
        f"  实际字段: {list(body)}\n"
        f"  响应体: {body}"
    )

def assert_round_robin(served, backends, tolerance=0.25):
    """校验轮询分布：每个后端都被命中，且各自命中次数大致均匀。

    为什么不能只用 `set(served) == set(backends)`：集合只记"有没有"、不记"有几个"。
    打 10 次请求，5:5 和 9:1 在集合眼里完全一样 —— 严重倾斜发现不了。

    :param served: 每次请求命中的后端标识（**列表**，顺序无所谓但次数不能丢）
    :param backends: 期望出现的全部后端
    :param tolerance: 允许的相对偏差，默认 ±25%，用于吸收并发下的轻微抖动
    """
    counts = Counter(served)

    missing = set(backends) - set(counts)
    assert not missing, (
        f"这些后端一次都没被命中：{sorted(missing)}\n"
        f"  可能是轮询异常，或坏节点被静默跳过\n"
        f"  实际分布: {dict(counts)}（共 {len(served)} 次请求）"
    )

    expected = len(served) / len(backends)
    for backend in backends:
        deviation = abs(counts[backend] - expected) / expected
        assert deviation <= tolerance, (
            f"后端 {backend} 命中 {counts[backend]} 次，期望约 {expected:.1f} 次，"
            f"偏差 {deviation:.0%} 超过容差 {tolerance:.0%}\n"
            f"  实际分布: {dict(counts)}（共 {len(served)} 次请求）"
        )



def served_by(resp):
    """取后端标识（mock backend 在 X-Served-By 回填 SERVICE_NAME）。"""
    return resp.headers.get("X-Served-By", "")