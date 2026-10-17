import {
  EditorView,
  lineNumbers,
  highlightActiveLineGutter,
  keymap,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  syntaxHighlighting,
  defaultHighlightStyle,
  bracketMatching,
} from "@codemirror/language";
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
} from "@codemirror/autocomplete";
import { python } from "@codemirror/lang-python";
import { cpp } from "@codemirror/lang-cpp";

export interface OpcoesEditor {
  valor?: string;
  linguagem?: string;
  autocomplete?: boolean;
  onchange?: (valor: string) => void;
}

export interface InstanciaEditor {
  getValor: () => string;
  setValor: (novoValor: string) => void;
  destruir: () => void;
}

export function criarEditor(
  parent: HTMLElement,
  opts: OpcoesEditor = {},
): InstanciaEditor {
  const extensions = [
    lineNumbers(),
    highlightActiveLineGutter(),
    history(),
    bracketMatching(),
    closeBrackets(),
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    keymap.of([...closeBracketsKeymap, ...defaultKeymap, ...historyKeymap]),
    EditorView.theme({
      "&": {
        minHeight: "180px",
        height: "100%",
        fontSize: "0.9rem",
      },
      ".cm-scroller": {
        overflow: "auto",
        fontFamily:
          'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
        lineHeight: "1.5",
      },
      ".cm-content": {
        padding: "8px 0",
      },
      ".cm-gutters": {
        backgroundColor: "#f8f9fa",
        color: "#6c757d",
        borderRight: "1px solid #dee2e6",
      },
    }),
  ];

  if (opts.autocomplete) {
    extensions.push(autocompletion(), keymap.of(completionKeymap));
  }

  const lang = (opts.linguagem || "").trim().toLowerCase();
  if (lang === "python" || lang === "py") {
    extensions.push(python());
  } else if (lang === "c" || lang === "cpp" || lang === "c++") {
    extensions.push(cpp());
  }

  if (opts.onchange) {
    const callback = opts.onchange;
    extensions.push(
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          callback(update.state.doc.toString());
        }
      }),
    );
  }

  const view = new EditorView({
    doc: opts.valor ?? "",
    extensions,
    parent,
  });

  return {
    getValor: () => view.state.doc.toString(),
    setValor: (novoValor: string) => {
      const atual = view.state.doc.toString();
      if (atual !== novoValor) {
        view.dispatch({
          changes: { from: 0, to: atual.length, insert: novoValor },
        });
      }
    },
    destruir: () => {
      view.destroy();
    },
  };
}
