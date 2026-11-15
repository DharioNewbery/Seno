import { describe, it, expect } from "vitest";
import { extrairLimite } from "#lib/server/ensino";
import { formatarBytes } from "#lib/formato";
import {
  adicionarTesteNaLista,
  ajustarIndiceTeste,
  atualizarTesteNaLista,
  navegarIndiceTeste,
  removerTesteDaLista,
  type TesteFormulario,
} from "#lib/tarefas-testes";

describe("Criação de nova tarefa - extração de limites padrão", () => {
  it("usa valores padrão quando os campos não são enviados (null no FormData)", () => {
    const fd = new FormData();
    fd.set("nome", "Tarefa 1");
    fd.set("enunciado", "Enunciado teste");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBe(2000);
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBe(5000);
    expect(extrairLimite(fd, "memoria_mb", 256)).toBe(256);
  });

  it("utiliza os valores submetidos pelo formulário padrão", () => {
    const fd = new FormData();
    fd.set("nome", "Tarefa 1");
    fd.set("enunciado", "Enunciado teste");
    fd.set("tempo_cpu_ms", "2000");
    fd.set("tempo_total_ms", "5000");
    fd.set("memoria_mb", "256");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBe(2000);
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBe(5000);
    expect(extrairLimite(fd, "memoria_mb", 256)).toBe(256);
  });

  it("aceita personalizações nas opções avançadas", () => {
    const fd = new FormData();
    fd.set("nome", "Tarefa Personalizada");
    fd.set("enunciado", "Enunciado teste");
    fd.set("tempo_cpu_ms", "3500");
    fd.set("tempo_total_ms", "8000");
    fd.set("memoria_mb", "512");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBe(3500);
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBe(8000);
    expect(extrairLimite(fd, "memoria_mb", 256)).toBe(512);
  });

  it("permite limpar explicitamente para usar padrão global (undefined)", () => {
    const fd = new FormData();
    fd.set("nome", "Tarefa Sem Limites");
    fd.set("enunciado", "Enunciado teste");
    fd.set("tempo_cpu_ms", "");
    fd.set("tempo_total_ms", "   ");
    fd.set("memoria_mb", "");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBeUndefined();
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBeUndefined();
    expect(extrairLimite(fd, "memoria_mb", 256)).toBeUndefined();
  });

  it("retorna undefined para zero ou valores negativos (padrão global)", () => {
    const fd = new FormData();
    fd.set("tempo_cpu_ms", "0");
    fd.set("tempo_total_ms", "-100");
    fd.set("memoria_mb", "-1");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBeUndefined();
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBeUndefined();
    expect(extrairLimite(fd, "memoria_mb", 256)).toBeUndefined();
  });

  it("retorna undefined para entradas não numéricas", () => {
    const fd = new FormData();
    fd.set("tempo_cpu_ms", "invalido");
    fd.set("tempo_total_ms", "NaN");
    fd.set("memoria_mb", "abc");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBeUndefined();
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBeUndefined();
    expect(extrairLimite(fd, "memoria_mb", 256)).toBeUndefined();
  });

  it("arredonda para baixo valores decimais", () => {
    const fd = new FormData();
    fd.set("tempo_cpu_ms", "1999.9");
    fd.set("memoria_mb", "256.4");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBe(1999);
    expect(extrairLimite(fd, "memoria_mb", 256)).toBe(256);
  });

  it("retorna undefined quando valor decimal positivo é menor que 1 (arredonda para 0)", () => {
    const fd = new FormData();
    fd.set("tempo_cpu_ms", "0.5");
    fd.set("tempo_total_ms", "0.99");

    expect(extrairLimite(fd, "tempo_cpu_ms", 2000)).toBeUndefined();
    expect(extrairLimite(fd, "tempo_total_ms", 5000)).toBeUndefined();
  });

  it("formata bytes de arquivos submetidos corretamente (B, KB, MB)", () => {
    expect(formatarBytes(0)).toBe("0 B");
    expect(formatarBytes(512)).toBe("512 B");
    expect(formatarBytes(2048)).toBe("2.0 KB");
    expect(formatarBytes(10 * 1024 * 1024)).toBe("10.0 MB");
  });
});

describe("Visualização unitária e edição de testes (tarefas-testes)", () => {
  const base: TesteFormulario[] = [
    { stdin: "1\n2", stdout: "3", publico: true },
    { stdin: "5\n5", stdout: "10", publico: false },
    { stdin: "10\n20", stdout: "30", publico: true },
  ];

  it("navega com setas esquerda/direita respeitando os limites", () => {
    expect(navegarIndiceTeste(0, -1, 3)).toBe(0);
    expect(navegarIndiceTeste(0, 1, 3)).toBe(1);
    expect(navegarIndiceTeste(1, 1, 3)).toBe(2);
    expect(navegarIndiceTeste(2, 1, 3)).toBe(2);
    expect(navegarIndiceTeste(2, -1, 3)).toBe(1);
    expect(ajustarIndiceTeste(5, 0)).toBe(0);
  });

  it("adiciona teste ao final e posiciona o visor no novo teste", () => {
    const res = adicionarTesteNaLista(base, {
      stdin: "7\n8",
      stdout: "15",
      publico: false,
    });
    expect(res.lista).toHaveLength(4);
    expect(res.indice).toBe(3);
    expect(res.lista[3]).toEqual({
      stdin: "7\n8",
      stdout: "15",
      publico: false,
    });
  });

  it("edita um teste existente mantendo o índice selecionado", () => {
    const res = atualizarTesteNaLista(base, 1, {
      stdin: "6\n6",
      stdout: "12",
      publico: true,
    });
    expect(res.lista).toHaveLength(3);
    expect(res.indice).toBe(1);
    expect(res.lista[1]).toEqual({
      stdin: "6\n6",
      stdout: "12",
      publico: true,
    });
  });

  it("remove um teste e reajusta o índice caso o último seja removido", () => {
    const resUltimo = removerTesteDaLista(base, 2);
    expect(resUltimo.lista).toHaveLength(2);
    expect(resUltimo.indice).toBe(1);

    const resMeio = removerTesteDaLista(base, 0);
    expect(resMeio.lista).toHaveLength(2);
    expect(resMeio.indice).toBe(0);
    expect(resMeio.lista[0]?.stdout).toBe("10");
  });
});
