// For process.env: this file is outside the SvelteKit tsconfig, and
// TypeScript 6 no longer loads Node's types on its own.
/// <reference types="node" />
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
	testDir: "tests",
	testMatch: "**/*.e2e.{ts,js}",
	globalSetup: "./tests/coverage.ts",
	forbidOnly: !!process.env.CI,
	reporter: [["list"], ["html", { open: "never" }]],
	use: {
		baseURL: "http://localhost:4173",
		trace: "retain-on-failure",
	},
	// page.coverage only works in Chromium.
	projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
	webServer: {
		command: "npm run build && npm run preview",
		port: 4173,
		env: { COVERAGE: "true" },
		timeout: 120_000,
	},
});
