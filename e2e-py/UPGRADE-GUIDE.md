# 框架升级指南(以后想升级看这份)

这份讲**从当前水平往上升的完整路线 + 每一步用什么技术栈 + 怎么做 + 达到什么效果**。

---

## 先认清当前所处的档位

| 档位 | 特征 | 对应本仓库 |
|---|---|---|
| **L1 求职作品(基础)** | 分层、fixture、数据驱动、CI 门禁、基本压测 | `e2e-py/`(你亲手搭的) |
| **L2 求职作品(进阶)** | + DRY 客户端、会话探活、契约测试、指标解析、双压测、日志/Allure 步骤 | `tests/`(已达到) |
| **L3 Ultra(准生产)** | + 全局限流(Redis)、分布式/多副本测试、契约自动生成、报告发布、安全专项 | ← 本指南目标 |
| **L4 生产级企业框架** | + 共享库、报告门户、flaky 看板、多环境/密钥管理、团队协作设施 | 需要团队+时间沉淀 |

> `tests/` 已在 L2;`e2e-py/` 在 L1(但概念齐)。下面是 L1→L2→L3 每一步怎么做。

---

## 一、客户端健壮性(L1→L2,e2e-py 可照做)

- **目的**:消除重复、可观测、抗抖动。
- **技术栈**:`requests.Session`、`HTTPAdapter` + `urllib3.util.retry.Retry`、`logging`、`allure`。
- **怎么做**:
  1. **DRY**:抽单一 `request(method, path, **kwargs)` 核心,`get/post/put/delete` 委托它(`tests/` 已这样)。
  2. **重试(谨慎)**:给 session mount 一个 `Retry`,但**只重试连接/读错误,`status=0` 不重试 HTTP 状态码**——否则会把熔断/超时想要的 503/504 也重试掉,毁掉故障注入用例。
  3. **日志 + Allure 步骤**:每次请求打 `→/←` 日志,并 `allure.attach` 请求/响应,报告里点开每条用例能看全链路。
- **效果**:样板集中一处;网络抖动不误伤;报告可追溯到每个请求。

## 二、会话探活 / fail-fast(L1→L2)

- **目的**:栈没起时快速失败给指引,而不是一屏红。
- **技术栈**:pytest `scope="session", autouse=True` fixture + `pytest.exit`。
- **怎么做**:session 级探 `/healthz`,失败就 `pytest.exit("先 docker compose up ...")`(`tests/conftest.py` 已实现)。
- **效果**:环境问题 3 秒内定位并给操作指引。

## 三、质量闸加固(L1→L2)

- **技术栈**:`pytest` 内置 + `pytest-randomly`。
- **怎么做**:`--strict-markers`(打错标记报错)、`filterwarnings`(消噪音)、`pytest-randomly`(**随机用例顺序**,能抓出"隐藏的用例间依赖")。
- **效果**:拼写错/隐藏耦合尽早暴露,报告干净。

## 四、契约测试进阶(L2→L3)★

- **目的**:从"内联手写 schema"升到"以 OpenAPI 为唯一真相源",甚至自动生成用例。
- **技术栈**:`prance` / `openapi-core`(解析 OpenAPI)、**`schemathesis`**(基于 OpenAPI 自动生成契约+模糊测试)、`hypothesis`(property-based)。
- **怎么做**:
  - 基础:用 `prance` 从 `docs/openapi.yaml` 提取每个响应的 schema,喂 `jsonschema` 校验。
  - 进阶:`schemathesis run docs/openapi.yaml --base-url=...` **自动**对每个接口生成大量边界/异常输入,校验响应是否符合契约。
- **效果**:契约一改测试自动跟;自动发现你没想到的边界 bug。

## 五、测试数据管理(L2→L3)

- **目的**:数据构造与用例解耦、可复用、可随机。
- **技术栈**:`factory_boy`(数据工厂)、`faker`(造假数据)。
- **怎么做**:为 CRUD 资源写工厂,用例里 `ItemFactory()` 造数据,而不是手写 dict。
- **效果**:数据构造集中、可参数化、大量用例不重复造数。

## 六、报告发布 + 趋势(L2→L3)

- **技术栈**:`allure`、`peaceiris/actions-gh-pages`。
- **怎么做**:CI 里 `allure generate` 出 HTML,用 `actions-gh-pages` 发到 `gh-pages` 分支;保留 `history/` 目录可看**通过率趋势**。
- **效果**:每次跑完有**在线报告链接**,可放简历/PR;趋势图看质量走向。

## 七、性能压测进阶(L2→L3)

- **技术栈**:`k6`(stages/soak/`__ENV`)、`Locust`(Python 替代)、Grafana(k6→Prometheus 可视化)。
- **怎么做**:k6 脚本 base_url 走 `__ENV`;`options.stages` 做阶梯加压;加长时间 soak;`thresholds` 接 CI。
- **效果**:一套脚本压多环境;看限流在渐进加压/长稳下的真实表现。

## 八、★ 架构级:全局限流(Redis)+ 分布式测试(L3)

这是**唯一还没实现、也是最能体现深度**的一步。

- **目的**:让限流/熔断从"每实例"升级为"全局",并补对应测试。
- **技术栈(被测侧)**:`Redis` + Go `go-redis`;固定窗口用 `INCR`+过期,或 **Lua 脚本**实现原子令牌桶。`docker compose` 加 redis 服务 + 网关多副本。
- **技术栈(测试侧)**:`pytest`(多客户端并发)、`docker compose --scale`/`k8s` 多副本、`k6`(跨副本压)。
- **怎么做**:
  1. 网关限流中间件改成读写 Redis 全局计数;compose 加 `redis` + 网关 `deploy.replicas: 2`,前面挂个 nginx/LB。
  2. 新增测试点:**多副本同时压,断言总放行量受全局阈值约束**(而不是每实例各放一份);Redis 挂掉时的**降级策略**(失败开放 vs 失败关闭)也要测。
- **效果**:限流真正全局;拿到"分布式一致性/降级"这类高级测试点,面试可讲很久。

## 九、分布式 / 高可用 / 故障注入(L3→L4)

- **技术栈**:`k8s`(多副本、探针)、`chaos-mesh` / `toxiproxy`(注入网络延迟/丢包/分区)、`docker compose` 杀容器。
- **怎么做**:杀一个副本验证流量重路由 + 在途请求不丢(优雅关闭 drain);toxiproxy 注入弱网验证熔断/重试;chaos-mesh 做网络分区。
- **效果**:覆盖真实生产故障场景(右移),不只是 happy path。

## 十、安全 & 契约驱动(L3→L4)

- **技术栈**:`OWASP ZAP`(被动/主动扫描)、`pact`(消费者驱动契约)、`schemathesis`(已含安全用例)。
- **怎么做**:CI 里接 ZAP 扫常见漏洞;服务间用 pact 保证契约不漂。
- **效果**:安全左移;跨服务契约有保障。

---

## 技术栈总表(按层)

| 层 | 技术栈 |
|---|---|
| 测试框架 | Python · pytest · pytest-xdist(并行)· pytest-randomly(乱序抓依赖)· pytest-rerunfailures(谨慎用) |
| HTTP / 客户端 | requests · urllib3 Retry · HTTPAdapter |
| 鉴权/数据 | PyJWT · PyYAML · factory_boy · faker |
| 契约 | jsonschema · prance/openapi-core · **schemathesis** · hypothesis · pact |
| 报告 | Allure · pytest-html · GitHub Pages(actions-gh-pages) |
| 监控断言 | Prometheus 文本解析 · prometheus-api-client(PromQL) |
| 压测 | **k6**(stages/soak/thresholds)· Locust · Grafana |
| 被测侧(架构升级) | Redis + go-redis(全局限流)· Lua |
| 分布式/故障注入 | Docker Compose scale · Kubernetes · chaos-mesh · toxiproxy |
| 安全 | OWASP ZAP |
| CI/CD | GitHub Actions(矩阵/缓存/定时/通知)· 可观测:OpenTelemetry |

---

## 建议的升级顺序(性价比)

1. **e2e-py 补齐到 L2**:DRY 客户端 + 会话探活 + `--strict-markers`/`filterwarnings`(照 `tests/` 抄)。
2. **契约升级**:接 `schemathesis`(投入小、亮点大)。
3. **报告发布**:Allure → GitHub Pages(拿在线链接)。
4. **架构级**:Redis 全局限流 + 分布式测试(投入大、最能体现深度,面试尖刺)。
5. **故障注入/安全**:toxiproxy 弱网、ZAP 安全扫描。

> 一句话:**先把工程实践补满(L2),再上一个架构级亮点(Redis 全局限流 + 分布式测试)冲 L3**,就是一份很能打的作品。
