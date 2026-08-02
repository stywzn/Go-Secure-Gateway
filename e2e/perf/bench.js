// Capacity benchmark for the gateway's RAW forwarding path.
// Unlike load.js (gentle, 20 VUs + sleep), this pushes concurrency with no
// think-time to measure steady-state throughput (QPS) and tail latency.
//
// Concurrency + duration are env-driven so we can sweep tiers:
//   VUS=50  DURATION=30s   -> one tier
// Run via compose (handles network + script mount):
//   docker compose -f docker-compose.yml -f docker-compose.bench.yml \
//     run --rm -e TOKEN=$TOKEN -e BASE_URL=http://gateway:8080 -e VUS=50 \
//     k6 run /scripts/bench.js
import http from "k6/http";
import { check } from "k6";

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const TOKEN = __ENV.TOKEN || "";
const VUS = parseInt(__ENV.VUS || "50");
const DURATION = __ENV.DURATION || "30s";

export const options = {
  scenarios: {
    steady: {
      executor: "constant-vus",
      vus: VUS,
      duration: DURATION,
    },
  },
  thresholds: {},
};

const params = { headers: { Authorization: `Bearer ${TOKEN}` } };

export default function () {
  const res = http.get(`${BASE}/storage/ping`, params);
  check(res, { "status is 200": (r) => r.status === 200 });
  // No sleep: push as hard as the given concurrency allows.
}
