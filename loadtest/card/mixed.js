// Mixed scenario: 70% GET list, 20% invoice_for, 10% POST create — 300 RPS @ 120s.
// Cobre regressão de uso combinado e contenção entre operações no mesmo banco.

import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import {
  cfg,
  authHeaders,
  postHeaders,
  cardsURL,
  invoiceURL,
  idemKey,
  todayISO,
  timestamp,
  resultsPath,
} from './common.js';

const RATE = parseInt(__ENV.RATE || '300', 10);
const DURATION = __ENV.DURATION || cfg.mixedDuration;
const PRE_VUS = parseInt(__ENV.PRE_VUS || '100', 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || '300', 10);

const cardIDs = new SharedArray('cards', function () {
  try {
    const raw = open('./state/cards.json');
    const data = JSON.parse(raw);
    if (data && Array.isArray(data.card_ids) && data.card_ids.length > 0) {
      return data.card_ids;
    }
  } catch (_) {
    // ignore
  }
  const fallback = (__ENV.CARD_IDS || '').split(',').map((s) => s.trim()).filter(Boolean);
  return fallback.length > 0 ? fallback : ['00000000-0000-0000-0000-000000000001'];
});

const forDate = __ENV.INVOICE_FOR || todayISO();

export const options = {
  scenarios: {
    mixed: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'http_req_duration{op:list}': ['p(99)<80'],
    'http_req_duration{op:invoice_for}': ['p(99)<80'],
    'http_req_duration{op:create}': ['p(99)<400'],
    'http_req_failed': ['rate<0.01'],
  },
};

export default function () {
  const roll = Math.random();
  if (roll < 0.7) {
    const res = http.get(`${cardsURL()}?limit=100`, { headers: authHeaders(), tags: { op: 'list' } });
    check(res, { 'mixed list 200': (r) => r.status === 200 });
  } else if (roll < 0.9) {
    const id = cardIDs[Math.floor(Math.random() * cardIDs.length)];
    const res = http.get(invoiceURL(id, forDate), { headers: authHeaders(), tags: { op: 'invoice_for' } });
    check(res, { 'mixed invoice 200': (r) => r.status === 200 });
  } else {
    const body = JSON.stringify({
      name: `Mixed VU${__VU} Iter${__ITER}`,
      nickname: `mx-${__VU}-${__ITER}`,
      closing_day: ((__ITER % 28) + 1),
      due_day: (((__ITER + 7) % 28) + 1),
    });
    const key = idemKey('mixed', __VU, __ITER);
    const res = http.post(cardsURL(), body, { headers: postHeaders(key), tags: { op: 'create' } });
    check(res, { 'mixed create 201': (r) => r.status === 201 });
  }
}

export function handleSummary(data) {
  const ts = timestamp();
  return {
    'stdout': `Mixed scenario complete @ ${cfg.baseURL}\n`,
    [resultsPath(`mixed-${ts}.json`)]: JSON.stringify(data, null, 2),
  };
}
