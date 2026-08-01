import pytest

@pytest.mark.crud
def test_item_lifecycle(reset_data):
    resp = reset_data.post("/data/items",json={"name":"alpha"})
    assert resp.status_code == 201
    item_id = resp.json()["id"]

    resp = reset_data.get(f"/data/items/{item_id}")
    assert resp.status_code == 200
    assert resp.json()["name"] == "alpha"

    resp = reset_data.put(f"/data/items/{item_id}",json={"name":"beta"})
    assert resp.status_code == 200      
    assert resp.json()["name"] == "beta"

    resp = reset_data.delete(f"/data/items/{item_id}")
    assert resp.status_code == 204

    resp = reset_data.get(f"/data/items/{item_id}")
    assert resp.status_code == 404



