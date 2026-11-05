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
		isSatelliteDown,
		type Telemetry,
		type CommandDownlink,
		type ObjectiveDetail,
	} from "$lib/api";
	import { gameState } from "$lib/components/ground-system/GameState.svelte";
	import TelemetryButtons from "$lib/components/ground-system/TelemetryButtons.svelte";

	const RECONNECT_BASE_DELAY = 1000;
	const MAX_RECONNECT_DELAY = 10000;

	let ws = $state<WebSocket | undefined>(undefined);
	let ready = $state(false);

	let commandDownlink = $state<CommandDownlink | undefined>(undefined);
	let uplinkInFlight = $state(false);

	let tlm = $state<string[]>([]);
	let readOutTlm = $state<Telemetry>();
	let lastCommsTelemetryAt = $state<number>();
	let now = $state(Date.now());
	let satelliteDown = $derived(
		isSatelliteDown(readOutTlm, lastCommsTelemetryAt, now)
	);

	// Keep the package's declared order while applying state updates by ID.
	let objectives = $state<Record<string, ObjectiveDetail>>(
		Object.fromEntries(
			(gameState.scenario?.objectives ?? []).map((o) => [o.id, { ...o }])
		)
	);

	let attempts = $state(0);
	let retryTimer: ReturnType<typeof setTimeout> | undefined;
	let retry = $state(0);

	// constraints
	let maxWrites = $state(gameState.scenario?.constraints.writes ?? 0);
	let maxReads = $state(gameState.scenario?.constraints.reads ?? 0);
	let writes = $state(maxWrites);
	let reads = $state(maxReads);

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
				if (message.telemetry.crcOK && message.telemetry.comms) {
					lastCommsTelemetryAt = Date.now();
				}
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
					Object.assign(objective, update);
				}

				for (const [key, value] of Object.entries(
					message.constraints
				)) {
					if (key === "writes") {
						writes = value;
						console.log("writes", writes);
					} else if (key === "reads") {
						reads = value;
						console.log("reads", reads);
					}
				}
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

	// batch file upload
	let terminal = $state<Terminal>();
	let dragging = $state(false);

	async function runBatchFile(file: File) {
		const lines = (await file.text())
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter((line) => line && !line.startsWith("#"));
		terminal?.sendBatch(lines);
	}

	function onDragOver(e: DragEvent) {
		// prevent file open
		e.preventDefault();
		dragging = true;
	}

	function onDragLeave(e: DragEvent) {
		const target = e.currentTarget as HTMLElement;
		if (target.contains(e.relatedTarget as Node | null)) {
			return;
		}
		dragging = false;
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		const file = e.dataTransfer?.files[0];
		if (file) runBatchFile(file);
	}

	$effect(() => {
		const clock = setInterval(() => (now = Date.now()), 1000);

		return () => clearInterval(clock);
	});

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
		<Resizable.Pane
			defaultSize={30}
			minSize={10}
		>
			<MissionStatus
				{objectives}
				{writes}
				{reads}
				{maxWrites}
				{maxReads}
			/>
		</Resizable.Pane>

		<Resizable.Handle />

		<!-- Control Panel -->
		<Resizable.Pane
			defaultSize={70}
			minSize={20}
		>
			<Resizable.PaneGroup direction="horizontal">
				<Resizable.Pane
					defaultSize={75}
					minSize={10}
				>
					<Resizable.PaneGroup direction="vertical">
						<Resizable.Pane
							defaultSize={15}
							minSize={10}
						>
							<TelemetryConsole
								{commandDownlink}
								{uplinkInFlight}
								{satelliteDown}
							/>
						</Resizable.Pane>

						<Resizable.Handle />

						<!-- Console -->
						<Resizable.Pane
							defaultSize={85}
							minSize={10}
						>
							<div
								class="relative h-full"
								role="region"
								aria-label="Console"
								ondragover={onDragOver}
								ondragleave={onDragLeave}
								ondrop={onDrop}
							>
								<Screen
									class="h-full"
									bezelClass="h-full"
								>
									{#if dragging}
										<div
											class="pointer-events-none absolute inset-0 flex items-center justify-center text-xl font-bold text-sys-text-light"
										>
											Drop Batch File
										</div>
									{:else}
										<Resizable.PaneGroup
											direction="vertical"
										>
											<Resizable.Pane
												defaultSize={60}
												minSize={10}
											>
												<Downlink {tlm} />
											</Resizable.Pane>

											<Resizable.Handle />

											<Resizable.Pane
												defaultSize={40}
												minSize={10}
											>
												<Terminal
													bind:this={terminal}
													{ws}
													{ready}
													onUplinkInFlight={(
														inFlight
													) =>
														(uplinkInFlight =
															inFlight)}
													uplinkDelayS={gameState
														.scenario
														?.uplinkDelayS ?? 0}
												/>
											</Resizable.Pane>
										</Resizable.PaneGroup>
									{/if}
								</Screen>
							</div>
						</Resizable.Pane>
					</Resizable.PaneGroup>
				</Resizable.Pane>

				<Resizable.Handle />

				<!-- Read Out -->
				<Resizable.Pane
					defaultSize={25}
					minSize={10}
				>
					<div class="flex h-full min-h-0 flex-col">
						<div class="shrink-0">
							<TelemetryButtons />
						</div>
						<div class="min-h-0 flex-1">
							<ReadOut
								{readOutTlm}
								onBatchFile={runBatchFile}
							/>
						</div>
					</div>
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
