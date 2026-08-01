import pytest


@pytest.mark.timeout
def test_delay_passes_through(auth_client):
    resp = auth_client.get("/compute/run?delay=1s")
    assert resp.status_code == 200

@pytest.mark.timeout
def test_slow_upstream_returns_504(auth_client):
    resp = auth_client.get("/compute/run?delay=3s")
    assert resp.status_code == 504
   