import {
	test as base,
	expect,
	type Page,
	type WebSocketRoute,
} from "@playwright/test";
import { MCR } from "monocart-coverage-reports";
import type { CommandDownlink, ScenarioResponse } from "../src/lib/api";
import { coverageOptions } from "./coverage";

// Out of order on purpose: level select sorts by name, so First Contact
// comes first on screen.
export const scenarios: ScenarioResponse[] = [
	{
		id: "quiet-orbit",
		name: "Quiet Orbit",
		revision: 1,
		author: "tests",
		description: "Nothing to fix yet.",
		difficulty: "advanced",
		objectives: [],
		constraints: {},
	},
	{
		id: "first-contact",
		name: "First Contact",
		revision: 1,
		author: "tests",
		description: "Wake the probe and read its downlink.",
		difficulty: "tutorial",
		objectives: [
			{
				id: "read-downlink",
				name: "Read the downlink",
				description: "Send PING and read the reply.",
				status: "active",
			},
			{
				id: "fix-sensor",
				name: "Fix the sensor",
				description: "Patch the sensor table.",
				status: "locked",
			},
		],
		constraints: { writes: 5, reads: 20 },
	},
	{
		id: "ion-storm",
		name: "Ion Storm",
		revision: 1,
		author: "tests",
		description: "Ride out the storm.",
		difficulty: "intermediate",
		objectives: [
			{
				id: "shed-load",
				name: "Shed load",
				description: "Turn the heater off.",
				status: "active",
			},
		],
		constraints: { writes: 2 },
	},
];

// The fake backend sends this frame as soon as a socket opens.
export const firstFrame = {
	type: "telemetry",
	TLM: "TLM 0A1B2C3D",
	telemetry: {
		crcOK: true,
		frame: 1,
		uptime: 5,
		mode: "SAFE",
		reboots: 0,
		lastFault: "NONE",
		bus: 28,
		load: 1200,
		sensors: { 0: 12, 1: 0.5, 2: 21, 3: 28000, 4: 3, 5: 0.7 },
		hk: { heaterOn: 1, shed: 0, prop: 500, mom: 3, rec: 40, auth: 0 },
		unknown: {},
	},
} satisfies CommandDownlink;

export type Backend = {
	sockets: WebSocketRoute[];
	commands: string[];
	closes: (number | undefined)[];
	send: (message: CommandDownlink) => void;
};

export const test = base.extend<{ coverage: void; backend: Backend }>({
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
	// Answers both backend calls in src/lib/api.ts inside the browser, so the
	// tests need no Go server, QEMU or Docker.
	backend: [
		async ({ page }, use) => {
			const backend: Backend = {
				sockets: [],
				commands: [],
				closes: [],
				send(message) {
					const socket = backend.sockets.at(-1);
					if (!socket) throw new Error("The page has no socket yet");
					socket.send(JSON.stringify(message));
				},
			};
			await page.route("http://localhost:8080/api/scenarios", (route) =>
				route.fulfill({ json: scenarios })
			);
			await page.routeWebSocket(/\/api\/select-scenario/, (ws) => {
				backend.sockets.push(ws);
				ws.onMessage((message) =>
					backend.commands.push(message.toString())
				);
				ws.onClose((code) => backend.closes.push(code));
				ws.send(JSON.stringify(firstFrame));
			});
			await use(backend);
		},
		{ auto: true },
	],
});

export { expect };

// Plays First Contact and waits for its first telemetry frame, so the socket
// is open before a test types or sends anything.
export async function playFirstLevel(page: Page) {
	await page.goto("/level-select/");
	// Play can be in the prerendered HTML, where it does nothing until the
	// app has loaded the levels, so wait for a fetched level first.
	await expect(
		page.getByRole("img", { name: "tutorial probe" })
	).toBeVisible();
	await page.getByRole("button", { name: "Play" }).first().click();
	await expect(page).toHaveURL(/\/ground-system\/?$/);
	await expect(page.getByText(firstFrame.TLM)).toBeVisible();
}
