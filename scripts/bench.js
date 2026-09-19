#!/usr/bin/env node
// 素见性能压测脚本（零依赖，Node 内置 http）
// 用法：先启动服务（默认 http://127.0.0.1:8099，可用 BASE 环境变量覆盖），再运行：
//   node scripts/bench.js                 # 默认 50 并发
//   CONC=500 node scripts/bench.js        # 500 并发压力测试
// 输出各接口的吞吐(QPS)与延迟(p50/p95/p99)，以及模拟多设备同时打开应用的场景。
const http = require('http');
const BASE = process.env.BASE || 'http://127.0.0.1:8099';
const CONC = parseInt(process.env.CONC || '50', 10);
const TOTAL = parseInt(process.env.TOTAL || '400', 10);
const agent = new http.Agent({ keepAlive: true, maxSockets: CONC + 100 });

function bench(path, opts = {}, concurrency = CONC, total = TOTAL) {
  return new Promise((resolve) => {
    const lat = [];
    let done = 0, errs = 0, sent = 0;
    const start = Date.now();
    function worker() {
      while (sent < total) {
        sent++;
        const t0 = process.hrtime.bigint();
        const req = http.request(BASE + path, Object.assign({ agent }, opts), res => {
          res.resume();
          res.on('end', () => {
            lat.push(Number(process.hrtime.bigint() - t0) / 1e6);
            done++;
            if (done >= total) finish();
          });
        });
        req.on('error', () => { errs++; done++; if (done >= total) finish(); });
        req.end();
      }
    }
    function finish() {
      const elapsed = (Date.now() - start) / 1000;
      lat.sort((a, b) => a - b);
      const q = p => lat[Math.min(lat.length - 1, Math.floor(lat.length * p))];
      console.log(`  ${path.padEnd(34)} ${concurrency}并发×${total}次: QPS=${(total / elapsed).toFixed(0).padStart(6)}  p50=${q(0.5).toFixed(1).padStart(6)}ms  p95=${q(0.95).toFixed(1).padStart(6)}ms  p99=${q(0.99).toFixed(1).padStart(6)}ms  err=${errs}`);
      resolve();
    }
    for (let i = 0; i < concurrency; i++) worker();
  });
}

async function login() {
  return await new Promise((resolve, reject) => {
    const body = JSON.stringify({ username: 'admin', password: 'admin123' });
    const req = http.request(BASE + '/api/login', { method: 'POST', agent, headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) } }, res => {
      let d = ''; res.on('data', c => d += c); res.on('end', () => { try { resolve(JSON.parse(d).token); } catch (e) { reject(e); } });
    });
    req.on('error', reject); req.end(body);
  });
}

(async () => {
  console.log(`素见性能压测  BASE=${BASE}  ${CONC}并发×${TOTAL}次/接口`);
  let token = '';
  try { token = await login(); } catch (e) { console.log('  (未登录，跳过个性化推荐测试)'); }

  console.log('--- 读路径 ---');
  await bench('/api/feed?sort=hot');
  if (token) await bench('/api/feed?sort=hot', { headers: { Cookie: 'session=' + token } }); // 智能推荐
  await bench('/api/feed?q=测试');
  await bench('/');
  console.log('--- 写路径（需登录 + NOTE_ID 指定一篇存在的笔记）---');
  if (token && process.env.NOTE_ID) {
    const cookie = { Cookie: 'session=' + token };
    let n = 0;
    await bench('/api/notes/' + process.env.NOTE_ID + '/like', { method: 'POST', headers: cookie }, CONC, Math.max(100, Math.floor(TOTAL / 2)));
  } else if (!token) {
    console.log('  (未登录，跳过)');
  } else {
    console.log('  (未指定 NOTE_ID，跳过；用法：NOTE_ID=n_xxx node scripts/bench.js)');
  }
  console.log('完成。提示：单接口延迟含客户端开销，服务端真实延迟通常更低（curl 直测为亚毫秒~毫秒级）。');
  process.exit(0);
})().catch(e => { console.error(e); process.exit(1); });
