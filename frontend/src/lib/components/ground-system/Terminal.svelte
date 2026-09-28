<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";

	let {
		ws,
		ready,
	}: {
		ws: WebSocket | undefined;
		ready: boolean;
	} = $props();

	let command = $state("");
	let history = $state<string[]>([]);

	const sendCommand = () => {
		if (ready && ws) {
			ws.send(command);
			history.push(command);
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
			placeholder="Enter Command"
			bind:value={command}
			onkeydown={(e) => {
				if (e.key === "Enter") {
					sendCommand();
				}
			}}
		/>
	</div>

	<!-- Terminal History -->
	<TerminalScroll class="p-2">
		{#each history as command, index (index)}
			<p class="break-all pb-2 text-sm">{command}</p>
		{/each}
	</TerminalScroll>
</div>
