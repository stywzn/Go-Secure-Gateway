import requests
from framework.config import BASE_URL, HTTP_TIMEOUT


class GatewayClient:
    def __init__(self,token=None):
        self.base_url = BASE_URL
        self.token = token

    def _headers(self):
        headers = {}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return headers
   
    def get(self,path):
        url = self.base_url + path
        return requests.get(url,headers=self._headers(),timeout=HTTP_TIMEOUT)
    


