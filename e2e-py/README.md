# Go-Secure-Gateway 接口自动化测试框架

基于 **Python + pytest** 从 0 手搭的分层接口自动化框架。被测系统(SUT)是一个 Go 编写的 API 网关,具备**鉴权 / 限流 / 反向代理 / 负载均衡 / 熔断 / 上游超时 / 可观测**能力,后端为一组**可控测试桩**(能按需返回任意状态码、注入延迟、支持内存 CRUD)。

覆盖四条"高含金量"主线:**接口自动化 + CI 门禁 · 故障注入 · 性能压测 · 契约 + 监控断言**。

---

## 亮点

- **分层框架**:配置 / HTTP 客户端 / JWT 工具 / 断言解耦,用例只写业务意图。
- **数据驱动**:鉴权负向用例外置到 `data/auth_cases.yaml`,加一行 = 加一个攻击面,不动代码。
- **故障注入(最见深度)**:用可控测试桩 `?status=` / `?delay=` 稳定复现**熔断状态机**(closed→open→half-open→closed)与**上游超时(504)**,这些异常路径几乎无法用真实后端稳定复现,靠桩做到 100% 确定、零 flaky。
- **性能压测**:k6 以远超阈值的并发压限流,断言 429 触发 + p95 延迟 + 无 5xx,thresholds 不达标即失败,**接入 CI 当性能门禁**。性能测试的**取舍/全套体系/为什么不做负载与 soak/面试话术**见 [`PERFORMANCE-QA.md`](PERFORMANCE-QA.md)。
- **契约测试**:用 JSON Schema 校验真实响应结构,面向契约而非面向实现。
- **并行 + 隔离**:除熔断外全部 `pytest-xdist` 并行(约 3× 提速);破坏性的熔断用例单独串行,避免共享服务端状态串扰。
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
| 监控 `ops` | 探针 / `/metrics` 指标存在 / debug token |
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
│  └─ jwt_utils.py     #   JWT 工厂:合法 + 过期/错签名/算法混淆等负向 token
├─ data/
│  └─ auth_cases.yaml  # 数据驱动:鉴权用例
├─ testcases/          # 按被测模块组织的用例
├─ perf/
│  └─ ratelimit.js     # k6 限流压测
├─ conftest.py         # 全局 fixtures:环境探活 / client / 存储复位
└─ pytest.ini          # 标记 / 路径配置
```

## 为什么熔断用例要单独串行

熔断器是**按路由的服务端共享状态**。若熔断用例(往 `/compute` 打 5xx 打开熔断)与超时用例(也打 `/compute`)并行,超时用例会撞上 503 → flaky。因此按"是否共享服务端状态"把用例分成**可并行**与**必串行**两组:前者 `-n auto` 并行提速,后者独占串行。这是框架里"识别隐藏耦合、保证测试隔离"的关键设计。

## 发现的风险 / 已知限制(有依据的评估)

1. **限流 / 熔断为每实例内存态** → 多副本部署下非全局,需引入 Redis 做全局计数。
2. `TrustedProxies` 未显式收窄 → 客户端可伪造 `X-Forwarded-For` 影响限流键(测试正是利用这点做隔离)。
3. **负载均衡无后端健康检查** → 坏节点仍被轮询。
4. `/metrics` 公开无鉴权;JWT 为 HS256 对称密钥,无 issuer/audience 校验与轮换。

---

## 📑 面试准备文档(本目录)

| 文档 | 内容 |
|---|---|
| [`PERFORMANCE-QA.md`](PERFORMANCE-QA.md) | **性能测试**:做了什么 / 全套体系 / 为什么不做负载与 soak / 何时怎么做 / 30 秒话术 |
| [`STAR.md`](STAR.md) | 用 STAR 讲这个项目 |
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
- **R(结果)**:21 条用例覆盖 9 个模块,回归从手工数十分钟 → 自动约 16 秒(并行 4.6s + 熔断串行 11s);并行提速约 3×;做成 PR 门禁;额外产出 4 条有依据的风险评估。

### 面试可能追问(预演)

- **怎么保证不 flaky?** 按 IP 隔离限流桶(每 client 独立源 IP);共享服务端状态的熔断用例串行独占并前后复位;故障场景用可控桩做到确定性。
- **踩过的坑?** 随机源 IP 的 f-string 多写了个逗号 → 生成非法 IP(元组)→ 网关解析不了 XFF 退回真实连接 IP → 所有 client 挤回同一个限流桶 → 限流用例一打空连累了不相干的路由用例。症状(路由测试 429)离病根(一个逗号)十万八千里,靠**打印真实值**一步定位。
