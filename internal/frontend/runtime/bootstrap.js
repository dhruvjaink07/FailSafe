"use strict";

(function initFailSafeBootstrap(globalObj) {
	function bootstrapRuntime(state) {
		if (!state) {
			return;
		}

		if (typeof globalObj.__FAILSAFE_INSTALL_VITALS__ === "function") {
			globalObj.__FAILSAFE_INSTALL_VITALS__(state);
		}
		if (typeof globalObj.__FAILSAFE_INSTALL_ERRORS__ === "function") {
			globalObj.__FAILSAFE_INSTALL_ERRORS__(state);
		}
		if (typeof globalObj.__FAILSAFE_INSTALL_NETWORK__ === "function") {
			globalObj.__FAILSAFE_INSTALL_NETWORK__(state);
		}
	}

	globalObj.__FAILSAFE_BOOTSTRAP_RUNTIME__ = bootstrapRuntime;
})(window);

