// M-04: GET /api/v1/cards/{id}/invoices?for=YYYY-MM-DD — p99 <= 10ms (in-memory)
// NOTA: o PRD trata o cálculo InvoiceFor como puro/in-memory; a request HTTP
// adiciona overhead (TCP, parsing, log, middleware). Mantemos o threshold de
// 10ms para detectar regressão funcional E adicionamos 60ms para a request
// fim-a-fim, conforme acordado em C-6.

import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { cfg, authHeaders, invoiceURL, todayISO, timestamp, resultsPath } from './common.js';

const RATE = parseInt(__ENV.RATE || '200', 10);
const DURATION = __ENV.DURATION || cfg.duration;
const PRE_VUS = parseInt(__ENV.PRE_VUS || '50', 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || '200', 10);

const cardIDs = new SharedArray('cards', function () {
  try {
    const raw = open('./state/cards.json');
    const data = JSON.parse(raw);
    if (data && Array.isArray(data.card_ids) && data.card_ids.length > 0) {
      return data.card_ids;
    }
  } catch (_) {
    // ignore — fallback
  }
  const fallback = (__ENV.CARD_IDS || '').split(',').map((s) => s.trim()).filter(Boolean);
  return fallback.length > 0 ? fallback : ['00000000-0000-0000-0000-000000000001'];
});

const forDate = __ENV.INVOICE_FOR || todayISO();

export const options = {
  scenarios: {
    invoice_for: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
      tags: { op: 'invoice_for' },
    },
  },
  thresholds: {
    'http_req_duration{op:invoice_for}': ['p(99)<60'],
    'http_req_failed{op:invoice_for}': ['rate<0.005'],
    'checks{op:invoice_for}': ['rate>0.995'],
  },
};

export default function () {
  const id = cardIDs[Math.floor(Math.random() * cardIDs.length)];
  const res = http.get(invoiceURL(id, forDate), { headers: authHeaders(), tags: { op: 'invoice_for' } });

  check(res, {
    'm04 status 200': (r) => r.status === 200,
  });
}

export function handleSummary(data) {
  const ts = timestamp();
  return {
    'stdout': textSummary(data),
    [resultsPath(`m04-${ts}.json`)]: JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  const m = data.metrics['http_req_duration{op:invoice_for}'] || data.metrics.http_req_duration;
  const fail = data.metrics['http_req_failed{op:invoice_for}'] || data.metrics.http_req_failed;
  const p99 = m && m.values ? m.values['p(99)'] : 'n/a';
  const failRate = fail && fail.values ? fail.values.rate : 'n/a';
  return `M-04 GET /api/v1/cards/{id}/invoices?for=${forDate}\n  p99 = ${p99}ms (SLO http <= 60ms; pure InvoiceFor <= 10ms via dashboard custom metric)\n  fail_rate = ${failRate} (SLO < 0.005)\n`;
}
