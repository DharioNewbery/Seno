// Identidade do sistema e versões exibidas na bottom bar dos portais.
export const NOME_SISTEMA = "Seno";
export const DESCRICAO_SISTEMA =
  "Envio e correção automática de provas de programação — UFPA.";

export const APP_VERSION = "0.1.0";
export const API_VERSION = "0.1.0";

import { dev } from "$app/env";

/** Indicador de modo DEV (bottom bar), ativo no build de desenvolvimento. */
export const modoDev = dev;
