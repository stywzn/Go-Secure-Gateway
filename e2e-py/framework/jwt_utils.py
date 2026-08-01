
import jwt
import time
from framework.config import JWT_SECRET
def valid_token(user_id=9527):
    payload = {
        "user_id": user_id,
        "exp": int(time.time()) + 3600  # Token expires in 1 hour
    }
    token = jwt.encode(payload, JWT_SECRET, algorithm="HS256")
    return token

def expired_token(used_id=9527):
    payload = {
        "used_id": used_id,
        "exp": int(time.time()) - 3600  # Token expired 1 hour ago
    }
    token = jwt.encode(payload, JWT_SECRET, algorithm="HS256")
    return token

def wrong_secret_token(user_id=9527):
    payload = {
        "user_id": user_id,
        "exp": int(time.time()) + 3600  # Token expires in 1 hour
    }
    token = jwt.encode(payload, "wrong_secret", algorithm="HS256")
    return token

def none_alg_token(user_id=9527):
    payload = {
        "user_id": user_id,
        "exp": int(time.time()) + 3600  # Token expires in 1 hour
    }
    token = jwt.encode(payload, "", algorithm="none")
    return token
    