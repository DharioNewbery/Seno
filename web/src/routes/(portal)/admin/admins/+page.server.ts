import { gestaoDeUsuarios } from "#lib/server/gestao";

const gestao = gestaoDeUsuarios("admin");

export const load = gestao.load;
export const actions = gestao.actions;
