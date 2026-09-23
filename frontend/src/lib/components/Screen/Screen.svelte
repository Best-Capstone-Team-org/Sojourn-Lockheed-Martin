<script lang="ts">
	import type { Snippet } from "svelte";
	import Box from "$lib/components/Box/Box.svelte";
	import "./screen.css";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<Box class={"screen " + className}>
	{@render children()}
</Box>
