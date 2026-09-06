import pytest

from framework.client import GatewayClient
from framework.jwt_utils import valid_token


@pytest.mark.ratelimit
def test_rate_limit_returns_429():
    client = GatewayClient(token=valid_token())
    got_429 = False
    for _ in range(200):
        resp = client.get("/interaction/ping")
        if resp.status_code == 429:
            got_429 = True
            break

    assert got_429 