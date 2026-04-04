"use strict";

(function initFailSafeSender(globalObj) {
  async function postViaNodeBridge(metrics) {
    if (typeof globalObj.__FAILSAFE_NODE_POST_METRICS__ !== "function") {
      return false;
    }
    await globalObj.__FAILSAFE_NODE_POST_METRICS__(metrics);
    return true;
  }

  function createSender(options) {
    var cfg = Object.assign(
      {
        endpoint: "http://localhost:8000/frontend/metrics",
        batchSize: 10,
        flushIntervalMs: 5000,
        retryLimit: 3,
      },
      options || {}
    );

    var state = {
      queue: [],
      flushing: false,
    };

    function enqueue(payload) {
      state.queue.push({ payload: payload, retries: 0 });
      if (state.queue.length >= cfg.batchSize) {
        void flush();
      }
    }

    async function flush() {
      if (state.flushing || state.queue.length === 0) {
        return;
      }

      state.flushing = true;
      var batch = state.queue.splice(0, cfg.batchSize);

      try {
        await postBatch(batch.map(function (item) { return item.payload; }));
      } catch (_) {
        requeue(batch);
      } finally {
        state.flushing = false;
      }
    }

    async function postBatch(metrics) {
      var sentByBridge = await postViaNodeBridge(metrics);
      if (sentByBridge) {
        return;
      }

      var response = await fetch(cfg.endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ metrics: metrics }),
        keepalive: true,
      });

      if (!response.ok) {
        throw new Error("frontend metrics post failed");
      }
    }

    function requeue(batch) {
      batch.forEach(function (item) {
        if (item.retries >= cfg.retryLimit) {
          return;
        }

        item.retries += 1;
        setTimeout(function () {
          state.queue.push(item);
        }, 500 * item.retries);
      });
    }

    setInterval(function () {
      void flush();
    }, cfg.flushIntervalMs);

    return {
      enqueue: enqueue,
      flush: flush,
    };
  }

  globalObj.__FAILSAFE_CREATE_SENDER__ = createSender;
})(window);