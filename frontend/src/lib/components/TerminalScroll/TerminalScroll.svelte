<script lang="ts">
    import type { Snippet } from "svelte";

	let {
		children,
		class: className = "",
	}: { children: Snippet; class?: string } = $props();

	let scroller = $state<HTMLDivElement>();

	// stay pinned while the view is already at the latest line.
	let pinned = true;
	const nearBottom = (el: HTMLDivElement) => el.scrollHeight - el.scrollTop - el.clientHeight < 8;

	const onScroll = () => {
		if (!scroller) return;
		pinned = nearBottom(scroller);
	};

	$effect(() => {
        const el = scroller;
        if (!el) return;

        const observer = new MutationObserver(() => {
            if (pinned) el.scrollTop = el.scrollHeight;
        });
        
        observer.observe(el, { childList: true, subtree: true, characterData: true });
        return () => observer.disconnect();
    });
</script>

<div
	bind:this={scroller}
	onscroll={onScroll}
	class={[ "flex h-full min-h-0 flex-col overflow-y-auto", className ]}
>
	<div class="mt-auto">
		{@render children()}
	</div>
</div>