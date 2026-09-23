<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<div class={["border-2 border-sys-green border-solid mx-auto shadow-[2px_2px_0px_0px] shadow-sys-green bg-sys-grey", className]}>
	{@render children()}
</div>
