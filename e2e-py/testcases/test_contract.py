import jsonschema
import pytest

from framework.assertions import assert_status

TOKEN_SCHEMA = {
    "type":"object",
    "required":["token"],
    "properties":{"token":{"type":"string"}},
}

@pytest.mark.contract
def test_debug_token_matches_schema(client):
    resp = client.get("/debug/token")
    assert_status(resp, 200)
    jsonschema.validate(instance=resp.json(),schema=TOKEN_SCHEMA)



# 契约:echo 响应必须含 service/method/path 三个字符串字段
ECHO_SCHEMA = {
    "type": "object",
    "required": ["service", "method", "path"],
    "properties": {
        "service": {"type": "string"},
        "method":  {"type": "string"},
        "path":    {"type": "string"},
    },
}

@pytest.mark.contract
def test_echo_matches_schema(auth_client):
    resp = auth_client.get("/interaction/ping")
    assert_status(resp, 200)
    jsonschema.validate(instance=resp.json(),schema=ECHO_SCHEMA)