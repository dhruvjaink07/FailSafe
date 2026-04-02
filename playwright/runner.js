"use strict";

const { chromium } = require("playwright");
const fs = require("fs");
const path = require("path");

const BASE_URL = process.env.BASE_URL || "https://example.com";
const EXPERIMENT_ID = process.env.EXPERIMENT_ID || "exp-live-1";
const FRONTEND_METRICS_ENDPOINT =
  process.env.FAILSAFE_FRONTEND_ENDPOINT || "http://localhost:8000/frontend/metrics";
const CONTROLLER_BASE_URL =
  process.env.FAILSAFE_CONTROLLER_URL || "http://localhost:8000";

const senderScript = fs.readFileSync(path.join(__dirname, "sender.js"), "utf-8");
const collectorScript = fs.readFileSync(path.join(__dirname, "collector.js"), "utf-8");

const SCENARIO = {
  pages: [
    { path: "/", actions: ["wait:1500"] },
  ],
  phases: {
    baselineMs: 5000,
    injectingMs: 10000,
    recoveryMs: 5000,
  },
  chaos: {
    enabled: true,
    networkDelayMs: 700,
    failureRate: 0.15,
    cpuSlowdownRate: 4,
    targetUrls: [""],
  },
};

function firstNonEmpty() {
  for (const value of arguments) {
    if (typeof value === "string" && value.trim() !== "") {
      return value.trim();
    }
  }
  return "";
}

function normalizeTargetUrls(value) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((entry) => (typeof entry === "string" ? entry : ""))
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0 || entry === "");
}

async function fetchExperimentFrontendConfig(controllerBaseUrl, experimentId) {
  const url = new URL("/experiments/frontend/status", controllerBaseUrl);
  url.searchParams.set("id", experimentId);

  const response = await fetch(url.toString());
  if (!response.ok) {
    throw new Error(`experiment fetch failed (${response.status})`);
  }

  const body = await response.json();
  const experiment = body && body.experiment ? body.experiment : body;
  if (!experiment || typeof experiment !== "object") {
    return null;
  }

  const frontendRun = experiment.frontend_run || experiment.frontendRun;
  if (!frontendRun || typeof frontendRun !== "object") {
    return null;
  }

  const baseUrl = firstNonEmpty(frontendRun.base_url, frontendRun.baseUrl);
  const metricsEndpoint = firstNonEmpty(
    frontendRun.metrics_endpoint,
    frontendRun.metricsEndpoint
  );
  const targetUrls = normalizeTargetUrls(
    frontendRun.target_urls || frontendRun.targetUrls
  );

  if (!baseUrl && !metricsEndpoint && targetUrls.length === 0) {
    return null;
  }

  return { baseUrl, metricsEndpoint, targetUrls };
}

async function resolveRuntimeConfig() {
  let experimentConfig = null;

  if (!process.env.BASE_URL || !process.env.FAILSAFE_FRONTEND_ENDPOINT) {
    try {
      experimentConfig = await fetchExperimentFrontendConfig(
        CONTROLLER_BASE_URL,
        EXPERIMENT_ID
      );
    } catch (err) {
      console.warn(`Failed to load experiment config: ${err.message}`);
    }
  }

  const baseUrl =
    process.env.BASE_URL ||
    (experimentConfig && experimentConfig.baseUrl) ||
    BASE_URL;

  const frontendMetricsEndpoint =
    process.env.FAILSAFE_FRONTEND_ENDPOINT ||
    (experimentConfig && experimentConfig.metricsEndpoint) ||
    FRONTEND_METRICS_ENDPOINT;

  const targetUrls =
    (experimentConfig && experimentConfig.targetUrls.length > 0 &&
      experimentConfig.targetUrls) ||
    SCENARIO.chaos.targetUrls;

  return {
    baseUrl,
    frontendMetricsEndpoint,
    scenario: {
      ...SCENARIO,
      chaos: {
        ...SCENARIO.chaos,
        targetUrls,
      },
    },
  };
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function hashCode(str) {
  let hash = 0;
  for (let i = 0; i < str.length; i += 1) {
    hash = (hash << 5) - hash + str.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

async function runActions(page, actions = []) {
  for (const action of actions) {
    if (action.startsWith("wait:")) {
      await page.waitForTimeout(Number(action.split(":")[1]) || 1000);
      continue;
    }

    if (action === "wait") {
      await page.waitForTimeout(1000);
      continue;
    }

    if (action.startsWith("click:")) {
      const selector = action.slice("click:".length);
      await page.click(selector).catch(() => {});
      continue;
    }

    if (action.startsWith("type:")) {
      const parts = action.split(":");
      const selector = parts[1];
      const value = parts.slice(2).join(":");
      await page.fill(selector, value).catch(() => {});
    }
  }
}

async function setPhase(page, phase) {
  await page.evaluate((phaseValue) => {
    window.__FAILSAFE_PHASE__ = phaseValue;
  }, phase);
}

async function applyChaos(context, page, chaos) {
  await context.route("**/*", async (route) => {
    const requestUrl = route.request().url();
    const isTarget = chaos.targetUrls.some((token) => requestUrl.includes(token));

    if (!isTarget) {
      return route.continue();
    }

    await sleep(chaos.networkDelayMs);

    if ((hashCode(requestUrl) % 100) < chaos.failureRate * 100) {
      return route.abort();
    }

    return route.continue();
  });

  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", {
    rate: chaos.cpuSlowdownRate,
  });
}

async function removeChaos(context, page) {
  await context.unroute("**/*");
  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", { rate: 1 });
}

async function run() {
  const runtimeConfig = await resolveRuntimeConfig();
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext();
  const page = await context.newPage();

  await page.addInitScript((expId) => {
    window.__FAILSAFE_EXPERIMENT_ID__ = expId;
  }, EXPERIMENT_ID);

  await page.addInitScript(() => {
    window.__FAILSAFE_PHASE__ = "baseline";
  });

  await page.addInitScript((endpoint) => {
    window.__FAILSAFE_ENDPOINT__ = endpoint;
  }, runtimeConfig.frontendMetricsEndpoint);

  await page.addInitScript({ content: senderScript });
  await page.addInitScript({ content: collectorScript });

  for (const pageConfig of runtimeConfig.scenario.pages) {
    const url = new URL(pageConfig.path, runtimeConfig.baseUrl).toString();
    console.log(`Running ${url}`);

    await page.goto(url, { waitUntil: "domcontentloaded" });

    await setPhase(page, "baseline");
    await runActions(page, pageConfig.actions);
    await sleep(runtimeConfig.scenario.phases.baselineMs);

    if (runtimeConfig.scenario.chaos.enabled) {
      await applyChaos(context, page, runtimeConfig.scenario.chaos);
    }

    await setPhase(page, "injecting");
    await runActions(page, pageConfig.actions);
    await sleep(runtimeConfig.scenario.phases.injectingMs);

    await removeChaos(context, page);
    await setPhase(page, "recovery");
    await runActions(page, pageConfig.actions);
    await sleep(runtimeConfig.scenario.phases.recoveryMs);

    await page.evaluate(() => {
      if (window.__FAILSAFE_FLUSH__) {
        window.__FAILSAFE_FLUSH__();
      }
    });
  }

  await page.evaluate(async () => {
    if (window.__FAILSAFE_FLUSH__) {
      await window.__FAILSAFE_FLUSH__();
    }
  });

  await browser.close();
}

run().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
