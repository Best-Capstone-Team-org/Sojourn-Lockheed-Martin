<script lang="ts">
	import type { Snippet } from 'svelte';
	import './screen.css';

	let {
		children,
		class: className = ''
	}: { children: Snippet; class?: string } = $props();
</script>

<div class={['screen mx-auto', className]}>
	{@render children()}
</div>
