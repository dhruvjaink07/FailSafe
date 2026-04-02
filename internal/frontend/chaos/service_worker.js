'use strict';

/**
 * Service Worker for offline fault injection.
 * Installed in browser context to intercept network requests and simulate offline.
 *
 * This script is injected into the page context and installed as a service worker.
 * It runs in a separate worker context and can intercept all fetch requests.
 */

if (typeof self !== 'undefined' && typeof self.addEventListener === 'function') {
  // Service Worker context
  self.addEventListener('install', function (event) {
    self.skipWaiting();
  });

  self.addEventListener('activate', function (event) {
    event.waitUntil(self.clients.claim());
  });

  self.addEventListener('fetch', function (event) {
    // Check if offline mode is enabled via client
    event.respondWith(
      (async function () {
        // Check global config set by client
        var offlineMode = false;
        try {
          var clients = await self.clients.matchAll({ type: 'window' });
          if (clients.length > 0 && clients[0].__FAILSAFE_OFFLINE__) {
            offlineMode = clients[0].__FAILSAFE_OFFLINE__;
          }
        } catch (err) {
          // Fallback: check IndexedDB or localStorage
          try {
            var db = await openIndexedDB();
            offlineMode = (await dbGet(db, 'failsafeConfig', 'offline')) || false;
          } catch (e) {
            // Ignore
          }
        }

        if (offlineMode) {
          // Return offline response
          return new Response('Offline', { status: 503, statusText: 'Service Unavailable' });
        }

        // Pass through to network
        try {
          return await fetch(event.request);
        } catch (err) {
          // Network error; return fallback
          return new Response('Network Error', { status: 0, statusText: 'Network Failure' });
        }
      })()
    );
  });
}

/**
 * Helper to open IndexedDB.
 */
function openIndexedDB() {
  return new Promise(function (resolve, reject) {
    var req = indexedDB.open('FailSafeDB', 1);
    req.onsuccess = function () {
      resolve(req.result);
    };
    req.onerror = function () {
      reject(req.error);
    };
  });
}

/**
 * Helper to read from IndexedDB.
 */
function dbGet(db, storeName, key) {
  return new Promise(function (resolve, reject) {
    var tx = db.transaction(storeName, 'readonly');
    var store = tx.objectStore(storeName);
    var req = store.get(key);
    req.onsuccess = function () {
      resolve(req.result);
    };
    req.onerror = function () {
      reject(req.error);
    };
  });
}
