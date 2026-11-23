import adapter from "@sveltejs/adapter-node";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

// Origem opcional do build (adapter-node). Em desenvolvimento sem proxy, pode-se
// fixar SENO_SITE_ORIGIN no build; em produção atrás do Caddy/proxy reverso,
// deixe-a vazia para o @sveltejs/adapter-node v6 resolver a origem dinamicamente
// por requisição via PROTOCOL_HEADER=x-forwarded-proto e HOST_HEADER=x-forwarded-host
// (suportando qualquer host, IP, porta e camadas de proxy sem rebuild).
const origemBuild = process.env.SENO_SITE_ORIGIN?.trim();

export default defineConfig({
  plugins: [
    sveltekit({
      adapter: adapter(),
      ...(origemBuild ? { paths: { origin: origemBuild } } : {}),
      compilerOptions: {
        // Force runes mode for the project, except for libraries. Can be removed in svelte 6.
        runes: ({ filename }) =>
          filename.split(/[/\\]/).includes("node_modules") ? undefined : true,
      },
    }),
  ],
});
