import pytest,time
from framework.client import GatewayClient
from framework.assertions import assert_status
from framework.jwt_utils import valid_token

@pytest.mark.breaker
def test_circuit_opens_after_failures():
    client = GatewayClient(token=valid_token())
    for _ in range(6):
        client.get("/compute/run?status=500")
    resp = client.get("/compute/run")
    assert_status(resp, 503)
    assert resp.headers.get("X-Served-By") is None      



@pytest.mark.breaker
def test_circuit_recovers_after_cooldown():
    client = GatewayClient(token=valid_token())
    # open  breaker
    for _ in range(6):
        client.get("/compute/run?status=500")
    resp = client.get("/compute/run")
    assert_status(resp, 503)
    time.sleep(11)
    resp = client.get("/compute/run")
    assert_status(resp, 200)
    assert resp.headers.get("X-Served-By") == "compute"
