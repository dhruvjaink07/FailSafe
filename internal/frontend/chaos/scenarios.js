'use strict';

const fs = require('fs');
const path = require('path');

/**
 * Chaos scenario loader and validator.
 * Merges scenario configs with defaults and provides schema validation.
 */

const DEFAULT_SCENARIO = {
  name: 'default',
  pages: [{ path: '/', actions: ['wait:1500'] }],
  phases: {
    baselineMs: 5000,
    injectingMs: 10000,
    recoveryMs: 5000,
  },
  chaos: {
    enabled: true,
    networkDelayMs: 0,
    failureRate: 0,
    cpuSlowdownRate: 1,
    targetUrls: [''],
  },
};

class ScenarioLoader {
  constructor(scenarioDir = null) {
    // Default to repository-level configs/scenarios/web when present.
    const repoConfigs = path.join(__dirname, '../../../configs/scenarios/web');
    this.scenarioDir = scenarioDir || (fs.existsSync(repoConfigs) ? repoConfigs : path.join(__dirname, '../../configs/scenarios/web'));
    this.scenarios = new Map();
  }

  /**
   * Load a scenario by name (e.g., "latency", "offline", "cpu_throttle").
   * Merges with defaults and validates.
   */
  async load(scenarioName) {
    if (this.scenarios.has(scenarioName)) {
      return this.scenarios.get(scenarioName);
    }

    const filePath = path.join(this.scenarioDir, `${scenarioName}.json`);
    if (!fs.existsSync(filePath)) {
      throw new Error(`Scenario not found: ${filePath}`);
    }

    const content = fs.readFileSync(filePath, 'utf-8');
    const config = JSON.parse(content);
    const merged = this._merge(DEFAULT_SCENARIO, config);
    this._validate(merged);

    this.scenarios.set(scenarioName, merged);
    return merged;
  }

  /**
   * Load all available scenarios.
   */
  async loadAll() {
    const scenarios = new Map();
    const files = fs.readdirSync(this.scenarioDir).filter((f) => f.endsWith('.json'));

    for (const file of files) {
      const name = file.replace('.json', '');
      try {
        scenarios.set(name, await this.load(name));
      } catch (err) {
        console.error(`Failed to load scenario ${name}:`, err.message);
      }
    }

    return scenarios;
  }

  /**
   * Deep merge config with defaults, preserving custom values.
   */
  _merge(defaults, config) {
    const result = JSON.parse(JSON.stringify(defaults));

    if (!config) return result;

    Object.keys(config).forEach((key) => {
      if (typeof config[key] === 'object' && config[key] !== null && !Array.isArray(config[key])) {
        result[key] = this._merge(result[key] || {}, config[key]);
      } else {
        result[key] = config[key];
      }
    });

    return result;
  }

  /**
   * Validate scenario schema.
   */
  _validate(scenario) {
    if (!scenario.pages || !Array.isArray(scenario.pages) || scenario.pages.length === 0) {
      throw new Error('Scenario must have at least one page in pages array');
    }

    if (!scenario.phases || typeof scenario.phases !== 'object') {
      throw new Error('Scenario must have phases object with timing configuration');
    }

    const { baselineMs, injectingMs, recoveryMs } = scenario.phases;
    if (typeof baselineMs !== 'number' || typeof injectingMs !== 'number' || typeof recoveryMs !== 'number') {
      throw new Error('All phase timings must be numbers (ms)');
    }

    if (!scenario.chaos || typeof scenario.chaos !== 'object') {
      throw new Error('Scenario must have chaos object with fault configuration');
    }

    const { enabled, networkDelayMs, failureRate, cpuSlowdownRate } = scenario.chaos;
    if (typeof enabled !== 'boolean') throw new Error('chaos.enabled must be boolean');
    if (typeof networkDelayMs !== 'number' || networkDelayMs < 0) throw new Error('chaos.networkDelayMs must be non-negative number');
    if (typeof failureRate !== 'number' || failureRate < 0 || failureRate > 1) throw new Error('chaos.failureRate must be between 0 and 1');
    if (typeof cpuSlowdownRate !== 'number' || cpuSlowdownRate < 1) throw new Error('chaos.cpuSlowdownRate must be >= 1');
  }
}

module.exports = ScenarioLoader;
