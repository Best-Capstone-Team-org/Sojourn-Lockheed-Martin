import { test, expect } from "./fixtures";

test("Start opens level select", async ({ page }) => {
	// Start is in the prerendered HTML. Wait until the app's scripts have
	// loaded, or the click lands before the button has a click handler.
	await page.goto("/", { waitUntil: "networkidle" });
	await page.getByRole("button", { name: "Start" }).click();
	await expect(page).toHaveURL(/\/level-select\/?$/);
	await expect(
		page.getByRole("heading", { name: "Level Select" })
	).toBeVisible();
});
