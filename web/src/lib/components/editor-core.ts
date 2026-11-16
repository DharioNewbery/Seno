import {
  EditorView,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
} from "@codemirror/view";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import {
  syntaxHighlighting,
  defaultHighlightStyle,
  bracketMatching,
  indentOnInput,
} from "@codemirror/language";
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completeAnyWord,
  completeFromList,
  completionKeymap,
  ifNotIn,
  type Completion,
} from "@codemirror/autocomplete";
import { python } from "@codemirror/lang-python";
import { cpp, cppLanguage } from "@codemirror/lang-cpp";

export interface OpcoesEditor {
  valor?: string;
  linguagem?: string;
  autocomplete?: boolean;
  somenteLeitura?: boolean;
  onchange?: (valor: string) => void;
}

export interface InstanciaEditor {
  getValor: () => string;
  setValor: (novoValor: string) => void;
  destruir: () => void;
}

const CPP_COMPLETIONS: Completion[] = [
  // Palavras-chave C/C++
  ...[
    "auto",
    "break",
    "case",
    "char",
    "const",
    "continue",
    "default",
    "do",
    "double",
    "else",
    "enum",
    "extern",
    "float",
    "for",
    "if",
    "inline",
    "int",
    "long",
    "namespace",
    "nullptr",
    "operator",
    "private",
    "protected",
    "public",
    "return",
    "short",
    "signed",
    "sizeof",
    "static",
    "struct",
    "switch",
    "template",
    "this",
    "throw",
    "true",
    "false",
    "try",
    "catch",
    "typedef",
    "typename",
    "unsigned",
    "using",
    "virtual",
    "void",
    "volatile",
    "while",
    "bool",
    "class",
    "new",
    "delete",
  ].map((label) => ({ label, type: "keyword" })),
  // Biblioteca padrão C/C++ comum em provas
  ...[
    { label: "printf", type: "function", info: "stdio.h" },
    { label: "scanf", type: "function", info: "stdio.h" },
    { label: "fprintf", type: "function", info: "stdio.h" },
    { label: "fgets", type: "function", info: "stdio.h" },
    { label: "malloc", type: "function", info: "stdlib.h" },
    { label: "calloc", type: "function", info: "stdlib.h" },
    { label: "realloc", type: "function", info: "stdlib.h" },
    { label: "free", type: "function", info: "stdlib.h" },
    { label: "strlen", type: "function", info: "string.h" },
    { label: "strcpy", type: "function", info: "string.h" },
    { label: "strcmp", type: "function", info: "string.h" },
    { label: "memcpy", type: "function", info: "string.h" },
    { label: "memset", type: "function", info: "string.h" },
    { label: "std", type: "namespace" },
    { label: "cout", type: "variable", info: "iostream" },
    { label: "cin", type: "variable", info: "iostream" },
    { label: "endl", type: "variable", info: "iostream" },
    { label: "string", type: "type", info: "string" },
    { label: "vector", type: "type", info: "vector" },
    { label: "map", type: "type", info: "map" },
    { label: "unordered_map", type: "type", info: "unordered_map" },
    { label: "set", type: "type", info: "set" },
    { label: "pair", type: "type", info: "utility" },
    { label: "sort", type: "function", info: "algorithm" },
    { label: "max", type: "function", info: "algorithm" },
    { label: "min", type: "function", info: "algorithm" },
    { label: "push_back", type: "method" },
    { label: "size", type: "method" },
    { label: "empty", type: "method" },
    { label: "begin", type: "method" },
    { label: "end", type: "method" },
  ],
];

const cppCompletionSource = ifNotIn(
  ["Comment", "String"],
  completeFromList(CPP_COMPLETIONS),
);

export function criarEditor(
  parent: HTMLElement,
  opts: OpcoesEditor = {},
): InstanciaEditor {
  const keymaps = [
    ...closeBracketsKeymap,
    ...(opts.autocomplete ? completionKeymap : []),
    ...defaultKeymap,
    ...historyKeymap,
    ...(!opts.somenteLeitura ? [indentWithTab] : []),
  ];

  const extensions = [
    lineNumbers(),
    highlightActiveLine(),
    highlightActiveLineGutter(),
    history(),
    indentOnInput(),
    bracketMatching(),
    closeBrackets(),
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    keymap.of(keymaps),
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
      ".cm-tooltip.cm-tooltip-autocomplete": {
        border: "1px solid #cbd5e1",
        borderRadius: "6px",
        boxShadow: "0 8px 24px rgba(15, 23, 42, 0.12)",
        backgroundColor: "#ffffff",
        zIndex: "60",
      },
    }),
  ];

  const lang = (opts.linguagem || "").trim().toLowerCase();
  const isPython = lang === "python" || lang === "py" || lang === "python3";
  const isCpp =
    lang === "c" || lang === "cpp" || lang === "c++" || lang === "cxx";

  if (isPython) {
    extensions.push(python());
  } else if (isCpp) {
    extensions.push(cpp());
  }

  if (opts.autocomplete && !opts.somenteLeitura) {
    extensions.push(
      autocompletion({
        activateOnTyping: true,
        defaultKeymap: true,
        ...(!isPython && !isCpp ? { override: [completeAnyWord] } : {}),
      }),
    );
    if (isCpp) {
      extensions.push(
        cppLanguage.data.of({ autocomplete: cppCompletionSource }),
        cppLanguage.data.of({ autocomplete: completeAnyWord }),
      );
    }
  }

  if (opts.somenteLeitura) {
    extensions.push(EditorView.editable.of(false));
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
