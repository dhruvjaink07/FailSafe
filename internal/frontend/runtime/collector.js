"use strict";

(function initFailSafeCollector(globalObj) {
  if (globalObj.__FAILSAFE_COLLECTOR_INSTALLED__) {
    return;
  }
  globalObj.__FAILSAFE_COLLECTOR_INSTALLED__ = true;

  var senderFactory = globalObj.__FAILSAFE_CREATE_SENDER__;
  if (typeof senderFactory !== "function") {
    return;
  }

  var sender = senderFactory({
    endpoint: globalObj.__FAILSAFE_ENDPOINT__ || "http://localhost:8000/frontend/metrics",
  });

  function getExperimentId() {
    return globalObj.__FAILSAFE_EXPERIMENT_ID__ || "exp-live-1";
  }

  function getPhase() {
    return globalObj.__FAILSAFE_PHASE__ || "baseline";
  }

  function resetState() {
    return {
      page: location.pathname || "/",
      lcp: 0,
      cls: 0,
      inp: 0,
      longTasks: 0,
      errors: 0,
      unhandledRejections: 0,
      apiCalls: [],
      clsSessionValue: 0,
      clsLastTimestamp: 0,
      lcpFinal: false,
    };
  }

  var state = resetState();

  if (typeof globalObj.__FAILSAFE_BOOTSTRAP_RUNTIME__ === "function") {
    globalObj.__FAILSAFE_BOOTSTRAP_RUNTIME__(state);
  }

  var perfObserver = new PerformanceObserver(function (list) {
    list.getEntries().forEach(function (entry) {
      if (entry.entryType === "largest-contentful-paint" && !state.lcpFinal) {
        state.lcp = entry.startTime;
      }

      if (entry.entryType === "layout-shift" && !entry.hadRecentInput) {
        var now = entry.startTime;
        if (now - state.clsLastTimestamp > 1000 || state.clsSessionValue > 5) {
          state.clsSessionValue = 0;
        }
        state.clsSessionValue += entry.value;
        state.clsLastTimestamp = now;
        state.cls = Math.max(state.cls, state.clsSessionValue);
      }

      if (entry.entryType === "longtask") {
        state.longTasks += 1;
      }
    });
  });

  perfObserver.observe({
    entryTypes: ["largest-contentful-paint", "layout-shift", "longtask"],
  });

  try {
    var inpObserver = new PerformanceObserver(function (list) {
      list.getEntries().forEach(function (entry) {
        if (entry.duration > state.inp) {
          state.inp = entry.duration;
        }
      });
    });

    inpObserver.observe({
      type: "event",
      buffered: true,
      durationThreshold: 40,
    });
  } catch (_) {
    // Unsupported on some pages.
  }

  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "hidden") {
      state.lcpFinal = true;
    }
  });

  globalObj.addEventListener("error", function () {
    state.errors += 1;
  });

  globalObj.addEventListener("unhandledrejection", function () {
    state.unhandledRejections += 1;
  });

  var originalFetch = globalObj.fetch;
  globalObj.fetch = async function failsafeFetch() {
    var args = Array.prototype.slice.call(arguments);
    var started = performance.now();
    try {
      var res = await originalFetch.apply(globalObj, args);
      state.apiCalls.push({
        url: String(args[0] || ""),
        duration: performance.now() - started,
        status: res.status,
      });
      return res;
    } catch (err) {
      state.apiCalls.push({
        url: String(args[0] || ""),
        duration: performance.now() - started,
        status: 0,
      });
      throw err;
    }
  };

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

  function flush() {
    sender.enqueue(buildPayload());
    state.longTasks = 0;
    state.errors = 0;
    state.unhandledRejections = 0;
    state.apiCalls = [];
  }

  var originalPushState = history.pushState;
  history.pushState = function failsafePushState() {
    var args = Array.prototype.slice.call(arguments);
    originalPushState.apply(history, args);
    flush();
    state = resetState();
  };

  var originalReplaceState = history.replaceState;
  history.replaceState = function failsafeReplaceState() {
    var args = Array.prototype.slice.call(arguments);
    originalReplaceState.apply(history, args);
    flush();
    state = resetState();
  };

  globalObj.addEventListener("popstate", function () {
    flush();
    state = resetState();
  });

  globalObj.__FAILSAFE_FLUSH__ = function failsafeForceFlush() {
    flush();
    return sender.flush();
  };

  setInterval(function () {
    flush();
  }, 5000);
})(window);