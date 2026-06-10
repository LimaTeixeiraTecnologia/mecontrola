// Setup: seeds N test cards used by m03/m04/mixed scenarios.
// k6 cannot easily write files; the IDs are emitted via handleSummary into
// `loadtest/card/results/setup-<ts>.json`. Operator copies them to
// `loadtest/card/state/cards.json` before running list/invoice/mixed scenarios.

import http from 'k6/http';
import { check } from 'k6';
import { cfg, postHeaders, cardsURL, idemKey, timestamp, resultsPath, statePath } from './common.js';

const SEED_COUNT = parseInt(__ENV.SEED_COUNT || '20', 10);

export const options = {
  scenarios: {
    seed: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: SEED_COUNT,
      maxDuration: '60s',
    },
  },
  thresholds: {
    'http_req_failed': ['rate<0.01'],
  },
};

const createdIDs = [];

export default function () {
  const i = __ITER + 1;
  const body = JSON.stringify({
    name: `Card Seed ${i}`,
    nickname: `seed-${i}`,
    closing_day: ((i % 28) + 1),
    due_day: (((i + 7) % 28) + 1),
  });

  const key = idemKey('setup', __VU, __ITER);
  const res = http.post(cardsURL(), body, { headers: postHeaders(key), tags: { op: 'seed' } });
  const ok = check(res, {
    'setup status 201/200': (r) => r.status === 201 || r.status === 200,
  });

  if (ok) {
    try {
      const parsed = JSON.parse(res.body);
      if (parsed && parsed.id) {
        createdIDs.push(parsed.id);
      }
    } catch (e) {
      // ignore parse errors; surfaced in summary as missing ID
    }
  }
}

export function handleSummary(data) {
  const ts = timestamp();
  const payload = {
    base_url: cfg.baseURL,
    user_id: cfg.userID,
    created_at: new Date().toISOString(),
    card_ids: createdIDs,
    seed_count: SEED_COUNT,
  };
  return {
    'stdout': `Seeded ${createdIDs.length} cards. IDs:\n${JSON.stringify(payload, null, 2)}\n`,
    [resultsPath(`setup-${ts}.json`)]: JSON.stringify(payload, null, 2),
    [statePath(`cards.json`)]: JSON.stringify(payload, null, 2),
  };
}
