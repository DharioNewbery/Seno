// Fábrica do load/actions das telas de gestão de usuários do portal
// admin — compartilhada entre professores e admins (mesma estrutura,
// cargo diferente; tela de admins exige super — ARQUITETURA §5.4).
import { fail, redirect, type Actions, type Cookies } from "@sveltejs/kit";

import {
  acaoUsuario,
  criarUsuario,
  editarUsuario,
  listarUsuarios,
  mensagemErro,
  type PaginaUsuarios,
} from "#lib/server/usuarios";
import { tokenDaSessao } from "#lib/server/sessao";
import type { Usuario } from "#lib/tipos";

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

export interface PaginaGestao {
  total: number;
  pagina: number;
  por: number;
  usuarios: Usuario[];
  busca: string;
  status: string;
  erro: string | null;
}

export function gestaoDeUsuarios(cargo: "professor" | "admin"): {
  load: (evento: LoadEvento) => Promise<PaginaGestao>;
  actions: Actions;
} {
  const precisaSuper = cargo === "admin";

  const respostaVazia = (
    busca: string,
    status: string,
    erro: string,
  ): PaginaGestao => ({
    ...VAZIA,
    busca,
    status,
    erro,
  });

  const load = async (evento: LoadEvento): Promise<PaginaGestao> => {
    if (!evento.locals.usuario) redirect(303, "/");
    if (precisaSuper && !evento.locals.usuario.roles.includes("super")) {
      redirect(303, "/admin");
    }
    const token = tokenDaSessao(evento.cookies);
    const busca = evento.url.searchParams.get("busca") ?? "";
    const status = evento.url.searchParams.get("status") ?? "";
    const atual = Math.max(
      1,
      Number(evento.url.searchParams.get("pagina")) || 1,
    );
    const filtros = new URLSearchParams({ cargo });
    if (busca) filtros.set("busca", busca);
    if (status) filtros.set("status", status);
    if (atual > 1) filtros.set("pagina", String(atual));
    try {
      const { res, body } = await listarUsuarios(
        evento.request,
        token,
        `?${filtros.toString()}`,
      );
      if (res.status === 401) redirect(303, "/");
      if (!res.ok) return respostaVazia(busca, status, mensagemErro(body));
      const dados = body as PaginaUsuarios;
      return {
        total: dados.total,
        pagina: dados.pagina,
        por: dados.por,
        usuarios: dados.usuarios,
        busca,
        status,
        erro: null,
      };
    } catch {
      return respostaVazia(busca, status, "Sem comunicação com a API.");
    }
  };

  // Guard das ações: devolve id/token ou a falha pronta. O formulário é
  // lido UMA vez pelo chamador e repassado (streams não relêem).
  const idEToken = async (
    form: FormData,
    cookies: Cookies,
  ): Promise<{ id: number; token: string } | { falha: FalhaDaAcao }> => {
    const token = tokenDaSessao(cookies);
    if (!token) return { falha: fail(401, { erro: "Sessão encerrada." }) };
    const id = Number(form.get("id"));
    if (!Number.isInteger(id) || id < 1) {
      return { falha: fail(422, { erro: "Identificador inválido." }) };
    }
    return { id, token };
  };

  /** Ação padrão: POST /v1/users/:id/{rotulo}. */
  function porId(
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

  const actions: Actions = {
    criar: async (evento: AcaoEvento) => {
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
          cargo,
        });
      } catch {
        return fail(502, { erro: "Sem comunicação com a API." });
      }
      if (!res.ok) {
        return fail(res.status, { erro: mensagemErro(await corpoJSON(res)) });
      }
      return {
        ok: `Convite enviado para ${email} — pendente até definir a senha.`,
      };
    },

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

    bloquear: porId(
      "desativar",
      "Conta bloqueada — sessões encerradas na hora.",
    ),
    desbloquear: porId("ativar", "Conta reativada."),
    resetar: porId(
      "resetar-senha",
      "Link de redefinição enviado (se o usuário existir).",
    ),
  };

  return { load, actions };
}
