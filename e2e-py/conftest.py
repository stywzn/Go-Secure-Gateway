import pytest
import requests
from framework.config import BASE_URL
from framework.client import GatewayClient
from framework.jwt_utils import valid_token

@pytest.fixture(scope="session", autouse=True)
def require_stack_up():
    """整场测试开始前探活一次:被测栈没起就直接退出,别让 21 条用例各刷一屏堆栈。

    不用 yield —— 探活没有任何东西需要清理,写成普通函数即可。
    except 只收窄到 RequestException:pytest.exit() 内部抛的 Exit 继承自 Exception,
    用 `except Exception` 会把它一起吞掉,导致退出失效。
    """
    health_url = BASE_URL + "/healthz"
    try:
        resp = requests.get(health_url, timeout=5)
    except requests.exceptions.RequestException as exc:
        pytest.exit(
            f"\n被测栈不可达: {health_url}"
            f"\n  原因: {type(exc).__name__}: {exc}"
            f"\n  多半是栈没起来。在项目根目录执行:  docker compose up -d"
            f"\n  若网关不在默认地址,用环境变量指定:  GATEWAY_BASE_URL=http://<host>:<port>",
            returncode=1,
        )
    if resp.status_code != 200:
        pytest.exit(
            f"\n网关健康检查未通过: {health_url} 返回 {resp.status_code}(期望 200)"
            f"\n  栈起来了但网关不健康。查看日志:  docker compose logs gateway",
            returncode=1,
        )


@pytest.fixture
def client():
    return GatewayClient()

@pytest.fixture
def auth_client():
    return GatewayClient(token = valid_token())

@pytest.fixture
def reset_data(auth_client):
    auth_client.post("/data/_reset")
    return auth_client