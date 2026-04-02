const fs = require('fs');
const path = require('path');

console.log('╔════════════════════════════════════════════════════════════╗');
console.log('║ FailSafe Integration Test - Verification Results            ║');
console.log('╚════════════════════════════════════════════════════════════╝');
console.log('');

let passed = 0;
let failed = 0;
// Set root directory context for requires
process.chdir(path.join(__dirname, '..'));


// Test 1: JavaScript files exist
console.log('Phase 1: Component Files');
console.log('─────────────────────────────────────────────────────────────');
const jsFiles = [
  'internal/frontend/runtime/vitals.js',
  'internal/frontend/runtime/errors.js',
  'internal/frontend/runtime/network.js',
  'internal/frontend/runtime/bootstrap.js',
  'internal/frontend/runtime/collector.js',
  'internal/frontend/transport/sender.js',
  'internal/frontend/chaos/scenarios.js',
  'internal/frontend/chaos/interceptor.js',
  'internal/frontend/chaos/service_worker.js',
  'internal/frontend/automation/lighthouse/config.js',
  'internal/frontend/automation/lighthouse/runner.js',
];

jsFiles.forEach(file => {
  const fullPath = path.join(__dirname, '..', file);
  if (fs.existsSync(fullPath)) {
    console.log('  ✓', file);
    passed++;
  } else {
    console.log('  ✗', file, '(NOT FOUND)');
    failed++;
  }
});

console.log('');
console.log('Phase 2: Scenario Configuration Files');
console.log('─────────────────────────────────────────────────────────────');
const scenarios = [
  { name: 'latency.json', expectedDelay: 700 },
  { name: 'cpu_throttle.json', expectedRate: 4 },
  { name: 'offline.json', expectedOffline: true }
];

scenarios.forEach(scenario => {
  const fullPath = path.join(__dirname, '..', 'configs/scenarios/web', scenario.name);
  if (fs.existsSync(fullPath)) {
    try {
      const content = JSON.parse(fs.readFileSync(fullPath, 'utf-8'));
      console.log(`  ✓ ${scenario.name} - name="${content.name}"`);
      passed++;
    } catch (e) {
      console.log(`  ✗ ${scenario.name} - INVALID JSON: ${e.message}`);
      failed++;
    }
  } else {
    console.log(`  ✗ ${scenario.name} - NOT FOUND`);
    failed++;
  }
});

console.log('');
console.log('Phase 3: Module Imports & Exports');
console.log('─────────────────────────────────────────────────────────────');

// Test ScenarioLoader
try {
  const Loader = require('../internal/frontend/chaos/scenarios');
  if (typeof Loader === 'function') {
    console.log('  ✓ ScenarioLoader (class)');
    passed++;
  } else {
    console.log('  ✗ ScenarioLoader - not a class');
    failed++;
  }
} catch (e) {
  console.log(`  ✗ ScenarioLoader - ${e.message}`);
  failed++;
}

// Test NetworkInterceptor
try {
  const Interceptor = require('../internal/frontend/chaos/interceptor');
  if (typeof Interceptor === 'function') {
    console.log('  ✓ NetworkInterceptor (class)');
    passed++;
  } else {
    console.log('  ✗ NetworkInterceptor - not a class');
    failed++;
  }
} catch (e) {
  console.log(`  ✗ NetworkInterceptor - ${e.message}`);
  failed++;
}

// Test Lighthouse config
try {
  const config = require('../internal/frontend/automation/lighthouse/config');
  if (config.LIGHTHOUSE_PRESETS && config.buildLighthouseConfig) {
    console.log('  ✓ Lighthouse config (presets + builder)');
    passed++;
  } else {
    console.log('  ✗ Lighthouse config - missing exports');
    failed++;
  }
} catch (e) {
  console.log(`  ✗ Lighthouse config - ${e.message}`);
  failed++;
}

console.log('');
console.log('Phase 4: Scenario Loading');
console.log('─────────────────────────────────────────────────────────────');

const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
const loader = new ScenarioLoaderClass();

Promise.all([
  loader.load('latency'),
  loader.load('cpu_throttle'),
  loader.load('offline')
])
  .then((loadedScenarios) => {
    loadedScenarios.forEach((s) => {
      console.log(`  ✓ ${s.name} loaded (${s.phases.injectingMs}ms injecting)`);
      passed++;
    });

    console.log('');
    console.log('╔════════════════════════════════════════════════════════════╗');
    if (failed === 0) {
      console.log('║ VERIFICATION PASSED ✓                                  ║');
      console.log(`║ Total: ${passed + failed} components verified             ║`);
    } else {
      console.log('║ VERIFICATION FAILED ✗                                  ║');
      console.log(`║ Passed: ${passed} | Failed: ${failed}`);
    }
    console.log('╚════════════════════════════════════════════════════════════╝');
    console.log('');
    console.log('Ready to run integration tests. Execute:');
    console.log('  .\test\integration-test.ps1');
    console.log('');

    process.exit(failed > 0 ? 1 : 0);
  })
  .catch((err) => {
    console.error(`  ✗ Scenario loading failed - ${err.message}`);
    failed++;

    console.log('');
    console.log('╔════════════════════════════════════════════════════════════╗');
    console.log('║ VERIFICATION FAILED ✗                                  ║');
    console.log(`║ Passed: ${passed} | Failed: ${failed}`);
    console.log('╚════════════════════════════════════════════════════════════╝');
    process.exit(1);
  });
