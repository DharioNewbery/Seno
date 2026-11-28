import { describe, expect, it } from "vitest";
import {
  dataHora,
  deCampoDataHora,
  paraCampoDataHora,
} from "../src/lib/formato";

describe("formato de data/hora para atribuições", () => {
  it("converte ISO UTC para datetime-local em America/Belem (-03:00) e volta sem deslocamento", () => {
    const iso = "2026-11-20T17:00:00.000Z";
    const campo = paraCampoDataHora(iso);
    expect(campo).toBe("2026-11-20T14:00");
    expect(dataHora(iso)).toBe("20/11/2026 14:00");

    const parsed = deCampoDataHora(campo);
    expect(parsed?.toISOString()).toBe(iso);
    expect(paraCampoDataHora(parsed)).toBe("2026-11-20T14:00");
  });

  it("lida com valores vazios ou inválidos", () => {
    expect(paraCampoDataHora(null)).toBe("");
    expect(paraCampoDataHora("")).toBe("");
    expect(deCampoDataHora("")).toBeNull();
    expect(deCampoDataHora("invalido")).toBeNull();
  });
});
