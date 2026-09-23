<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<div
	class={[
		"mx-auto border-2 border-solid border-sys-green bg-sys-grey shadow-[2px_2px_0px_0px] shadow-sys-green",
		className,
	]}
>
	{@render children()}
</div>
