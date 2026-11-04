<script lang="ts">
	import {
		BookIcon,
		DownloadIcon,
		FileDigitIcon,
		UploadIcon,
	} from "@lucide/svelte/icons";
	import { fade } from "svelte/transition";
	import { apiFetch, type DownloadBinaryResponse } from "$lib/api";

	let {
		visible,
		onBatchFile,
	}: {
		visible: boolean;
		onBatchFile: (file: File) => void;
	} = $props();

	let fileInput: HTMLInputElement;

	// batch commands
	const openBatchPicker = () => fileInput.click();
	const handleBatchFile = (
		e: Event & { currentTarget: HTMLInputElement }
	) => {
		const file = e.currentTarget.files?.[0];
		if (file) onBatchFile(file);
		e.currentTarget.value = "";
	};

	// downloads
	const downloadManual = () => console.log("TODO download manual");
	async function downloadBinary() {
		try {
			const response = await apiFetch("download-binary");
			if (!response.ok) {
				const { message }: DownloadBinaryResponse =
					await response.json();
				throw new Error(message);
			}

			const url = URL.createObjectURL(await response.blob());
			const link = document.createElement("a");
			link.href = url;
			link.download = "probe.bin";
			link.click();
			URL.revokeObjectURL(url);
		} catch (error) {
			console.error("Failed to download binary", error);
		}
	}

	// upload binary
	const uploadBinary = () => console.log("TODO: download manual");

	const quickActions = [
		{
			label: "Upload Commands in Bulk (Line Separated, Limit 100)",
			icon: FileDigitIcon,
			onclick: openBatchPicker,
		},
		{
			label: "Download Manual",
			icon: BookIcon,
			onclick: downloadManual,
		},
		{
			label: "Download Binary",
			icon: DownloadIcon,
			onclick: downloadBinary,
		},
		{
			label: "Upload Binary",
			icon: UploadIcon,
			onclick: uploadBinary,
		},
	];
</script>

<input
	bind:this={fileInput}
	type="file"
	accept=".txt,text/plain"
	hidden
	onchange={handleBatchFile}
/>

{#if visible}
	<div
		class="ml-auto flex shrink-0 gap-1"
		transition:fade={{ duration: 150 }}
	>
		{#each quickActions as action (action.label)}
			<button
				type="button"
				class="cursor-pointer rounded p-1 hover:bg-white/10"
				title={action.label}
				aria-label={action.label}
				onclick={action.onclick}
			>
				<action.icon class="size-4" />
			</button>
		{/each}
	</div>
{/if}
