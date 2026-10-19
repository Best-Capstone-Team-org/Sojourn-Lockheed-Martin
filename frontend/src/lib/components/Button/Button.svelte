<script lang="ts">
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";

	let {
		children,
		buttonClass = "",
		wellClass = "",
		type = "button",
		alert = false,
		...rest
	}: HTMLButtonAttributes & {
		children: Snippet;
		buttonClass?: string;
		wellClass?: string;
		alert?: boolean;
	} = $props();

	const buttonColor = $derived(
		alert
			? "bg-sys-button-alert border-t-sys-button-alert-top border-r-sys-button-alert-left border-b-sys-button-alert-bottom border-l-sys-button-alert-right noise-opacity-30"
			: "bg-sys-button border-t-sys-button-top border-r-sys-button-left border-b-sys-button-bottom border-l-sys-button-right noise-opacity-20"
	);
</script>

<div
	class={[
		"relative rounded bg-black p-0.5 pt-px",

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
			"noise-bg rounded font-bold text-white",
			"border-t-2 border-r-5 border-b-10 border-l-5 ",
			"transition-all duration-75 active:translate-y-0.5",
			buttonColor,
			buttonClass,
		]}
		{...rest}
	>
		{@render children()}
	</button>
</div>
