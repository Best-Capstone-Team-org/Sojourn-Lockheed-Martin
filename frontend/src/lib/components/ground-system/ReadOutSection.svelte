<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "../Button/Button.svelte";

	let {
		title,
		items,
	}: {
		title: string;
		items: {
			label: string;
			value?: string | number | null;
			unit?: string;
			button?: () => void;
		}[];
	} = $props();

	const isMissing = (value: string | number | null | undefined) =>
		value === undefined || value === null || value === "";
</script>

<Box class="w-full p-2">
	<span class="font-semibold">{title}</span>
	<hr
		class="my-1 border-t border-black/35 shadow-[0_1px_0_rgba(255,255,255,0.4)]"
	/>
	{#each items as item (item.label)}
		<div class="flex justify-between">
			<span>{item.label}</span>
			{#if item.button}
				<Button
					class="px-2"
					onclick={item.button}>Toggle</Button
				>
			{:else if isMissing(item.value)}
				<span class="text-sys-alert-red">absent</span>
			{:else}
				<span>{item.value}{item.unit ? ` ${item.unit}` : ""}</span>
			{/if}
		</div>
	{/each}
</Box>
