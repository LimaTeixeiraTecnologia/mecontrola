// k6 load test — Telegram webhook (LLM call internally).
//
// Envs:
//   BACKEND                  base URL of API (default: http://localhost:8080)
//   TELEGRAM_WEBHOOK_SECRET  value sent in X-Telegram-Bot-Api-Secret-Token (required)
//   VUS                      virtual users (default: 30)
//   DURATION                 test duration (default: 2m)
//
// Run:
//   k6 run scripts/loadtest/telegram-webhook.js
//
// Acceptance:
//   - p95 < 1000 ms (LLM call dominates)
//   - error rate < 2 %

import http from 'k6/http';
import { check } from 'k6';

const BACKEND = __ENV.BACKEND || 'http://localhost:8080';
const SECRET = __ENV.TELEGRAM_WEBHOOK_SECRET;

if (!SECRET) {
  throw new Error('TELEGRAM_WEBHOOK_SECRET is required');
}

export const options = {
  vus: parseInt(__ENV.VUS || '30', 10),
  duration: __ENV.DURATION || '2m',
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.02'],
  },
};

const MESSAGES = [
  'oi',
  'gastei 25 reais no almoço',
  'qual meu saldo do mês?',
  'lancei 120 no mercado ontem',
  'extrato',
];

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function buildUpdate() {
  const updateID = Math.floor(Math.random() * 1e9);
  const userID = 100000 + Math.floor(Math.random() * 10000);
  const chatID = userID;
  const ts = Math.floor(Date.now() / 1000);
  return {
    update_id: updateID,
    message: {
      message_id: updateID,
      from: {
        id: userID,
        is_bot: false,
        first_name: 'Load',
        last_name: 'Tester',
        username: 'load_' + userID,
        language_code: 'pt-br',
      },
      chat: {
        id: chatID,
        first_name: 'Load',
        last_name: 'Tester',
        type: 'private',
      },
      date: ts,
      text: pick(MESSAGES),
    },
  };
}

export default function () {
  const body = JSON.stringify(buildUpdate());
  const url = `${BACKEND}/api/v1/channels/telegram/webhook`;

  const res = http.post(url, body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Telegram-Bot-Api-Secret-Token': SECRET,
    },
    tags: { endpoint: 'telegram_webhook' },
  });

  check(res, {
    'status is 2xx': (r) => r.status >= 200 && r.status < 300,
  });
}
