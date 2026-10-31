import http from 'k6/http';
import grpc from 'k6/net/grpc';
import crypto from 'k6/crypto';
import { check } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';

const protocol = __ENV.PROTOCOL || 'rest';
const operation = __ENV.OPERATION || 'get';
const content = 'x'.repeat(1024);
const client = new grpc.Client();
client.load(['/work/proto'], 'paste.proto');
let connected = false;
const latency = new Trend('latency_ms', true);
const failed = new Rate('request_failed');
const successful = new Counter('successful_requests');
const uuid = () => {
  const b = new Uint8Array(crypto.randomBytes(16));
  b[6] = (b[6] & 15) | 64;
  b[8] = (b[8] & 63) | 128;
  const h = Array.from(b, x => ('0' + x.toString(16)).slice(-2)).join('');
  return `${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`;
};

export const options = {
  scenarios: { load: { executor: 'constant-arrival-rate', rate: Number(__ENV.RATE || 20), timeUnit: '1s', duration: __ENV.DURATION || '60s', preAllocatedVUs: 20, maxVUs: 100 } },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
  thresholds: { request_failed: ['rate==0'], checks: ['rate==1'], dropped_iterations: ['count==0'] },
};

export default function () {
  const key = uuid();
  let ok = false;
  let start;
  try {
    if (protocol === 'grpc' && !connected) {
      client.connect('host.docker.internal:9090', { plaintext: true, timeout: '5s' });
      connected = true;
    }
    start = Date.now();
    if (protocol === 'rest') {
      const response = operation === 'create'
        ? http.post('http://host.docker.internal:8080/api/v1/pastes', JSON.stringify({ content, visibility: 'public', expires_in: 3600 }), { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key }, timeout: '3s' })
        : http.get(`http://host.docker.internal:8080/api/v1/pastes/${__ENV.CODE}`, { timeout: '3s' });
      latency.add(Date.now() - start);
      ok = response.status === (operation === 'create' ? 201 : 200);
      if (ok) { const body = response.json(); ok = operation === 'create' ? !!body.short_code : body.content === content; }
    } else {
      const response = client.invoke(`paste.PasteService/${operation === 'create' ? 'CreatePaste' : 'GetPaste'}`, operation === 'create' ? { content, visibility: 'public', expiresIn: 3600 } : { shortCode: __ENV.CODE }, { metadata: { 'idempotency-key': key }, timeout: '3s' });
      latency.add(Date.now() - start);
      ok = response.status === grpc.StatusOK && (operation === 'create' ? !!response.message.shortCode : response.message.content === content);
    }
  } catch (_) { if (start !== undefined) latency.add(Date.now() - start); }
  check(ok, { 'correct response': value => value });
  failed.add(!ok);
  if (ok) successful.add(1);
}

export function handleSummary(data) {
  const row = { protocol, operation, rate: Number(__ENV.RATE || 20), duration: __ENV.DURATION || '60s', content_bytes: 1024, latency_ms: data.metrics.latency_ms?.values, failures: data.metrics.request_failed?.values, successful: data.metrics.successful_requests?.values, dropped: data.metrics.dropped_iterations?.values };
  const result = JSON.stringify(row, null, 2);
  return { stdout: result + '\n', [`/results/${__ENV.RUN || protocol + '-' + operation}.json`]: JSON.stringify(data, null, 2), [`/results/${__ENV.RUN || protocol + '-' + operation}-compact.json`]: result };
}
