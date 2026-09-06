import pytest

from framework.assertions import assert_status


@pytest.mark.crud
def test_item_lifecycle(reset_data):
    resp = reset_data.post("/data/items",json={"name":"alpha"})
    assert_status(resp, 201)
    item_id = resp.json()["id"]

    resp = reset_data.get(f"/data/items/{item_id}")
    assert_status(resp, 200)
    assert resp.json()["name"] == "alpha"

    resp = reset_data.put(f"/data/items/{item_id}",json={"name":"beta"})
    assert_status(resp, 200)
    assert resp.json()["name"] == "beta"

    resp = reset_data.delete(f"/data/items/{item_id}")
    assert_status(resp, 204)

    resp = reset_data.get(f"/data/items/{item_id}")
    assert_status(resp, 404)



