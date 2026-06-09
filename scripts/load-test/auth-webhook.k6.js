import http from 'k6/http';
import crypto from 'k6/crypto';
import { check, sleep } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';

export const options = {
  scenarios: {
    sustained: {
      executor: 'constant-arrival-rate',
      rate: 500,
      timeUnit: '1m',
      duration: '10m',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(99)<300'],
    http_req_failed: ['rate<0.001'],
    checks: ['rate>0.999'],
  },
};

const webhookUrl = __ENV.WEBHOOK_URL;
const metaAppSecret = __ENV.META_APP_SECRET;

if (!webhookUrl) {
  throw new Error('WEBHOOK_URL env var e obrigatorio');
}
if (!metaAppSecret) {
  throw new Error('META_APP_SECRET env var e obrigatorio');
}

function samplePayload() {
  const wamid = 'wamid.load' + Math.random().toString(36).substring(2, 18);
  const from = '5511' + String(Math.floor(Math.random() * 900000000) + 100000000);
  const ts = Math.floor(Date.now() / 1000).toString();

  return {
    object: 'whatsapp_business_account',
    entry: [
      {
        id: 'load-test-entry',
        changes: [
          {
            field: 'messages',
            value: {
              messaging_product: 'whatsapp',
              messages: [
                {
                  from: from,
                  id: wamid,
                  timestamp: ts,
                  type: 'text',
                  text: { body: 'load test message' },
                },
              ],
            },
          },
        ],
      },
    ],
  };
}

export default function () {
  const body = JSON.stringify(samplePayload());
  const sig = 'sha256=' + crypto.hmac('sha256', metaAppSecret, body, 'hex');

  const res = http.post(webhookUrl, body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Hub-Signature-256': sig,
    },
    timeout: '5s',
  });

  check(res, {
    'status 200': (r) => r.status === 200,
    'nao eh 5xx': (r) => r.status < 500,
  });
}
