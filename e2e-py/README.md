# Go-Secure-Gateway 接口自动化测试框架

基于 **Python + pytest** 从 0 手搭的分层接口自动化框架。被测系统(SUT)是一个 Go 编写的 API 网关,具备**鉴权 / 限流 / 反向代理 / 负载均衡 / 熔断 / 上游超时 / 可观测**能力,后端为一组**可控测试桩**(能按需返回任意状态码、注入延迟、支持内存 CRUD)。

覆盖四条"高含金量"主线:**接口自动化 + CI 门禁 · 故障注入 · 性能压测 · 契约 + 监控断言**。

---

## 亮点

- **分层框架**:配置 / HTTP 客户端 / JWT 工具 / Prometheus 解析 / 领域断言层解耦,用例只写业务意图。断言失败时带出请求 URL 与响应体,不必回头猜。
- **数据驱动**:鉴权负向用例外置到 `data/auth_cases.yaml`,加一行 = 加一个攻击面,不动代码。
- **故障注入(最见深度)**:用可控测试桩 `?status=` / `?delay=` 稳定复现**熔断状态机**(closed→open→half-open→closed)与**上游超时(504)**,这些异常路径几乎无法用真实后端稳定复现,靠桩做到 100% 确定、零 flaky。
- **性能压测**:k6 以远超阈值的并发压限流,断言 429 触发 + p95 延迟 + 无 5xx,thresholds 不达标即失败,**接入 CI 当性能门禁**。性能测试的**取舍/全套体系/为什么不做负载与 soak/面试话术**见 [`PERFORMANCE-QA.md`](PERFORMANCE-QA.md)。
- **契约测试**:用 JSON Schema 校验真实响应结构,面向契约而非面向实现。
- **并行 + 隔离**:按"是否共享服务端状态"分两阶段执行 —— 破坏性的熔断用例单独串行,避免与超时用例互相串扰。**这个拆分的目的是测试隔离,不是提速**:实测两阶段(3.4s + 11.3s)与全量串行(14.4s)耗时基本相同,并行本身收益有限(见下方"并行到底提速多少")。
- **CI 门禁**:GitHub Actions 每次 push/PR 自动起栈 → 两阶段跑 pytest → k6 压测 → 传 Allure 报告,任一环节失败即拦 PR。

## 覆盖模块

| 模块 | 测什么 |
|---|---|
| 鉴权 `auth` | 合法放行;过期 / 错签名 / 算法 none 攻击 → 401(数据驱动) |
| 路由 `routing` | 前缀剥离 / 保留(用回显的 `path` 断言) |
| 限流 `ratelimit` | 单 IP 打爆 → 429;按源 IP 隔离 |
| 负载均衡 `loadbalance` | 轮询命中两个副本(`X-Served-By` 交替) |
| 熔断 `breaker` | 连续 5xx → 打开(503 且无 `X-Served-By` = 网关短路);冷却后半开恢复 |
| 超时 `timeout` | 正常延迟放行;慢后端 → 504 上游超时 |
| 有状态 CRUD `crud` | 建→查→改→删→再查 404 端到端时序 |
| 监控 `ops` | 探针 / `/metrics` **计数器随请求增长** / debug token |
| 契约 `contract` | 响应结构符合 JSON Schema |

## 快速开始

```bash
# 1) 项目根目录起被测栈(网关 + 可控 mock 后端)
docker compose up -d --build

# 2) 装依赖
pip install -r e2e-py/requirements.txt

# 3) 跑用例(两阶段)
cd e2e-py
pytest -m "not breaker" -n auto     # 大部队并行(快)
pytest -m breaker                   # 熔断串行(含冷却等待,约 11s)

# 4) 看 Allure 报告
pytest --alluredir=allure-results
allure serve allure-results

# 5) 限流压测
k6 run perf/ratelimit.js
```

> 换环境零改代码:`GATEWAY_BASE_URL` 指向别的实例即可(默认 `http://127.0.0.1:8080`)。

## 分层架构

```
e2e-py/
├─ framework/          # 核心层(与用例解耦,可复用)
│  ├─ config.py        #   环境配置:全走环境变量,切环境零改代码
│  ├─ client.py        #   HTTP 客户端封装:鉴权头 / 独立源 IP / 超时
│  ├─ jwt_utils.py     #   JWT 工厂:合法 + 过期/错签名/算法混淆等负向 token
│  ├─ prometheus.py    #   Prometheus 文本解析:支撑监控断言(见"修过的一个真 flaky")
│  └─ assertions.py    #   领域断言层:状态码 / 未授权契约 / 轮询分布(带容差)
├─ data/
│  └─ auth_cases.yaml  # 数据驱动:鉴权用例
├─ testcases/          # 按被测模块组织的用例
├─ perf/
│  └─ ratelimit.js     # k6 限流压测
├─ conftest.py         # 全局 fixtures:会话级环境探活 / client / 带鉴权 client / 存储复位
└─ pytest.ini          # 标记 / 路径配置
```

> **待补**:暂无。四项框架短板已全部补齐。
>
> ~~领域断言层~~ ✅ 已完成 —— `assert_status` / `assert_unauthorized` / `assert_round_robin`,
> 全部 20 处状态码断言已收敛;轮询校验从"两个后端都出现过"升级为"分布是否均匀"
> (原写法对 9:1 的严重倾斜完全无检出能力)。见 [`面试笔记.md`](面试笔记.md) 第 6、7 条。
>
> ~~`client.py` 的 `request()` 收口~~ ✅ 已完成 —— 四个重复方法收敛为一个
> `request(method, path, **kwargs)`;header 改为合并且调用方优先,
> 负向鉴权用例可直接覆盖 `Authorization` 而无需新建 client。
> 见 [`面试笔记.md`](面试笔记.md) 第 3 条。
>
> ~~会话级环境探活 fixture~~ ✅ 已实现 —— 栈没起时一句话报错并 fail fast,
> 输出从 14095 行降到 16 行,退出码 1。见 [`面试笔记.md`](面试笔记.md) 第 2 条。

## 为什么熔断用例要单独串行

熔断器是**按路由的服务端共享状态**。若熔断用例(往 `/compute` 打 5xx 打开熔断)与超时用例(也打 `/compute`)并行,超时用例会撞上 503 → flaky。因此按"是否共享服务端状态"把用例分成**可并行**与**必串行**两组,后者独占串行。这是框架里"识别隐藏耦合、保证测试隔离"的关键设计 —— **动机是正确性,不是速度**。

## 修过的一个真 flaky:Prometheus CounterVec 的惰性输出

**症状**:`test_metrics_exposed` 断言 `"gateway_http_requests_total" in resp.text`,
本地怎么跑都绿,CI 上却间歇性失败。

**根因**在被测系统一侧。`internal/metrics/metrics.go` 里这个指标是带 label 的
`CounterVec`(`{method, route, status}`),而 Prometheus 客户端对**零观测的 CounterVec
完全不输出** —— 连 `# HELP` / `# TYPE` 说明行都没有。所以在一个刚启动、还没处理过
任何业务请求的网关上,`/metrics` 里根本搜不到这个指标名。

**为什么本地发现不了**:本地栈一开就是几小时甚至几天,指标早被喂满;
而 CI 每次 `docker compose up --build` 都是全新网关。
**这个 flaky 在开发机上几乎隐形,在 CI 上却是常态** —— 最难查的那类。

**为什么串行也不报**:`-m "not breaker"` 串行执行时,`test_auth.py` 按文件名字母序
排在 `test_ops.py` 前面,它打的那些请求顺手把指标喂了出来。
换成 `-n auto` 并行,这条用例可能抢在所有业务请求之前跑完,指标就是空的。
**它依赖的不是自己的操作,而是"别人恰好先跑过"。**

**修法**:把断言从「存在性」改成「增量」——

```
读一次 /metrics → 数出 before
打一个 /interaction/ping
再读一次 /metrics → 数出 after
assert after > before
```

配套写了 `framework/prometheus.py` 解析 Prometheus 文本格式,
其中 `counter_total()` 在**找不到指标时返回 0.0 而不是抛异常** ——
因为"指标不存在"和"值为 0"语义上是一回事,这样调用方能直接写 `after > before`。

两个实现细节:

- **断言写 `after > before`,不写 `after == before + 1`**。并行执行时别的 worker
  也在打这个网关,计数器涨的不止 1。共享环境里应当断言**变化的方向**,而非精确值。
- **必须打代理路由**(如 `/interaction/ping`)。计数器只在 `internal/proxy/proxy.go`
  里递增,`/healthz`、`/readyz`、`/metrics` 都注册在 proxy 之外,打它们不会计数。
  顺带,读 `/metrics` 本身不计数,所以测量动作不会干扰被测量的值。

**验证**:重启网关(指标完全清空)后连跑 4 轮 `-n auto`,全部 19 passed;
修复前同样条件稳定复现 `1 failed`。

---

## 并行到底提速多少(实测,别信直觉)

装了 `pytest-xdist` 不等于会变快。本机 16 核实测同一批用例(`-m "not breaker"`,19 条):

| 配置 | 耗时 | 相对串行 |
|---|---|---|
| 串行 | 3.40s / 3.45s | 1.0× |
| `-n 2` | 3.77s / 3.76s | **0.9×(更慢)** |
| `-n 4` | 2.82s / 2.86s | **1.2×(最优)** |
| `-n auto`(16 worker) | 3.40s / 3.49s | 1.0× |

**为什么并行几乎没用**——看单条用例耗时分布:

```
2.00s  test_slow_upstream_returns_504     ← 等上游超时
1.00s  test_delay_passes_through           ← 等注入的 1s 延迟
0.03s  test_rate_limit_returns_429
其余 16 条          全部 < 0.01s
```

19 条用例的**测试执行时间合计约 3.05s,其中 3.00s 是两条"故意等"的故障注入用例**。
于是:

- **并行的下限 = 最长那一条用例的耗时 = 2.00s**,再多 worker 也突破不了。
- 理论上限只有 `3.40 / 2.00 ≈ 1.7×`,而 xdist 的 worker 启动开销把大部分吃掉了。
- `-n auto` 在 16 核上开 16 个 worker,启动成本 ≈ 全部理论收益,所以等于串行。
- `-n 2` 甚至更慢:两个 worker 的启动开销 + 任务分配不均(那条 2s 的用例落在谁头上谁就是瓶颈)。

**结论**:这套用例的耗时被**故障注入的等待时间**占满,不是被 CPU 或用例数量占满,
所以并行没有发挥空间。真要缩短,方向是**降低注入的延迟**(把 `?delay=3s` 调小、
把熔断冷却从 11s 调短),而不是加 worker。

这一节本身就是个面试点:**"我装了 xdist 并行"和"我实测了并行收益并解释了为什么只有 1.2×"是两个层次。**
前者是会用工具,后者是懂性能分析 —— 先看耗时分布(`pytest --durations`)找到瓶颈,
再判断该不该并行。

---

## 发现的风险 / 已知限制(有依据的评估)

1. **限流 / 熔断为每实例内存态** → 多副本部署下非全局,需引入 Redis 做全局计数。
2. `TrustedProxies` 未显式收窄 → 客户端可伪造 `X-Forwarded-For` 影响限流键(测试正是利用这点做隔离)。
3. **负载均衡无后端健康检查** → 坏节点仍被轮询。
4. `/metrics` 公开无鉴权;JWT 为 HS256 对称密钥,无 issuer/audience 校验与轮换。
5. **JWT 密钥长度不足**:`compose-dev-secret` 为 18 字节,低于 RFC 7518 §3.2 对 HS256 的
   建议下限(32 字节,即哈希输出长度)。密钥越短,离线暴力破解成本越低。
   这条不是人工审出来的 —— 是跑测试时 PyJWT 抛的 `InsecureKeyLengthWarning` 提示的,
   追进去核对 RFC 后确认成立。
   *当前仅影响 compose 开发环境(生产密钥走 GitHub Secrets 注入),但同一份配置常被直接
   照搬到测试/预发环境,值得收紧。*

> **为什么不直接把密钥改长**:该值同时被 `docker-compose.yml`、`e2e-py/framework/config.py`、
> `tests/framework/config.py`、`e2e/internal/config/config.go` 四处引用,改一处就得同步四处,
> 收益(消掉 15 行警告)配不上破坏面。**这类警告应当记录为发现,而不是用 `filterwarnings` 压掉** ——
> 压制会把真实的安全信号一起藏起来。

---

## 📑 面试准备文档(本目录)

| 文档 | 内容 |
|---|---|
| [`PERFORMANCE-QA.md`](PERFORMANCE-QA.md) | **性能测试**:做了什么 / 全套体系 / 为什么不做负载与 soak / 何时怎么做 / 30 秒话术 |
| [`DB-ASSERTION-QA.md`](DB-ASSERTION-QA.md) | **数据库断言**:为什么响应对≠数据对 / pytest 实现 / 数据清理 / 无状态网关为何没有 |
| [`面试笔记.md`](面试笔记.md) | **技术点编号笔记**:每条来自真实改动,侧重原理 |
| [`STAR.md`](STAR.md) | 用 STAR 讲这个项目(含「核心谈资」7 条) |
| [`INTERVIEW-QA.md`](INTERVIEW-QA.md) | 项目相关面试问答 |
| [`TESTING-GENERAL-QA.md`](TESTING-GENERAL-QA.md) | 通用测试面试题 |
| [`PYTHON-QA.md`](PYTHON-QA.md) | Python 面试题 |
| [`INTERVIEW-COMMUNICATION.md`](INTERVIEW-COMMUNICATION.md) | 沟通表达 |
| [`MOCK-INTERVIEW-PROMPT.md`](MOCK-INTERVIEW-PROMPT.md) | 模拟面试 |

---

## 简历写法(可直接改)

> **Go API 网关接口自动化框架(个人项目)** — Python / pytest / Docker / k6 / GitHub Actions
> - 从 0 设计**分层**接口自动化框架,覆盖鉴权/限流/负载均衡/熔断/超时/可观测/契约共 **21 条**用例(9 模块);数据驱动 + Allure 报告 + pytest-xdist 并行。
> - 利用**可控测试桩**做**故障注入**,稳定复现熔断状态机与上游超时(504),零 flaky。
> - 补充 **k6 限流压测**:14000 req/s 下限流精确放行 ~122 个、其余 141,625 全部 429、p95 2.4ms、零 5xx,thresholds 接入 CI 做性能门禁。
> - 引入 **JSON Schema 契约测试**;用 `docker compose` 一键起栈 + **GitHub Actions 两阶段执行**实现每次 PR 自动回归门禁;额外产出 4 条设计风险评估。

### STAR 口述模板

- **S(情境)**:一个承担鉴权/限流/负载均衡/熔断的 Go API 网关,改配置全靠手工验证,缺自动化回归。
- **T(任务)**:从 0 搭接口自动化框架,覆盖核心能力并接入 CI 做 PR 门禁。
- **A(行动)**:分层设计(config/client/jwt/断言解耦);数据驱动负向&安全用例;**可控桩做故障注入**稳定复现熔断/超时;JSON Schema 契约校验;k6 限流压测;按共享状态分并行/串行两阶段;docker-compose 一键起栈 + GitHub Actions。
- **R(结果)**:21 条用例覆盖 9 个模块,回归从手工数十分钟 → **自动约 14 秒**(非熔断阶段 3.4s + 熔断串行 11.3s,后者含 11s 冷却等待);做成 PR 门禁;额外产出 4 条有依据的风险评估。并行收益经实测仅约 1.2×(原因见 README"并行到底提速多少"一节)—— 两阶段拆分的价值在隔离,不在提速。

### 面试可能追问(预演)

- **怎么保证不 flaky?** 按 IP 隔离限流桶(每 client 独立源 IP);共享服务端状态的熔断用例串行独占并前后复位;故障场景用可控桩做到确定性。
- **踩过的坑?** 随机源 IP 的 f-string 多写了个逗号 → 生成非法 IP(元组)→ 网关解析不了 XFF 退回真实连接 IP → 所有 client 挤回同一个限流桶 → 限流用例一打空连累了不相干的路由用例。症状(路由测试 429)离病根(一个逗号)十万八千里,靠**打印真实值**一步定位。
