<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import MissionStatus from "$lib/components/ground-system/MissionStatus.svelte";
	import Telemetry from "$lib/components/ground-system/Telemetry.svelte";
	import Downlink from "$lib/components/ground-system/Downlink.svelte";
	import ReadOut from "$lib/components/ground-system/ReadOut.svelte";
	import * as Resizable from "$lib/components/ui/resizable/index.js";
	import type { MissionDetailType } from "$lib/types";
	import { MissionStatusEnum } from "$lib/enums";
	import { apiWebSocket } from "$lib/api";
	import { onMount } from "svelte";

	// dummy data for the mission status
	// TODO remove once we get actual data
	const missionDetails: MissionDetailType[] = [
		{
			id: 1,
			title: "Mission 1",
			status: MissionStatusEnum.INVALID,
			description:
				"This is the description for mission one. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission one. There are lots of important details here for the player to read.",
		},
		{
			id: 2,
			title: "Mission 2",
			status: MissionStatusEnum.LOCKED,
			description:
				"This is the description for mission two. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission two. There are lots of important details here for the player to read.",
		},
		{
			id: 3,
			title: "Mission 3",
			status: MissionStatusEnum.ACTIVE,
			description:
				"This is the description for mission three. There are lost of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission three. There are lots of important details here for the player to read.",
		},
		{
			id: 4,
			title: "Mission 4",
			status: MissionStatusEnum.COMPLETE,
			description:
				"This is the description for mission four. There are lots of important details here for the player to read.",
			diagnostic:
				"This is the diagnostic for mission four. There are lots of important details here for the player to read.",
		},
	];

	// websocket connection
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

	// sending commands
	let command = $state("");

	const sendCommand = () => {
		if (ready && ws) {
			ws.send(command);
			command = "";
		}
	};
</script>

<div class="h-screen w-screen overflow-hidden">
	<Resizable.PaneGroup
		direction="horizontal"
		class="h-full w-full rounded-lg border text-white"
	>
		<!-- Mission Status -->
		<Resizable.Pane defaultSize={30}>
			<MissionStatus {missionDetails} />
		</Resizable.Pane>

		<Resizable.Handle />

		<!-- Control Panel -->
		<Resizable.Pane defaultSize={70}>
			<Resizable.PaneGroup direction="vertical">
				<!-- Telemetry -->
				<Resizable.Pane defaultSize={15}>
					<Telemetry />
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
										<Downlink {resp} />
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
							<ReadOut />
						</Resizable.Pane>
					</Resizable.PaneGroup>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		</Resizable.Pane>
	</Resizable.PaneGroup>
</div>
