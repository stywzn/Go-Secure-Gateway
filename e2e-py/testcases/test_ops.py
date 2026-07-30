
def test_healthz_ok(client):
    resp = client.get("/healthz")
    assert resp.status_code == 200

def test_missing_token_rejected(client):
    resp = client.get("/interaction/ping")
    assert resp.status_code == 401

def test_valid_token_allows(auth_client):
    resp = auth_client.get("/interaction/ping")
    assert resp.status_code == 200