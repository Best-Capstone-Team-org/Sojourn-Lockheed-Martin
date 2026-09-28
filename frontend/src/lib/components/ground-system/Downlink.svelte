<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";

	let { tlm }: { tlm: string[] } = $props();
</script>

<TerminalScroll class="p-2">
	{#each tlm as hex, index (index)}
		<p class="pb-2 text-sm break-all">{hex}</p>
	{/each}
</TerminalScroll>
