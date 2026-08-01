# Redis 全局限流 + 分布式测试(L3 Ultra)

这次给网关新增了**可选的全局限流模式**,把限流从"每实例"升级为"跨副本全局",并配套了分布式测试。**默认不启用**(默认仍是内存令牌桶),对现有 e2e-py / tests 用例零影响。

---

## 一、解决什么问题(背景)

原来的限流是**每实例内存令牌桶**:每个网关副本各有一套桶。多副本部署时,同一个 IP 在 N 个副本上就能各放一份 → **有效限流阈值 ≈ ×N**,不是真正的全局限流。

**新增方案**:把计数放到 **Redis**,所有副本共享同一个计数器 → 单 IP 的配额在**全局**范围内统一约束。

## 二、新增 / 改动的文件

| 文件 | 类型 | 作用 |
|---|---|---|
| `internal/middleware/ratelimit_redis.go` | **新增** | `RedisLimiter`:用 Redis + Lua 固定窗口脚本做全局限流 |
| `internal/middleware/ratelimit.go` | 改 | 抽出 `Limiter` 接口;给内存限流器加 `Allow(ip)`;中间件改收 `Limiter`(模式可切换) |
| `internal/config/config.go` | 改 | `RateLimitConfig` 加 `mode` / `redis_addr` / `window_seconds`;默认 `memory`;支持 `RATE_LIMIT_MODE` / `REDIS_ADDR` 环境变量覆盖 |
| `cmd/gateway/main.go` | 改 | 按 `mode` 选择内存或 Redis 限流器 |
| `configs/config.distributed.yaml` | **新增** | 分布式 profile 用的配置(redis 模式、10s 窗口、全局阈值 20) |
| `docker-compose.yml` | 改 | 新增 `redis` + `gateway-a`(:8082)+ `gateway-b`(:8083),归到 **`distributed` profile**;基础网关打标签 `go-secure-gateway:local` 供副本复用 |
| `tests/testcases/test_distributed.py` | **新增** | 2 条分布式测试 |
| `tests/pytest.ini` | 改 | 注册 `distributed` 标记 |
| `go.mod` / `go.sum` / `vendor/` | 改 | 引入 `github.com/redis/go-redis/v9` |

## 三、架构

```
                       ┌─────────────┐
  单个 IP 猛打  ─┬──▶  │ gateway-a   │─┐
   (轮流两副本)  │     │  :8082      │ │   INCR ratelimit:<ip>
                 │     └─────────────┘ ├──────────────▶  ┌────────┐
                 │     ┌─────────────┐ │   (原子 Lua)     │ Redis  │
                 └──▶  │ gateway-b   │─┘                  └────────┘
                       │  :8083      │      两副本共享同一个计数器
                       └─────────────┘      → 全局阈值统一约束
```

**工作原理**:每个请求执行一段 **Lua 脚本**(`INCR` 计数 + 首次 `PEXPIRE` 设窗口),原子地判断"本窗口内该 IP 是否超过阈值"。因为是单条 Lua,多副本并发下 `INCR+过期+比较` 也是原子的。

**降级策略**:Redis 不可达时**失败开放(fail-open)**——不阻断流量(可用性优先),并打日志。生产也可选失败关闭,是个可测的策略点。

## 四、测试点

| # | 测试点 | 用例 | 断言 |
|---|---|---|---|
| 1 | **全局阈值约束** | `test_global_limit_holds_across_replicas` | 单 IP 跨两副本打 100 次,放行数 ≈ 全局阈值(~20),远小于每实例独立时的 ~2×(40) |
| 2 | **跨副本共享状态** | `test_quota_used_on_one_replica_is_seen_by_the_other` | 副本 A 上打满配额,副本 B 立刻返回 429 → 证明计数是共享的 |
| 3 | Redis 降级(fail-open)| (可扩展)| 停掉 redis,请求仍放行(不阻断)——验证降级策略 |
| 4 | 窗口重置 | (可扩展)| 等过一个窗口,配额恢复 |

## 五、怎么跑

```bash
# 1) 起分布式栈(redis + 两个网关副本)
docker compose --profile distributed up -d

# 2) 跑分布式测试(在 tests/ 下)
cd tests
pytest -m distributed -v        # 期望 2 passed
```

不需要时收掉副本(不影响基础栈):
```bash
docker compose stop gateway-a gateway-b redis
```

> 默认栈(`docker compose up -d`)**不含** redis/副本,限流仍是内存模式,e2e-py 等原有用例不受任何影响。
