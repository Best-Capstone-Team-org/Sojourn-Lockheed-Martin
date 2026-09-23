<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import MissionDetail from "$lib/components/ground-system/MissionDetail.svelte";
	import type { MissionDetailType } from "$lib/types";

	let { missionDetails }: { missionDetails: MissionDetailType[] } = $props();
</script>

<div class="flex h-full min-h-0 flex-col bg-red-500">
    <Box class="w-full shrink-0 p-2">
        <span class="font-semibold">Mission Details</span>
    </Box>
    <div class="min-h-0 flex-1 overflow-y-auto">
        {#each missionDetails as missionDetail (missionDetail.id)}
            <MissionDetail
                title={missionDetail.title}
                status={missionDetail.status}
                description={missionDetail.description}
                diagnostic={missionDetail.diagnostic}
            />
        {/each}
    </div>
</div>