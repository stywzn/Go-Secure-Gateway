import os

BASE_URL = os.environ.get("GATEWAY_BASE_URL", "http://127.0.0.1:8080")
JWT_SECRET = os.environ.get("GATEWAY_JWT_SECRET", "compose-dev-secret")
HTTP_TIMEOUT = float(os.environ.get("GATEWAY_HTTP_TIMEOUT", 20))