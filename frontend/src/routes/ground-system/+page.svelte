<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import * as Resizable from "$lib/components/ui/resizable/index.js";
	import { MissionStatus } from "$lib/enums";
	import MissionDetail from "$lib/components/MissionDetail/MissionDetail.svelte";

	const missionDetails = [
		{
			title: "Mission 1",
			status: MissionStatus.INVALID,
			description:
				"This is the description for mission one. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission one. There are lots of important details here for the player to read.",
		},
		{
			title: "Mission 2",
			status: MissionStatus.LOCKED,
			description:
				"This is the description for mission two. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission two. There are lots of important details here for the player to read.",
		},
		{
			title: "Mission 3",
			status: MissionStatus.ACTIVE,
			description:
				"This is the description for mission three. There are lost of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission three. There are lots of important details here for the player to read.",
		},
		{
			title: "Mission 4",
			status: MissionStatus.COMPLETE,
			description:
				"This is the description for mission four. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission four. There are lots of important details here for the player to read.",
		},
	];

	import { apiWebSocket } from "$lib/api";
	import { onMount } from "svelte";

	let ws = $state<WebSocket | undefined>(undefined);
	let ready = $state(false);
	let resp = $state("");

	onMount(() => {
		if (!ws) {
			ws = apiWebSocket("command-ws");

			ws.onopen = () => {
				ready = true;
			};
			ws.onmessage = (ev) => {
				resp += `${ev.data}\n`;
			};
		}
	});

	$effect(() => {
		if (ready && ws) {
			ws.send("hello world");
		}
	});

	// sending commands
	let command = $state("");

	const sendCommand = () => {
		if (ready && ws) {
			ws.send(command);
			command = "";
		}
	};
</script>

<!-- <div class="flex justify-center items-center h-screen flex-col gap-0">
	<Screen class="w-50 h-50 flex items-center justify-center">
		<h1 class="text-center text-2xl">Sojourn</h1>
	</Screen>
</div> -->

<div class="h-screen w-screen overflow-hidden">
	<Resizable.PaneGroup
		direction="horizontal"
		class="h-full w-full rounded-lg border text-white"
	>
		<!-- Mission Status -->
		<Resizable.Pane defaultSize={30}>
			<div class="flex h-full min-h-0 flex-col bg-red-500">
				<Box class="w-full shrink-0 p-2">
					<span class="font-semibold">Mission Details</span>
				</Box>
				<div class="min-h-0 flex-1 overflow-y-auto">
					{#each missionDetails as missionDetail}
						<MissionDetail
							title={missionDetail.title}
							status={missionDetail.status}
							description={missionDetail.description}
							diagnostic={missionDetail.diagnostic}
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
										<div
											class="flex h-full flex-col-reverse p-2"
										>
											<p>{resp}</p>
										</div>
									</Resizable.Pane>

									<Resizable.Handle />

									<!-- Terminal -->
									<Resizable.Pane
										defaultSize={40}
										class="flex h-full flex-col-reverse overflow-y-hidden"
									>
										<!-- Terminal History -->
										<div
											class="flex w-full items-center justify-center border-t p-2 font-semibold"
										>
											<span class="w-5"> > </span>
											<input
												type="text"
												class="w-full border-none outline-none"
												autocomplete="off"
												autocorrect="off"
												spellcheck="false"
												autocapitalize="off"
												placeholder="Enter command"
												bind:value={command}
												onkeydown={(e) => {
													if (e.key === "Enter") {
														sendCommand();
													}
												}}
											/>
										</div>

										<!-- Terminal Input -->
										<div
											class="tems-center flex h-full justify-center bg-green-500 p-6"
										>
											<span class="font-semibold"
												>Terminal</span
											>
										</div>
									</Resizable.Pane>
								</Resizable.PaneGroup>
							</Screen>
						</Resizable.Pane>

						<Resizable.Handle />

						<!-- Read Out -->
						<Resizable.Pane defaultSize={25}>
							<div
								class="flex h-full items-center justify-center p-6"
							>
								<span class="font-semibold">Read Out</span>
							</div>
						</Resizable.Pane>
					</Resizable.PaneGroup>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		</Resizable.Pane>
	</Resizable.PaneGroup>
</div>
