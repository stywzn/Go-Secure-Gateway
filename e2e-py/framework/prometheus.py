"""解析 Prometheus 的文本格式，支撑"监控断言"。

`/metrics` 返回的是一坨纯文本，长这样：

    # HELP gateway_http_requests_total Total number of HTTP requests ...
    # TYPE gateway_http_requests_total counter
    gateway_http_requests_total{method="GET",route="/interaction",status="200"} 27
    gateway_http_requests_total{method="GET",route="/storage",status="200"} 11

`#` 开头的是说明行，不是数据。数据行的结构是 `名字{标签...} 数值`。

网关侧的指标定义见 internal/metrics/metrics.go：
    gateway_http_requests_total{method,route,status}
注意 route 是**路由前缀**（如 /interaction），不是原始请求路径 ——
Go 那边这样设计是为了控制 Prometheus 的 label 基数，避免每个 URL 一条时间序列。
"""


def counter_total(text: str, name: str) -> float:
    """把某个指标所有 label 组合的值加总；一条样本都没有则返回 0.0。

    返回 0.0 而不是抛异常，是刻意的：**指标"不存在"和"值为 0"语义上是一回事**，
    都表示这件事还没发生过。带 label 的 CounterVec 在任何 label 组合被观测到之前，
    /metrics 里连 `# HELP` 行都不会输出（这正是 test_request_counter_increments
    曾经顺序依赖的根因）。返回 0.0 之后，调用方就能直接写 `after > before`，
    不必先判断指标存在与否。

    加总而不按 label 精确匹配，也是刻意的：用例只关心"我这次操作有没有让计数器动"，
    加总对路由前缀、状态码的变化免疫，比锁死某个 label 组合更稳。
    """
    total = 0.0
    for line in text.splitlines():
        if line.startswith("#"):
            continue
        # 从右边最后一个空格切：label 值里可能含空格，从左边切会把 label 切断。
        left, _, value = line.rpartition(" ")
        if left.split("{")[0] == name:
            total += float(value)
    return total
