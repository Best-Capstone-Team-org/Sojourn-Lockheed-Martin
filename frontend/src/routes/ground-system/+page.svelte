<script lang="ts">
	import Screen from '$lib/components/Screen/Screen.svelte';
    import * as Resizable from "$lib/components/ui/resizable/index.js";
</script>

<!-- <div class="flex justify-center items-center h-screen flex-col gap-0">
	<Screen class="w-50 h-50 flex items-center justify-center">
		<h1 class="text-center text-2xl">Sojourn</h1>
	</Screen>
</div> -->

<Resizable.PaneGroup direction="horizontal" class="w-screen min-h-screen rounded-lg border text-white">

    <!-- Mission Status -->
    <Resizable.Pane defaultSize={30}>
        <div class="flex h-full items-center justify-center p-6">
            <span class="font-semibold">Mission Details</span>
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
                            <Resizable.Pane defaultSize={40}>
                                <div class="flex h-full items-center justify-center p-6">
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