// Common helpers shared across card load test scripts.
// Read env knobs once and expose typed helpers + headers builder.

const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const USER_ID = __ENV.X_USER_ID || __ENV.USER_ID || '00000000-0000-0000-0000-000000000001';
const IDEMPOTENCY_KEY_PREFIX = __ENV.IDEMPOTENCY_KEY_PREFIX || 'k6-loadtest';
const DURATION = __ENV.DURATION || '60s';
const MIXED_DURATION = __ENV.MIXED_DURATION || '120s';

export const cfg = {
  baseURL: BASE_URL,
  userID: USER_ID,
  idemPrefix: IDEMPOTENCY_KEY_PREFIX,
  duration: DURATION,
  mixedDuration: MIXED_DURATION,
};

export function authHeaders() {
  return {
    'Content-Type': 'application/json',
    'X-User-ID': USER_ID,
  };
}

export function postHeaders(idemKey) {
  return Object.assign(authHeaders(), { 'Idempotency-Key': idemKey });
}

export function idemKey(scope, vu, iter) {
  return `${IDEMPOTENCY_KEY_PREFIX}-${scope}-${vu}-${iter}-${Date.now()}`;
}

export function cardsURL() {
  return `${BASE_URL}/api/v1/cards`;
}

export function cardURL(id) {
  return `${BASE_URL}/api/v1/cards/${id}`;
}

export function invoiceURL(id, forDate) {
  const q = forDate ? `?for=${encodeURIComponent(forDate)}` : '';
  return `${BASE_URL}/api/v1/cards/${id}/invoices${q}`;
}

export function todayISO() {
  return new Date().toISOString().slice(0, 10);
}

export function timestamp() {
  return new Date().toISOString().replace(/[:.]/g, '-');
}

const RESULTS_DIR = __ENV.RESULTS_DIR || '/loadtest/card/results';
const STATE_DIR = __ENV.STATE_DIR || '/loadtest/card/state';

export function resultsPath(filename) {
  return `${RESULTS_DIR}/${filename}`;
}

export function statePath(filename) {
  return `${STATE_DIR}/${filename}`;
}
