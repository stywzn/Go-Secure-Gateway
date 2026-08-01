import pytest

@pytest.mark.loadbalance
def test_round_robin_hits_both_replicas(auth_client):
    seen = set()
    for _ in range(10):
        resp = auth_client.get("/storage/ping")
        assert resp.status_code == 200
        seen.add(resp.headers.get("X-Served-By"))
    assert seen == {"storage-a","storage-b"}