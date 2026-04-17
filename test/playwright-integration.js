'use strict';

/**
 * Integration Test: Playwright Runner with Chaos Integration
 * Tests the full chaos injection pipeline with actual Playwright.
 */

const { chromium } = require('playwright');
const ScenarioLoader = require('../internal/frontend/chaos/scenarios');
const NetworkInterceptor = require('../internal/frontend/chaos/interceptor');
const fs = require('fs');
const path = require('path');

const BASE_URL = process.env.BASE_URL || 'http://127.0.0.1:3001';
const METRICS_ENDPOINT = process.env.METRICS_ENDPOINT || 'http://localhost:8000/frontend/metrics';
const EXPERIMENT_ID = process.env.EXPERIMENT_ID || 'integration-test-1';

const METRIC_INTERVAL_MS = parseInt(process.env.METRIC_INTERVAL_MS || '500', 10);
const EXTRA_KEEPALIVE_MS = parseInt(process.env.EXTRA_KEEPALIVE_MS || '10000', 10);
const EMIT_SYNTHETIC_METRICS = process.env.EMIT_SYNTHETIC_METRICS !== 'false';
const KEEP_BROWSER_OPEN = process.env.KEEP_BROWSER_OPEN === 'true';
const PLAYWRIGHT_HEADLESS = process.env.PLAYWRIGHT_HEADLESS === 'false' ? false : true;
const FORWARD_METRICS = process.env.FORWARD_METRICS === 'true';

async function runTest() {
  console.log('[playwright-integration-test] starting');
  console.log(`[playwright-integration-test] base_url=${BASE_URL}`);
  console.log(`[playwright-integration-test] metrics_endpoint=${METRICS_ENDPOINT}`);
  console.log(`[playwright-integration-test] experiment_id=${EXPERIMENT_ID}`);

  let browser;
  let page;

  try {
    // Launch browser
    console.log('[playwright-integration-test] launching chromium');
    browser = await chromium.launch({ headless: PLAYWRIGHT_HEADLESS, args: ['--disable-dev-shm-usage'] });
    page = await browser.newPage();

    // Load scenario
    console.log('[playwright-integration-test] loading scenario: latency');
    const scenarioLoader = new ScenarioLoader();
    const scenario = await scenarioLoader.load('latency');
    console.log('[playwright-integration-test] scenario loaded:', JSON.stringify(scenario.chaos, null, 2));

    // Setup chaos interceptor
    console.log('[playwright-integration-test] installing network interceptor');
    const interceptor = new NetworkInterceptor(page);
    interceptor.setFaults(scenario.chaos);
    await interceptor.install();

    // Setup metric collection
    const collectedMetrics = [];
    page.on('console', (msg) => {
      const text = msg.text();
      if (text && text.includes && text.includes('[failsafe-metric]')) {
        collectedMetrics.push(text);
      }
    });

    // Note: synthetic emitter will be started after navigation so it survives page load

    // Navigate
    console.log(`[playwright-integration-test] navigating to ${BASE_URL}`);
    const navigationPromise = page.goto(BASE_URL, { waitUntil: 'networkidle' }).catch((err) => {
      console.warn(`[playwright-integration-test] navigation warning: ${err.message}`);
    });

    // Execute baseline phase
    console.log(`[playwright-integration-test] baseline phase (${scenario.phases.baselineMs}ms)`);
    await navigationPromise;
    await page.waitForTimeout(scenario.phases.baselineMs);

    // Execute injecting phase with chaos
    console.log(`[playwright-integration-test] injecting phase (${scenario.phases.injectingMs}ms)`);
    // Start synthetic emitter now so it runs in the loaded page context
    if (EMIT_SYNTHETIC_METRICS) {
      try {
        await page.evaluate(({ interval, experimentId, phase }) => {
          if (window._failsafe_metric_interval) clearInterval(window._failsafe_metric_interval);
          window._failsafe_metric_interval = setInterval(() => {
            try {
              const rand = Math.random();
              const metric = {
                experiment_id: experimentId,
                phase: phase,
                page: (window.location && window.location.pathname) ? window.location.pathname : '/',
                metrics: {
                  lcp: rand * 2000, // synthetic LCP in ms
                  cls: rand * 0.25, // synthetic CLS
                  inp: rand * 100, // synthetic INP
                  long_tasks: Math.floor(rand * 3),
                  errors: 0,
                  unhandled_rejections: 0
                },
                api_calls: [],
                timestamp: Date.now()
              };
              console.log('[failsafe-metric] ' + JSON.stringify(metric));
            } catch (e) {}
          }, interval || 500);
        }, { interval: METRIC_INTERVAL_MS, experimentId: EXPERIMENT_ID, phase: 'injecting' });
      } catch (e) {
        console.warn('[playwright-integration-test] could not start synthetic emitter', e.message);
      }
    }

    // Perform repeated interactions to generate more metrics and exercise UI
    const injectingEnd = Date.now() + scenario.phases.injectingMs;
    while (Date.now() < injectingEnd) {
      await page.click('button.button').catch(() => {});
      await page.waitForTimeout(500).catch(() => {});
    }

    // Execute recovery phase
    console.log(`[playwright-integration-test] recovery phase (${scenario.phases.recoveryMs}ms)`);
    await page.unroute('**/*');
    await page.waitForTimeout(scenario.phases.recoveryMs);

    // Give extra time for any remaining synthetic metrics to flush
    if (EXTRA_KEEPALIVE_MS > 0) {
      console.log(`[playwright-integration-test] extra keepalive (${EXTRA_KEEPALIVE_MS}ms)`);
      await page.waitForTimeout(EXTRA_KEEPALIVE_MS).catch(() => {});
    }

    // Stop synthetic emitter if it was started
    try {
      await page.evaluate(() => {
        if (window._failsafe_metric_interval) {
          clearInterval(window._failsafe_metric_interval);
          window._failsafe_metric_interval = null;
        }
      });
    } catch (e) {
      // ignore
    }

    console.log('[playwright-integration-test] experiment completed');
    console.log(`[playwright-integration-test] metrics collected: ${collectedMetrics.length}`);

    // Parse collected metric strings into JSON objects where possible
    const parsedMetrics = [];
    for (const entry of collectedMetrics) {
      try {
        const idx = entry.indexOf('{');
        if (idx >= 0) {
          const jsonStr = entry.slice(idx);
          parsedMetrics.push(JSON.parse(jsonStr));
        } else {
          parsedMetrics.push({ raw: entry });
        }
      } catch (e) {
        parsedMetrics.push({ raw: entry, parseError: e.message });
      }
    }

    // Persist collected metric entries for inspection
    try {
      const outDir = path.join(__dirname, '..', 'experiments', 'results');
      if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });
      const outPath = path.join(outDir, `${EXPERIMENT_ID}-metrics.json`);
      fs.writeFileSync(outPath, JSON.stringify(parsedMetrics, null, 2));
      console.log(`[playwright-integration-test] metrics saved: ${outPath}`);
    } catch (e) {
      console.warn('[playwright-integration-test] could not save metrics', e.message);
    }

    // Optionally forward metrics to configured ingestion endpoint
    if (FORWARD_METRICS) {
      try {
        console.log(`[playwright-integration-test] forwarding ${parsedMetrics.length} metrics to ${METRICS_ENDPOINT}`);
        const payload = { experiment: EXPERIMENT_ID, metrics: parsedMetrics };
        const resp = await fetch(METRICS_ENDPOINT, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        console.log('[playwright-integration-test] forward response:', resp.status, resp.statusText);
      } catch (e) {
        console.warn('[playwright-integration-test] failed to forward metrics', e.message);
      }
    }

    return {
      success: true,
      scenario,
      metricsCollected: collectedMetrics.length,
    };
  } catch (err) {
    console.error('[playwright-integration-test] error:', err.message);
    return {
      success: false,
      error: err.message,
    };
  } finally {
    if (page) {
      if (!KEEP_BROWSER_OPEN) await page.close().catch(() => {});
      else console.log('[playwright-integration-test] keeping page open (KEEP_BROWSER_OPEN=true)');
    }
    if (browser) {
      if (!KEEP_BROWSER_OPEN) await browser.close().catch(() => {});
      else console.log('[playwright-integration-test] keeping browser open (KEEP_BROWSER_OPEN=true)');
    }
  }
}

// Run test
runTest()
  .then((result) => {
    console.log('[playwright-integration-test] result:', JSON.stringify(result, null, 2));
    process.exit(result.success ? 0 : 1);
  })
  .catch((err) => {
    console.error('[playwright-integration-test] fatal error:', err);
    process.exit(1);
  });
