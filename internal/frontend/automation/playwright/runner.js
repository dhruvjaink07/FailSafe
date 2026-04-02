"use strict";

const { chromium } = require("playwright");
const fs = require("fs");
const path = require("path");
const { buildDefaultScenario, normalizeTargetUrls } = require("./config");

const BASE_URL = process.env.BASE_URL || "https://example.com";
const EXPERIMENT_ID = process.env.EXPERIMENT_ID || "exp-live-1";
const FRONTEND_METRICS_ENDPOINT =
  process.env.FAILSAFE_FRONTEND_ENDPOINT || "http://localhost:8000/frontend/metrics";
const CONTROLLER_BASE_URL =
  process.env.FAILSAFE_CONTROLLER_URL || "http://localhost:8000";

const SCENARIO = buildDefaultScenario();

const senderScript = fs.readFileSync(path.join(__dirname, "../../transport/sender.js"), "utf-8");
const vitalsScript = fs.readFileSync(path.join(__dirname, "../../runtime/vitals.js"), "utf-8");
const errorsScript = fs.readFileSync(path.join(__dirname, "../../runtime/errors.js"), "utf-8");
const networkScript = fs.readFileSync(path.join(__dirname, "../../runtime/network.js"), "utf-8");
const bootstrapScript = fs.readFileSync(path.join(__dirname, "../../runtime/bootstrap.js"), "utf-8");
const collectorScript = fs.readFileSync(path.join(__dirname, "../../runtime/collector.js"), "utf-8");

function logAudit(event, details = {}) {
  const payload = {
    ts: new Date().toISOString(),
    exp: EXPERIMENT_ID,
    event,
    ...details,
  };
  console.log(`[failsafe-audit] ${JSON.stringify(payload)}`);
}

function firstNonEmpty() {
  for (const value of arguments) {
    if (typeof value === "string" && value.trim() !== "") {
      return value.trim();
    }
  }
  return "";
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
  const targetUrls = normalizeTargetUrls(frontendRun.target_urls || frontendRun.targetUrls);

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

async function fetchActiveFaultCommand(controllerBaseUrl, experimentId) {
  const url = new URL("/experiments/frontend/fault-command", controllerBaseUrl);
  url.searchParams.set("id", experimentId);

  try {
    const response = await fetch(url.toString());
    if (!response.ok) {
      logAudit("fault_command_poll_error", { status: response.status });
      return null;
    }

    const body = await response.json();
    if (!body || !body.active) {
      return null;
    }

    logAudit("fault_command_active", {
      type: body.command && body.command.type,
      intensity: body.command && body.command.intensity,
      remaining_ms: body.command && body.command.remaining_time_ms,
    });
    return body.command || null;
  } catch (_) {
    logAudit("fault_command_poll_exception");
    return null;
  }
}

function commandToChaos(command, fallbackChaos) {
  const intensity = Number(command.intensity || 0);
  const targetUrls = Array.isArray(command.targets) && command.targets.length > 0
    ? command.targets
    : fallbackChaos.targetUrls;

  const normalized = {
    ...fallbackChaos,
    targetUrls,
  };

  switch (String(command.type || "")) {
    case "offline":
      return {
        ...normalized,
        forceOffline: true,
      };
    case "request_abort":
    case "packet_loss":
    case "network_packet_loss":
      return {
        ...normalized,
        failureRate: Math.min(1, Math.max(0.1, intensity / 100)),
      };
    case "cpu_throttle":
      return {
        ...normalized,
        cpuSlowdownRate: Math.max(2, Math.round(intensity / 20) + 1),
      };
    case "network_delay":
    case "network_latency":
      return {
        ...normalized,
        networkDelayMs: Math.max(150, intensity * 20),
      };
    default:
      return normalized;
  }
}

async function applyChaos(context, page, chaos) {
  logAudit("chaos_apply", {
    offline: Boolean(chaos.forceOffline),
    delay_ms: chaos.networkDelayMs,
    failure_rate: chaos.failureRate,
    cpu_rate: chaos.cpuSlowdownRate,
  });

  // Ensure no existing routes before adding new ones
  try {
    await context.unroute("**/*");
  } catch (_) {
    // Route might not exist, that's ok
  }

  if (chaos.forceOffline) {
    await context.setOffline(true);
  }

  await context.route("**/*", async (route) => {
    try {
      const requestUrl = route.request().url();
      const isTarget = chaos.targetUrls.some((token) => requestUrl.includes(token));

      if (!isTarget) {
        await route.continue();
        return;
      }

      await sleep(chaos.networkDelayMs);

      if ((hashCode(requestUrl) % 100) < chaos.failureRate * 100) {
        await route.abort();
        return;
      }

      await route.continue();
    } catch (err) {
      // Route already handled or page closed
      logAudit("route_handler_error", { message: err.message });
    }
  });

  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", {
    rate: chaos.cpuSlowdownRate,
  });
}

async function removeChaos(context, page) {
  await context.unroute("**/*");
  await context.setOffline(false);
  const client = await context.newCDPSession(page);
  await client.send("Emulation.setCPUThrottlingRate", { rate: 1 });
  logAudit("chaos_remove");
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
  await page.addInitScript({ content: vitalsScript });
  await page.addInitScript({ content: errorsScript });
  await page.addInitScript({ content: networkScript });
  await page.addInitScript({ content: bootstrapScript });
  await page.addInitScript({ content: collectorScript });

  for (const pageConfig of runtimeConfig.scenario.pages) {
    const url = new URL(pageConfig.path, runtimeConfig.baseUrl).toString();
    console.log(`Running ${url}`);
    logAudit("page_start", { url });

    await page.goto(url, { waitUntil: "domcontentloaded" });

    await setPhase(page, "baseline");
    await runActions(page, pageConfig.actions);
    await sleep(runtimeConfig.scenario.phases.baselineMs);

    await setPhase(page, "injecting");
    await runActions(page, pageConfig.actions);

    if (runtimeConfig.scenario.chaos.enabled) {
      let chaosApplied = false;
      const injectingUntil = Date.now() + runtimeConfig.scenario.phases.injectingMs;

      while (Date.now() < injectingUntil) {
        if (!chaosApplied) {
          const command = await fetchActiveFaultCommand(CONTROLLER_BASE_URL, EXPERIMENT_ID);
          if (command) {
            const commandedChaos = commandToChaos(command, runtimeConfig.scenario.chaos);
            await applyChaos(context, page, commandedChaos);
            chaosApplied = true;
            logAudit("chaos_source", { source: "command", type: command.type });
          } else {
            await applyChaos(context, page, runtimeConfig.scenario.chaos);
            chaosApplied = true;
            logAudit("chaos_source", { source: "fallback" });
          }
        }

        const remaining = injectingUntil - Date.now();
        if (remaining <= 0) {
          break;
        }
        await sleep(Math.min(2000, remaining));
      }
    } else {
      await sleep(runtimeConfig.scenario.phases.injectingMs);
    }

    await removeChaos(context, page);
    await setPhase(page, "recovery");
    await runActions(page, pageConfig.actions);
    await sleep(runtimeConfig.scenario.phases.recoveryMs);

    await page.evaluate(() => {
      if (window.__FAILSAFE_FLUSH__) {
        window.__FAILSAFE_FLUSH__();
      }
    });
    logAudit("page_complete", { url });
  }

  await page.evaluate(async () => {
    if (window.__FAILSAFE_FLUSH__) {
      await window.__FAILSAFE_FLUSH__();
    }
  });

  await browser.close();
}

module.exports = { run };

if (require.main === module) {
  run().catch((err) => {
    console.error(err);
    process.exitCode = 1;
  });
}