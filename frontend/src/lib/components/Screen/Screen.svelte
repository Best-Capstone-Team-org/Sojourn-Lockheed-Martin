<script lang="ts">
	import type { Snippet } from "svelte";
	import Box from "$lib/components/Box/Box.svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<Box
	class={`bg-[repeating-linear-gradient(to_bottom,var(--color-sys-screen-light-green)_0px,var(--color-sys-screen-light-green)_2px,var(--color-sys-screen-dark-green)_2px,var(--color-sys-screen-dark-green)_4px)] ${className}`}
>
	{@render children()}
</Box>
