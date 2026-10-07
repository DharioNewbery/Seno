import { acoesDeConta, cargosAction, loadDetalhe } from "#lib/server/gestao";
import type { Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = (evento) =>
  loadDetalhe(evento, Number(evento.params.id));

export const actions: Actions = {
  ...acoesDeConta(),
  cargos: cargosAction,
};
