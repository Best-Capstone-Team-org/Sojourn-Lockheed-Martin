import { test, expect, playFirstLevel } from "./fixtures";

test("lists the backend's levels sorted by name", async ({ page }) => {
	await page.goto("/level-select/");
	// One probe picture per card, named after the level's difficulty:
	// First Contact = tutorial, Ion Storm = intermediate, Quiet Orbit = advanced.
	const probes = page.getByRole("img", { name: /probe$/ });
	await expect(probes).toHaveCount(3);
	await expect(probes.nth(0)).toHaveAttribute("alt", "tutorial probe");
	await expect(probes.nth(1)).toHaveAttribute("alt", "intermediate probe");
	await expect(probes.nth(2)).toHaveAttribute("alt", "advanced probe");
});

test("shows no levels when the backend fails", async ({ page }) => {
	await page.route("http://localhost:8080/api/scenarios", (route) =>
		route.fulfill({ status: 500 })
	);
	const error = page.waitForEvent("console", (message) =>
		message.text().includes("Failed to load scenarios")
	);
	await page.goto("/level-select/");
	await error;
	await expect(page.getByRole("img", { name: /probe$/ })).toHaveCount(0);
});

test("Play opens the ground system for the first level", async ({
	page,
	backend,
}) => {
	await playFirstLevel(page);
	expect(backend.sockets[0].url()).toContain("scenario-id=first-contact");
	await expect(page.getByText("Read the downlink")).toBeVisible();
});

test("the side panel shows the selected level and Quit goes home", async ({
	page,
}) => {
	await page.goto("/level-select/");
	await expect(
		page.getByText("Read the downlink - Send PING and read the reply.")
	).toBeVisible();
	await expect(page.getByText("Writes - 5")).toBeVisible();
	await expect(page.getByText("Reads - 20")).toBeVisible();

	await page.getByRole("button", { name: "Quit" }).click();
	await expect(page).toHaveURL(/:4173\/$/);
});
