import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const rate429 = new Counter('rate_limited_429');   // 自定义指标:被限流的次数

export const options = {
  vus: 20,            // 20 个虚拟用户并发
  duration: '10s',    // 压 10 秒
  thresholds: {                                 // 通过线,不达标 → k6 退出码非0(可当门禁)
    'rate_limited_429': ['count>0'],            // ① 必须真的触发了限流
    'http_req_duration': ['p(95)<800'],         // ② 整体 p95 延迟 < 800ms
    'checks': ['rate>0.99'],                    // ③ 99%+ 响应正常(没 5xx)
  },
};

export function setup() {                        // 只跑一次:拿一个合法 token 给所有 VU 用
  const res = http.get('http://127.0.0.1:8080/debug/token');
  return { token: res.json('token') };
}

export default function (data) {                 // 每个 VU 每轮跑一次
  const res = http.get('http://127.0.0.1:8080/interaction/ping', {
    headers: { Authorization: `Bearer ${data.token}` },
  });
  if (res.status === 429) rate429.add(1);        // 数一下被限流的
  check(res, { 'no server error': (r) => r.status < 500 });
}