// ===== FAILSAFE TRANSPORT LAYER =====

// CONFIG (can be overridden)
const DEFAULT_CONFIG = {
  endpoint: "http://localhost:8080/failsafe/frontend-metrics",
  batchSize: 10,
  flushInterval: 5000,
  retryLimit: 3,
  retryDelay: 1000,
};

// INTERNAL STATE
let config = { ...DEFAULT_CONFIG };

let queue = [];
let isFlushing = false;

// ===== INIT =====
export function initFailsafeTransport(customConfig = {}) {
  config = { ...config, ...customConfig };

  startAutoFlush();

  console.log("FailSafe Transport Initialized");
}

// ===== ENQUEUE =====
export function enqueueMetric(payload) {
  queue.push({
    payload,
    retries: 0,
  });

  if (queue.length >= config.batchSize) {
    flush();
  }
}

// ===== AUTO FLUSH =====
function startAutoFlush() {
  setInterval(() => {
    flush();
  }, config.flushInterval);
}

// ===== FLUSH =====
async function flush() {

  if (isFlushing || queue.length === 0) return;

  isFlushing = true;

  const batch = queue.splice(0, config.batchSize);

  try {
    await sendBatch(batch.map(item => item.payload));
  } catch (err) {
    handleFailure(batch);
  }

  isFlushing = false;
}

// ===== SEND =====
async function sendBatch(batch) {

  const body = JSON.stringify({
    metrics: batch,
  });

  // try sendBeacon first (non-blocking, survives unload)
  if (navigator.sendBeacon) {
    const blob = new Blob([body], { type: "application/json" });
    const success = navigator.sendBeacon(config.endpoint, blob);

    if (success) return;
  }

  // fallback to fetch
  const res = await fetch(config.endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body,
    keepalive: true,
  });

  if (!res.ok) {
    throw new Error("Failed to send metrics");
  }
}

// ===== FAILURE HANDLING =====
function handleFailure(batch) {

  for (const item of batch) {

    if (item.retries < config.retryLimit) {
      item.retries++;

      // exponential backoff
      setTimeout(() => {
        queue.push(item);
      }, config.retryDelay * item.retries);

    }
    // else drop silently
  }
}

// ===== FORCE FLUSH =====
export function forceFlush() {
  return flush();
}

// ===== VISIBILITY / UNLOAD HANDLING =====

// flush when tab hidden
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    flush();
  }
});

// flush before unload
window.addEventListener("beforeunload", () => {
  flush();
});