import pytest
from framework.client import GatewayClient
from framework.jwt_utils import valid_token

@pytest.fixture
def client():
    return GatewayClient()

@pytest.fixture
def auth_client():
    return GatewayClient(token = valid_token())

@pytest.fixture
def reset_data(auth_client):
    auth_client.post("/data/_reset")
    return auth_client