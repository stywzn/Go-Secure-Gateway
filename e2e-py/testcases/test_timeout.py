import pytest

from framework.assertions import assert_status


@pytest.mark.timeout
def test_delay_passes_through(auth_client):
    resp = auth_client.get("/compute/run?delay=1s")
    assert_status(resp, 200)

@pytest.mark.timeout
def test_slow_upstream_returns_504(auth_client):
    resp = auth_client.get("/compute/run?delay=3s")
    assert_status(resp, 504)
   