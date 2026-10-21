<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import MissionStatus from "$lib/components/ground-system/MissionStatus.svelte";
	import TelemetryConsole from "$lib/components/ground-system/TelemetryConsole.svelte";
	import Downlink from "$lib/components/ground-system/Downlink.svelte";
	import ReadOut from "$lib/components/ground-system/ReadOut.svelte";
	import Terminal from "$lib/components/ground-system/Terminal.svelte";
	import * as Resizable from "$lib/components/ui/resizable/index.js";
	import {
		apiWebSocket,
		type Telemetry,
		type CommandDownlink,
		type CommandUplink,
		type ObjectiveDetail,
	} from "$lib/api";
	import { gameState } from "$lib/components/ground-system/GameState.svelte";

	const RECONNECT_BASE_DELAY = 1000;
	const MAX_RECONNECT_DELAY = 10000;

	let ws = $state<WebSocket | undefined>(undefined);
	let ready = $state(false);

	let commandDownlink = $state<CommandDownlink | undefined>(undefined);
	let commandUplink = $state<CommandUplink | undefined>(undefined);

	let tlm = $state<string[]>([]);
	let readOutTlm = $state<Telemetry>();

	// key objectives by id
	let objectives = $state<Record<string, ObjectiveDetail>>(
		Object.fromEntries(
			(gameState.scenario?.objectives ?? []).map((o) => [o.id, { ...o }])
		)
	);

	let attempts = $state(0);
	let retryTimer: ReturnType<typeof setTimeout> | undefined;
	let retry = $state(0);

	function scheduleReconnect() {
		const delay = Math.min(
			RECONNECT_BASE_DELAY * 2 ** attempts,
			MAX_RECONNECT_DELAY
		);
		attempts++;

		clearTimeout(retryTimer);
		retryTimer = setTimeout(() => retry++, delay);
	}

	function handleDownlink(message: CommandDownlink) {
		commandDownlink = message;

		switch (message.type) {
			case "telemetry":
				tlm.push(message.TLM);
				readOutTlm = message.telemetry;
				break;

			case "commandResponse":
				tlm.push(message.response);
				break;

			case "state":
				for (const update of message.objectives) {
					const objective = objectives[update.id];
					if (!objective) {
						continue;
					}
					Object.assign(objective, { hint: undefined }, update);
				}

				// TODO add constraints {}
				break;
		}
	}

	function handleSocketOpen() {
		attempts = 0;
		ready = true;
	}

	function handleSocketDisconnect() {
		ready = false;
		scheduleReconnect();
	}

	function clearSocketHandlers(socket: WebSocket) {
		socket.onopen = null;
		socket.onclose = null;
		socket.onerror = null;
		socket.onmessage = null;
	}

	$effect(() => {
		// Reconnect when the scenario changes or the retry signal is incremented.
		void retry;

		const socket = apiWebSocket(
			`select-scenario?scenario-id=${gameState.scenario?.id ?? ""}`
		);
		ws = socket;
		ready = false;

		socket.onopen = handleSocketOpen;
		socket.onclose = handleSocketDisconnect;
		socket.onerror = handleSocketDisconnect;
		socket.onmessage = (event) => {
			handleDownlink(JSON.parse(event.data) as CommandDownlink);
		};

		return () => {
			clearTimeout(retryTimer);
			clearSocketHandlers(socket);
			// Use a normal close code so the server treats navigation as a clean exit.
			socket.close(1000, "client leaving");
		};
	});
</script>

<div class="h-screen w-screen overflow-hidden">
	<Resizable.PaneGroup
		direction="horizontal"
		class="h-full w-full"
	>
		<!-- Mission Status -->
		<Resizable.Pane
			defaultSize={30}
			minSize={10}
		>
			<MissionStatus {objectives} />
		</Resizable.Pane>

		<Resizable.Handle />

		<!-- Control Panel -->
		<Resizable.Pane
			defaultSize={70}
			minSize={20}
		>
			<Resizable.PaneGroup direction="vertical">
				<!-- Telemetry -->
				<Resizable.Pane
					defaultSize={15}
					minSize={10}
				>
					<TelemetryConsole
						{commandDownlink}
						{commandUplink}
					/>
				</Resizable.Pane>

				<Resizable.Handle />

				<!-- Control Panel -->
				<Resizable.Pane
					defaultSize={85}
					minSize={10}
				>
					<Resizable.PaneGroup direction="horizontal">
						<!-- Console -->
						<Resizable.Pane
							defaultSize={75}
							minSize={10}
						>
							<Screen
								class="h-full"
								bezelClass="h-full"
							>
								<Resizable.PaneGroup direction="vertical">
									<!-- Downlink -->
									<Resizable.Pane
										defaultSize={60}
										minSize={10}
									>
										<Downlink {tlm} />
									</Resizable.Pane>

									<Resizable.Handle />

									<!-- Terminal -->
									<Resizable.Pane
										defaultSize={40}
										minSize={10}
									>
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
						<Resizable.Pane
							defaultSize={25}
							minSize={10}
						>
							<ReadOut {readOutTlm} />
						</Resizable.Pane>
					</Resizable.PaneGroup>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		</Resizable.Pane>
	</Resizable.PaneGroup>
</div>

<style>
	/* 
		hide resize handles 
		will reduce the discoverability for the style
		TODO: we need to find a good solution 
	*/
	div :global([data-slot="resizable-handle"]) {
		background-color: transparent;
		width: 0;
	}

	div :global([data-slot="resizable-handle"][data-direction="vertical"]) {
		width: 100%;
		height: 0;
	}
</style>
