import { describe, it, expect, beforeEach } from "vitest";
import "fake-indexeddb/auto";
import {
  gravarRascunhoLocal,
  obterRascunhoLocal,
  sincronizarRascunho,
  limparRascunhoLocal,
} from "../src/lib/autosave";

describe("autosave local (IndexedDB)", () => {
  const atribuicaoID = 101;
  const alunoID = 42;
  const snapshotExemplo = {
    tasks: [{ id: 1, text: "print('olá mundo')" }],
  };

  beforeEach(async () => {
    await limparRascunhoLocal(atribuicaoID, alunoID);
  });

  it("grava e recupera rascunho localmente", async () => {
    await gravarRascunhoLocal(atribuicaoID, alunoID, snapshotExemplo, 1, false);

    const r = await obterRascunhoLocal(atribuicaoID, alunoID);
    expect(r).not.toBeNull();
    expect(r?.atribuicao_id).toBe(atribuicaoID);
    expect(r?.aluno_id).toBe(alunoID);
    expect(r?.revisao_base).toBe(1);
    expect(r?.pendente).toBe(false);
    expect(r?.snapshot.tasks[0]?.text).toBe("print('olá mundo')");
  });

  it("sincroniza rascunho pendente com o servidor com sucesso", async () => {
    await gravarRascunhoLocal(atribuicaoID, alunoID, snapshotExemplo, 2, true);

    let chamouComRev = -1;
    const resultado = await sincronizarRascunho(
      atribuicaoID,
      alunoID,
      async (revisao, snapshot) => {
        chamouComRev = revisao;
        expect(snapshot.tasks[0]?.text).toBe("print('olá mundo')");
        return { ok: true, status: 200, revisao: 3 };
      },
    );

    expect(resultado.sincronizado).toBe(true);
    expect(resultado.conflito).toBe(false);
    expect(resultado.novaRevisao).toBe(3);
    expect(chamouComRev).toBe(2);

    const r = await obterRascunhoLocal(atribuicaoID, alunoID);
    expect(r?.pendente).toBe(false);
    expect(r?.revisao_base).toBe(3);
  });

  it("detecta conflito 409 quando o servidor já possui revisão mais recente", async () => {
    await gravarRascunhoLocal(atribuicaoID, alunoID, snapshotExemplo, 1, true);

    const resultado = await sincronizarRascunho(
      atribuicaoID,
      alunoID,
      async () => {
        return { ok: false, status: 409 };
      },
    );

    expect(resultado.sincronizado).toBe(false);
    expect(resultado.conflito).toBe(true);

    // O rascunho deve continuar preservado e marcado como pendente
    const r = await obterRascunhoLocal(atribuicaoID, alunoID);
    expect(r?.pendente).toBe(true);
    expect(r?.revisao_base).toBe(1);
  });

  it("mantém pendente quando ocorre falha de conexão/offline", async () => {
    await gravarRascunhoLocal(atribuicaoID, alunoID, snapshotExemplo, 1, true);

    const resultado = await sincronizarRascunho(
      atribuicaoID,
      alunoID,
      async () => {
        throw new Error("Failed to fetch (offline)");
      },
    );

    expect(resultado.sincronizado).toBe(false);
    expect(resultado.conflito).toBe(false);

    const r = await obterRascunhoLocal(atribuicaoID, alunoID);
    expect(r?.pendente).toBe(true);
  });
});
