export interface SnapshotItem {
  id: number;
  text: string;
}

export interface Snapshot {
  tasks: SnapshotItem[];
}

export interface RascunhoLocal {
  atribuicao_id: number;
  aluno_id: number;
  snapshot: Snapshot;
  revisao_base: number;
  gravado_em: string;
  pendente: boolean;
}

const DB_NAME = "seno_autosave";
const DB_VERSION = 1;
const STORE_NAME = "rascunhos";

export function abrirBanco(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === "undefined") {
      reject(new Error("IndexedDB não disponível no ambiente."));
      return;
    }
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, {
          keyPath: ["atribuicao_id", "aluno_id"],
        });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

export async function gravarRascunhoLocal(
  atribuicaoID: number,
  alunoID: number,
  snapshot: Snapshot,
  revisaoBase: number,
  pendente: boolean = false,
): Promise<RascunhoLocal> {
  const db = await abrirBanco();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readwrite");
    const store = tx.objectStore(STORE_NAME);
    const item: RascunhoLocal = {
      atribuicao_id: atribuicaoID,
      aluno_id: alunoID,
      snapshot,
      revisao_base: revisaoBase,
      gravado_em: new Date().toISOString(),
      pendente,
    };
    const req = store.put(item);
    req.onsuccess = () => resolve(item);
    req.onerror = () => reject(req.error);
  });
}

export async function obterRascunhoLocal(
  atribuicaoID: number,
  alunoID: number,
): Promise<RascunhoLocal | null> {
  const db = await abrirBanco();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readonly");
    const store = tx.objectStore(STORE_NAME);
    const req = store.get([atribuicaoID, alunoID]);
    req.onsuccess = () => resolve(req.result ?? null);
    req.onerror = () => reject(req.error);
  });
}

export async function marcarPendente(
  atribuicaoID: number,
  alunoID: number,
  pendente: boolean,
): Promise<void> {
  const rascunho = await obterRascunhoLocal(atribuicaoID, alunoID);
  if (!rascunho) return;
  await gravarRascunhoLocal(
    atribuicaoID,
    alunoID,
    rascunho.snapshot,
    rascunho.revisao_base,
    pendente,
  );
}

export async function limparRascunhoLocal(
  atribuicaoID: number,
  alunoID: number,
): Promise<void> {
  const db = await abrirBanco();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readwrite");
    const store = tx.objectStore(STORE_NAME);
    const req = store.delete([atribuicaoID, alunoID]);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
}

export interface ResultadoSincronizacao {
  sincronizado: boolean;
  conflito: boolean;
  novaRevisao?: number;
}

export async function sincronizarRascunho(
  atribuicaoID: number,
  alunoID: number,
  enviarFn: (
    revisao: number,
    snapshot: Snapshot,
  ) => Promise<{ ok: boolean; status: number; revisao?: number }>,
): Promise<ResultadoSincronizacao> {
  const rascunho = await obterRascunhoLocal(atribuicaoID, alunoID);
  if (!rascunho || !rascunho.pendente) {
    return { sincronizado: false, conflito: false };
  }

  try {
    const res = await enviarFn(rascunho.revisao_base, rascunho.snapshot);
    if (res.status === 409) {
      // 409 conflito: a revisão no servidor é mais nova. Não sobrescreve.
      return { sincronizado: false, conflito: true };
    }
    if (res.ok) {
      const novaRev = res.revisao ?? rascunho.revisao_base + 1;
      await gravarRascunhoLocal(
        atribuicaoID,
        alunoID,
        rascunho.snapshot,
        novaRev,
        false,
      );
      return { sincronizado: true, conflito: false, novaRevisao: novaRev };
    }
    return { sincronizado: false, conflito: false };
  } catch {
    // Continua offline / erro transitório; mantém pendente
    return { sincronizado: false, conflito: false };
  }
}
