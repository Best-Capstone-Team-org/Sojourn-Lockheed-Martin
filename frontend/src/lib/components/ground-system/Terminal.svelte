<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";

	let {
		ws,
		ready,
	}: {
		ws: WebSocket | undefined;
		ready: boolean;
	} = $props();

	let terminalMode = $state<"history" | "search">("history");
	let textEntry = $state("");
	let history = $state<string[]>([]);
	let historyIndex = $state(-1);

	let searchResults = $derived(
		history.filter((cmd) => cmd.includes(textEntry))
	);
	let command = $derived.by(() => {
		switch (terminalMode) {
			case "history":
				return textEntry;
			case "search":
				if (searchResults.length === 0) return textEntry;
				return searchResults[searchResults.length - 1];
		}
	});

	// actions
	const sendCommand = () => {
		if (ready && ws) {
			ws.send(command);
			history.push(command);
			textEntry = "";
			historyIndex = -1;
		}

		terminalMode = "history";
	};

	const clearCommand = () => {
		terminalMode = "history";

		textEntry = "";
		historyIndex = -1;
	};

	const historyUp = () => {
		terminalMode = "history";

		if (history.length === 0) return;
		historyIndex = Math.min(historyIndex + 1, history.length - 1);
		textEntry = history[history.length - historyIndex - 1];
	};

	const historyDown = () => {
		terminalMode = "history";

		if (historyIndex <= 0) {
			clearCommand();
			return;
		}

		historyIndex--;
		textEntry = history[history.length - historyIndex - 1];
	};

	const historySearch = () => {
		// toggle off
		if (terminalMode === "search") {
			terminalMode = "history";
			return;
		}

		terminalMode = "search";
	};

	// mappings
	const hotKeys = (e: KeyboardEvent) => {
		const keys = [
			e.ctrlKey && "ctrl",
			e.altKey && "alt",
			e.shiftKey && "shift",
			e.metaKey && "meta",
		].filter(Boolean);
		return [...keys, e.key].join("+");
	};

	const actions: Record<string, () => void> = {
		Enter: sendCommand,
		"ctrl+p": historyUp,
		ArrowUp: historyUp,
		"ctrl+n": historyDown,
		ArrowDown: historyDown,
		"ctrl+l": clearCommand,
		"ctrl+r": historySearch,
	};

	// event handle
	const onkeydown = (e: KeyboardEvent) => {
		const hotKey = hotKeys(e);
		const action = actions[hotKey];
		if (action) {
			e.preventDefault();
			action();
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
			bind:value={textEntry}
			{onkeydown}
		/>
	</div>

	<!-- Terminal History -->
	<TerminalScroll class="p-3">
		{#if terminalMode === "history"}
			{#each history as command, index (index)}
				<p class="mb-1 text-sm break-all">{command}</p>
			{/each}
		{:else if terminalMode === "search"}
			{#each searchResults as result, index (index)}
				{#if index === searchResults.length - 1}
					<p
						class="mb-1 bg-white text-sm break-all text-black mix-blend-screen"
					>
						{result}
					</p>
				{:else}
					<p class="mb-1 text-sm break-all">{result}</p>
				{/if}
			{/each}
		{/if}
	</TerminalScroll>
</div>
