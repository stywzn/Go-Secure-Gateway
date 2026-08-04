# 数据库断言:概念 / 怎么做 / 本框架为什么没有(面试应对)

> 面试常问:"你接口测试**只断言响应,还是也查数据库?**" 这题考的是"数据校验维度"。本框架的被测网关**无状态(no DB)**,但这块要能讲清楚——**为什么这里不需要、有 DB 时该怎么做、以及本框架已有的"接口层状态断言"**。真实 DB 断言在项目二(投保系统,有 MySQL)才用得上。

---

## 一、什么是数据库断言 / 为什么需要 ⭐

**只断言接口响应是不够的**——接口返回 `200 success` **不等于**数据真的正确落库。经典坑:
- 接口返回成功,但**数据没写进 DB**(异步写失败、事务回滚了却返回了成功)。
- 接口返回成功,但**写错了值**(金额算错、状态没更新)。
- **脏数据**:多写了、少写了、写重了。

所以严谨的接口测试有**三个校验层次**:
1. **响应断言**:状态码、响应体字段(必做,最基础)。
2. **数据库断言** ⭐:直连 DB 查表,验证**数据真的按预期落库/更新/删除**。
3. **对账 / 一致性**:多系统间数据是否一致(如订单表 vs 账户表 vs 保单表)。

> 面试金句:"**接口返回成功只证明'接口说成功了',数据库断言才证明'数据真的对了'**。尤其涉及金额、状态、库存这类,必须查库确认,不能只信响应。"

## 二、什么场景必须做数据库断言

| 场景 | 为什么 |
|---|---|
| **写操作(增删改)** | 验证真的写/改/删了,值对不对 |
| **金额 / 库存 / 状态** | 算错、少扣多扣是重大 bug |
| **异步写**(MQ 落库) | 响应先返回,数据稍后才落 → 必须查库确认 |
| **对账** | 跨表/跨系统数据一致性(订单-支付-保单) |
| **软删除** | "删除"其实是改 `is_deleted` 标志,得查库确认 |
> 纯**查询接口**通常响应断言就够(数据本来就在库里);**写接口**才是 DB 断言的主战场。

## 三、怎么做(pytest 完整实现)⭐

### 1) 连库 fixture(放 conftest.py)
```python
import pymysql, pytest

@pytest.fixture(scope="session")
def db():
    conn = pymysql.connect(
        host="127.0.0.1", port=3306, user="test_ro", password="***",
        database="insure", charset="utf8mb4",
        cursorclass=pymysql.cursors.DictCursor,  # 返回 dict,好断言
        autocommit=True,                         # ⭐ 关键:见"注意事项"第1条
    )
    yield conn
    conn.close()

@pytest.fixture
def db_query(db):
    def _q(sql, args=None):
        db.commit()                              # ⭐ 查前提交/刷新,拿到最新数据
        with db.cursor() as cur:
            cur.execute(sql, args)               # 参数化,别拼字符串
            return cur.fetchall()
    return _q
```

### 2) 用例:接口 + 数据库双断言
```python
from decimal import Decimal

def test_create_policy(client, db_query, clean_policy):
    # ① 调接口 + 响应断言
    resp = client.post("/policy", json={"user_id": 999, "amount": 999.00})
    assert resp.status_code == 200
    pid = resp.json()["policy_id"]

    # ② 数据库断言:查表确认真的落库 + 值对
    rows = db_query("SELECT * FROM policy WHERE id=%s", (pid,))
    assert len(rows) == 1                            # 确实写了 1 条(不多不少)
    assert rows[0]["amount"] == Decimal("999.00")    # ⭐ 金额用 Decimal,别用 float
    assert rows[0]["status"] == "created"            # 状态对
    assert rows[0]["user_id"] == 999
```

### 3) 更新 / 删除 / 软删除断言
```python
# 更新:改完查库确认新值真落库
client.put(f"/policy/{pid}", json={"status": "paid"})
assert db_query("SELECT status FROM policy WHERE id=%s", (pid,))[0]["status"] == "paid"

# 软删除:查 is_deleted 标志,别以为真删了(常见坑)
client.delete(f"/policy/{pid}")
row = db_query("SELECT is_deleted FROM policy WHERE id=%s", (pid,))[0]
assert row["is_deleted"] == 1                        # 是标记删除,不是物理删
```

### 4) 异步写:轮询等待,别写完立刻查 ⭐
```python
import time
def wait_until(fn, timeout=5, interval=0.3):
    """轮询直到 fn() 返回真值或超时——用于 MQ/异步落库"""
    end = time.time() + timeout
    while time.time() < end:
        r = fn()
        if r: return r
        time.sleep(interval)
    raise AssertionError("等待数据落库超时")

def test_async_write(client, db_query):
    client.post("/order", json={"user_id": 999})      # 异步下单,响应先回
    # 数据经 MQ 稍后才落库 → 轮询,不能立即查(否则查不到)
    rows = wait_until(lambda: db_query(
        "SELECT * FROM `order` WHERE user_id=%s", (999,)))
    assert rows[0]["status"] == "paid"
```

### 5) 对账断言(跨表一致性)
```python
def test_reconcile(db_query, pid):
    order = db_query("SELECT amount FROM `order` WHERE policy_id=%s", (pid,))[0]
    policy = db_query("SELECT premium FROM policy WHERE id=%s", (pid,))[0]
    assert order["amount"] == policy["premium"]       # 订单金额 == 保单保费
```

### 6) 数据准备与清理(防脏数据串扰)⭐
```python
# 方案A:测后清理(用唯一标识,如 user_id=999 / test_ 前缀)
@pytest.fixture
def clean_policy(db):
    yield
    with db.cursor() as cur:
        cur.execute("DELETE FROM policy WHERE user_id=999")  # 只清自己造的
    db.commit()

# 方案B:事务回滚(更干净,但要求测试和应用用同一连接/同一事务,常难做到)
#   适合直接测 DAO/repository 层;测 HTTP 接口时应用是独立连接,回滚管不到它。
```
> 呼应 [[pytest进阶]] 的 fixture 与隔离:**造数据 → 用 → 保证清理**,和限流用例按 IP 隔离同理。

---

## 三·五、注意事项(踩坑清单)⭐⭐(面试问"注意什么"就答这些)

| # | 坑 | 怎么破 |
|---|---|---|
| 1 | **读到旧数据(最大坑)** | 你的测试连接和应用是**两个事务**;RR 隔离级别下测试连接用 MVCC 快照,**读不到应用刚提交的新数据**。→ 断言前 `db.commit()` 刷新事务 / 用 `autocommit` / 每次查开新事务。**这是 DB 断言第一大坑,必答。** |
| 2 | **异步写查不到** | MQ/异步落库有延迟,写完立刻查是空的 → **轮询重试**(wait_until),别用固定 sleep。 |
| 3 | **金额用 float 出精度问题** | `0.1+0.2≠0.3`。→ 用 **Decimal**,DB 存 `DECIMAL` 类型。 |
| 4 | **时间/时区** | datetime 时区、毫秒精度对比易错 → 断言时统一时区、用范围或截断到秒。 |
| 5 | **并行串扰(xdist)** | 多进程共用一个库,数据互相污染 → 每个用例用**唯一标识/独立数据**,或独立库/schema。 |
| 6 | **清理顺序 / 外键** | 有外键时删子表再删父表,否则删不掉 → 按依赖顺序清,或先关外键检查。 |
| 7 | **别碰生产库** | 只连**测试库**;查询断言用**只读账号**防误写;要在类生产测就用**影子表**(见项目二 Q13)。 |
| 8 | **断言粒度** | 别只 `count>0`(太松,写错值也过);也别每个字段都断(太脆,一改表就挂)→ 断**关键字段**(金额/状态/记录数)。 |
| 9 | **性能** | 别每一步都查库;只在**关键节点**查;断言 SQL 带条件走索引,别全表扫。 |
| 10 | **SQL 注入 / 参数化** | 断言 SQL 也用**参数化**(`%s` + args),别字符串拼接(养成习惯 + 防测试数据里的特殊字符出错)。 |
| 11 | **连接泄漏** | 用完关连接 / 用连接池;session fixture 复用一个连接,别每用例开新的不关。 |
| 12 | **NULL 与默认值** | 断言可空字段注意 `IS NULL`;新记录的默认值(如 `status` 默认 created)别漏。 |

## 四、本框架的情况(诚实讲)⭐

**被测网关是无状态的(no DB)**——它只做鉴权/限流/转发/熔断,不持久化数据。所以:
- **没有真正的数据库断言**,因为没库可查。
- **但框架已经做了"接口层的状态断言"**:`/data` 路由的 CRUD 用例(`crud` 模块)验证了**端到端状态时序**——**建→查(确认存在)→改→查(确认改了)→删→再查(确认 404 消失)**。这本质就是"数据校验维度",只是通过 API 而非直连库。
- 面试可这么答:

> "我这个网关是**无状态**的,不落库,所以没有直连数据库的断言;但我用 CRUD 路由做了**接口层的状态断言**——创建后再查确认存在、更新后查确认改了、删除后查确认 404,验证状态真的按预期流转。**如果被测系统有数据库(比如我另一个投保系统压测项目),我会加数据库断言**:写操作后直连库查表,确认金额/状态/记录数正确,并用事务回滚或测后清理保证隔离——因为**接口返回成功不代表数据真的对**。"

## 五、和你项目二的连接
- **项目二(投保系统)是 Java + MySQL,有真库** → 那里才是数据库断言的真实用武之地:
  - 承保接口调用后,查 `policy` 表确认保单生成、状态正确、保费金额对。
  - 对账:`order` 表金额 == `policy` 表保费 == 支付流水,跨表一致。
- 想真练:在项目二的 B 轨(自建 Java+MySQL SUT,见"项目二-实操复现")里,用 pytest + pymysql 加一组 DB 断言用例,就补齐了这个维度。

## 六、面试题(数据库断言高频)
- **接口测试只断言响应够吗?为什么要查数据库?**(响应成功≠数据对;金额/状态/异步写)
- 哪些场景必须做数据库断言?(写操作/金额/异步/对账/软删除)
- 怎么实现数据库断言?(连库 fixture + 查表断言,pymysql/SQLAlchemy)
- **做数据库断言要注意什么?**(⭐ 高频)→ 事务隔离读到旧数据(要 commit/新事务)、异步写要轮询、金额用 Decimal、别碰生产库用只读账号、并行串扰、断言粒度、清理外键顺序
- **写完数据库里查不到 / 查到旧值,为什么?**(测试连接和应用是两个事务,MVCC 快照读 → commit 刷新 / autocommit)
- **异步接口(消息队列落库)怎么断言?**(轮询重试 wait_until,别 sleep 死等)
- **测试数据怎么准备和清理,防止脏数据?**(事务回滚 / 测后清理 / 唯一标识隔离)
- 为什么金额断言不用 float?(精度问题,用 Decimal / DECIMAL)
- 你的框架有数据库断言吗?(诚实:网关无状态,但有接口层状态断言;有库的系统会加)
- 什么是对账测试?(跨表/跨系统数据一致性,如订单金额==保单保费==支付流水)
- 线上/类生产环境怎么做数据库断言不影响真实数据?(测试库 / 只读账号 / 影子表 / 业务低峰)
