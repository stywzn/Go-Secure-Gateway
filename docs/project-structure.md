# 项目结构说明

Go-Secure-Gateway 是一个 Go 写的 **API 网关**:统一鉴权、限流、反向代理、
负载均衡、熔断,并暴露监控指标。它自身**无状态、无数据库**。

本文说明每个目录/文件的作用,并把测试点关联到具体文件。

## 顶层目录一览

```
Go-Secure-Gateway/
├── cmd/                    # 可执行程序入口(main 包)
│   ├── gateway/            #   网关本体
│   └── mockbackend/        #   测试用的可控假后端(测试桩)
├── internal/               # 网关内部实现(不对外暴露的包)
│   ├── config/             #   配置加载与校验
│   ├── middleware/         #   Gin 中间件:鉴权、限流
│   ├── proxy/              #   反向代理、负载均衡、熔断
│   └── metrics/            #   Prometheus 指标定义
├── configs/                # 配置文件(本地 / docker / nginx)
├── k8s/                    # Kubernetes 部署清单
├── observability/          # Prometheus / Grafana / Loki / ELK 配置
├── web/                    # 演示前端(nginx 托管)
├── docs/                   # 文档(OpenAPI、测试点、本文件)
├── e2e/                    # 接口自动化测试框架(独立 Go module)
├── .github/workflows/      # CI/CD 流水线
├── Dockerfile              # 网关镜像
├── Dockerfile.mock         # 假后端镜像
├── docker-compose.yml      # 一键起演示/测试环境
├── Makefile                # 常用命令
└── go.mod / go.sum         # Go 依赖
```

---

## 逐文件说明

### cmd/ —— 程序入口

| 文件 | 作用 |
|---|---|
| [cmd/gateway/main.go](../cmd/gateway/main.go) | 网关主程序:加载配置 → 注册公开接口(`/metrics`、`/healthz`、`/readyz`、可选 `/debug/token`)→ 组装中间件(限流+JWT)→ 按配置动态挂载各路由的反向代理 → 启动带超时的 HTTP Server + 优雅关闭。**这是理解全局的入口。** |
| [cmd/mockbackend/main.go](../cmd/mockbackend/main.go) | 可控假后端(测试桩)。默认 echo 回显请求;支持 `?status=`/`?delay=` 强制状态码/延迟;`/items` 内存 CRUD;`/_reset` 清空。**不是被测对象,是测试的"陪练"。** |

### internal/config/ —— 配置

| 文件 | 作用 |
|---|---|
| [internal/config/config.go](../internal/config/config.go) | 定义配置结构;`LoadConfig` 读 YAML、应用默认值、`JWT_SECRET` 环境变量覆盖、**前缀归一化**(补 `/`、去尾斜杠)、校验(密钥必填、路由非空、前缀去重)。 |
| [internal/config/config_test.go](../internal/config/config_test.go) | 上述逻辑的单元测试(§7 测试点)。 |

→ 关联测试点:**§7 配置模块**。

### internal/middleware/ —— 中间件

| 文件 | 作用 |
|---|---|
| [internal/middleware/jwt.go](../internal/middleware/jwt.go) | JWT 鉴权:校验 Bearer 头、强制 HS256、强制 `exp`;解析出 `user_id` 存入请求上下文,供代理透传下游。 |
| [internal/middleware/ratelimit.go](../internal/middleware/ratelimit.go) | 按 IP 令牌桶限流;后台 goroutine 定期清理空闲 IP(防内存泄漏)。 |
| `*_test.go` | 对应单元测试(§1、§2)。 |

→ 关联测试点:**§1 鉴权、§2 限流、§8 安全**。

### internal/proxy/ —— 代理核心

| 文件 | 作用 |
|---|---|
| [internal/proxy/proxy.go](../internal/proxy/proxy.go) | 每个路由一个 `ProxyEngine`:用负载均衡选后端、剥离前缀、注入 `X-User-Id`(并删除客户端伪造值)、设置转发头、记录指标、按 5xx 驱动熔断。 |
| [internal/proxy/loadbalancer.go](../internal/proxy/loadbalancer.go) | 轮询负载均衡,读路径无锁(atomic + 写时复制)。 |
| [internal/proxy/breaker.go](../internal/proxy/breaker.go) | 熔断器状态机:关闭→打开→半开(单探测)。 |
| [internal/proxy/response_writer.go](../internal/proxy/response_writer.go) | 包装 ResponseWriter 抓状态码,并用 `Unwrap` 保住流式/劫持能力。 |
| `*_test.go` | 单元/集成测试(§3、§4、§5)。 |

→ 关联测试点:**§3 路由代理、§4 负载均衡、§5 熔断、§9 CRUD、§12 协议超时**。

### internal/metrics/

| 文件 | 作用 |
|---|---|
| [internal/metrics/metrics.go](../internal/metrics/metrics.go) | 定义 Prometheus 指标:请求计数、延迟直方图。label 用**路由前缀**(非原始 path)防基数爆炸。 |

→ 关联测试点:**§6 可观测**。

### configs/ —— 配置文件

| 文件 | 作用 |
|---|---|
| [configs/config.yaml](../configs/config.yaml) | 本地运行默认配置(`debug:true`,路由指向 localhost)。 |
| [configs/config.docker.yaml](../configs/config.docker.yaml) | docker-compose 用:路由指向容器服务名。**四条路由各司其职**(见下)。 |
| [configs/nginx.conf](../configs/nginx.conf) | 可选的边缘 nginx(负载均衡多个网关副本)示例。 |

**config.docker.yaml 的四条路由(测试时按角色选用)**:

| 路由 | 后端 | strip | 用途 |
|---|---|---|---|
| `/interaction` | 单个 | 否 | echo,测鉴权 / 头透传 / 前缀不剥离 |
| `/storage` | 2 副本 | 是 | 测负载均衡(看 `X-Served-By` 轮转) |
| `/compute` | 单个 | 是 | 故障注入:`?status=`/`?delay=` 测熔断/超时 |
| `/data` | 单个 | 是 | 有状态 CRUD(与熔断路由隔离,互不干扰) |

### observability/ —— 监控与日志配置

| 路径 | 作用 |
|---|---|
| `prometheus/prometheus.yml` | Prometheus 抓取网关 `/metrics`。 |
| `grafana/provisioning/` | 自动配置 Grafana 数据源(Prometheus + Loki)和看板加载。 |
| `grafana/dashboards/gateway.json` | 预置网关看板(QPS、状态码、P95、错误率)。 |
| `promtail/promtail.yml` | Promtail 采集容器日志推给 Loki。 |
| `elk/filebeat.yml` | ELK 备用栈的日志采集(默认不启用)。 |

### web/ —— 演示前端

| 文件 | 作用 |
|---|---|
| [web/index.html](../web/index.html) | 单页 Demo:取 token、调路由、看 LB 轮转、演示限流。 |
| [web/nginx.conf](../web/nginx.conf) | nginx 托管前端并把 `/api/*` **同源**反代到网关(免 CORS)。 |

### k8s/ —— 部署清单

`configmap.yaml`(配置+Secret)、`deployment.yaml`(2 副本+探针+安全上下文+资源限制)、`service.yaml`(NodePort 暴露)。

### .github/workflows/ —— CI/CD

`ci.yml`(测试+lint+构建)、`release.yml`(打 tag 推镜像到 GHCR)、`deploy.yml`(手动部署到 K8s)。

### e2e/ —— 接口自动化测试框架(独立 module)

见 [e2e/README.md](../e2e/README.md)。这是你要重点建设的部分,**通过 HTTP 黑盒测网关**,
与 `internal/**/*_test.go` 的白盒单元测试是不同层次。

---

## 两层测试的关系

| | 白盒单元测试 | 黑盒接口自动化(e2e) |
|---|---|---|
| 位置 | `internal/**/*_test.go` | `e2e/`(独立 module) |
| 方式 | 直接调 Go 函数 | 发 HTTP 请求打网关 |
| 依赖运行环境 | 不需要 | 需要 `docker compose up` |
| 覆盖 | 单个组件逻辑(限流算法、熔断状态机…) | 端到端行为(鉴权链路、LB、CRUD 时序…) |
| 何时跑 | 每次提交(CI 的 `go test`) | 对着起好的环境跑 |

两者互补:单元测试保证"零件对",e2e 保证"组装起来对"。

---

## 请求生命周期(按经过顺序讲)

一个请求 `GET /storage/files/1`(带 JWT)从进来到出去,**依次**经过:

1. **进入 Gin**([main.go](../cmd/gateway/main.go))。若命中 `/healthz`、`/readyz`、`/metrics`、`/debug/token` 这类**公开接口**,直接处理返回,不走下面的保护链。
2. **限流中间件**([ratelimit.go](../internal/middleware/ratelimit.go)):按客户端 IP 取令牌。**没令牌 → 直接 429**,请求到此为止。(注意顺序:限流在鉴权**之前**)
3. **鉴权中间件**([jwt.go](../internal/middleware/jwt.go)):校验 `Bearer` 头 → HS256 → 未过期 → 签名正确。**任一不过 → 401**;通过则解析出 `user_id` 存进请求上下文。
4. **路由匹配**:命中 `/storage` 对应的 `ProxyEngine`。
5. **代理引擎处理**([proxy.go](../internal/proxy/proxy.go)),内部顺序:
   1. **熔断器判断** `Allow()`([breaker.go](../internal/proxy/breaker.go)):若该路由熔断**打开 → 直接 503 短路**,不碰后端。
   2. **负载均衡选后端** `lb.Next()`([loadbalancer.go](../internal/proxy/loadbalancer.go)):storage-a / storage-b 轮流。
   3. **改写请求**(director):剥离前缀 `/storage`、注入 `X-User-Id`(**先删客户端伪造值**)、补转发头。
   4. **转发到后端**,`statusWriter` 抓回状态码。
   5. **记录指标**([metrics.go](../internal/metrics/metrics.go)):按**路由前缀**打 label。
   6. **回写熔断器**:后端 5xx → `RecordFailure`,否则 `RecordSuccess`。
6. **响应返回**客户端。

> 一句话记忆:**限流 → 鉴权 → 路由 → (熔断闸门 → 负载均衡 → 改写转发 → 指标 → 回写熔断)**。

## 状态流转(两个有状态组件)

- **熔断器**(按路由,服务端共享):
  `closed` --连续 5 次 5xx--> `open` --冷却 10s--> `half-open` --探测成功--> `closed`;`half-open` --探测失败--> `open`。
- **限流令牌桶**(按 IP,每实例内存):令牌以 `rps` 速率补充、`burst` 为上限;空闲 IP 由后台 TTL 清理。**每实例独立 → 多副本下非全局**(要全局需 Redis)。
