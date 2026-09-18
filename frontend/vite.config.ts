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
	ssr: {
		// paneforge imports svelte-toolbelt `.svelte.js` modules; Vite must compile them
		// during SSR instead of loading the raw files from node_modules.
		noExternal: ["svelte-toolbelt", "runed"],
	},
});
