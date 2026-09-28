<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();
</script>

<div
	class={[
		"relative isolate mx-auto border-2 border-solid border-sys-green bg-sys-grey shadow-[2px_2px_0px_0px] shadow-sys-green",
		"before:pointer-events-none before:absolute before:inset-0 before:-z-10 before:bg-size-[182px] before:bg-repeat before:opacity-40 before:content-['']",
		"before:bg-[url(data:image/svg+xml,%3Csvg%20xmlns=%27http://www.w3.org/2000/svg%27%20viewBox=%270%200%20600%20600%27%3E%3Cfilter%20id=%27a%27%3E%3CfeTurbulence%20type=%27fractalNoise%27%20baseFrequency=%27.65%27%20numOctaves=%273%27%20stitchTiles=%27stitch%27/%3E%3C/filter%3E%3Crect%20width=%27100%25%27%20height=%27100%25%27%20filter=%27url%28%23a%29%27/%3E%3C/svg%3E)]",
		className,
	]}
>
	{@render children()}
</div>
