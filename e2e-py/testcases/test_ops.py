import pytest
from framework.prometheus import counter_total
def test_healthz_ok(client):
    resp = client.get("/healthz")
    assert resp.status_code == 200

def test_missing_token_rejected(client):
    resp = client.get("/interaction/ping")
    assert resp.status_code == 401

def test_valid_token_allows(auth_client):
    resp = auth_client.get("/interaction/ping")
    assert resp.status_code == 200


@pytest.mark.ops
def test_readyz_ok(client):
    resp = client.get("/readyz")
    assert resp.status_code == 200


@pytest.mark.ops
def test_request_counter_increments(auth_client):
    resp = auth_client.get("/metrics")
    before = counter_total(resp.text, "gateway_http_requests_total")
    auth_client.get("/interaction/ping")
    resp = auth_client.get("/metrics")
    after = counter_total(resp.text, "gateway_http_requests_total")
    assert resp.status_code == 200
    assert after > before


@pytest.mark.ops
def test_debug_token_issues_token(client):
    resp = client.get("/debug/token")
    assert resp.status_code == 200
    assert resp.json()["token"] is not None 