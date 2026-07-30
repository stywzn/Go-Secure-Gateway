# Go-Secure-Gateway 接口自动化测试框架

基于 **Python + pytest** 从 0 搭建的分层接口自动化框架,被测系统是一个 Go 微服务 API 网关(鉴权 / 限流 / 负载均衡 / 熔断 / 可观测)。

覆盖四条"面试高含金量"主线:**接口自动化 + CI 门禁 · 故障注入 · 性能压测 · 契约 + 监控断言**。

---

## 1. 快速开始

```bash
# 1) 在项目根目录起被测栈(网关 + 可控 mock 后端)
docker compose up --build -d --wait

# 2) 装依赖
pip install -r tests/requirements.txt

# 3) 跑用例(两阶段,见下)
cd tests
pytest -m "not breaker" -n auto        # 阶段 A:并行(快)
pytest -m "breaker" -p no:xdist        # 阶段 B:熔断串行(约 30s,含冷却等待)

# 4) 看 Allure 报告(可选)
pytest --alluredir=../allure-results
allure serve ../allure-results
```

> 环境切换零改代码:`GATEWAY_BASE_URL` 指向别的实例即可(默认 `http://localhost:8080`,经 nginx 用 `http://localhost:8088/api`)。

### 为什么分两阶段跑?(这是个加分点)

| 用例类型 | 共享状态? | 能否并行 | 原因 |
|---|---|---|---|
| 鉴权/路由/负载均衡/限流/契约/监控 | 否(**按 IP 隔离**) | ✅ 并行 | 每个 client 用**独立源 IP**(`X-Forwarded-For`),限流桶天然隔离 |
| 熔断 | 是(**按路由的服务端共享状态**) | ❌ 串行 | 熔断器打开会影响所有打同一路由的请求,必须独占 + 冷却复位 |

> 面试可讲:"我先分析了**哪些状态是按客户端隔离、哪些是服务端全局共享**,据此设计并行/串行策略 —— 既跑得快,又不会因为共享状态互相污染出现 flaky。"

---

## 2. 分层架构

```
tests/
├─ framework/               # 核心层(与用例解耦,可复用)
│  ├─ config.py             #   环境配置:全部走环境变量,切环境零改代码
│  ├─ client.py             #   HTTP 会话封装:鉴权头 / 独立源IP / 日志 / Allure 步骤
│  ├─ jwt_utils.py          #   JWT 构造工厂:正向 + 6 类负向/安全 token(算法混淆…)
│  ├─ assertions.py         #   领域断言:状态码 / 轮询分布 / 指标增长
│  └─ prometheus.py         #   /metrics 文本解析,支撑"监控断言"
├─ data/
│  └─ auth_cases.yaml       # 数据驱动:鉴权负向用例,加一行 = 加一个攻击面
├─ testcases/               # 按被测模块组织
│  ├─ test_auth.py          #   模块1 鉴权(数据驱动 + 安全)
│  ├─ test_ratelimit.py     #   模块2 限流(令牌桶 + 多IP隔离 + 中间件顺序)
│  ├─ test_routing.py       #   模块3 路由/反向代理(前缀剥离 + 身份注入/防伪造)
│  ├─ test_loadbalance.py   #   模块4 负载均衡(轮询分布)
│  ├─ test_circuitbreaker.py#   模块5 熔断状态机 ★故障注入核心
│  ├─ test_timeout.py       #   延迟注入 + 记录"上游无独立超时"风险
│  └─ test_ops_metrics.py   #   模块6 探针 / 后门 / ★监控断言
├─ contract/
│  └─ test_openapi_contract.py  # ★契约测试:用 OpenAPI schema 校验响应
├─ perf/                    # ★性能/限流压测(k6 + locust)
├─ conftest.py              # 全局 fixtures:环境探活 / client / 存储复位
└─ pytest.ini               # 标记 / 报告 / 日志配置
```

**分层的价值(面试)**:用例只表达"业务意图",样板(鉴权、源 IP、日志、断言)全部下沉到 framework;新增用例改动小、可读性高;换被测环境只动 config。

---

## 3. 四条主线怎么体现(逐条对应亮点)

### ① 接口自动化 + CI 门禁
- 分层框架 + **数据驱动**(`auth_cases.yaml`)+ **Allure 报告** + **pytest-xdist 并行**。
- `.github/workflows/e2e-tests.yml`:**一键 `docker compose up` 起被测栈 → 分阶段跑 → 传 Allure**,挂在 push/PR 上做**门禁**。

### ② 故障注入(最能体现深度)
- **熔断状态机**:用可控后端 `?status=503` 连续打到阈值 → 断言 `closed→open→half-open→closed` 全流程;用"**响应无 `X-Served-By`**"精确区分"网关短路"与"后端错误"。
- **延迟注入**:`?delay=` 触发超时;并把"**上游无独立超时**"作为**发现的风险**用 `xfail` 记录。
- 讲法:"这些异常路径**几乎无法用真实后端稳定复现**,靠**可控测试桩**才能做到 100% 确定性、零 flaky。"

### ③ 性能 / 限流压测
- k6 以 4× 阈值加压,**断言放行流量 p95 与 429 比例**,退出码可接 CI 当性能门禁。
- 对比单 IP / 多 IP 两种画像,佐证限流键设计。

### ④ 契约 + 监控断言
- **契约**:直接拿 `docs/openapi.yaml` 的 schema 用 `jsonschema` 校验真实响应 —— 面向契约而非面向实现,接口漂移自动暴露。
- **监控断言**:发请求前后读 `/metrics`,断言 `gateway_http_requests_total` 计数器**确实 +1**,并验证 label 用**路由前缀而非原始 path**(高基数防护)。把"功能对不对"和"监控采得准不准"一起测。

---

## 4. 简历写法(直接可用)

> **Go API 网关接口自动化框架(个人项目)** — Python / pytest / Docker / GitHub Actions
> - 基于 OpenAPI 从 0 设计**分层**接口自动化框架,覆盖**鉴权、限流、负载均衡、熔断、超时、可观测**六大模块共 40+ 用例;数据驱动 + Allure 报告 + pytest-xdist 并行。
> - 利用**可控测试桩**做**故障注入**,稳定复现熔断状态机与超时,零 flaky;补充 **k6 限流压测**(p95 与 429 比例断言)。
> - 引入 **OpenAPI 契约测试**与 **Prometheus 监控断言**,把接口测试从"面向实现"升级到"面向契约 + 可观测"。
> - 用 `docker compose` 一键起被测栈并接入 **GitHub Actions**,实现每次 PR 自动回归门禁;评估并记录 4 项设计风险(上游无超时、TrustedProxies、/metrics 无鉴权、LB 无健康检查)。

### STAR 口述模板

- **S**:一个承担鉴权/限流/负载均衡/熔断的 Go API 网关,改配置全靠手工验证,缺自动化回归。
- **T**:从 0 搭接口自动化框架,覆盖核心能力并接入 CI 做 PR 门禁。
- **A**:分层设计(config/client/jwt/断言解耦);数据驱动负向&安全用例;**可控桩做故障注入**稳定复现熔断/超时;OpenAPI **契约校验** + Prometheus **监控断言**;docker-compose 一键起栈 + GitHub Actions 两阶段执行 + Allure。
- **R**:40+ 用例覆盖 6 模块,回归从手工数十分钟→自动数十秒,并行提速约 N×;做成 PR 门禁;额外产出 4 条**有依据的风险评估**。

---

## 5. 面试高频追问(预演)

**Q:怎么保证测试不 flaky?**
A:①区分**按 IP 隔离**与**服务端共享**状态,并行的用独立源 IP,共享的(熔断)串行独占并前后复位;②有状态用例前 `POST /data/_reset` 清库;③故障场景用**可控桩**做到确定性,不依赖真实后端的"心情"。

**Q:熔断怎么测的?**
A:连续 `?status=503` 打到阈值使其打开,再断言后续请求被**网关短路**(503 且**无 `X-Served-By`**,证明没到后端);等冷却进入半开,放行**一个探测**,探测成功→关闭、探测失败→立即重开,完整走一遍状态机。

**Q:契约测试和普通断言区别?**
A:普通断言把"响应长什么样"写死在用例里,实现和用例容易一起漂;契约测试以 **OpenAPI 为真相源**校验结构,接口一旦与契约不符立刻暴露,维护成本更低。

**Q:发现过什么问题 / 风险?**
A:4 条(见简历):**上游调用无独立超时**(慢后端拖到 WriteTimeout)、`TrustedProxies` 未显式配置可能导致限流失效或被 XFF 绕过、`/metrics` 公开无鉴权、负载均衡无健康检查坏节点仍被轮询。每条都有复现依据和改进建议。

**Q:为什么选它当被测系统?**
A:它把常见后端能力(鉴权/限流/LB/熔断/可观测)浓缩在一个边界清晰的服务里,且自带**可控测试桩**,既能覆盖足够多的测试类型,又不至于庞大到无法完全掌控 —— 适合把每个点都讲透。

---

## 6. 常用命令

```bash
pytest -m security          # 只跑安全用例(算法混淆/身份伪造/后门)
pytest -m "auth or routing" # 按模块跑
pytest -m breaker -p no:xdist  # 熔断(串行)
pytest -k loadbalance -v    # 按名字筛
pytest -n auto -m "not breaker"  # 并行全量(除熔断)
```
