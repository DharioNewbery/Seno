import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Testa que as cores do tema (branco + tons de azul) estão declaradas no CSS
// global — é o contrato visual mínimo do esqueleto de UI.
const css = readFileSync(
  fileURLToPath(new URL("../styles/global.css", import.meta.url)),
  "utf8",
);

describe("tema da fundação", () => {
  it("usa branco como tema principal", () => {
    expect(css).toContain("--seno-white: #ffffff");
  });

  it("declara a escala de azul", () => {
    for (const tone of [100, 200, 300, 400, 500, 600, 700, 800, 900]) {
      expect(css).toContain(`--seno-blue-${tone}`);
    }
  });
});
