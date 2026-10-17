import { criarEditor } from "./components/editor-core";

declare global {
  interface Window {
    SenoEditor: {
      criar: typeof criarEditor;
    };
  }
}

window.SenoEditor = {
  criar: criarEditor,
};
