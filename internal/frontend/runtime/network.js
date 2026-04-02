"use strict";

(function initFailSafeNetwork(globalObj) {
	function installNetworkTracker(state) {
		if (!state || typeof globalObj.fetch !== "function") {
			return;
		}

		var originalFetch = globalObj.fetch;
		globalObj.fetch = async function failsafeTrackedFetch() {
			var args = Array.prototype.slice.call(arguments);
			var started = performance.now();
			try {
				var response = await originalFetch.apply(globalObj, args);
				state.apiCalls.push({
					url: String(args[0] || ""),
					duration: performance.now() - started,
					status: response.status,
				});
				return response;
			} catch (err) {
				state.apiCalls.push({
					url: String(args[0] || ""),
					duration: performance.now() - started,
					status: 0,
				});
				throw err;
			}
		};
	}

	globalObj.__FAILSAFE_INSTALL_NETWORK__ = installNetworkTracker;
})(window);

