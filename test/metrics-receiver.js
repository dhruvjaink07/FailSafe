#!/usr/bin/env node
'use strict';

const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = process.env.METRICS_RECEIVER_PORT || 8000;
const RECEIVER_PATH = '/frontend/metrics';

const server = http.createServer((req, res) => {
  if (req.method === 'POST' && req.url === RECEIVER_PATH) {
    let body = '';
    req.on('data', (chunk) => { body += chunk; });
    req.on('end', () => {
      try {
        const payload = JSON.parse(body);
        const id = payload && payload.experiment ? payload.experiment : `unknown-${Date.now()}`;
        const outDir = path.join(__dirname, '..', 'experiments', 'results');
        if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });
        const outPath = path.join(outDir, `${id}-forwarded.json`);
        fs.writeFileSync(outPath, JSON.stringify(payload, null, 2));
        console.log(`[metrics-receiver] saved ${Object.keys(payload.metrics || {}).length || (payload.metrics? payload.metrics.length:0)} metrics to ${outPath}`);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'ok' }));
      } catch (e) {
        console.error('[metrics-receiver] invalid payload', e.message);
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: e.message }));
      }
    });
    return;
  }

  // Simple health
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok' }));
    return;
  }

  res.writeHead(404);
  res.end();
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`[metrics-receiver] listening on http://127.0.0.1:${PORT}${RECEIVER_PATH}`);
});

process.on('SIGINT', () => {
  console.log('[metrics-receiver] shutting down');
  server.close(() => process.exit(0));
});
