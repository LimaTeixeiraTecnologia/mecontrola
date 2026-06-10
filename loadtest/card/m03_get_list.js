// M-03: GET /api/v1/cards — p99 <= 50ms @ 200 RPS sustained.

import http from 'k6/http';
import { check } from 'k6';
import { cfg, authHeaders, cardsURL, timestamp, resultsPath } from './common.js';

const RATE = parseInt(__ENV.RATE || '200', 10);
const DURATION = __ENV.DURATION || cfg.duration;
const PRE_VUS = parseInt(__ENV.PRE_VUS || '50', 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || '200', 10);
const LIMIT = parseInt(__ENV.LIMIT || '100', 10);

export const options = {
  scenarios: {
    list_cards: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
      tags: { op: 'list' },
    },
  },
  thresholds: {
    'http_req_duration{op:list}': ['p(99)<50'],
    'http_req_failed{op:list}': ['rate<0.005'],
    'checks{op:list}': ['rate>0.995'],
  },
};

export default function () {
  const url = `${cardsURL()}?limit=${LIMIT}`;
  const res = http.get(url, { headers: authHeaders(), tags: { op: 'list' } });

  check(res, {
    'm03 status 200': (r) => r.status === 200,
    'm03 body has items': (r) => {
      try {
        const parsed = JSON.parse(r.body);
        return Array.isArray(parsed.items) || Array.isArray(parsed);
      } catch (_) {
        return false;
      }
    },
  });
}

export function handleSummary(data) {
  const ts = timestamp();
  return {
    'stdout': textSummary(data),
    [resultsPath(`m03-${ts}.json`)]: JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  const m = data.metrics['http_req_duration{op:list}'] || data.metrics.http_req_duration;
  const fail = data.metrics['http_req_failed{op:list}'] || data.metrics.http_req_failed;
  const p99 = m && m.values ? m.values['p(99)'] : 'n/a';
  const failRate = fail && fail.values ? fail.values.rate : 'n/a';
  return `M-03 GET /api/v1/cards\n  p99 = ${p99}ms (SLO <= 50ms)\n  fail_rate = ${failRate} (SLO < 0.005)\n`;
}
