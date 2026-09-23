<script lang="ts">
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";
	import Box from "$lib/components/Box/Box.svelte";

	let {
		children,
		class: className = "",
		type = "button",
		...rest
	}: HTMLButtonAttributes & { children: Snippet } = $props();
</script>

<button
	{type}
	class="m-0 block cursor-pointer border-none bg-transparent p-0 text-inherit [font:inherit]"
	{...rest}
>
	<Box
		class={`hover:bg-sys-checker-bg-dark-grey hover:shadow-[inset_-2px_-2px_0_var(--color-sys-white)] ${className}`}
	>
		{@render children()}
	</Box>
</button>
