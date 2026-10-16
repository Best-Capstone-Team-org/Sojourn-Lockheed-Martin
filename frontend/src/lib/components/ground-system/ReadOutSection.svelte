<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import type { Snippet } from "svelte";

	let {
		title,
		children
	}: { 
		title: string;
		children: Snippet;
	} = $props();
</script>

<Box class="w-full p-2">
	<span class="font-semibold">{title}</span>
	<hr
		class="my-1 border-t border-black/35 shadow-[0_1px_0_rgba(255,255,255,0.4)]"
	/>
	{@render children()}
</Box>