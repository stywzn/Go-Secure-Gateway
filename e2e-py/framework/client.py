import random

import requests

from framework.config import BASE_URL, HTTP_TIMEOUT


class GatewayClient:
    def __init__(self,token=None,source_ip=None):
        self.base_url = BASE_URL
        self.token = token
        self.source_ip = source_ip or f"10.{random.randint(0,255)}.{random.randint(0,255)}.{random.randint(0,255)}"

    def _headers(self):
        headers = {"X-Forwarded-For": self.source_ip}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return headers

    def request(self,method,path,**kwargs):
        url = self.base_url + path
        kwargs.setdefault("timeout",HTTP_TIMEOUT)

        caller_headers = kwargs.pop("headers",{})
        kwargs["headers"] = {**self._headers(),**caller_headers}
        return requests.request(method,url,**kwargs)

    def get(self,path,**kwargs):
        return self.request("GET",path,**kwargs)

    def post(self,path,**kwargs):
        return self.request("POST",path,**kwargs)

    def put(self,path,**kwargs):
        return self.request("PUT",path,**kwargs)

    def delete(self,path,**kwargs):
        return self.request("DELETE",path,**kwargs)

