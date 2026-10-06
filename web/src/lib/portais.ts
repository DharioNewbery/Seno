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
        sub: [{ rotulo: "Listar turmas", href: "/aluno/turmas" }],
      },
      {
        rotulo: "Atividades",
        href: "/aluno/atividades",
        sub: [
          { rotulo: "Todas as atividades", href: "/aluno/atividades" },
          { rotulo: "Continuar em andamento", href: "/aluno/atividades" },
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
        sub: [
          { rotulo: "Listar turmas", href: "/professor/turmas" },
          { rotulo: "Criar turma", href: "/professor/turmas/nova" },
        ],
      },
      {
        rotulo: "Banco de tarefas",
        href: "/professor/tarefas",
        sub: [
          { rotulo: "Listar tarefas", href: "/professor/tarefas" },
          { rotulo: "Criar tarefa e testes", href: "/professor/tarefas/nova" },
        ],
      },
      {
        rotulo: "Banco de atividades",
        href: "/professor/atividades",
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
      { rotulo: "Dashboard e métricas", href: "/admin/dashboard" },
      {
        rotulo: "Gestão de professores",
        href: "/admin/professores",
        sub: [
          { rotulo: "Listar professores", href: "/admin/professores" },
          { rotulo: "Criar professor", href: "/admin/professores/novo" },
        ],
      },
      {
        rotulo: "Gestão de alunos",
        href: "/admin/alunos",
        sub: [
          { rotulo: "Listar alunos", href: "/admin/alunos" },
          { rotulo: "Criar aluno", href: "/admin/alunos/novo" },
        ],
      },
      {
        rotulo: "Gestão de matérias",
        href: "/admin/materias",
        sub: [
          { rotulo: "Listar matérias", href: "/admin/materias" },
          { rotulo: "Criar matéria", href: "/admin/materias/nova" },
        ],
      },
      {
        rotulo: "Períodos e turmas",
        href: "/admin/periodos",
        sub: [
          { rotulo: "Períodos letivos", href: "/admin/periodos" },
          { rotulo: "Novo período", href: "/admin/periodos/nova" },
          { rotulo: "Todas as turmas", href: "/admin/turmas" },
        ],
      },
      {
        rotulo: "Gestão de admins",
        href: "/admin/admins",
        soSuper: true,
        sub: [
          { rotulo: "Listar admins", href: "/admin/admins" },
          { rotulo: "Criar admin", href: "/admin/admins/novo" },
        ],
      },
    ],
  },
};
