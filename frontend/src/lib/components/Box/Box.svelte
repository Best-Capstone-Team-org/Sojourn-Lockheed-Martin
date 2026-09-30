<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<div
	class={[
		"noise-bg border border-sys-shadow-bottom bg-sys-grey text-sys-text-dark shadow-bevel",
		className,
	]}
>
	{@render children()}
</div>
