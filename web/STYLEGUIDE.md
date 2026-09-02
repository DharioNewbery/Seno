# Plano de estilo — interfaces do Seno (MVP)

Guia pragmático para manter a interface simples e consistente enquanto o MVP cresce
(login → professores → turmas → tarefas → editor).

## Princípios

1. **Sem framework, sem build step.** HTML + CSS + JS puros, servidos pela própria API
   (embed em `web/static`). Adicionar tooling só quando houver dor real.
2. **Tokens antes de componentes.** Toda cor, espaço, raio e fonte sai das custom
   properties do `:root` em `styles.css`. Nenhum valor mágico inline.
3. **Mobile-first, uma coluna.** Telas de autenticação: coluna central de 400px.
   Telas da aplicação: container de 960px. Grid só em listas/tabelas, quando necessário.
4. **Um conceito, uma classe.** Classes semânticas minúsculas (`.card`, `.btn`, `.field`),
   no máximo um modificador (`.btn--primary`). Sem BEM profundo, sem utilitários.
5. **Acessibilidade mínima nativa.** `label` para todo input, contraste AA, foco visível,
   `lang="pt-BR"`, textos de erro com `role="alert"`.

## Tokens (`:root` em `styles.css`)

| Grupo    | Tokens                                                                                     |
|----------|--------------------------------------------------------------------------------------------|
| Cor      | `--color-primary` (#0f766e), `--color-primary-strong` (#0b5d56), `--color-bg` (#f4f6f7), `--color-surface` (#fff), `--color-text` (#1f2d33), `--color-text-muted` (#5d6b72), `--color-border` (#d9e0e3), `--color-error` (#b3261e), `--color-error-bg` (#fdecec), `--color-success` (#146c2e), `--color-success-bg` (#e6f4ea) |
| Fonte    | system stack (sem webfonts); tamanhos `--font-size-sm` 14 / base 16 / `--font-size-lg` 20 / `--font-size-xl` 28 |
| Espaço   | escala de 4: `--space-1..6` = 4, 8, 12, 16, 24, 32                                          |
| Forma    | `--radius` 8px; bordas 1px sólidas `--color-border`; sombra só no `.card` (sutil)           |

Cinza-azulado para neutros, teal para ação. Uma cor de destaque apenas — sem
paletas secundárias no MVP.

## Componentes base

- `.card` — superfície de conteúdo (formulários, painéis)
- `.auth-wrap` — centralizador de telas de autenticação
- `.btn`, `.btn--primary`, `.btn--block` — botões (default = contorno neutro)
- `.field` (label + input) — campo de formulário; erro fica no `.alert--error` do form
- `.alert--error` / `.alert--success` — mensagens de resultado
- `.badge` — etiquetas pequenas (papéis: `super`, `professor`, `student`)
- `.profile` (dl/dt/dd) — pares rótulo→valor de dados de uma entidade
- `.muted` — texto secundário; `.hidden` — utilitário de ocultar
- Futuro próximo: `.table` (listas de turmas/usuários), `.page-header`, `.nav`

## Estados e interações

- Botão: `hover` escurece (`--color-primary-strong`), `disabled` = opacity .6 +
  cursor not-allowed; `:focus-visible` com outline 2px
- Input `:focus`: borda primária + anel suave (`box-shadow` 3px rgba teal .15)
- Submit em andamento: botão disabled + texto "Entrando..." (sem spinners)
- Erros de API aparecem no `.alert--error` do próprio formulário — nunca em `alert()`

## JavaScript (convenções em `app.js`)

- Sem módulos nem bundler: um `app.js` por página, IIFE implícita, `"use strict"`
- `fetch` centralizado numa função `request()` que injeta o Bearer, interpreta o
  envelope padrão `{success, message, data, error}` e faz **um** retry após renovar
  o access token com o refresh
- Sessão em `localStorage` (`seno.access` / `seno.refresh`); logout local apenas limpa
- Nunca inserir resposta de servidor com `innerHTML` — sempre `textContent`
- Textos da UI em português; nomes de arquivos e identificadores em inglês

## Fora do MVP (não fazer agora)

Dark mode, ícones/biblioteca de ícones, animações além de transições simples,
framework CSS/JS, toast global. Revisar quando o editor online (milestone 7) chegar.
