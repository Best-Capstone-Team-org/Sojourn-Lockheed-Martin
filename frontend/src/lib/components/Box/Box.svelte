<script lang="ts">
	import type { Snippet } from 'svelte';
	import './box.css';

	let {
		children
	}: { children: Snippet } = $props();
</script>

<div class="standard-dialog w-96 mx-auto mt-16">
	{@render children()}
</div>
