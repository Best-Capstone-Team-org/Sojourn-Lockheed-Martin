import fs from "node:fs";
import { MCR, type CoverageReportOptions } from "monocart-coverage-reports";

// No "lines": markup lines in .svelte files and type-only lines in .ts files
// count as covered even when nothing ran (a component that never rendered
// still showed 56% of its lines covered), so statements are the honest total.
type Metric = "statements" | "branches" | "functions";

const outputDir = "coverage";
const metrics: Metric[] = ["statements", "branches", "functions"];

export const coverageOptions: CoverageReportOptions = {
	name: "Frontend coverage",
	outputDir,
	// Only our own code. The source maps also list Svelte and SvelteKit files
	// from node_modules and image imports, and SvelteKit's small start chunk
	// has no source map, so it would show up as a bundle file.
	outputFilter: (file) => /^src\/.*\.(svelte|ts|js)$/.test(file.sourcePath),
	reports: [
		["v8"], // index.html
		["console-details", { metrics }],
		["markdown-details", { metrics, maxCols: 100 }], // table in the PR comment
		["v8-json"], // coverage-report.json, headline of the PR comment
	],
};

// Playwright global setup. Playwright calls the returned function after all
// tests, once every worker has added its coverage.
export default function globalSetup() {
	fs.rmSync(outputDir, { recursive: true, force: true });
	return async () => {
		const results = await MCR(coverageOptions).generate();
		if (!results?.files.length) {
			throw new Error(
				"No frontend coverage was collected. Specs must import test from ./fixtures."
			);
		}
	};
}
