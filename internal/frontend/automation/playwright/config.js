"use strict";

function normalizeTargetUrls(value) {
	if (!Array.isArray(value)) {
		return [""];
	}
	const urls = value
		.map((item) => (typeof item === "string" ? item.trim() : ""))
		.filter((item) => item.length > 0 || item === "");
	return urls.length > 0 ? urls : [""];
}

function buildDefaultScenario() {
	return {
		pages: [{ path: "/", actions: ["wait:1500"] }],
		phases: {
			baselineMs: 5000,
			injectingMs: 10000,
			recoveryMs: 5000,
		},
		chaos: {
			enabled: true,
			networkDelayMs: 700,
			failureRate: 0.15,
			cpuSlowdownRate: 4,
			targetUrls: normalizeTargetUrls([""]),
		},
	};
}

module.exports = {
	buildDefaultScenario,
	normalizeTargetUrls,
};

