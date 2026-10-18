<script lang="ts">
  import { onMount } from "svelte";
  import { criarEditor, type InstanciaEditor } from "./editor-core";

  let {
    linguagem = "python",
    valor = "",
    autocomplete = false,
    somenteLeitura = false,
    onchange,
  }: {
    linguagem?: string;
    valor?: string;
    autocomplete?: boolean;
    somenteLeitura?: boolean;
    onchange?: (valor: string) => void;
  } = $props();

  let container: HTMLDivElement | undefined = $state();
  let editor: InstanciaEditor | null = null;

  onMount(() => {
    if (!container) return;
    editor = criarEditor(container, {
      valor,
      linguagem,
      autocomplete,
      somenteLeitura,
      onchange: (v) => {
        onchange?.(v);
      },
    });

    return () => {
      editor?.destruir();
      editor = null;
    };
  });

  $effect(() => {
    if (editor && valor !== undefined) {
      if (editor.getValor() !== valor) {
        editor.setValor(valor);
      }
    }
  });
</script>

<div class="editor-codigo-container" bind:this={container}></div>

<style>
  .editor-codigo-container {
    border: 1px solid var(--seno-gray-300, #ccc);
    border-radius: 8px;
    overflow: hidden;
    background: #fff;
    min-height: 180px;
    margin: 0.4rem 0;
  }
</style>
