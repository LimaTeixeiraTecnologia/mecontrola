import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';

const BACKEND = __ENV.BACKEND || 'http://localhost:8080';
const SECRET = __ENV.WHATSAPP_APP_SECRET;
const PHONE_NUMBER_ID = __ENV.PHONE_NUMBER_ID || '100000000000000';
const FROM_NUMBER = __ENV.FROM_NUMBER || '5511900000000';

const ENVELOPE = __ENV.ENVELOPE || 'a';

const ENVELOPES = {
  a: {
    vus: parseInt(__ENV.VUS || '2', 10),
    duration: __ENV.DURATION || '5m',
    thinkTimeSec: parseFloat(__ENV.THINK_TIME || '8'),
  },
  b: {
    vus: parseInt(__ENV.VUS || '5', 10),
    duration: __ENV.DURATION || '10m',
    thinkTimeSec: parseFloat(__ENV.THINK_TIME || '2'),
  },
};

const profile = ENVELOPES[ENVELOPE] || ENVELOPES['a'];

export const options = {
  vus: profile.vus,
  duration: profile.duration,
  thresholds: {
    'http_req_duration{endpoint:whatsapp_inbound}': ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

function wamid() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let id = 'wamid.';
  for (let i = 0; i < 40; i++) {
    id += chars[Math.floor(Math.random() * chars.length)];
  }
  return id;
}

function buildPayload(from) {
  const ts = Math.floor(Date.now() / 1000).toString();
  return {
    object: 'whatsapp_business_account',
    entry: [
      {
        id: PHONE_NUMBER_ID,
        changes: [
          {
            field: 'messages',
            value: {
              messaging_product: 'whatsapp',
              metadata: {
                display_phone_number: '5511900000001',
                phone_number_id: PHONE_NUMBER_ID,
              },
              messages: [
                {
                  from: from,
                  id: wamid(),
                  timestamp: ts,
                  type: 'text',
                  text: { body: 'quanto gastei este mes' },
                },
              ],
            },
          },
        ],
      },
    ],
  };
}

function sign(body, secret) {
  const mac = crypto.hmac('sha256', secret, body, 'hex');
  return 'sha256=' + mac;
}

export default function () {
  const from = FROM_NUMBER;
  const body = JSON.stringify(buildPayload(from));

  const headers = {
    'Content-Type': 'application/json',
  };

  if (SECRET) {
    headers['X-Hub-Signature-256'] = sign(body, SECRET);
  }

  const res = http.post(`${BACKEND}/api/v1/whatsapp/inbound`, body, {
    headers: headers,
    tags: { endpoint: 'whatsapp_inbound' },
  });

  check(res, {
    'status accepted or ok': (r) => r.status === 200 || r.status === 202 || r.status === 204,
    'not server error': (r) => r.status < 500,
  });

  if (profile.thinkTimeSec > 0) {
    sleep(profile.thinkTimeSec);
  }
}
