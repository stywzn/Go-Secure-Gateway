import pytest
from framework.client import GatewayClient
from framework.jwt_utils import valid_token, expired_token, wrong_secret_token, none_alg_token


@pytest.mark.auth
@pytest.mark.parametrize("make_token",[expired_token, wrong_secret_token, none_alg_token])
def test_bad_token_rejected(make_token):
    resp = GatewayClient(token=make_token()).get("/interaction/ping")
    assert resp.status_code == 401
                      