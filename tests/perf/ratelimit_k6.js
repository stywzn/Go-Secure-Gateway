// k6 性能/限流压测脚本。
//
// 目的（两个亮点合一）：
//   1) 性能基线：在可控并发下测网关的吞吐与 p95 延迟。
//   2) 限流阈值验证：把速率压到远超 rps=50/burst=100，观察 429 比例是否符合预期，
//      并断言 p95 延迟不劣化 —— 证明限流在保护后端的同时不拖垮正常请求。
//
// 运行：
//   TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
//   k6 run -e TOKEN=$TOKEN tests/perf/ratelimit_k6.js
//
// 用不同源 IP（X-Forwarded-For）可分别压"单 IP 被限流"与"多 IP 各自计数"。

import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE = __ENV.GATEWAY_BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || '';

const rate429 = new Rate('rate_limited_429');
const okLatency = new Trend('ok_latency_ms', true);

export const options = {
  scenarios: {
    // 恒定到达率，远超 rps=50，制造限流压力。
    flood: {
      executor: 'constant-arrival-rate',
      rate: 200,            // 200 req/s，约 4× 阈值
      timeUnit: '1s',
      duration: '20s',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
  thresholds: {
    // 正常放行请求的 p95 延迟应保持低位（限流不拖累放行流量）。
    'ok_latency_ms': ['p(95)<200'],
    // 在 4× 阈值压力下，应观测到相当比例的 429（限流确实生效）。
    'rate_limited_429': ['rate>0.3'],
  },
};

export default function () {
  const res = http.get(`${BASE}/interaction/ping`, {
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      // 固定单一源 IP：让所有 VU 共用一个限流桶，压出 429。
      'X-Forwarded-For': '198.51.100.7',
    },
  });

  const limited = res.status === 429;
  rate429.add(limited);
  if (res.status === 200) okLatency.add(res.timings.duration);

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
  });
}
