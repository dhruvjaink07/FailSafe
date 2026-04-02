#!/usr/bin/env node
'use strict';

/**
 * Integration Test: Web Server
 * Simple test server for Playwright to visit during experiments.
 */

const http = require('http');
const url = require('url');

const PORT = process.env.TEST_SERVER_PORT || 3001;

const server = http.createServer((req, res) => {
  const parsedUrl = url.parse(req.url, true);
  const pathname = parsedUrl.pathname;

  // CORS headers
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  // Health check
  if (pathname === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', uptime: process.uptime() }));
    return;
  }

  // Simulate slow API
  if (pathname === '/api/slow') {
    setTimeout(() => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ message: 'delayed response', delay: 'slow' }));
    }, 1000);
    return;
  }

  // Simulate fast API
  if (pathname === '/api/fast') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ message: 'fast response', delay: 'instant' }));
    return;
  }

  // Main page with interactive elements
  if (pathname === '/' || pathname === '') {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    const html = `
<!DOCTYPE html>
<html>
<head>
  <title>FailSafe Test App</title>
  <style>
    body { font-family: Arial; margin: 20px; background: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    h1 { color: #333; }
    .button { display: inline-block; padding: 10px 20px; margin: 5px; cursor: pointer; background: #0066cc; color: white; border: none; border-radius: 4px; font-size: 14px; }
    .button:hover { background: #0052a3; }
    .status { margin-top: 20px; padding: 10px; background: #e8f4f8; border-left: 4px solid #0066cc; border-radius: 4px; }
    .log { margin-top: 10px; padding: 10px; background: #f9f9f9; border: 1px solid #ddd; border-radius: 4px; max-height: 150px; overflow-y: auto; font-family: monospace; font-size: 12px; }
  </style>
</head>
<body>
  <div class="container">
    <h1>FailSafe Integration Test</h1>
    <p>This page is used for Playwright automation testing.</p>
    
    <button class="button" onclick="testFastAPI()">Test Fast API</button>
    <button class="button" onclick="testSlowAPI()">Test Slow API</button>
    <button class="button" onclick="triggerError()">Trigger Error</button>
    <button class="button" onclick="clearLog()">Clear Log</button>
    
    <div class="status">
      <strong>Status:</strong> <span id="status">Ready</span>
    </div>
    
    <div class="log">
      <div id="log-content"></div>
    </div>
  </div>

  <script>
    function log(msg) {
      const logDiv = document.getElementById('log-content');
      const line = document.createElement('div');
      line.textContent = new Date().toISOString() + ' - ' + msg;
      logDiv.appendChild(line);
      logDiv.scrollTop = logDiv.scrollHeight;
    }

    function clearLog() {
      document.getElementById('log-content').innerHTML = '';
    }

    async function testFastAPI() {
      document.getElementById('status').textContent = 'Testing fast API...';
      try {
        const resp = await fetch('/api/fast');
        const data = await resp.json();
        log('Fast API: ' + JSON.stringify(data));
        document.getElementById('status').textContent = 'Fast API OK';
      } catch (err) {
        log('Fast API error: ' + err.message);
        document.getElementById('status').textContent = 'Error';
      }
    }

    async function testSlowAPI() {
      document.getElementById('status').textContent = 'Testing slow API...';
      try {
        const resp = await fetch('/api/slow');
        const data = await resp.json();
        log('Slow API: ' + JSON.stringify(data));
        document.getElementById('status').textContent = 'Slow API OK';
      } catch (err) {
        log('Slow API error: ' + err.message);
        document.getElementById('status').textContent = 'Error';
      }
    }

    function triggerError() {
      try {
        throw new Error('Intentional test error');
      } catch (err) {
        log('Error triggered: ' + err.message);
        document.getElementById('status').textContent = 'Error thrown';
      }
    }

    log('Page loaded');
  </script>
</body>
</html>
    `;
    res.end(html);
    return;
  }

  // 404
  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ error: 'not found' }));
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`[test-server] listening on http://127.0.0.1:${PORT}`);
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('[test-server] shutting down');
  server.close(() => {
    process.exit(0);
  });
});
