import pytest
from framework.client import GatewayClient
from framework.jwt_utils import valid_token

@pytest.fixture
def client():
    return GatewayClient()

@pytest.fixture
def auth_client():
    return GatewayClient(token = valid_token())