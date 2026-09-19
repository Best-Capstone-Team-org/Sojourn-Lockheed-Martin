/** @type {import("prettier").Config} */
const config = {
	arrowParens: "always",
	endOfLine: "lf",
	semi: true,
	singleAttributePerLine: true,
	singleQuote: false,
	tabWidth: 4,
	trailingComma: "es5",
	printWidth: 80,
	useTabs: true,
	plugins: ["prettier-plugin-svelte", "prettier-plugin-tailwindcss"],
	overrides: [{ files: "*.svelte", options: { parser: "svelte" } }],
	tailwindStylesheet: "./src/routes/layout.css",
};

export default config;
