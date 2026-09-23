<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import "./mission-detail.css";
	import { MissionStatus } from "$lib/enums";

	let {
		title,
		status,
		description,
		diagnostic,
	}: {
		title: string;
		status: MissionStatus;
		description: string;
		diagnostic?: string;
	} = $props();
</script>

<Box class="w-full p-2">
	{#if status === MissionStatus.INVALID}
		<span class="text-red-500">Status: INVALID, something went wrong!</span>
	{:else}
		<div class="flex items-center justify-between">
			<p class="font-semibold">{title}</p>
			<div class="flex items-center gap-2">
				<span>{status}</span>
				<!-- LED Indicator TODO make colors standard in a css file -->
				<div
					class="medium-grey flex h-4 w-4 items-center justify-center rounded-full"
				>
					{#if status === MissionStatus.ACTIVE}
						<div class="alert-yellow h-3 w-3 rounded-full"></div>
					{:else if status === MissionStatus.COMPLETE}
						<div class="alert-green h-3 w-3 rounded-full"></div>
					{/if}
				</div>
			</div>
		</div>

		{#if status !== MissionStatus.LOCKED}
			<div class="p-2">
				<p class="text-sm">{description}</p>

				{#if diagnostic && status !== MissionStatus.COMPLETE}
					<div class="diagnostic">
						<p class="text-sm">{diagnostic}</p>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</Box>
