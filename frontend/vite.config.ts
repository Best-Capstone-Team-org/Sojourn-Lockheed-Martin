import tailwindcss from "@tailwindcss/vite";
import adapter from "@sveltejs/adapter-static";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes("node_modules")
						? undefined
						: true,
			},
			adapter: adapter(),
		}),
	],
	resolve: process.env.VITEST
		? {
				conditions: ["browser"],
			}
		: undefined,
	// Set by playwright.config.ts. Coverage needs source maps to map the
	// bundles back to src/, and unminified code to map them accurately.
	build: process.env.COVERAGE
		? { sourcemap: true, minify: false }
		: undefined,
});
