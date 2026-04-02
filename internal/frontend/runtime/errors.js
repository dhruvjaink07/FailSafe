"use strict";

(function initFailSafeErrors(globalObj) {
	function installErrorTracker(state) {
		if (!state) {
			return;
		}

		globalObj.addEventListener("error", function () {
			state.errors = (state.errors || 0) + 1;
		});

		globalObj.addEventListener("unhandledrejection", function () {
			state.unhandledRejections = (state.unhandledRejections || 0) + 1;
		});
	}

	globalObj.__FAILSAFE_INSTALL_ERRORS__ = installErrorTracker;
})(window);

