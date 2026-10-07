import { criarAction } from "#lib/server/gestao";
import type { Actions } from "@sveltejs/kit";

export const actions: Actions = { criar: criarAction };
