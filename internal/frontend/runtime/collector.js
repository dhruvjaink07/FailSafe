// ===== FAILSAFE COLLECTOR (FINAL - INTEGRATED) =====

import { enqueueMetric, initFailsafeTransport } from "./sender.js";

// ===== CONFIG =====
const BATCH_INTERVAL = 5000;

// ===== INIT TRANSPORT =====
initFailsafeTransport();

// ===== CONTEXT =====
function getExperimentId() {
  return window.__FAILSAFE_EXPERIMENT_ID__ || "local-dev";
}

function getPhase() {
  return window.__FAILSAFE_PHASE__ || "baseline";
}

// ===== STATE =====
let state = resetState();

// ===== RESET STATE PER PAGE =====
function resetState() {
  return {
    page: location.pathname,
    lcp: 0,
    cls: 0,
    inp: 0,

    longTasks: 0,
    errors: 0,
    unhandledRejections: 0,

    apiCalls: [],

    clsSessionValue: 0,
    clsSessionEntries: [],
    clsLastTimestamp: 0,

    lcpFinal: false,
  };
}

// ===== PERFORMANCE OBSERVER =====
const observer = new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {

    // LCP
    if (entry.entryType === "largest-contentful-paint") {
      if (!state.lcpFinal) {
        state.lcp = entry.startTime;
      }
    }

    // CLS (session window)
    if (entry.entryType === "layout-shift" && !entry.hadRecentInput) {
      const now = entry.startTime;

      if (
        now - state.clsLastTimestamp > 1000 ||
        state.clsSessionValue > 5
      ) {
        state.clsSessionValue = 0;
        state.clsSessionEntries = [];
      }

      state.clsSessionValue += entry.value;
      state.clsSessionEntries.push(entry);
      state.clsLastTimestamp = now;

      state.cls = Math.max(state.cls, state.clsSessionValue);
    }

    // Long task
    if (entry.entryType === "longtask") {
      state.longTasks += 1;
    }
  }
});

observer.observe({
  entryTypes: ["largest-contentful-paint", "layout-shift", "longtask"],
});

// ===== FINALIZE LCP =====
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    state.lcpFinal = true;
  }
});

// ===== INP =====
try {
  const inpObserver = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      if (entry.duration > state.inp) {
        state.inp = entry.duration;
      }
    }
  });

  inpObserver.observe({
    type: "event",
    buffered: true,
    durationThreshold: 40,
  });
} catch {}

// ===== ERROR TRACKING =====
window.addEventListener("error", () => {
  state.errors += 1;
});

window.addEventListener("unhandledrejection", () => {
  state.unhandledRejections += 1;
});

// ===== FETCH INTERCEPTOR =====
const originalFetch = window.fetch;

window.fetch = async function (...args) {
  const start = performance.now();

  try {
    const res = await originalFetch(...args);

    const duration = performance.now() - start;

    state.apiCalls.push({
      url: args[0],
      duration,
      status: res.status,
    });

    return res;

  } catch (err) {
    const duration = performance.now() - start;

    state.apiCalls.push({
      url: args[0],
      duration,
      status: 0,
    });

    throw err;
  }
};

// ===== SPA NAVIGATION =====
const originalPushState = history.pushState;
history.pushState = function (...args) {
  originalPushState.apply(this, args);
  onRouteChange();
};

const originalReplaceState = history.replaceState;
history.replaceState = function (...args) {
  originalReplaceState.apply(this, args);
  onRouteChange();
};

window.addEventListener("popstate", onRouteChange);

function onRouteChange() {
  flushMetrics();
  state = resetState();
}

// ===== PAYLOAD =====
function buildPayload() {
  return {
    experiment_id: getExperimentId(),
    phase: getPhase(),
    page: state.page,

    metrics: {
      lcp: Math.round(state.lcp),
      cls: Number(state.cls.toFixed(4)),
      inp: Math.round(state.inp),

      long_tasks: state.longTasks,
      errors: state.errors,
      unhandled_rejections: state.unhandledRejections,
    },

    api_calls: state.apiCalls,
    timestamp: Date.now(),
  };
}

// ===== FLUSH =====
function flushMetrics() {
  const payload = buildPayload();
  enqueueMetric(payload);
}

// expose for runner
window.__FAILSAFE_FLUSH__ = function () {
  flushMetrics();
};

// ===== PERIODIC =====
setInterval(() => {
  flushMetrics();

  // reset volatile metrics only
  state.longTasks = 0;
  state.apiCalls = [];
  state.errors = 0;
  state.unhandledRejections = 0;

}, BATCH_INTERVAL);

// ===== BOOT =====
console.log("FailSafe Collector (final) running");