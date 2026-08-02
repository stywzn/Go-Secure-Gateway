# 实操深挖 Checklist(把"笔记知识"变"项目证据")

> 目的:亲手跑一次,拿到**你网关特有的观察**,面试时能自信讲"我做过 X,看到 Y"。
> 前置:分支 `perf/benchmark`(已含 pprof + bench 配置);装了 Docker Desktop、Go(`go version`)。
> pprof 的火焰图视图需要 **Graphviz**(可选):`winget install graphviz` 或 https://graphviz.org/download/ 。没装也能看 top 列表和火焰图 web 视图。

---

## ⭐ 主菜:pprof 火焰图(必做,面试要讲)

### 第 1 步 · 起压测栈(pprof 端口已开在 6060)
```bash
docker compose -f docker-compose.yml -f docker-compose.bench.yml up --build -d
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/healthz     # 期望 200
curl -s -o /dev/null -w 'pprof=%{http_code}\n' localhost:6060/debug/pprof/    # 期望 200
```
> `pprof=200` 说明剖析端口通了(它只在 debug 配置下开,bench 配置默认 debug:true)。

### 第 2 步 · 一边压,一边抓 CPU profile(两个终端)
**终端 A**——打流量(用之前的 bench,50 VUs 足够让 CPU 忙起来):
```bash
export MSYS_NO_PATHCONV=1
TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  run --rm -e TOKEN="$TOKEN" -e VUS=100 -e DURATION=60s k6 run /scripts/bench.js
```
**终端 B**——在压测的这 60 秒内,抓 30 秒 CPU 火焰图:
```bash
go tool pprof -http=:8081 "http://localhost:6060/debug/pprof/profile?seconds=30"
```
浏览器自动打开 http://localhost:8081 → 左上角 `VIEW` 菜单选 **Flame Graph**。

### 第 3 步 · 读火焰图,记下你的观察 ⭐(这步是面试素材)
- **横轴 = CPU 占比**(越宽越耗),纵轴 = 调用栈。找**最宽的顶层函数**。
- 把你看到的记下来(填这句):
  > "网关在 100 并发下,CPU 火焰图里最宽的是 `________`,占约 __%。"
- 命令行版(不想开浏览器):
  ```bash
  go tool pprof -top "http://localhost:6060/debug/pprof/profile?seconds=30"
  # 看 flat/cum 最高的前几个函数
  ```

### 第 4 步 · 对照判断优化方向
| 火焰图里宽的是… | 说明瓶颈 | 优化方向(你的笔记) |
|---|---|---|
| `net/http` 反向代理 / `io.Copy` 数据复制 | 转发时反复搬数据 | **零拷贝**(sendfile) |
| `runtime.mallocgc` / `runtime.gcBgMarkWorker` | 每请求分配内存 → GC 压力 | **内存池**(sync.Pool) |
| `crypto` / JWT 解析 | 每请求验签开销 | 缓存解析结果 / 调鉴权 |
| `syscall` / 网络等待 | 瓶颈在 IO 而非 CPU | 加副本(横向扩) |

> 记 1 条结论就够:"瓶颈在 ___,方向是 ___"。这就把 BENCHMARK.md 的"为什么 12k 饱和"补成了闭环。

### 第 5 步 · (可选)顺手抓内存 profile
```bash
go tool pprof -http=:8082 "http://localhost:6060/debug/pprof/heap"
# 看谁在分配内存,验证要不要上内存池
```

---

## 🟡 配菜 1:混沌实验(你已做过,重新包装成"韧性工程")

不用重写代码,按混沌五步法**重跑 + 记录**即可(用 `/compute` 路由,它是给故障注入准备的):
```bash
TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')

# 实验A:后端变慢 > 2s 上游超时 → 期望 504
curl -s -o /dev/null -w '延迟注入 status=%{http_code}\n' -H "Authorization: Bearer $TOKEN" "localhost:8080/compute/run?delay=3000"

# 实验B:后端持续 500 → 期望熔断打开后快速失败(连打几次看状态变化)
for i in $(seq 1 10); do curl -s -o /dev/null -w '%{http_code} ' -H "Authorization: Bearer $TOKEN" "localhost:8080/compute/run?status=500"; done; echo
```
按这个模板记录(就是面试讲法):
> "**稳态**:成功率>99%。**假设**:后端延迟>2s 时网关应返回 504 而非挂起。**注入**:`?delay=3000`。**观测**:返回 504,网关自身不受影响。**结论**:超时/熔断生效。"

---

## 🟢 配菜 2:PromQL 看压测曲线(10 分钟,眼见为实)

```bash
# 起监控栈
docker compose -f docker-compose.yml -f docker-compose.bench.yml --profile monitoring up -d
# Grafana: http://localhost:3000  (admin/admin)   Prometheus: http://localhost:9090
```
一边压测,一边在 Grafana(或 Prometheus 的 Graph 页)敲这几条,看曲线:
```promql
sum(rate(http_requests_total[1m]))                         # QPS
sum(rate(http_requests_total[1m])) by (status)             # 按状态码
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))   # P99
```
> 观察点:QPS 爬到 ~12k 后压不动、P99 陡升——这就是**饱和拐点的可视化证据**,和 k6 的数字对上。

---

## 收尾:清理
```bash
docker compose -f docker-compose.yml -f docker-compose.bench.yml --profile bench --profile monitoring down
```

## 做完你会多出的 3 句面试话术
1. "我用 pprof 抓火焰图,瓶颈在 ___,优化方向是 ___。"
2. "我按混沌工程的稳态→假设→注入→观测,验证了网关的超时/熔断/降级。"
3. "我用 PromQL 在 Grafana 上看到 QPS 到 ~12k 饱和、P99 陡升,和压测数字互相印证。"
