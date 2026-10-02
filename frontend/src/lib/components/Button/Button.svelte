<script lang="ts">
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	let {
		children,
		buttonClass = "",
		wellClass = "",
		type = "button",
		...rest
	}: HTMLButtonAttributes & { children: Snippet; buttonClass?: string; wellClass?: string } = $props();
</script>

<div
	class={[
		"relative rounded p-0.5 pt-px overflow-hidden bg-black",

		// overlay goes from transparent to shadow on button press
		"after:pointer-events-none after:absolute after:inset-0 after:z-10 after:rounded",
		"after:transition-shadow after:duration-75",
		
		// apply shadow to button when down
		"has-[button:active]:after:shadow-[inset_-8px_-8px_6px_-3px_rgb(0_0_0/0.7),inset_8px_0_6px_-3px_rgb(0_0_0/0.7),inset_0_0_6px_rgb(0_0_0/0.4)]",
		wellClass
	]}
>
	<button
		{type}
		class={[
			"mx-auto w-full h-full cursor-pointer",
			"noise-bg noise-opacity-20 bg-sys-button text-white font-bold rounded", 
			"border-t-2 border-b-10 border-r-5 border-l-5 border-t-sys-button-top border-l-sys-button-right border-b-sys-button-bottom border-r-sys-button-left",
			"transition-all duration-75 active:translate-y-0.5",
			buttonClass,
		]}
		{...rest}
	>
		{@render children()}
	</button>
</div>