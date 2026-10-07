<script lang="ts">
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	let {
		children,
		buttonClass = "",
		wellClass = "",
		type = "button",
		...rest
	}: HTMLButtonAttributes & {
		children: Snippet;
		buttonClass?: string;
		wellClass?: string;
	} = $props();
</script>

<div
	class={[
		"relative overflow-hidden rounded bg-black p-0.5 pt-px",

		// overlay goes from transparent to shadow on button press
		"after:pointer-events-none after:absolute after:inset-0 after:z-10 after:rounded",
		"after:transition-shadow after:duration-75",

		// apply shadow to button when down
		"has-[button:active]:after:shadow-[inset_-8px_-8px_6px_-3px_rgb(0_0_0/0.7),inset_8px_0_6px_-3px_rgb(0_0_0/0.7),inset_0_0_6px_rgb(0_0_0/0.4)]",
		wellClass,
	]}
>
	<button
		{type}
		class={[
			"mx-auto h-full w-full cursor-pointer",
			"noise-bg rounded bg-sys-button font-bold text-white noise-opacity-20",
			"border-t-2 border-r-5 border-b-10 border-l-5 border-t-sys-button-top border-r-sys-button-left border-b-sys-button-bottom border-l-sys-button-right",
			"transition-all duration-75 active:translate-y-0.5",
			buttonClass,
		]}
		{...rest}
	>
		{@render children()}
	</button>
</div>
