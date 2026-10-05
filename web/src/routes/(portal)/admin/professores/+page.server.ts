import { gestaoDeUsuarios } from "#lib/server/gestao";

const gestao = gestaoDeUsuarios("professor");

export const load = gestao.load;
export const actions = gestao.actions;
