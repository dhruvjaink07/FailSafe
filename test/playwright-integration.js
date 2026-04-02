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
    browser = await chromium.launch({ headless: true });
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
      if (text.includes('[failsafe-metric]')) {
        collectedMetrics.push(text);
      }
    });

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
    await page.click('button.button').catch(() => {});
    await page.waitForTimeout(1000).catch(() => {});
    await page.waitForTimeout(scenario.phases.injectingMs);

    // Execute recovery phase
    console.log(`[playwright-integration-test] recovery phase (${scenario.phases.recoveryMs}ms)`);
    await page.unroute('**/*');
    await page.waitForTimeout(scenario.phases.recoveryMs);

    console.log('[playwright-integration-test] experiment completed');
    console.log(`[playwright-integration-test] metrics collected: ${collectedMetrics.length}`);

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
    if (page) await page.close().catch(() => {});
    if (browser) await browser.close().catch(() => {});
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
