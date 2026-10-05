<script lang="ts">
	import favicon from '#lib/assets/favicon.svg';
	import '#lib/styles/global.css';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
