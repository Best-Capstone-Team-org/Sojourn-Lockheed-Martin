<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";

	let { tlm }: { tlm: string[] } = $props();
</script>

<TerminalScroll class="border-b border-white p-3">
	{#each tlm as hex, index (index)}
		<p class="pb-1 text-sm break-all">{hex}</p>
	{/each}
</TerminalScroll>
