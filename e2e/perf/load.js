// k6 load test for the gateway. Run against a running stack:
//
//   # 1) get a token (debug mode)
//   TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
//   # 2) run
//   k6 run -e TOKEN=$TOKEN e2e/perf/load.js
//
// No local k6? Use docker:
//   docker run --rm -i --network host -e TOKEN=$TOKEN grafana/k6 run - < e2e/perf/load.js
//
// Watch latency/throughput live in Grafana while this runs.
import http from "k6/http";
import { check, sleep } from "k6";

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const TOKEN = __ENV.TOKEN || "";

export const options = {
  // Ramp up to 20 virtual users, hold, then ramp down.
  stages: [
    { duration: "15s", target: 20 },
    { duration: "30s", target: 20 },
    { duration: "15s", target: 0 },
  ],
  thresholds: {
    // 95% of requests under 250ms; error rate under 1%.
    http_req_duration: ["p(95)<250"],
    http_req_failed: ["rate<0.01"],
  },
};

const params = { headers: { Authorization: `Bearer ${TOKEN}` } };

export default function () {
  // Hit the load-balanced route so replicas share the load.
  const res = http.get(`${BASE}/storage/ping`, params);
  check(res, { "status is 200": (r) => r.status === 200 });
  sleep(0.2);
}
