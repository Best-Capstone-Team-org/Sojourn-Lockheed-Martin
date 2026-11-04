<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";
	import QuickActions from "./QuickActions.svelte";

	let {
		ws,
		ready,
		uplinkDelayS,
	}: {
		ws: WebSocket | undefined;
		ready: boolean;
		uplinkDelayS: number;
	} = $props();

	let terminalMode = $state<"history" | "search">("history");
	let textEntry = $state("");

	// dueAt is set only while the command is still travelling to the probe.
	type Entry = { command: string; dueAt?: number };
	let entries = $state<Entry[]>([]);
	let history = $derived(entries.map((entry) => entry.command));
	let now = $state(Date.now());
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

	// Whole seconds left, rounded up so the player never sees 0 s early.
	const remaining = (entry: Entry) =>
		entry.dueAt === undefined
			? 0
			: Math.max(0, Math.ceil((entry.dueAt - now) / 1000));
	let inFlight = $derived(entries.some((entry) => remaining(entry) > 0));

	$effect(() => {
		if (!inFlight) return;
		const clock = setInterval(() => (now = Date.now()), 250);
		return () => clearInterval(clock);
	});

	// actions
	const send = (cmd: string) => {
		if (!ready || !ws || !cmd.trim()) {
			return false;
		}
		ws.send(cmd);
		now = Date.now();
		entries.push({ command: cmd, dueAt: now + uplinkDelayS * 1000 });
		return true;
	};

	const sendCommand = () => {
		if (send(command)) {
			textEntry = "";
			historyIndex = -1;
		}

		terminalMode = "history";
	};

	export async function sendBatch(lines: string[], delayMs = 250) {
		for (const line of lines) {
			if (!send(line)) {
				break;
			}

			// pause for next command
			await new Promise((r) => setTimeout(r, delayMs));
		}
	}

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
		<QuickActions visible={textEntry === ""} />
	</div>

	<!-- Terminal History -->
	<TerminalScroll class="p-3">
		{#if terminalMode === "history"}
			{#each entries as entry, index (index)}
				<p class="mb-1 text-sm break-all">
					<span>{entry.command}</span>
					{#if remaining(entry) > 0}
						<span class="ml-2 opacity-70">{remaining(entry)} s</span
						>
					{/if}
				</p>
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
