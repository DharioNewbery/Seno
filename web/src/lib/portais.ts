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
        sub: [
          { rotulo: "Listar turmas", href: "/aluno/turmas" },
          { rotulo: "Professor e colegas", href: "/aluno/turmas/colegas" },
          { rotulo: "Atividades da turma", href: "/aluno/turmas/atividades" },
        ],
      },
      { rotulo: "Atividades pendentes", href: "/aluno/pendentes" },
      {
        rotulo: "Atividades entregues",
        href: "/aluno/entregues",
        sub: [
          { rotulo: "Listar entregas", href: "/aluno/entregues" },
          { rotulo: "Notas e feedback", href: "/aluno/entregues/notas" },
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
          { rotulo: "Painel da turma", href: "/professor/turmas" },
          { rotulo: "Gerenciar alunos", href: "/professor/turmas/alunos" },
          { rotulo: "Atribuir atividade", href: "/professor/turmas/atribuir" },
          {
            rotulo: "Entregas e correções",
            href: "/professor/turmas/entregas",
          },
          { rotulo: "Analytics", href: "/professor/turmas/analytics" },
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
