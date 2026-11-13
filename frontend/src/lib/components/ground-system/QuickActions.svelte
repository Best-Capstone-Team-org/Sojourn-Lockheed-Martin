<script lang="ts">
	import {
		BookIcon,
		DownloadIcon,
		FileDigitIcon,
		HardDriveDownloadIcon,
		UploadIcon,
	} from "@lucide/svelte/icons";
	import { fade } from "svelte/transition";
	import { apiFetch, type DownloadBinaryResponse } from "$lib/api";

	let {
		visible,
		onBatchFile,
		onPatchFile,
	}: {
		visible: boolean;
		onBatchFile: (file: File) => void;
		onPatchFile: (file: File) => void;
	} = $props();

	let fileInput: HTMLInputElement;
	let patchInput: HTMLInputElement;

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
	async function downloadBinary(endpoint: string, filename: string) {
		try {
			const response = await apiFetch(endpoint);
			if (!response.ok) {
				const { message }: DownloadBinaryResponse =
					await response.json();
				throw new Error(message);
			}

			const url = URL.createObjectURL(await response.blob());
			const link = document.createElement("a");
			link.href = url;
			link.download = filename;
			link.click();
			URL.revokeObjectURL(url);
		} catch (error) {
			console.error("Failed to download binary", error);
		}
	}

	// upload binary
	const openPatchPicker = () => patchInput.click();
	const handlePatchFile = (
		e: Event & { currentTarget: HTMLInputElement }
	) => {
		const file = e.currentTarget.files?.[0];
		if (file) onPatchFile(file);
		e.currentTarget.value = "";
	};

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
			onclick: () => downloadBinary("download-binary", "probe.bin"),
		},
		{
			label: "Download Golden Binary",
			icon: HardDriveDownloadIcon,
			onclick: () =>
				downloadBinary("download-golden-binary", "probe_golden.bin"),
		},
		{
			label: "Upload Binary",
			icon: UploadIcon,
			onclick: openPatchPicker,
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

<input
	bind:this={patchInput}
	type="file"
	accept=".bin,application/octet-stream"
	hidden
	onchange={handlePatchFile}
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
