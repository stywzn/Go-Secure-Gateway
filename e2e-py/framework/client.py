import requests
import random
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
   
    def get(self,path):
        url = self.base_url + path
        return requests.get(url,headers=self._headers(),timeout=HTTP_TIMEOUT)
    
    def post(self,path,json=None):
        url = self.base_url + path
        return requests.post(url,headers=self._headers(),json=json,timeout=HTTP_TIMEOUT)

    def put(self,path,json=None):
        url = self.base_url + path
        return requests.put(url,headers=self._headers(),json=json,timeout=HTTP_TIMEOUT)

    def delete(self,path):
        url = self.base_url + path
        return requests.delete(url,headers=self._headers(),timeout=HTTP_TIMEOUT)
