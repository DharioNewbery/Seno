import { describe, it, expect } from "vitest";
import { extrairLimite } from "#lib/server/ensino";

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
});
