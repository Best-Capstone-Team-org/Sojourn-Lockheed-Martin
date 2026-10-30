import { test as base, expect } from "@playwright/test";
import { MCR } from "monocart-coverage-reports";
import { coverageOptions } from "./coverage";

export const test = base.extend<{ coverage: void }>({
	// V8 coverage works in Chromium only. MCR keeps each test's data in
	// coverage/.cache until the global teardown merges it.
	coverage: [
		async ({ page }, use) => {
			await page.coverage.startJSCoverage({ resetOnNavigation: false });
			try {
				await use();
			} finally {
				const coverage = await page.coverage.stopJSCoverage();
				await MCR(coverageOptions).add(coverage);
			}
		},
		{ auto: true },
	],
});

export { expect };
