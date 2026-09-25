<script lang="ts">
	let {
		ws,
		ready,
	}: {
		ws: WebSocket | undefined;
		ready: boolean;
	} = $props();

	let command = $state("");

	const sendCommand = () => {
		if (ready && ws) {
			ws.send(command);
			command = "";
		}
	};
</script>

<div class="flex h-full flex-col-reverse overflow-y-hidden">
	<!-- Terminal Input -->
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

	<!-- Terminal History -->
	<div class="flex h-full items-center justify-center p-6">
		<span class="font-semibold">Terminal</span>
	</div>
</div>
