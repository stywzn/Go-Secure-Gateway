"""Go-Secure-Gateway 接口自动化框架核心层。

分层设计：
    config     —— 环境配置（从环境变量读取，带默认值，测试环境零改代码切换）
    client     —— HTTP 会话封装（统一鉴权头、日志、超时、Allure 步骤）
    jwt_utils  —— JWT 构造工厂（正向 / 负向 / 算法混淆），负向安全测试的核心
    assertions —— 领域断言（状态码、轮询分布、指标增长）
    prometheus —— /metrics 文本解析，支撑"监控断言"
"""
