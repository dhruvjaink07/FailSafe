// playwright/runner.js

import { chromium } from "playwright";
import fs from "fs";

// ===== CONFIG =====
const BASE_URL = "http://localhost:3000";

const SCENARIO = {
  experimentId: "exp-local-1",

  pages: [
    {
      path: "/",
      actions: ["wait"],
    },
    {
      path: "/dashboard",
      actions: ["wait", "click:#refresh-btn"],
    },
    {
      path: "/checkout",
      actions: ["wait", "type:#card-input:4111111111111111"],
    },
  ],

  phases: {
    baseline: 5000,
    injecting: 10000,
    recovery: 5000,
  },

  chaos: {
    enabled: true,
    networkDelayMs: 1000,
    failureRate: 0.2,
    cpuSlowdownRate: 4,
    targetUrls: ["/api/"], // ONLY affect API calls
  },
};

// ===== LOAD COLLECTOR =====
const collectorScript = fs.readFileSync("./collector.js", "utf-8");

// ===== MAIN =====
async function run() {

  const browser = await chromium.launch({ headless: false });

  const context = await browser.newContext();

  const page = await context.newPage();

  // inject experiment context
  await page.addInitScript((expId) => {
    window.__FAILSAFE_EXPERIMENT_ID__ = expId;
  }, SCENARIO.experimentId);

  // inject collector
  await page.addInitScript(collectorScript);

  for (const pageConfig of SCENARIO.pages) {

    console.log("Running:", pageConfig.path);

    // ===== BASELINE =====
    await setPhase(page, "baseline");
    await navigate(page, pageConfig.path);
    await runActions(page, pageConfig.actions);
    await wait(SCENARIO.phases.baseline);

    // ===== CHAOS =====
    if (SCENARIO.chaos.enabled) {
      await applyChaos(context, page);
    }

    await setPhase(page, "injecting");
    await runActions(page, pageConfig.actions);
    await wait(SCENARIO.phases.injecting);

    // ===== RECOVERY =====
    await removeChaos(context, page);
    await setPhase(page, "recovery");
    await runActions(page, pageConfig.actions);
    await wait(SCENARIO.phases.recovery);

    // force flush before moving page
    await flush(page);
  }

  await browser.close();
}

// ===== NAVIGATION =====
async function navigate(page, path) {
  await page.goto(BASE_URL + path, {
    waitUntil: "networkidle",
  });
}

// ===== ACTION ENGINE =====
async function runActions(page, actions = []) {

  for (const action of actions) {

    if (action === "wait") {
      await page.waitForTimeout(1000);
      continue;
    }

    if (action.startsWith("click:")) {
      const selector = action.split(":")[1];
      await page.click(selector).catch(() => {});
      continue;
    }

    if (action.startsWith("type:")) {
      const [, selector, value] = action.split(":");
      await page.fill(selector, value).catch(() => {});
      continue;
    }
  }
}

// ===== PHASE CONTROL =====
async function setPhase(page, phase) {
  await page.evaluate((p) => {
    window.__FAILSAFE_PHASE__ = p;
  }, phase);
}

// ===== CHAOS ENGINE =====

async function applyChaos(context, page) {

  const chaos = SCENARIO.chaos;

  // --- NETWORK INTERCEPTION (API ONLY) ---
  await context.route("**/*", async (route) => {

    const url = route.request().url();

    const isTarget = chaos.targetUrls.some(t => url.includes(t));

    if (!isTarget) {
      return route.continue();
    }

    // deterministic delay
    await sleep(chaos.networkDelayMs);

    // deterministic failure using modulo (not random)
    const hash = hashCode(url);
    if ((hash % 100) < chaos.failureRate * 100) {
      return route.abort();
    }

    return route.continue();
  });

  // --- CPU THROTTLING ---
  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", {
    rate: chaos.cpuSlowdownRate,
  });
}

// ===== REMOVE CHAOS =====
async function removeChaos(context, page) {

  await context.unroute("**/*");

  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", {
    rate: 1,
  });
}

// ===== FLUSH COLLECTOR =====
async function flush(page) {
  await page.evaluate(() => {
    if (window.__FAILSAFE_FLUSH__) {
      window.__FAILSAFE_FLUSH__();
    }
  });
}

// ===== UTILS =====

function wait(ms) {
  return new Promise(res => setTimeout(res, ms));
}

function sleep(ms) {
  return new Promise(res => setTimeout(res, ms));
}

// deterministic hash (for repeatable failures)
function hashCode(str) {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash) + str.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

// ===== START =====
run();