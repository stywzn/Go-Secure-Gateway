from pathlib import Path

import pytest
import yaml

from framework.assertions import assert_status
from framework.client import GatewayClient
from framework.jwt_utils import (
    expired_token,
    none_alg_token,
    valid_token,
    wrong_secret_token,
)

# read test cases from yaml file
CASES = yaml.safe_load((Path(__file__).parent.parent / "data" / "auth_cases.yaml").read_text())

MAKERS = {"valid":valid_token,
          "expired":expired_token,
          "wrong_secret":wrong_secret_token,
          "none_alg":none_alg_token
          }



@pytest.mark.auth
@pytest.mark.parametrize("case",CASES,ids=[c["name"] for c in CASES])
def test_token_cases(case):
    token = MAKERS[case["kind"]]()
    resp =GatewayClient(token=token).get("/interaction/ping")
    assert_status(resp, case["expect"])
                      