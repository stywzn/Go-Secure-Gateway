# Python + 框架搭建 面试题

分两块:**Python 语言基础** + **pytest / 自动化框架**。答案是要点,理解后用自己的话讲。

---

## 一、Python 语言基础

**Q:深拷贝 vs 浅拷贝?★**
- **浅拷贝**(`copy.copy` / `list[:]` / `dict.copy()`):只复制最外层,内部的**嵌套对象仍是同一个引用**。改嵌套对象会互相影响。
- **深拷贝**(`copy.deepcopy`):**递归复制所有层**,完全独立,互不影响。
- 一句话:浅拷贝复制"壳",深拷贝连"里面的东西"一起复制。
```python
import copy
a = [[1, 2], 3]
b = copy.copy(a)      # 浅:b[0] 和 a[0] 是同一个 list
c = copy.deepcopy(a)  # 深:完全独立
a[0].append(9)        # b[0] 也变了,c[0] 不变
```

**Q:可变类型 vs 不可变类型?**
- 不可变:`int / float / str / tuple / bool / frozenset` —— 值不能原地改,改就是新建对象。
- 可变:`list / dict / set` —— 可原地修改。
- 影响:不可变类型可做字典 key;函数默认参数别用可变类型(会被多次调用共享,经典坑)。

**Q:`is` 和 `==` 区别?**
`==` 比**值**是否相等;`is` 比是否**同一个对象**(内存地址)。判空用 `is None`。

**Q:装饰器是什么?**
一个"包装函数的函数",在不改原函数代码的前提下**增强它**(加日志、计时、鉴权、缓存)。`@decorator` 是语法糖。pytest 的 `@pytest.fixture`、`@pytest.mark.xxx` 都是装饰器。

**Q:`*args` 和 `**kwargs`?**
`*args` 收集多余的**位置参数**成元组;`**kwargs` 收集多余的**关键字参数**成字典。用于写通用/转发函数(比如客户端封装里把参数透传给 requests)。

**Q:生成器 / `yield`?**
用 `yield` 的函数是生成器,**惰性求值、按需产出**,省内存(不用一次性把结果全放列表里)。适合大数据流。

**Q:列表推导式?**
`[x*2 for x in items if x > 0]` —— 简洁地"过滤 + 变换"生成列表,比 for 循环短、快。

**Q:闭包?**
函数内部返回一个引用了外层变量的内层函数,内层"记住"了外层的状态。装饰器就是靠闭包实现的。

**Q:魔术方法(dunder)?**
前后双下划线、Python 自动调用的钩子:`__init__`(构造)、`__str__`(打印)、`__len__`(len())、`__eq__`(==)、`__enter__/__exit__`(with)。你不主动调,特定时机自动触发。

**Q:上下文管理器 / `with`?**
实现了 `__enter__/__exit__` 的对象能用 `with`,**自动管理资源的获取和释放**(文件、连接、session),即使出异常也会正确关闭。

**Q:单下划线 / 双下划线?**
`_x` = 约定的"内部,别碰"(君子协定,不强制);`__x` = 名字改写(name mangling,防子类误覆盖);`__x__` = 魔术方法。Python 没有真正的 private,靠命名约定。

**Q:异常处理?**
`try/except/else/finally`:try 里放可能出错的;except 抓特定异常(别裸 `except:`);else 是没异常时执行;finally 无论如何都执行(清理)。

**Q:GIL 是什么?**
全局解释器锁,同一时刻只有一个线程执行 Python 字节码 → **多线程做不了 CPU 密集并行**(适合 IO 密集);CPU 密集要用多进程。(压测工具 Locust 高压要多 worker 就和这有关。)

---

## 二、pytest / 自动化框架

**Q:fixture 是什么?★**
可复用的**前置准备/资源**。用例把 fixture 名当参数"点名",pytest 自动创建并**注入**(依赖注入)。用于:建客户端、造 token、连数据库、测试前后清理。

**Q:fixture 的 scope(作用域)?★**
`function`(默认,每个用例一份)/ `class` / `module` / `package` / `session`(整轮共享一份)。
- 越宽 → 复用越多、越省开销,但**越容易共享状态串扰**。
- 每用例独立的(如带随机 IP 的 client)用 `function`;一次性的(如"网关是否就绪"探活、DB 连接)用 `session`。
- fixture 里用 `yield` 可写**前置 + 后置清理**(yield 前是 setup,后是 teardown)。

**Q:conftest.py 的作用?**
pytest **自动加载**的特殊文件,放公共 fixture / hook。里面的 fixture **不用 import**,同目录及子目录的用例都能直接用。

**Q:参数化 / 数据驱动?**
`@pytest.mark.parametrize("x", [1,2,3])` 让一个用例函数**跑多组数据**,自动变成多条用例。进阶:把数据外置到 YAML/CSV,`yaml.safe_load` 读进来再参数化,**加数据不改代码**。

**Q:marker(标记)?**
`@pytest.mark.smoke` 给用例贴标签,`pytest -m smoke` 只跑该组。要在 pytest.ini 里注册标记名;加 `--strict-markers` 可让打错的标记直接报错。

**Q:pytest 断言为什么直接用 `assert`?**
pytest 有断言重写,`assert a == b` 失败时会自动打印两边的实际值,不用像 unittest 那样记一堆 `assertEqual`。

**Q:★ 怎么从 0 搭一个接口自动化框架?(分层)**
按"关注点分离"分层,每层职责单一:
1. **配置层**:环境地址、密钥、超时全走环境变量 → 切环境零改代码。
2. **客户端/工具层**:封装 HTTP(自动带鉴权头、超时、日志),用例不写样板;JWT/加密等工具。
3. **数据层**:用例数据外置(YAML/JSON/CSV)→ 数据驱动。
4. **用例层**:按被测模块组织,只写业务意图,用 fixture 拿资源。
5. **断言层**:通用断言(状态码、schema、字段)复用。
6. **报告层**:Allure / pytest-html。
7. **执行/编排**:conftest(夹具、探活)、pytest.ini(标记、路径)、并行(xdist)。
8. **CI 层**:GitHub Actions 一键起栈 + 跑测试 + 出报告 + 门禁。
> 价值:新增用例成本低、可读性高、换环境只动配置、易接 CI。

**Q:框架里为什么要封装 client?**
不封装的话每条用例都要重复:拼 URL、塞鉴权头、设超时、发请求。封装成客户端后,脏活下沉一层,用例只表达"打哪个接口、断言什么",可读、好维护。

**Q:常用第三方库?**
`requests`(发请求)、`pytest`(框架)、`pytest-xdist`(并行)、`allure-pytest`(报告)、`jsonschema`(契约校验)、`PyYAML`(数据驱动)、`PyJWT`(造 token)。

**Q:怎么保证用例之间不互相影响?**
① fixture 用合适 scope(默认 function 独立);② 有状态用例前 reset/清数据;③ 每个用例用独立测试数据/独立源标识;④ 破坏共享状态的用例串行隔离。

**Q:怎么做测试报告?怎么接 CI?**
本地 `pytest --alluredir=... && allure serve`;CI 里跑完把结果传成产物或发布成网页;thresholds/退出码非 0 就让 job 变红拦 PR。

**Q:UI 自动化里的 POM 是什么?(可能延伸问)**
Page Object Model,把每个页面的元素定位和操作封装成一个类,用例调用页面对象的方法而不是直接写定位。好处:页面一改只改一处,用例稳定可读。(接口测试里对应的就是"客户端/接口对象封装"。)
