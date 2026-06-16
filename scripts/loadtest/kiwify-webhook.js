// k6 load test — Kiwify webhook (HMAC SHA-1 in query param `signature`).
//
// Envs:
//   BACKEND                 base URL of API (default: http://localhost:8080)
//   KIWIFY_WEBHOOK_SECRET   raw secret used for HMAC SHA-1 (required)
//   KIWIFY_PRODUCT_ID       product_id of the monthly plan (default: prod-load-monthly)
//   VUS                     virtual users (default: 50)
//   DURATION                test duration (default: 2m)
//
// Run:
//   k6 run scripts/loadtest/kiwify-webhook.js
//
// Acceptance:
//   - p95 of 202 responses < 500 ms
//   - error rate < 1 %

import http from 'k6/http';
import { check } from 'k6';
import crypto from 'k6/crypto';
import encoding from 'k6/encoding';

const BACKEND = __ENV.BACKEND || 'http://localhost:8080';
const SECRET = __ENV.KIWIFY_WEBHOOK_SECRET;
const PRODUCT_ID = __ENV.KIWIFY_PRODUCT_ID || 'prod-load-monthly';

if (!SECRET) {
  throw new Error('KIWIFY_WEBHOOK_SECRET is required');
}

export const options = {
  vus: parseInt(__ENV.VUS || '50', 10),
  duration: __ENV.DURATION || '2m',
  thresholds: {
    'http_req_duration{status:202}': ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

// 43-char URL-safe token similar to onboarding funnel tokens.
function funnelToken() {
  const bytes = new Uint8Array(32);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = Math.floor(Math.random() * 256);
  }
  return encoding
    .b64encode(bytes.buffer, 'rawstd')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .slice(0, 43);
}

function nowKiwify() {
  // Kiwify uses "YYYY-MM-DD HH:MM:SS" (no TZ).
  const d = new Date().toISOString();
  return d.slice(0, 10) + ' ' + d.slice(11, 19);
}

function buildPayload(token) {
  const now = nowKiwify();
  const orderID = 'order-load-' + token.slice(0, 12);
  return {
    order_id: orderID,
    order_ref: 'ref-' + token.slice(0, 8),
    order_status: 'paid',
    webhook_event_type: 'order_approved',
    subscription_id: 'sub-' + token.slice(0, 16),
    Product: { product_id: PRODUCT_ID, product_name: 'Load Test Plan' },
    Customer: {
      email: `load+${token.slice(0, 10)}@mecontrola.local`,
      mobile: '+5511900000000',
      CPF: '00000000000',
    },
    Subscription: {
      status: 'active',
      start_date: new Date().toISOString(),
      next_payment: new Date(Date.now() + 30 * 24 * 3600 * 1000).toISOString(),
    },
    TrackingParameters: { sck: token, s1: null, src: null },
    approved_date: now,
    updated_at: now,
    created_at: now,
  };
}

export default function () {
  const token = funnelToken();
  const body = JSON.stringify(buildPayload(token));
  const sig = crypto.hmac('sha1', SECRET, body, 'hex');
  const url = `${BACKEND}/api/v1/billing/webhooks/kiwify?signature=${sig}`;

  const res = http.post(url, body, {
    headers: { 'Content-Type': 'application/json' },
    tags: { endpoint: 'kiwify_webhook' },
  });

  check(res, {
    'status is 202': (r) => r.status === 202,
    'body ok': (r) => r.body && r.body.length >= 0,
  });
}
