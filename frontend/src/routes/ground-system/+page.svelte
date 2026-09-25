<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import MissionStatus from "$lib/components/ground-system/MissionStatus.svelte";
	import TelemetryConsole from "$lib/components/ground-system/TelemetryConsole.svelte";
	import Downlink from "$lib/components/ground-system/Downlink.svelte";
	import ReadOut from "$lib/components/ground-system/ReadOut.svelte";
	import Terminal from "$lib/components/ground-system/Terminal.svelte";
	import * as Resizable from "$lib/components/ui/resizable/index.js";
	import type { MissionDetailType } from "$lib/types";
	import { MissionStatusEnum } from "$lib/enums";
	import {
		apiWebSocket,
		type Telemetry,
		type CommandDownlink,
	} from "$lib/api";
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

	// NOTE might not need this but it's good for debug print
	let commandDownlink = $state<CommandDownlink | undefined>(undefined);

	let tlmHex = $state<string[]>([]);
	let telemetry = $state<Telemetry>();

	onMount(() => {
		if (!ws) {
			ws = apiWebSocket("command-ws");

			ws.onopen = () => {
				ready = true;
			};

			ws.onmessage = (ev) => {
				commandDownlink = JSON.parse(ev.data) as CommandDownlink;

				// handle telemetry
				if (commandDownlink.type === "telemetry") {
					tlmHex.push(commandDownlink.TLM);
					telemetry = commandDownlink.telemetry;
				}
			};
		}
	});
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
					<TelemetryConsole />
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
										<Downlink {tlmHex} />
									</Resizable.Pane>

									<Resizable.Handle />

									<!-- Terminal -->
									<Resizable.Pane defaultSize={40}>
										<Terminal
											{ws}
											{ready}
										/>
									</Resizable.Pane>
								</Resizable.PaneGroup>
							</Screen>
						</Resizable.Pane>

						<Resizable.Handle />

						<!-- Read Out -->
						<Resizable.Pane defaultSize={25}>
							<ReadOut {telemetry} />
						</Resizable.Pane>
					</Resizable.PaneGroup>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		</Resizable.Pane>
	</Resizable.PaneGroup>
</div>
