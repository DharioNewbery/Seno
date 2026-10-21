<script lang="ts">
	import favicon from '#lib/assets/favicon.svg';
	import '@fontsource-variable/ibm-plex-sans';
	import '@fontsource/ibm-plex-mono/latin-400.css';
	import '@fontsource/ibm-plex-mono/latin-500.css';
	import '#lib/styles/global.css';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta name="theme-color" content="#082a5a" />
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
