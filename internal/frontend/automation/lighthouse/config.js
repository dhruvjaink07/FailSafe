'use strict';

/**
 * Lighthouse audit configuration for baseline and stress profiling.
 * Used by lighthouse/runner.js to configure audit behaviors.
 */

const LIGHTHOUSE_PRESETS = {
  baseline: {
    emulatedFormFactor: 'desktop',
    throttling: {
      rttMs: 40,
      throughputKbps: 10240,
      cpuSlowdownMultiplier: 1,
    },
    disableFullPageScreenshot: false,
    onlyCategories: ['performance', 'accessibility'],
  },
  stress: {
    emulatedFormFactor: 'desktop',
    throttling: {
      rttMs: 400,
      throughputKbps: 1024,
      cpuSlowdownMultiplier: 4,
    },
    disableFullPageScreenshot: false,
    onlyCategories: ['performance', 'accessibility'],
  },
};

const AUDIT_METRICS = [
  'first-contentful-paint',
  'largest-contentful-paint',
  'cumulative-layout-shift',
  'total-blocking-time',
  'time-to-interactive',
  'speed-index',
];

function buildLighthouseConfig(phase = 'baseline') {
  const preset = LIGHTHOUSE_PRESETS[phase] || LIGHTHOUSE_PRESETS.baseline;
  return {
    logLevel: 'error',
    output: 'json',
    onlyAudits: AUDIT_METRICS,
    ...preset,
  };
}

module.exports = {
  LIGHTHOUSE_PRESETS,
  AUDIT_METRICS,
  buildLighthouseConfig,
};
