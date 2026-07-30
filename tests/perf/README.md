# 性能 / 限流压测

两份脚本，任选其一。都聚焦同一组"面试可讲"的问题:**限流阈值准不准、放行流量的 p95 会不会被拖垮**。

## 先拿一个 token

```bash
TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
```

## k6(推荐,阈值即断言)

```bash
k6 run -e TOKEN=$TOKEN tests/perf/ratelimit_k6.js
```

- 以 200 req/s(约 4× 限流阈值)恒定加压 20s。
- 内置 `thresholds`:放行请求 p95 < 200ms、429 比例 > 30%。任一不满足,k6 退出码非 0 → **可直接接入 CI 当性能门禁**。

## locust(看实时曲线)

```bash
locust -f tests/perf/locustfile.py --host http://localhost:8080 \
       --headless -u 100 -r 50 -t 30s
```

## 怎么讲(STAR-R 里的量化结果)

- "在 4× 限流阈值的压力下,网关拦下约 X% 请求(429),放行流量 p95 稳定在 Y ms,证明**限流有效隔离了突发流量对后端的冲击**。"
- 对比"单 IP"与"多 IP"(改 `X-Forwarded-For`):单 IP 很快触顶,多 IP 各自独立计数 —— 佐证限流键设计,也暴露 `TrustedProxies` 配置风险。
