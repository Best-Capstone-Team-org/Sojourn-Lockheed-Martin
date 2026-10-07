<script lang="ts">
	import type { HTMLButtonAttributes } from "svelte/elements";

	let {
		checked = $bindable(false),
		trackClass = "",
		wellClass = "",
		onclick,
		onchange,
		...rest
	}: Omit<HTMLButtonAttributes, "type" | "role" | "onchange"> & {
		checked?: boolean;
		onchange?: (checked: boolean) => void;
		trackClass?: string;
		wellClass?: string;
	} = $props();
</script>

<div
	class={[
		"relative h-7 w-14 overflow-hidden rounded bg-black p-0.5 pt-px",
		wellClass,
	]}
>
	<button
		type="button"
		role="switch"
		aria-checked={checked}
		class={[
			"flex h-full w-full cursor-pointer rounded",
			"shadow-[inset_0_2px_4px_rgb(0_0_0/0.8),inset_0_0_6px_rgb(0_0_0/0.4)]",
			"transition-colors duration-150",
			checked ? "bg-sys-screen-dark-green" : "bg-sys-medium-grey",
			trackClass,
		]}
		onclick={(e) => {
			checked = !checked;
			onchange?.(checked);
			onclick?.(e);
		}}
		{...rest}
	>
		<span
			class={[
				"h-full w-1/2",
				"noise-bg rounded bg-sys-switch",
				"border-t border-r-3 border-b-5 border-l-3 border-t-sys-switch-top border-r-sys-switch-left border-b-sys-switch-bottom border-l-sys-switch-right",
				"transition-transform duration-150",
				checked && "translate-x-full",
			]}
		></span>
	</button>
</div>
