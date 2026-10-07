import {
  acoesDeConta,
  loadLista,
} from "#lib/server/gestao";

export const load = loadLista;
export const actions = acoesDeConta();
