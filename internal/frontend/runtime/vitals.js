"use strict";

(function initFailSafeVitals(globalObj) {
	function installVitalsTracker(state) {
		if (!state || typeof PerformanceObserver !== "function") {
			return;
		}

		try {
			// Track LCP (Largest Contentful Paint) and CLS (Cumulative Layout Shift)
			var lcpObserver = new PerformanceObserver(function (list) {
				var entries = list.getEntries();
				if (entries.length > 0) {
					var lcp = entries[entries.length - 1].renderTime || entries[entries.length - 1].loadTime || 0;
					state.lcp = Math.max(state.lcp || 0, lcp);
				}
			});
			lcpObserver.observe({ type: "largest-contentful-paint", buffered: true });

			var clsObserver = new PerformanceObserver(function (list) {
				list.getEntries().forEach(function (entry) {
					if (!entry.hadRecentInput) {
						var now = entry.startTime || 0;
						if (!state.clsLastTimestamp || now - state.clsLastTimestamp > 1000) {
							state.clsSessionValue = 0;
						}
						state.clsSessionValue = (state.clsSessionValue || 0) + (entry.value || 0);
						state.clsLastTimestamp = now;
						state.cls = Math.max(state.cls || 0, state.clsSessionValue);
					}
				});
			});
			clsObserver.observe({ type: "layout-shift", buffered: true });

			// Track INP (Interaction to Next Paint)
			if (globalObj.PerformanceObserver && globalObj.PerformanceObserver.supportedEntryTypes && globalObj.PerformanceObserver.supportedEntryTypes.includes("interaction")) {
				var inpObserver = new PerformanceObserver(function (list) {
					var entries = list.getEntries();
					var maxDuration = 0;
					for (var i = 0; i < entries.length; i++) {
						maxDuration = Math.max(maxDuration, entries[i].duration || 0);
					}
					state.inp = Math.max(state.inp || 0, maxDuration);
				});
				inpObserver.observe({ type: "interaction", buffered: true });
			}
		} catch (_) {
			// Optional tracker; browser may not support PerformanceObserver.
		}
	}

	globalObj.__FAILSAFE_INSTALL_VITALS__ = installVitalsTracker;
})(window);

