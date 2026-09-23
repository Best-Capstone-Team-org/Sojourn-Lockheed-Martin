<script lang="ts">
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";
	import Box from "$lib/components/Box/Box.svelte";
	import "./button.css";

	let {
		children,
		class: className = "",
		type = "button",
		...rest
	}: HTMLButtonAttributes & { children: Snippet } = $props();
</script>

<button
	{type}
	class="standard-button-reset"
	{...rest}
>
	<Box class={`standard-button ${className}`}>
		{@render children()}
	</Box>
</button>
