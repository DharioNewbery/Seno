// Arquivo de declarações do SvelteKit (adapter-node / server-side).
// Veja https://svelte.dev/docs/kit/types#app.d.ts
import type { Usuario } from "#lib/tipos";

declare global {
  namespace App {
    // interface Error {}
    interface Locals {
      usuario: Usuario | null;
    }
    // interface PageData {}
    // interface PageState {}
    // interface Platform {}
  }
}

export {};
