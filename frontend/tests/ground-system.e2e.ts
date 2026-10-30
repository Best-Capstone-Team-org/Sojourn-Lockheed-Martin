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

test("Enter sends a command and adds it to the history", async ({
	page,
	backend,
}) => {
	const input = page.getByPlaceholder("Enter Command");
	await input.fill("PING");
	await input.press("Enter");

	await expect.poll(() => backend.commands).toEqual(["PING"]);
	await expect(input).toHaveValue("");
	await expect(page.getByText("PING", { exact: true })).toBeVisible();

	backend.send({ type: "commandResponse", response: "PONG" });
	await expect(page.getByText("PONG")).toBeVisible();
	await expect(
		page.getByRole("img", { name: "Satellite transmitting downlink" })
	).toBeVisible();
});

test("arrow keys bring back earlier commands", async ({ page }) => {
	const input = page.getByPlaceholder("Enter Command");
	for (const command of ["PING", "PEEK 0x2001E000 16"]) {
		await input.fill(command);
		await input.press("Enter");
	}

	await input.press("ArrowUp");
	await expect(input).toHaveValue("PEEK 0x2001E000 16");
	await input.press("ArrowUp");
	await expect(input).toHaveValue("PING");
	await input.press("ArrowDown");
	await expect(input).toHaveValue("PEEK 0x2001E000 16");
	await input.press("ArrowDown");
	await expect(input).toHaveValue("");
});

test("Ctrl+R finds an earlier command and Enter sends it again", async ({
	page,
	backend,
}) => {
	const input = page.getByPlaceholder("Enter Command");
	for (const command of ["PING", "PEEK 0x2001E000 16"]) {
		await input.fill(command);
		await input.press("Enter");
	}

	await input.press("Control+r");
	await input.fill("PI");
	await expect(page.getByText("PEEK 0x2001E000 16")).toBeHidden();
	await input.press("Enter");

	await expect
		.poll(() => backend.commands)
		.toEqual(["PING", "PEEK 0x2001E000 16", "PING"]);
});
