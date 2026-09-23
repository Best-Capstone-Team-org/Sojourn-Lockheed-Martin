<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import { MissionStatusEnum } from "$lib/enums";

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
				<!-- LED Indicator TODO make colors standard in a css file -->
				<div
					class="flex h-4 w-4 items-center justify-center rounded-full bg-sys-medium-grey"
				>
					{#if status === MissionStatusEnum.ACTIVE}
						<div
							class="h-3 w-3 rounded-full bg-sys-alert-yellow"
						></div>
					{:else if status === MissionStatusEnum.COMPLETE}
						<div
							class="h-3 w-3 rounded-full bg-sys-alert-green"
						></div>
					{/if}
				</div>
			</div>
		</div>

		{#if status !== MissionStatusEnum.LOCKED}
			<div class="p-2">
				<p class="text-sm">{description}</p>

				{#if diagnostic && status !== MissionStatusEnum.COMPLETE}
					<div
						class="border-l-6 border-sys-alert-yellow bg-sys-medium-grey p-2"
					>
						<p class="text-sm">{diagnostic}</p>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</Box>
