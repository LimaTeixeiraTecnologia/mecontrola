// Teardown: DELETE em loop dos cartões criados pelo setup.
// Lê `state/cards.json` produzido por setup.js. Se ausente, no-op.

import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { cfg, authHeaders, cardURL, postHeaders, idemKey, timestamp, resultsPath } from './common.js';

const cardIDs = new SharedArray('cards', function () {
  try {
    const raw = open('./state/cards.json');
    const data = JSON.parse(raw);
    if (data && Array.isArray(data.card_ids)) {
      return data.card_ids;
    }
  } catch (_) {
    // ignore
  }
  return (__ENV.CARD_IDS || '').split(',').map((s) => s.trim()).filter(Boolean);
});

export const options = {
  scenarios: {
    teardown: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: Math.max(cardIDs.length, 1),
      maxDuration: '120s',
    },
  },
  thresholds: {
    'http_req_failed': ['rate<0.05'],
  },
};

export default function () {
  if (cardIDs.length === 0) {
    return;
  }
  const id = cardIDs[__ITER % cardIDs.length];
  const key = idemKey('teardown', __VU, __ITER);
  const res = http.del(cardURL(id), null, { headers: postHeaders(key), tags: { op: 'delete' } });
  check(res, {
    'teardown status 204/200/404': (r) => r.status === 204 || r.status === 200 || r.status === 404,
  });
}

export function handleSummary(data) {
  const ts = timestamp();
  return {
    'stdout': `Teardown ran for ${cardIDs.length} cards @ ${cfg.baseURL}\n`,
    [resultsPath(`teardown-${ts}.json`)]: JSON.stringify(data, null, 2),
  };
}
