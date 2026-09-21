<script lang="ts">
	import { apiWebSocket } from "$lib/api";

	const ws = apiWebSocket("command-ws");
	let ready = $state(false);
	let resp = $state("");
	
	ws.onopen = (ev) => {
		ready = true;
	}; 
	ws.onmessage = (ev) => {
		resp += `${ev.data}\n`;
	};

	$effect(() => {
		if (ready) {
			ws.send("hello world");
		}
	});
</script>

<h1>Welcome to SvelteKit</h1>
<p>
	Visit <a href="https://svelte.dev/docs/kit">svelte.dev/docs/kit</a> to read the
	documentation
</p>

<h2>Responses:</h2>
<p>{resp}</p>
