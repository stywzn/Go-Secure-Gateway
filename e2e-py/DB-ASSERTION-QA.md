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

## 三、怎么做(pytest 实现)⭐

### 连库 fixture(放 conftest.py)
```python
import pymysql, pytest

@pytest.fixture(scope="session")
def db():
    conn = pymysql.connect(host="127.0.0.1", user="root",
                           password="root", database="insure",
                           cursorclass=pymysql.cursors.DictCursor)
    yield conn
    conn.close()

@pytest.fixture
def db_query(db):
    def _q(sql, args=None):
        with db.cursor() as cur:
            cur.execute(sql, args)
            return cur.fetchall()
    return db.commit() or _q      # 每次查前刷新,避免读到旧事务快照
```

### 用例:接口 + 数据库双断言
```python
def test_create_policy(client, db_query):
    # 1. 调接口
    resp = client.post("/policy", json={"user_id": 1, "amount": 999})
    assert resp.status_code == 200                    # 响应断言
    policy_id = resp.json()["policy_id"]

    # 2. 数据库断言:查表确认真的落库 + 值对
    rows = db_query("SELECT * FROM policy WHERE id=%s", (policy_id,))
    assert len(rows) == 1                             # 确实写了一条
    assert rows[0]["amount"] == 999                   # 金额没算错
    assert rows[0]["status"] == "created"             # 状态对
```

### 更新 / 删除断言
```python
# 更新:改完查库确认新值
client.put(f"/policy/{pid}", json={"status": "paid"})
assert db_query("SELECT status FROM policy WHERE id=%s", (pid,))[0]["status"] == "paid"

# 软删除:查 is_deleted 标志,别以为真删了
client.delete(f"/policy/{pid}")
assert db_query("SELECT is_deleted FROM policy WHERE id=%s", (pid,))[0]["is_deleted"] == 1
```

### 测试数据的准备与清理 ⭐(关键,否则脏数据串扰)
两种主流做法:
- **事务回滚**(推荐,干净):每个用例在事务里跑,`teardown` 里 `rollback` → 数据不真正落库,天然隔离。
- **测后清理**:`teardown` 里 `DELETE` 掉本用例造的数据(用唯一标识,如测试专用前缀 `test_xxx`)。
```python
@pytest.fixture
def clean_policy(db):
    yield
    with db.cursor() as cur:
        cur.execute("DELETE FROM policy WHERE user_id=999")  # 清测试数据
    db.commit()
```
> 呼应 [[pytest进阶]] 的 fixture 与隔离思想:**造数据 → 用 → 保证清理**,和限流用例按 IP 隔离一个道理。

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

## 六、面试题
- **接口测试只断言响应够吗?为什么要查数据库?**(响应成功≠数据对;金额/状态/异步写)
- 哪些场景必须做数据库断言?(写操作/金额/异步/对账)
- 怎么实现数据库断言?(连库 fixture + 查表断言)
- **测试数据怎么准备和清理,防止脏数据?**(事务回滚 / 测后清理 / 唯一标识隔离)
- 你的框架有数据库断言吗?(诚实:网关无状态,但有接口层状态断言;有库的系统会加)
- 什么是对账测试?(跨表/跨系统数据一致性)
