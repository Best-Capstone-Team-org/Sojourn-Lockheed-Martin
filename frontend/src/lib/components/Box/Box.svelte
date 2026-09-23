<script lang="ts">
	import type { Snippet } from "svelte";
	import "./box.css";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<div class={["standard-dialog mx-auto", className]}>
	{@render children()}
</div>
