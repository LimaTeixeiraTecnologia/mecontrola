// M-02: POST /api/v1/cards — p99 <= 300ms @ 1000 RPS sustained.
// Threshold falha => exit code != 0 (gate de regressão).

import http from 'k6/http';
import { check } from 'k6';
import { cfg, postHeaders, cardsURL, idemKey, timestamp, resultsPath } from './common.js';

const RATE = parseInt(__ENV.RATE || '1000', 10);
const DURATION = __ENV.DURATION || cfg.duration;
const PRE_VUS = parseInt(__ENV.PRE_VUS || '200', 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || '500', 10);

export const options = {
  scenarios: {
    post_create: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
      tags: { op: 'create' },
    },
  },
  thresholds: {
    'http_req_duration{op:create}': ['p(99)<300'],
    'http_req_failed{op:create}': ['rate<0.005'],
    'checks{op:create}': ['rate>0.995'],
  },
};

export default function () {
  const body = JSON.stringify({
    name: `Card VU${__VU} Iter${__ITER}`,
    nickname: `vu${__VU}-it${__ITER}`,
    closing_day: ((__ITER % 28) + 1),
    due_day: (((__ITER + 7) % 28) + 1),
  });

  const key = idemKey('m02', __VU, __ITER);
  const res = http.post(cardsURL(), body, { headers: postHeaders(key), tags: { op: 'create' } });

  check(res, {
    'm02 status 201': (r) => r.status === 201,
    'm02 has location': (r) => !!r.headers['Location'],
  });
}

export function handleSummary(data) {
  const ts = timestamp();
  return {
    'stdout': textSummary(data),
    [resultsPath(`m02-${ts}.json`)]: JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  const m = data.metrics['http_req_duration{op:create}'] || data.metrics.http_req_duration;
  const fail = data.metrics['http_req_failed{op:create}'] || data.metrics.http_req_failed;
  const p99 = m && m.values ? m.values['p(99)'] : 'n/a';
  const failRate = fail && fail.values ? fail.values.rate : 'n/a';
  return `M-02 POST /api/v1/cards\n  p99 = ${p99}ms (SLO <= 300ms)\n  fail_rate = ${failRate} (SLO < 0.005)\n`;
}
