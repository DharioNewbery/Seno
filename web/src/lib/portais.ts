// Definição dos portais e da navegação principal (baseada no PROJETO §Frontend).
// As seções ficam como placeholders até os módulos de domínio ficarem prontos.
import type { PortalId } from "#lib/tipos";

export interface NavSubItem {
  rotulo: string;
  href: string;
}

export interface NavItem {
  rotulo: string;
  href: string;
  /** Subopções expostas em menu dropdown ao clicar. */
  sub?: NavSubItem[];
  /** Visível apenas para super admin (ex.: gestão de admins). */
  soSuper?: boolean;
  /** Frase curta exibida no atalho da página inicial do portal. */
  descricao?: string;
}

export interface PortalDef {
  id: PortalId;
  titulo: string;
  descricao: string;
  nav: NavItem[];
}

export const PORTAIS: Record<PortalId, PortalDef> = {
  aluno: {
    id: "aluno",
    titulo: "Portal do Aluno",
    descricao: "Suas turmas, atividades e correções em um só lugar.",
    nav: [
      { rotulo: "Início", href: "/aluno" },
      {
        rotulo: "Minhas turmas",
        href: "/aluno/turmas",
        descricao: "Suas turmas, com colegas e atividades de cada uma.",
        sub: [{ rotulo: "Listar turmas", href: "/aluno/turmas" }],
      },
      {
        rotulo: "Atividades",
        href: "/aluno/atividades",
        descricao: "Faça as entregas e veja as correções publicadas.",
        sub: [
          { rotulo: "Todas as atividades", href: "/aluno/atividades" },
          { rotulo: "Execuções de teste", href: "/aluno/testes" },
        ],
      },
    ],
  },
  professor: {
    id: "professor",
    titulo: "Portal do Professor",
    descricao: "Turmas, tarefas, atividades e correções automáticas.",
    nav: [
      { rotulo: "Início", href: "/professor" },
      {
        rotulo: "Minhas turmas",
        href: "/professor/turmas",
        descricao: "Painel, alunos e análises de cada turma.",
        sub: [
          { rotulo: "Listar turmas", href: "/professor/turmas" },
          { rotulo: "Criar turma", href: "/professor/turmas/nova" },
        ],
      },
      {
        rotulo: "Banco de tarefas",
        href: "/professor/tarefas",
        descricao: "Enunciados e testes que você reaproveita.",
        sub: [
          { rotulo: "Listar tarefas", href: "/professor/tarefas" },
          { rotulo: "Criar tarefa e testes", href: "/professor/tarefas/nova" },
        ],
      },
      {
        rotulo: "Banco de atividades",
        href: "/professor/atividades",
        descricao: "Conjuntos de tarefas para atribuir às turmas.",
        sub: [
          { rotulo: "Listar atividades", href: "/professor/atividades" },
          { rotulo: "Criar atividade", href: "/professor/atividades/nova" },
        ],
      },
    ],
  },
  admin: {
    id: "admin",
    titulo: "Portal do Admin",
    descricao: "Pessoas, matérias e saúde do sistema.",
    nav: [
      { rotulo: "Início", href: "/admin" },
      {
        rotulo: "Dashboard e métricas",
        href: "/admin/dashboard",
        descricao: "Usuários, correções e fila de execução.",
      },
      {
        rotulo: "Usuários",
        href: "/admin/usuarios",
        descricao: "Contas, cargos e convites.",
        sub: [
          { rotulo: "Listar usuários", href: "/admin/usuarios" },
          { rotulo: "Criar usuário", href: "/admin/usuarios/novo" },
        ],
      },
      {
        rotulo: "Gestão de matérias",
        href: "/admin/materias",
        descricao: "As matérias oferecidas nas turmas.",
        sub: [
          { rotulo: "Listar matérias", href: "/admin/materias" },
          { rotulo: "Criar matéria", href: "/admin/materias/nova" },
        ],
      },
      {
        rotulo: "Períodos e turmas",
        href: "/admin/periodos",
        descricao: "Períodos letivos e todas as turmas.",
        sub: [
          { rotulo: "Períodos letivos", href: "/admin/periodos" },
          { rotulo: "Novo período", href: "/admin/periodos/nova" },
          { rotulo: "Todas as turmas", href: "/admin/turmas" },
        ],
      },
    ],
  },
};

export type IconeNav =
  | "inicio"
  | "turmas"
  | "tarefas"
  | "atividades"
  | "dashboard"
  | "usuarios"
  | "materias"
  | "periodos";

/** Ícone de um item da navegação, pelo trecho da rota depois de `/<portal>`. */
export function iconeDoItem(portal: PortalId, href: string): IconeNav {
  const trecho = href.replace(`/${portal}`, "").split("/").filter(Boolean)[0] ?? "";
  const mapa: Record<string, IconeNav> = {
    turmas: "turmas",
    tarefas: "tarefas",
    atividades: "atividades",
    dashboard: "dashboard",
    usuarios: "usuarios",
    materias: "materias",
    periodos: "periodos",
  };
  return mapa[trecho] ?? "inicio";
}
