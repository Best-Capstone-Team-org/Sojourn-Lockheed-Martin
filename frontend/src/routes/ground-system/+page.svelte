<script lang="ts">
	import Box from '$lib/components/Box/Box.svelte';
    import Screen from '$lib/components/Screen/Screen.svelte';
    import * as Resizable from "$lib/components/ui/resizable/index.js";
    import { MissionStatus } from '$lib/enums';
    import MissionDetail from '$lib/components/MissionDetail/MissionDetail.svelte';

    const missionDetails = [
        {
            title: "Mission 1",
            status: MissionStatus.INVALID,
            description: "This is the description for mission one. There are lots of important details here for the player to read.",
            diagnostic: "This is the diagnostic for mission one. There are lots of important details here for the player to read."
        },
        {
            title: "Mission 2",
            status: MissionStatus.LOCKED,
            description: "This is the description for mission two. There are lots of important details here for the player to read.",
            diagnostic: "This is the diagnostic for mission two. There are lots of important details here for the player to read."
        },
        {
            title: "Mission 3",
            status: MissionStatus.ACTIVE,
            description: "This is the description for mission three. There are lost of important details here for the player to read.",
            diagnostic: "This is the diagnostic for mission three. There are lots of important details here for the player to read."
        },
        {
            title: "Mission 4",
            status: MissionStatus.COMPLETE,
            description: "This is the description for mission four. There are lots of important details here for the player to read.",
            diagnostic: "This is the diagnostic for mission four. There are lots of important details here for the player to read."
        }
    ];
</script>

<!-- <div class="flex justify-center items-center h-screen flex-col gap-0">
	<Screen class="w-50 h-50 flex items-center justify-center">
		<h1 class="text-center text-2xl">Sojourn</h1>
	</Screen>
</div> -->

<div class="h-screen w-screen overflow-hidden">
<Resizable.PaneGroup direction="horizontal" class="h-full w-full rounded-lg border text-white">

    <!-- Mission Status -->
    <Resizable.Pane defaultSize={30}>
        <div class="flex flex-col h-full min-h-0 bg-red-500">
            <Box class="w-full p-2 shrink-0">
                <span class="font-semibold">Mission Details</span>
            </Box>
            <div class="flex-1 min-h-0 overflow-y-auto">
                {#each missionDetails as missionDetail}
                    <MissionDetail 
                        title = {missionDetail.title} 
                        status = {missionDetail.status} 
                        description = {missionDetail.description} 
                        diagnostic = {missionDetail.diagnostic} 
                    />
                {/each}
            </div>
        </div>

    </Resizable.Pane>

    <Resizable.Handle />

    <!-- Control Panel -->
    <Resizable.Pane defaultSize={70}>
        <Resizable.PaneGroup direction="vertical">

        <!-- Telemetry -->
        <Resizable.Pane defaultSize={15}>
            <Screen class="flex h-full items-center justify-center p-6">
                <span class="font-semibold">Dishes</span>
            </Screen>
        </Resizable.Pane>

        <Resizable.Handle />

        <!-- Control Panel -->
        <Resizable.Pane defaultSize={85}>
            <Resizable.PaneGroup direction="horizontal">

                <!-- Console -->
                <Resizable.Pane defaultSize={75}>
                    <Screen class="h-full">
                        <Resizable.PaneGroup direction="vertical">

                            <!-- Downlink -->
                            <Resizable.Pane defaultSize={60}>

                                <div class="flex h-full items-center justify-center p-6">
                                    <span class="font-semibold">Downlink</span>
                                </div>

                            </Resizable.Pane>

                            <Resizable.Handle />

                            <!-- Terminal -->
                            <Resizable.Pane defaultSize={40} class="h-full flex flex-col-reverse overflow-y-hidden">

                                <!-- Terminal History -->
                                <div class="p-2 font-semibold w-full flex items-center justify-center border-t">
                                    <span class="w-5"> > </span> 
                                        <input 
                                            type="text" 
                                            class="border-none outline-none w-full" 
                                            autocomplete="off" 
                                            autocorrect="off" 
                                            spellcheck="false" 
                                            autocapitalize="off"
                                            placeholder="Enter command"
                                        />
                                </div>

                                <!-- Terminal Input -->
                                <div class="flex tems-center h-full justify-center p-6 bg-green-500">
                                    <span class="font-semibold">Terminal</span>
                                </div>

                                

                            </Resizable.Pane>

                        </Resizable.PaneGroup>
                    </Screen>
                </Resizable.Pane>

                <Resizable.Handle />

                <!-- Read Out -->
                <Resizable.Pane defaultSize={25}>

                    <div class="flex h-full items-center justify-center p-6">
                        <span class="font-semibold">Read Out</span>
                    </div>

                </Resizable.Pane>

            </Resizable.PaneGroup>
        </Resizable.Pane>

        </Resizable.PaneGroup>
    </Resizable.Pane>

</Resizable.PaneGroup>
</div>
