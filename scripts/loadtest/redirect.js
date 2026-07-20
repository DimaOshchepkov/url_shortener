// redirect.js — k6 load test with realistic (80/20) traffic distribution
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8080';

// 1. Читаем все алиасы
const aliasesData = open('/tmp/aliases.json');
const allAliases = JSON.parse(aliasesData);

if (allAliases.length === 0) {
  throw new Error('aliases.json is empty! Run setup.sh first.');
}

// 2. Разделяем на "горячие" (топ 10% ссылок) и "холодные" (остальные 90%)
// В реальности именно эти 10% будут собирать 80%+ всего трафика
const hotCount = Math.floor(allAliases.length * 0.1);
const hotAliases = allAliases.slice(0, hotCount);
const coldAliases = allAliases.slice(hotCount);


export const options = {
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  stages: [
    { duration: '10s', target: 50 },
    { duration: '30s', target: 200 },
    { duration: '30s', target: 500 },
    { duration: '30s', target: 1000 },
    { duration: '30s', target: 2000 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    // Редирект должен быть молниеносным (пользователь ждет)
    'http_req_duration{operation:redirect}': [
      'p(95)<500',   // 95% < 500 мс
      'p(99)<1000',  // 99% < 1000 мс
    ],

    // Ошибок быть не должно вообще
    'http_req_failed{operation:redirect}': ['rate<0.001'], // < 0.1%

    // Все проверки должны проходить
    'checks': ['rate>0.99'], // 99% checks успешны
  },
};

export default function () {
  // 3. Эмуляция распределения Ципфа: 80% шанс выбрать "горячую" ссылку
  const isHotTraffic = Math.random() < 0.8;

  // Выбираем пул, а затем случайно берем элемент из этого пула
  const pool = isHotTraffic ? hotAliases : coldAliases;
  const alias = pool[Math.floor(Math.random() * pool.length)];

  const res = http.get(`${BASE_URL}/${alias}`, {
    redirects: 0,
    tags: {
      operation: 'redirect',
      // Добавляем тег, чтобы в отчетах видеть, был ли это горячий или холодный запрос
      traffic_type: isHotTraffic ? 'hot' : 'cold'
    },
  });

  check(res, {
    'status is 302': (r) => r.status === 302,
    'has location header': (r) => Object.keys(r.headers).some(key => key.toLowerCase() === 'location'),
  });

  sleep(0.1);
}

export function handleSummary(data) {
  // k6 stores metrics at the top level of data.metrics, NOT under .values
  // .values exists but contains internal aggregation data — use top-level keys for summary
  const reqs = data.metrics.http_reqs || {};
  const failed = data.metrics.http_req_failed || {};
  const duration = data.metrics.http_req_duration || {};

  console.log('\n=== 🚀 Redirect Load Test Results (80/20 Distribution) ===');
  console.log(`Total requests:  ${reqs.count || 0}`);
  console.log(`Request rate:    ${(reqs.rate || 0).toFixed(1)}/s`);
  console.log(`Error rate:      ${((failed.value || 0) * 100).toFixed(2)}%`);
  console.log(`p50 (med):      ${(duration.med || 0).toFixed(2)}ms`);
  console.log(`p90 latency:     ${(duration['p(90)'] || 0).toFixed(2)}ms`);
  console.log(`p95 latency:     ${(duration['p(95)'] || 0).toFixed(2)}ms`);
  console.log(`p99 latency:     ${(duration['p(99)'] || 0).toFixed(2)}ms`);
  console.log(`avg latency:     ${(duration.avg || 0).toFixed(2)}ms`);
  console.log(`max latency:     ${(duration.max || 0).toFixed(2)}ms`);
  console.log('==========================================================\n');

  return {
    'results.json': JSON.stringify(data),
    'stdout': '',
  };
}