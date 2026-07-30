# e2e — 网关接口自动化测试框架

黑盒接口测试:**只通过 HTTP 打网关**,不 import 网关源码。这是一个独立的
Go module(`gateway-e2e`),可对着任意环境跑(本地 compose / 测试环境),
只需改一个环境变量。

## 目录结构

```
e2e/
├── go.mod                       # 独立 module
├── Makefile                     # make test / make vet / make perf
├── internal/
│   ├── config/config.go         # 配置层:从环境变量读 BASE_URL / JWT_SECRET
│   ├── client/client.go         # 客户端封装:带 token、发请求、解 JSON
│   └── helpers/token.go         # 夹具:签发 valid/expired/wrong/none 各类 token
├── tests/
│   ├── main_test.go             # TestMain:等 /readyz 就绪 + 共享夹具
│   ├── health_test.go           # 最简示例(公开接口,无需 token)
│   ├── auth_test.go             # ★ 参考范例:表驱动 + 子测试 + 断言
│   └── crud_test.go             # ★ 有状态范例:reset 隔离 + 生命周期
└── perf/load.js                 # k6 压测脚本
```

## 分层说明

1. **配置层**(`internal/config`)——所有环境相关项从环境变量读,换环境不改代码。
2. **客户端层**(`internal/client`)——把 net/http 样板封装成 `Get/Post/...`,自动带 token、解 JSON。
3. **夹具层**(`internal/helpers` + `main_test.go` 里的 `authClient/freshData`)——
   动态签发 token(解决 token 过期)、`_reset` 清空(解决测试隔离)。
4. **用例层**(`tests/*_test.go`)——按模块分文件,表驱动 + `t.Run` 子测试。
5. **断言/报告**——`testify`(`require` 断言失败即停,`assert` 继续);CI 里配 `gotestsum` 出 JUnit。

## 怎么跑

```bash
# 1. 先起被测环境(在仓库根目录)
docker compose up -d

# 2. 跑测试(在 e2e 目录)
make test          # 或:go test ./tests/... -count=1 -v

# 指定其它环境
make test BASE_URL=http://staging.example.com
```

`TestMain` 会先轮询 `/readyz`,就绪后才开始跑,避免"环境没起好"的假失败。

## 怎么加新模块的用例(照着 auth_test.go 的套路)

以限流为例,新建 `tests/ratelimit_test.go`:

```go
package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRateLimit_BlocksAfterBurst(t *testing.T) {
	c := authClient(1)
	got429 := false
	for i := 0; i < 200; i++ {
		resp, err := c.Get("/interaction/ping")
		require.NoError(t, err)
		if resp.Status == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	require.True(t, got429, "应在超过 burst 后出现 429")
}
```

全部模块均已实现(对应 [../docs/test-points.md](../docs/test-points.md)),可作参考:

| 文件 | 覆盖 | 关键手法 |
|---|---|---|
| `auth_test.go` | §1 | 表驱动负向 + alg=none 伪造 + 身份注入/防伪造 |
| `crud_test.go` | §9 | `/data` 单后端 + `_reset` 隔离 + 生命周期 |
| `routing_test.go` | §3 | `/storage` strip、`/interaction` 不 strip;查询透传;未知路由 404 |
| `ratelimit_test.go` | §2 | 独立源 IP 连发触发 429;不同 IP 隔离 |
| `loadbalance_test.go` | §4 | 连发 `/storage/ping`,断言 `X-Served-By` 覆盖多副本 |
| `breaker_test.go` | §5 | `?status=500` 打到阈值 → 短路(503 且无 `X-Served-By`)→ 冷却恢复 |
| `timeout_test.go` | §12 | `?delay=1s` 正常返回 |
| `ops_test.go` / `health_test.go` | §6 | 探针 / 指标 / debug 后门 |

> 说明:此 Go 版是**参考实现**;你的练习目标是自己写 `e2e-py/`(Python)。
> 跑 Go 版:`make test`(熔断用例含 ~11s 冷却,想跳过用 `go test -short ./tests/...`)。
