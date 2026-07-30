# 测试框架架构图(面试讲解版)

三张图,对应三个层面讲:**怎么分层**、**怎么做故障注入**、**怎么在 CI 里跑**。

---

## 图 1 · 分层架构

从下往上:被测系统 → 框架核心(可复用)→ 支撑(夹具/配置/数据)→ 用例 → 执行与报告。
**核心思想:用例只写"业务意图",样板(鉴权、源 IP、断言、指标解析)全部下沉到 framework。**

```mermaid
flowchart TB
    subgraph EXEC["⑤ 执行与报告层"]
        direction LR
        XDIST["pytest-xdist 并行"]
        ALLURE["Allure 报告"]
        CI["GitHub Actions · PR 门禁"]
    end

    subgraph CASES["④ 用例层 testcases/ + contract/"]
        direction LR
        A1["test_auth<br/>鉴权+安全"]
        A2["test_ratelimit<br/>限流"]
        A3["test_routing<br/>路由/透传"]
        A4["test_loadbalance<br/>轮询"]
        A5["test_circuitbreaker ★<br/>熔断状态机"]
        A6["test_timeout<br/>超时注入"]
        A7["test_ops_metrics<br/>探针/监控断言"]
        A8["test_openapi_contract<br/>契约"]
    end

    subgraph SUPPORT["③ 支撑层"]
        direction LR
        CONF["conftest.py<br/>preflight / client / auth_client / reset_storage"]
        INI["pytest.ini<br/>markers / 日志"]
        DATA["data/auth_cases.yaml<br/>数据驱动负向用例"]
    end

    subgraph CORE["② 框架核心层 framework/ (与用例解耦)"]
        direction LR
        CFG["config.py<br/>环境配置(全走env)"]
        CLI["client.py<br/>HTTP封装 + 独立源IP"]
        JWT["jwt_utils.py<br/>JWT工厂(正向+6类攻击)"]
        ASSERT["assertions.py<br/>领域断言(轮询/状态)"]
        PROM["prometheus.py<br/>/metrics 解析"]
    end

    subgraph SUT["① 被测系统 (docker compose)"]
        direction LR
        GW["Go 网关<br/>鉴权→限流→代理→LB→熔断→/metrics"]
        MOCK["可控 Mock 后端<br/>?status ?delay CRUD /_reset"]
        OBS["Prometheus / Grafana / Loki"]
    end

    EXEC --> CASES --> SUPPORT --> CORE
    CORE -->|HTTP| GW
    GW --> MOCK
    GW --> OBS
```

---

## 图 2 · 故障注入 & 熔断判定(最能讲深度)

难点:熔断/超时这类异常路径,**真实后端几乎无法稳定复现**。用**可控 Mock**
主动制造故障 → 100% 确定性、零 flaky。判定技巧:**网关短路 = 503 且无
`X-Served-By`**(请求没到后端),以此区分"熔断"与"后端错误"。

```mermaid
sequenceDiagram
    autonumber
    participant T as 用例 (pytest)
    participant G as Go 网关
    participant M as Mock 后端

    Note over T,M: ① 连续打满阈值的 5xx
    loop 5 次
        T->>G: GET /interaction/ping?status=503 (带 JWT)
        G->>M: 转发
        M-->>G: 503 (带 X-Served-By)
        G-->>T: 503,熔断失败计数 +1
    end
    Note over G: 达阈值 → 熔断 open

    Note over T,M: ② 打开后:网关直接短路
    T->>G: GET /interaction/ping
    G-->>T: 503 且【无 X-Served-By】(未到后端)
    Note over T: 断言短路: status==503 && "X-Served-By" not in headers

    Note over T,M: ③ 冷却后半开,探测成功 → 关闭
    T->>G: (sleep 冷却) GET /interaction/ping
    G->>M: 放行一个探测
    M-->>G: 200 (带 X-Served-By)
    G-->>T: 200 → 熔断 closed
```

---

## 图 3 · CI 两阶段执行(体现 flaky 治理)

先分析**哪些状态按 IP 隔离、哪些是服务端共享**,据此决定并行/串行:

```mermaid
flowchart LR
    S["docker compose up --wait<br/>起网关+可控后端"] --> P["conftest 预检<br/>GET /healthz"]
    P --> A["阶段 A · 并行<br/>pytest -n auto -m 'not breaker'<br/>每用例独立源IP → 限流桶隔离"]
    A --> B["阶段 B · 串行<br/>pytest -m breaker -p no:xdist<br/>独占路由 + 冷却复位"]
    B --> R["Allure 报告<br/>+ CI 门禁(push/PR)"]

    classDef parallel fill:#1f6feb22,stroke:#1f6feb;
    classDef serial fill:#d2992222,stroke:#d29922;
    class A parallel;
    class B serial;
```

---

### 一分钟讲解脚本

> "框架分五层:最上是**用例**,只表达业务意图;往下是**支撑层**(夹具、数据驱动)
> 和**框架核心**(HTTP 封装、JWT 工厂、领域断言、指标解析),样板全部下沉,所以
> 加用例改动小、换环境只动 config。被测系统用 docker compose 一键起,包含一个
> **可控 Mock 后端**——这是关键,它让我能用 `?status`/`?delay` 主动注入故障,把
> 熔断、超时这些**真实后端难复现**的路径做到确定性、零 flaky。执行上我按状态是否
> 共享分了两阶段:大多数用例按 IP 隔离所以**并行**跑得快,熔断是服务端共享状态所以
> **串行**独占并冷却复位,最后出 Allure 报告并挂在 PR 上做门禁。"
