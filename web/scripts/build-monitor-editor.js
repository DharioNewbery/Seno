import { build } from "vite";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

async function run() {
  await build({
    build: {
      lib: {
        entry: path.resolve(__dirname, "../src/lib/monitor-editor-entry.ts"),
        name: "SenoEditorBundle",
        formats: ["iife"],
        fileName: () => "editor.js",
      },
      outDir: path.resolve(__dirname, "../../monitor/internal/httpapi/assets"),
      emptyOutDir: false,
      minify: true,
    },
    esbuild: {
      tsconfigRaw: {
        compilerOptions: {
          target: "es2020",
          module: "esnext",
        },
      },
    },
    configFile: false,
  });
  console.log("Monitor editor bundled successfully to monitor/internal/httpapi/assets/editor.js");
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
