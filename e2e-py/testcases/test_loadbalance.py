import pytest
from framework.assertions import served_by, assert_status,assert_round_robin
from framework.client import GatewayClient
from framework.jwt_utils import valid_token


@pytest.mark.loadbalance
def test_round_robin_hits_both_replicas():
    served = []
    for _ in range(10):
        client = GatewayClient(token=valid_token())
        for _ in range(10):
            resp = client.get("/storage/ping")
            assert_status(resp, 200)
            served.append(served_by(resp))
    assert_round_robin(served, ("storage-a", "storage-b"))