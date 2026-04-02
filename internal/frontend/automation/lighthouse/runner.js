'use strict';

const lighthouse = require('lighthouse');
const { buildLighthouseConfig } = require('./config');

/**
 * Lighthouse profiler for baseline and stress auditing.
 * Runs audits on the page and returns performance metrics.
 */

class LighthouseProfiler {
  constructor(chromePort = 9222) {
    this.chromePort = chromePort;
  }

  async runAudit(pageUrl, phase = 'baseline') {
    const config = buildLighthouseConfig(phase);
    const options = {
      logLevel: 'error',
      port: this.chromePort,
    };

    try {
      const runnerResult = await lighthouse(pageUrl, options, config);
      return this._normalizeResult(runnerResult, phase);
    } catch (err) {
      return {
        phase,
        pageUrl,
        success: false,
        error: err.message,
        metrics: {},
      };
    }
  }

  _normalizeResult(result, phase) {
    const audits = result.lhr.audits || {};
    const metrics = {};

    // Extract Core Web Vitals metrics
    const metricMap = {
      'first-contentful-paint': 'fcp',
      'largest-contentful-paint': 'lcp',
      'cumulative-layout-shift': 'cls',
      'total-blocking-time': 'tbt',
      'time-to-interactive': 'tti',
      'speed-index': 'speedIndex',
    };

    for (const [auditId, key] of Object.entries(metricMap)) {
      const audit = audits[auditId];
      if (audit && audit.numericValue !== undefined) {
        metrics[key] = Math.round(audit.numericValue);
      }
    }

    return {
      phase,
      pageUrl: result.lhr.requestedUrl,
      success: true,
      metrics,
      score: {
        performance: (result.lhr.categories.performance || {}).score || 0,
        accessibility: (result.lhr.categories.accessibility || {}).score || 0,
      },
    };
  }
}

module.exports = LighthouseProfiler;
