import http from 'k6/http';
import { check, sleep } from 'k6';

const BACKEND = __ENV.BACKEND || 'http://localhost:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';
const REF_MONTH = __ENV.REF_MONTH || new Date().toISOString().slice(0, 7);

const ENVELOPE = __ENV.ENVELOPE || 'a';

const ENVELOPES = {
  a: {
    vus: parseInt(__ENV.VUS || '1', 10),
    duration: __ENV.DURATION || '3m',
    thinkTimeSec: parseFloat(__ENV.THINK_TIME || '15'),
  },
  b: {
    vus: parseInt(__ENV.VUS || '3', 10),
    duration: __ENV.DURATION || '5m',
    thinkTimeSec: parseFloat(__ENV.THINK_TIME || '3'),
  },
};

const profile = ENVELOPES[ENVELOPE] || ENVELOPES['a'];

export const options = {
  vus: profile.vus,
  duration: profile.duration,
  thresholds: {
    'http_req_duration{endpoint:months_summary}': ['p(95)<500'],
    'http_req_duration{endpoint:list_entries}': ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

export function setup() {
  if (!AUTH_TOKEN) {
    throw new Error('AUTH_TOKEN is required: um token valido e precondition para o veredito de carga (401 invalidaria o threshold http_req_failed)');
  }
}

function authHeaders() {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${AUTH_TOKEN}`,
  };
}

export default function () {
  const headers = authHeaders();

  const summary = http.get(
    `${BACKEND}/api/v1/months/${REF_MONTH}`,
    { headers: headers, tags: { endpoint: 'months_summary' } },
  );
  check(summary, {
    'summary status ok': (r) => r.status === 200,
  });

  const entries = http.get(
    `${BACKEND}/api/v1/months/${REF_MONTH}/entries`,
    { headers: headers, tags: { endpoint: 'list_entries' } },
  );
  check(entries, {
    'entries status ok': (r) => r.status === 200,
  });

  if (profile.thinkTimeSec > 0) {
    sleep(profile.thinkTimeSec);
  }
}
