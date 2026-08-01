import pytest

@pytest.mark.routing
def test_prefix_not_stripped(auth_client):
    resp = auth_client.get("/interaction/ping")
    assert resp.status_code == 200
    assert resp.json()["path"] == "/interaction/ping"


@pytest.mark.routing
def test_prefix_stripped(auth_client):
    resp = auth_client.get("/storage/ping")
    assert resp.status_code == 200
    assert resp.json()["path"] == "/ping"