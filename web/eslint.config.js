// Configuração eslint 9 (flat config) para o esqueleto SvelteKit.
import js from "@eslint/js";
import ts from "typescript-eslint";
import svelte from "eslint-plugin-svelte";

export default ts.config(
  {
    ignores: [
      "build/*",
      ".svelte-kit/*",
      "dist/*",
      "node_modules/*",
      "src/lib/generated/*",
    ],
  },
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs["flat/recommended"],
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parserOptions: {
        parser: ts.parser,
        extraFileExtensions: [".svelte"],
        svelteConfig: true,
      },
      // Globais do navegador usados nos componentes (fetch, eventos DOM...).
      globals: {
        AbortController: "readonly",
        Blob: "readonly",
        CustomEvent: "readonly",
        Event: "readonly",
        EventTarget: "readonly",
        File: "readonly",
        FormData: "readonly",
        Headers: "readonly",
        HTMLElement: "readonly",
        HTMLInputElement: "readonly",
        KeyboardEvent: "readonly",
        Request: "readonly",
        Response: "readonly",
        SubmitEvent: "readonly",
        URL: "readonly",
        URLSearchParams: "readonly",
        console: "readonly",
        crypto: "readonly",
        document: "readonly",
        fetch: "readonly",
        getComputedStyle: "readonly",
        history: "readonly",
        localStorage: "readonly",
        location: "readonly",
        matchMedia: "readonly",
        navigator: "readonly",
        requestAnimationFrame: "readonly",
        sessionStorage: "readonly",
        setTimeout: "readonly",
        structuredClone: "readonly",
        window: "readonly",
      },
    },
  },
  {
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_" },
      ],
    },
  },
);
