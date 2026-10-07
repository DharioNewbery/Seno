// Camada server-side das telas de gestão de usuários do portal admin
// (listar/criar/inspecionar — ARQUITETURA §5.4). Os cargos NÃO são
// atribuídos na criação: nascem na inspeção, via PUT /cargos.
import { fail, redirect, type Actions, type Cookies } from "@sveltejs/kit";

import {
  acaoUsuario,
  alterarCargos,
  criarUsuario,
  editarUsuario,
  listarUsuarios,
  mensagemErro,
  obterUsuario,
  type PaginaUsuarios,
} from "#lib/server/usuarios";
import { tokenDaSessao } from "#lib/server/sessao";
import type { Cargo, Usuario } from "#lib/tipos";

// Subset estrutural dos eventos de load/action das páginas de gestão
// (os eventos reais do SvelteKit têm superconjunto destas propriedades).
interface LoadEvento {
  locals: { usuario: Usuario | null };
  cookies: Cookies;
  request: Request;
  url: URL;
}

interface AcaoEvento {
  cookies: Cookies;
  request: Request;
}

const VAZIA: PaginaUsuarios = { total: 0, pagina: 1, por: 50, usuarios: [] };

type FalhaDaAcao = ReturnType<typeof fail>;

/** Lê JSON com tolerância (corpo inválido → null). */
async function corpoJSON(res: Response): Promise<unknown> {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

/** Guard de acesso: só admins (e o próprio super) usam a gestão. */
function guardaStaff(evento: LoadEvento): Usuario {
  const usuario = evento.locals.usuario;
  if (!usuario) redirect(303, "/");
  const éStaff = usuario.roles.includes("admin") || usuario.roles.includes("super");
  if (!éStaff) redirect(303, "/");
  return usuario;
}

export interface PaginaGestao {
  total: number;
  pagina: number;
  por: number;
  usuarios: Usuario[];
  busca: string;
  status: string;
  cargo: string;
  erro: string | null;
}

/** Load da listagem: GET /v1/users com busca, estado e cargo. */
export async function loadLista(evento: LoadEvento): Promise<PaginaGestao> {
  guardaStaff(evento);
  const token = tokenDaSessao(evento.cookies);
  const busca = evento.url.searchParams.get("busca") ?? "";
  const status = evento.url.searchParams.get("status") ?? "";
  const cargo = evento.url.searchParams.get("cargo") ?? "";
  const atual = Math.max(1, Number(evento.url.searchParams.get("pagina")) || 1);
  const filtros = new URLSearchParams();
  if (busca) filtros.set("busca", busca);
  if (status) filtros.set("status", status);
  if (cargo) filtros.set("cargo", cargo);
  if (atual > 1) filtros.set("pagina", String(atual));
  try {
    const { res, body } = await listarUsuarios(
      evento.request,
      token,
      `?${filtros.toString()}`,
    );
    if (res.status === 401) redirect(303, "/");
    if (!res.ok) {
      return respostaVazia(busca, status, cargo, mensagemErro(body));
    }
    const dados = body as PaginaUsuarios;
    return {
      total: dados.total,
      pagina: dados.pagina,
      por: dados.por,
      usuarios: dados.usuarios,
      busca,
      status,
      cargo,
      erro: null,
    };
  } catch {
    return respostaVazia(busca, status, cargo, "Sem comunicação com a API.");
  }
}

function respostaVazia(
  busca: string,
  status: string,
  cargo: string,
  erro: string,
): PaginaGestao {
  return { ...VAZIA, busca, status, cargo, erro };
}

/** Load da inspeção: GET /v1/users/:id; 403/404 voltam à listagem. */
export async function loadDetalhe(
  evento: LoadEvento,
  id: number,
): Promise<{ alvo: Usuario }> {
  guardaStaff(evento);
  const token = tokenDaSessao(evento.cookies);
  let res: Response;
  let body: unknown;
  try {
    ({ res, body } = await obterUsuario(evento.request, token, id));
  } catch {
    redirect(303, "/admin/usuarios");
  }
  if (res.status === 401) redirect(303, "/");
  if (!res.ok) redirect(303, "/admin/usuarios");
  return { alvo: body as Usuario };
}

/** Guard de guarda + id/token das ações: falha pronta em caso de erro. */
async function idEToken(
  form: FormData,
  cookies: Cookies,
): Promise<{ id: number; token: string } | { falha: FalhaDaAcao }> {
  const token = tokenDaSessao(cookies);
  if (!token) return { falha: fail(401, { erro: "Sessão encerrada." }) };
  const id = Number(form.get("id"));
  if (!Number.isInteger(id) || id < 1) {
    return { falha: fail(422, { erro: "Identificador inválido." }) };
  }
  return { id, token };
}

/** Ação padrão: POST /v1/users/:id/{rotulo}. */
function acaoPorId(
  rotulo: "desativar" | "ativar" | "resetar-senha",
  mensagemOk: string,
): (evento: AcaoEvento) => Promise<{ ok: string } | FalhaDaAcao> {
  return async (evento: AcaoEvento) => {
    const form = await evento.request.formData();
    const pronto = await idEToken(form, evento.cookies);
    if ("falha" in pronto) return pronto.falha;
    let res: Response;
    try {
      res = await acaoUsuario(
        evento.request,
        pronto.token,
        pronto.id,
        rotulo,
      );
    } catch {
      return fail(502, { erro: "Sem comunicação com a API." });
    }
    if (!res.ok) {
      return fail(res.status, { erro: mensagemErro(await corpoJSON(res)) });
    }
    return { ok: mensagemOk };
  };
}

/** Actions compartilhadas de conta (listagem e inspeção). */
export function acoesDeConta(): Actions {
  return {
    editar: async (evento: AcaoEvento) => {
      const form = await evento.request.formData();
      const pronto = await idEToken(form, evento.cookies);
      if ("falha" in pronto) return pronto.falha;
      const first_name = String(form.get("first_name") ?? "").trim();
      const last_name = String(form.get("last_name") ?? "").trim();
      if (!first_name || !last_name) {
        return fail(422, { erro: "Nome e sobrenome são obrigatórios." });
      }
      let res: Response;
      try {
        res = await editarUsuario(evento.request, pronto.token, pronto.id, {
          first_name,
          last_name,
        });
      } catch {
        return fail(502, { erro: "Sem comunicação com a API." });
      }
      if (!res.ok) {
        return fail(res.status, { erro: mensagemErro(await corpoJSON(res)) });
      }
      return { ok: "Dados atualizados." };
    },
    bloquear: acaoPorId(
      "desativar",
      "Conta bloqueada — sessões encerradas na hora.",
    ),
    desbloquear: acaoPorId("ativar", "Conta reativada."),
    resetar: acaoPorId(
      "resetar-senha",
      "Link de redefinição enviado (se o usuário existir).",
    ),
  };
}

/**
 * Ação "cargos": substitui o conjunto de cargos do usuário (checkboxes da
 * inspeção). A API garante as regras de quem pode atribuir o quê.
 */
export const cargosAction = async (evento: AcaoEvento) => {
  const form = await evento.request.formData();
  const pronto = await idEToken(form, evento.cookies);
  if ("falha" in pronto) return pronto.falha;
  const rotulados: Cargo[] = ["student", "professor", "admin"];
  const cargos = rotulados.filter((cargo) => form.has(`cargo_${cargo}`));
  let res: Response;
  try {
    res = await alterarCargos(
      evento.request,
      pronto.token,
      pronto.id,
      cargos,
    );
  } catch {
    return fail(502, { erro: "Sem comunicação com a API." });
  }
  if (!res.ok) {
    return fail(res.status, { erro: mensagemErro(await corpoJSON(res)) });
  }
  return { ok: "Cargos atualizados." };
};

/** Ação "criar": o usuário nasce pendente e SEM cargos. */
export const criarAction = async (evento: AcaoEvento) => {
  const form = await evento.request.formData();
  const token = tokenDaSessao(evento.cookies);
  if (!token) return fail(401, { erro: "Sessão encerrada." });

  const email = String(form.get("email") ?? "").trim();
  const first_name = String(form.get("first_name") ?? "").trim();
  const last_name = String(form.get("last_name") ?? "").trim();
  if (!email || !first_name || !last_name) {
    return fail(422, { erro: "Informe e-mail, nome e sobrenome." });
  }
  let res: Response;
  try {
    res = await criarUsuario(evento.request, token, {
      email,
      first_name,
      last_name,
    });
  } catch {
    return fail(502, { erro: "Sem comunicação com a API." });
  }
  if (!res.ok) {
    return fail(res.status, { erro: mensagemErro(await corpoJSON(res)) });
  }
  redirect(303, "/admin/usuarios");
};
