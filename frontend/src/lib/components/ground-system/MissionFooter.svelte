<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";

	let {
		writes,
		reads,
		maxWrites,
		maxReads,
	}: {
		writes: number;
		reads: number;
		maxWrites: number;
		maxReads: number;
	} = $props();
</script>

<Box class="w-full p-2">
	<div class="flex flex-row gap-[1vw]">
		<div>
			<Button
				wellClass="h-full"
				buttonClass="p-2"
				alert
				onclick={() => goto(resolve("/level-select"))}
			>
				Abort Mission
			</Button>
		</div>
		<div class="flex flex-col gap-[1vw]">
			<span class="font-semibold"
				>Writes: {maxWrites - writes}/{maxWrites}</span
			>
			<span class="font-semibold"
				>Reads: {maxReads - reads}/{maxReads}</span
			>
		</div>
	</div>
</Box>
