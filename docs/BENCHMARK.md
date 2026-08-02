# 网关性能基准(Gateway Benchmark)

> 目的:给"高性能"一个**数字**,而不是形容词。测的是网关**裸转发路径**(JWT 鉴权 → 路由 → 轮询负载均衡 → 反向代理)的稳态吞吐(QPS)和尾延迟(P95/P99)。

## 测试方法

- **被测路由**:`GET /storage/ping`(带 JWT),该路由是**双副本轮询负载均衡** → 覆盖鉴权 + 路由 + LB + 反向代理完整链路。
- **限流**:压测专用配置 [`configs/config.bench.yaml`](../configs/config.bench.yaml) 把 rate limit 调到实质无限(`rps: 1000000`),以测**转发能力上限**,而非限流器。限流**精度**是另一组测试(用 `config.docker.yaml` 的 `rps:10`)。
- **压测工具**:k6,`constant-vus` 稳态模型,无 think-time,脚本 [`e2e/perf/bench.js`](../e2e/perf/bench.js)。
- **拓扑**:k6 与网关同在 compose 网络内直连(`http://gateway:8080`),排除宿主网络往返干扰。

## 结果(2026-08 本机实测)

| 并发 (VUs) | 吞吐 QPS | 中位延迟 | P95 | P99 | 错误率 |
|---|---|---|---|---|---|
| 50 | **11,457** | 4.1 ms | 9.5 ms | **12.8 ms** | **0%** |
| 200 | 12,298 | 14.5 ms | 41.8 ms | 74 ms | 0% |

- 30 秒稳态,单档处理 30 万+ 请求,**全程 0 失败**。
- **拐点判断**:并发从 50 → 200(×4),QPS 只从 11.5k → 12.3k(+7%),而 P99 从 13ms → 74ms(×5)。说明系统在 **~12k QPS 已饱和**,继续加并发只是排队(延迟涨、吞吐平)。
- **甜点**:约 **50 并发 / ~11.5k QPS / P99 13ms**,是吞吐与延迟的最佳平衡点。

## 诚实注意事项(必须一起讲)

- **环境**:Docker Desktop(Windows/WSL2)单机,**网关 + 5 个 mock 后端 + k6 抢同一台机器的 CPU**。这是"本机相对数字",不是硬件绝对上限——独占多核裸机会更高。
- 因此对外表述用**相对口径**:"在这套单机 compose 环境下,实测 ~11.5k QPS、P99 13ms、0 错误",而不是"网关能扛 N 万 QPS"。
- 数字的价值不在绝对值,而在于:**我能量化、能找到饱和拐点、能解释瓶颈**——这才是面试要的信号。

## 一句话简历/面试话术

> "我给网关做了容量基准:k6 稳态压测,覆盖鉴权 + 路由 + 轮询 LB + 反向代理链路,**单机 compose 环境实测约 1.1 万 QPS、P99 13ms、0 错误**;通过并发扫描定位到 ~12k QPS 的饱和拐点(再加并发吞吐不增、延迟陡增),并把性能纳入 CI 门禁防退化。"

## 复现步骤

```bash
# 1) 起压测栈(限流关闭的 bench 配置)
docker compose -f docker-compose.yml -f docker-compose.bench.yml up --build -d

# 2) 取 token
TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')

# 3) 跑一档(改 VUS 扫描:50 / 100 / 200)
#    Windows Git Bash 需 export MSYS_NO_PATHCONV=1 以免 /scripts 路径被转换
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  run --rm -e TOKEN="$TOKEN" -e VUS=50 -e DURATION=30s \
  k6 run --summary-trend-stats="avg,min,med,p(90),p(95),p(99),max" /scripts/bench.js

# 4) 收栈
docker compose -f docker-compose.yml -f docker-compose.bench.yml --profile bench down
```

## 下一步(想让"高性能"更硬)

- 用 `--profile monitoring` 起 Prometheus + Grafana,压测时用 **PromQL** 看网关自身指标(`rate(http_requests_total[1m])`、`histogram_quantile(0.99, ...)`),和 k6 的客户端视角对照。
- 用 **pprof** 抓 CPU 火焰图,看瓶颈在鉴权、代理复制还是 GC。
- 逐层优化(连接池复用 upstream 连接、减少每请求分配)后重测,用数字证明提升。
