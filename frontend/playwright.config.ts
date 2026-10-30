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
		// Records every test. The HTML report shows each test's video.
		video: { mode: "on", size: { width: 1280, height: 720 } },
		// The end-of-test screenshot also makes the browser draw one more
		// frame. Without it, the videos of the two quickest tests stay white.
		screenshot: "on",
		// Each test takes about a second, too fast to follow in its video.
		// SLOW_MO=500 npm run test:e2e waits 500 ms after every action.
		launchOptions: { slowMo: Number(process.env.SLOW_MO ?? 0) },
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
