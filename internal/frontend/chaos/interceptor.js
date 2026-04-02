'use strict';

/**
 * Network fault interception handler for Playwright route interception.
 * Implements network delays, packet loss, request failures, and offline mode.
 */

class NetworkInterceptor {
  constructor(page) {
    this.page = page;
    this.faults = {
      enabled: false,
      networkDelayMs: 0,
      failureRate: 0,
      offline: false,
      targetUrls: [],
    };
  }

  /**
   * Configure fault injection parameters.
   */
  setFaults(config) {
    this.faults = {
      enabled: config.enabled || false,
      networkDelayMs: config.networkDelayMs || 0,
      failureRate: config.failureRate || 0,
      offline: config.offline || false,
      targetUrls: config.targetUrls || [],
    };
  }

  /**
   * Install route interceptor for the page.
   * Intercepts Fetch and XHR, applying faults based on configuration.
   */
  async install() {
    if (!this.faults.enabled) {
      return;
    }

    await this.page.route('**/*', async (route) => {
      const request = route.request();
      const url = request.url();

      // Check if URL matches target list
      if (!this._matchesTarget(url)) {
        await route.continue();
        return;
      }

      // Apply offline mode
      if (this.faults.offline) {
        await route.abort('blockedbyclient');
        return;
      }

      // Apply random failure
      if (Math.random() < this.faults.failureRate) {
        await route.abort('failed');
        return;
      }

      // Apply network delay
      if (this.faults.networkDelayMs > 0) {
        await new Promise((resolve) => setTimeout(resolve, this.faults.networkDelayMs));
      }

      await route.continue();
    });
  }

  /**
   * Uninstall route interceptor.
   */
  async uninstall() {
    await this.page.unroute('**/*');
  }

  /**
   * Check if URL matches any target patterns.
   * Empty string matches all; otherwise uses substring matching.
   */
  _matchesTarget(url) {
    if (this.faults.targetUrls.length === 0) return true;

    return this.faults.targetUrls.some((target) => {
      if (target === '' || target === '*') return true; // Match all
      return url.includes(target);
    });
  }
}

module.exports = NetworkInterceptor;
