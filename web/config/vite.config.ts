import adapter from "@sveltejs/adapter-node";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

// Origem do build de produção (adapter-node). Sem ela, o SvelteKit 3 assume
// 'https' e o próprio CSRF do Kit bloqueia os POSTs de formulário em HTTP
// ("Cross-site POST form submissions are forbidden") — o ORIGIN em runtime
// deixou de existir no Kit 3: a origem é gravada no build (paths.origin).
// O dev nativo (vite at :5173) vem sem a variável e não é afetado.
const origemBuild = process.env.SENO_SITE_ORIGIN;

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
