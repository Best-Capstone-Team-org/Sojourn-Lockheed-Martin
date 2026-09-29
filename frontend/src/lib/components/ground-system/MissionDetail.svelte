<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import { MissionStatusEnum } from "$lib/enums";
	import Led from "./Led.svelte";
	import Screen from "../Screen/Screen.svelte";

	let {
		title,
		status,
		description,
		diagnostic,
	}: {
		title: string;
		status: MissionStatusEnum;
		description: string;
		diagnostic?: string;
	} = $props();
</script>

<Box class="w-full p-2">
	{#if status === MissionStatusEnum.INVALID}
		<span class="text-sys-alert-red"
			>Status: INVALID, something went wrong!</span
		>
	{:else}
		<div class="flex items-center justify-between">
			<p class="font-semibold">{title}</p>
			<div class="flex items-center gap-2">
				<span>{status}</span>
				{#if status === MissionStatusEnum.ACTIVE}
					<Led color="yellow" />
				{:else if status === MissionStatusEnum.COMPLETE}
					<Led color="green" />
				{:else}
					<Led />
				{/if}
			</div>
		</div>

		{#if status !== MissionStatusEnum.LOCKED}
			<hr
				class="my-1 border-t border-black/35 shadow-[0_1px_0_rgba(255,255,255,0.4)]"
			/>
			<div class="p-2">
				<p class="mb-2 text-sm">{description}</p>

				{#if diagnostic && status !== MissionStatusEnum.COMPLETE}
					<Screen
						class="p-3"
						bezelClass="h-full"
					>
						<p class="text-sm">{diagnostic}</p>
					</Screen>
				{/if}
			</div>
		{/if}
	{/if}
</Box>
