import { describe, it, expect } from "vitest";
import {
  adicionarAoWorkspace,
  removerDoWorkspace,
  moverWorkspaceParaAtividade,
  removerDaAtividade,
  reordenarAtividade,
  serializarAtributos,
  filtrarTarefas,
  paginar,
  type AtributoAtividade,
  type TarefaSimples,
} from "../src/lib/atividade-workspace";

describe("Lógica do Workspace de Tarefas e Drag-and-Drop", () => {
  describe("adicionarAoWorkspace", () => {
    it("adiciona tarefas ao workspace ignorando duplicatas existentes no workspace ou atividade", () => {
      const workspace = [1, 2];
      const novosIds = [2, 3, 4, 5];
      const atividadeIds = [5, 6];

      const resultado = adicionarAoWorkspace(workspace, novosIds, atividadeIds);
      // 2 já está no workspace; 5 já está na atividade. Devem entrar 3 e 4.
      expect(resultado).toEqual([1, 2, 3, 4]);
    });

    it("mantém workspace intacto se novosIds estiver vazio", () => {
      const resultado = adicionarAoWorkspace([1, 2], [], []);
      expect(resultado).toEqual([1, 2]);
    });
  });

  describe("removerDoWorkspace", () => {
    it("remove a tarefa indicada pelo id", () => {
      const resultado = removerDoWorkspace([10, 20, 30], 20);
      expect(resultado).toEqual([10, 30]);
    });

    it("não altera se o id não estiver no workspace", () => {
      const resultado = removerDoWorkspace([10, 30], 99);
      expect(resultado).toEqual([10, 30]);
    });
  });

  describe("moverWorkspaceParaAtividade", () => {
    it("move tarefa do workspace para o início da atividade como linha clicável (não expandida por padrão)", () => {
      const ws = [1, 2, 3];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 10, valor_pts: 10, linguagem: "python" },
      ];

      const res = moverWorkspaceParaAtividade(ws, atv, 2, 0);
      expect(res.workspace).toEqual([1, 3]);
      expect(res.atributos).toHaveLength(2);
      expect(res.atributos[0]).toEqual({
        tarefa_id: 2,
        valor_pts: 10,
        linguagem: "python",
        expandida: false,
      });
      expect(res.atributos[1]?.tarefa_id).toBe(10);
    });

    it("posiciona em qualquer posição intermediária ou final e suporta opções customizadas", () => {
      const ws = [5];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
      ];

      const res = moverWorkspaceParaAtividade(ws, atv, 5, 1, {
        valor_pts: 25,
        linguagem: "cpp",
        expandida: true,
      });
      expect(res.workspace).toEqual([]);
      expect(res.atributos.map((a) => a.tarefa_id)).toEqual([1, 5, 2]);
      expect(res.atributos[1]).toEqual({
        tarefa_id: 5,
        valor_pts: 25,
        linguagem: "cpp",
        expandida: true,
      });
    });

    it("evita duplicatas se a tarefa já estiver na atividade", () => {
      const ws = [10, 20];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 10, valor_pts: 15, linguagem: "python" },
      ];

      const res = moverWorkspaceParaAtividade(ws, atv, 10, 1);
      // Remove do workspace e não duplica na atividade
      expect(res.workspace).toEqual([20]);
      expect(res.atributos).toEqual(atv);
    });

    it("ajusta índices fora dos limites para 0 ou final da lista", () => {
      const res = moverWorkspaceParaAtividade([1], [], 1, 99);
      expect(res.atributos).toHaveLength(1);
      expect(res.atributos[0]?.tarefa_id).toBe(1);
    });
  });

  describe("removerDaAtividade", () => {
    it("remove tarefa da atividade e a devolve para o workspace", () => {
      const ws = [1];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 3, valor_pts: 15, linguagem: "c" },
      ];

      const res = removerDaAtividade(atv, 0, ws);
      expect(res.atributos.map((a) => a.tarefa_id)).toEqual([3]);
      expect(res.workspace).toEqual([1, 2]);
    });

    it("não duplica id se por algum motivo já estiver no workspace", () => {
      const ws = [2];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
      ];

      const res = removerDaAtividade(atv, 0, ws);
      expect(res.atributos).toEqual([]);
      expect(res.workspace).toEqual([2]);
    });

    it("não faz nada se o índice for inválido", () => {
      const ws = [1];
      const atv: AtributoAtividade[] = [
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
      ];

      const res = removerDaAtividade(atv, 5, ws);
      expect(res.atributos).toEqual(atv);
      expect(res.workspace).toEqual(ws);
    });
  });

  describe("reordenarAtividade", () => {
    it("move item do início para uma posição posterior", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 3, valor_pts: 10, linguagem: "python" },
      ];

      // Mover de index 0 para slot após index 1 (target 2) -> [2, 1, 3]
      const res = reordenarAtividade(atv, 0, 2);
      expect(res.map((a) => a.tarefa_id)).toEqual([2, 1, 3]);
    });

    it("move item do final para o início", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 3, valor_pts: 10, linguagem: "python" },
      ];

      const res = reordenarAtividade(atv, 2, 0);
      expect(res.map((a) => a.tarefa_id)).toEqual([3, 1, 2]);
    });

    it("move item para o final", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 3, valor_pts: 10, linguagem: "python" },
      ];

      const res = reordenarAtividade(atv, 0, 3);
      expect(res.map((a) => a.tarefa_id)).toEqual([2, 3, 1]);
    });

    it("retorna mesma lista se fromIndex for igual ao targetIndex", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 10, linguagem: "python" },
      ];

      const res = reordenarAtividade(atv, 1, 1);
      expect(res).toBe(atv);
    });
  });

  describe("serializarAtributos", () => {
    it("produz JSON limpo com tarefa_id, valor_pts e linguagem", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 1, valor_pts: 10, linguagem: "python", expandida: true },
        { tarefa_id: 2, valor_pts: 20, linguagem: "c", expandida: false },
      ];

      const json = serializarAtributos(atv);
      expect(JSON.parse(json)).toEqual([
        { tarefa_id: 1, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 2, valor_pts: 20, linguagem: "c" },
      ]);
    });

    it("filtra itens sem tarefa_id válido", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 0, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 5, valor_pts: 15, linguagem: "cpp" },
      ];

      const json = serializarAtributos(atv);
      expect(JSON.parse(json)).toEqual([
        { tarefa_id: 5, valor_pts: 15, linguagem: "cpp" },
      ]);
    });

    it("saneia pontuação negativa, nula ou inválida e linguagem desconhecida", () => {
      const atv: AtributoAtividade[] = [
        { tarefa_id: 8, valor_pts: -5, linguagem: "invalid" as string },
        { tarefa_id: 9, valor_pts: NaN, linguagem: "c" },
      ];

      const json = serializarAtributos(atv);
      expect(JSON.parse(json)).toEqual([
        { tarefa_id: 8, valor_pts: 10, linguagem: "python" },
        { tarefa_id: 9, valor_pts: 10, linguagem: "c" },
      ]);
    });
  });

  describe("filtrarTarefas e paginar", () => {
    const lista: TarefaSimples[] = [
      { id: 1, nome: "Soma Simples" },
      { id: 2, nome: "Média Ponderada" },
      { id: 3, nome: "Fatorial" },
      { id: 4, nome: "Ordenação Bubble Sort" },
      { id: 12, nome: "Fibonacci Recursivo" },
    ];

    it("filtra por nome ignorando maiúsculas/minúsculas", () => {
      const filtradas = filtrarTarefas(lista, "soma");
      expect(filtradas.map((t) => t.id)).toEqual([1]);
    });

    it("filtra por ID", () => {
      const filtradas = filtrarTarefas(lista, "12");
      expect(filtradas.map((t) => t.id)).toEqual([12]);
    });

    it("retorna tudo se a busca for vazia", () => {
      const filtradas = filtrarTarefas(lista, "   ");
      expect(filtradas).toHaveLength(5);
    });

    it("pagina corretamente a lista de itens", () => {
      const p1 = paginar(lista, 1, 2);
      expect(p1.paginaAtual).toBe(1);
      expect(p1.totalPaginas).toBe(3);
      expect(p1.itens.map((t) => t.id)).toEqual([1, 2]);

      const p2 = paginar(lista, 2, 2);
      expect(p2.paginaAtual).toBe(2);
      expect(p2.itens.map((t) => t.id)).toEqual([3, 4]);

      const p3 = paginar(lista, 3, 2);
      expect(p3.paginaAtual).toBe(3);
      expect(p3.itens.map((t) => t.id)).toEqual([12]);
    });

    it("ajusta página fora do intervalo", () => {
      const p = paginar(lista, 99, 2);
      expect(p.paginaAtual).toBe(3);
      expect(p.itens.map((t) => t.id)).toEqual([12]);
    });
  });
});
