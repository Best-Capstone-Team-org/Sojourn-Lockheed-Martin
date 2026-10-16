<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
		bezelClass = "",
	}: { children: Snippet; class?: string; bezelClass?: string } = $props();
</script>

<!-- Bezel -->
<div
	class={[
		"relative grid overflow-hidden border-3",
		"border-t-sys-shadow-top border-r-sys-shadow-right border-b-sys-shadow-bottom border-l-sys-shadow-left",
		bezelClass,
	]}
>
	<div
		class={[
			"relative col-start-1 row-start-1 h-full min-h-0 w-full min-w-0 overflow-hidden text-sys-text-light",

			// vignette (top layer) + CRT scanlines
			"bg-[radial-gradient(transparent_45%,rgb(0_0_0/0.15)_90%,rgb(0_0_0/0.65)_110%),repeating-linear-gradient(var(--color-sys-screen-light-green)_0_2px,var(--color-sys-screen-dark-green)_2px_4px)]",

			className,
		]}
	>
		{@render children()}

		<!-- Edge shadow overlay (paints above children) -->
		<div
			class="pointer-events-none absolute inset-0 shadow-crt"
			aria-hidden="true"
		></div>
	</div>
</div>
