import { test, expect, firstFrame, playFirstLevel } from "./fixtures";

test.beforeEach(async ({ page }) => {
	await playFirstLevel(page);
});

test("shows the probe's telemetry", async ({ page }) => {
	await expect(page.getByText("SAFE", { exact: true })).toBeVisible();
	await expect(page.getByText("5 s", { exact: true })).toBeVisible();
	await expect(page.getByText("21 °C", { exact: true })).toBeVisible();
	await expect(page.getByText("500 mg", { exact: true })).toBeVisible();
	await expect(
		page.getByRole("img", { name: "Satellite idle" })
	).toBeVisible();
});

test("the telemetry switch hides TLM lines", async ({ page, backend }) => {
	backend.send({ type: "commandResponse", response: "PONG" });
	await expect(page.getByText("PONG")).toBeVisible();

	await page.getByRole("switch").click();
	await expect(page.getByRole("switch")).toHaveAttribute(
		"aria-checked",
		"false"
	);
	await expect(page.getByText(firstFrame.TLM)).toBeHidden();
	await expect(page.getByText("PONG")).toBeVisible();
});
