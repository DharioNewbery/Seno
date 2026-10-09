# Especificação: Fluxo do aluno no Monitor

Documento de implementação da migração do fluxo de realização de atividades
para o serviço Monitor (API de gestão e Monitor de realização). Este documento
congela o contrato do fluxo do aluno. O fluxo do professor será especificado em
documento posterior.

## 1. Visão geral

A gestão (tarefas, atividades, atribuição, correções, notas e estatísticas)
permanece na API principal. A realização (abrir atividade, tentativas,
snapshots, ensaios, pré avaliação, submissão) passa ao Monitor, que mantém banco
de dados próprio e não acessa o Postgres da instituição.

Comunicam se por três pontas, todas autenticadas com HMAC:

1. Transferência da atividade (API para Monitor, chamada única)
2. SSO (BFF da API para Monitor, token escopado de vida curta)
3. Introspecção de estado (Monitor expõe estado consolidado para a API)

## 2. Transferência da atividade

Rota: `POST /interna/v1/atividades` (Monitor)

Autenticação: header `X Monitor MAC` com `hmac sha256(SENO_MONITOR_SECRET, corpo)`
e header `X Monitor Time` (janela de tolerância contra replay).

Corpo (JSON único, transferência integral):

```jsonc
{
  "api_id": 42,
  "monitor_id": "",
  "gerada_em": "2026-10-07T20:00:00Z",
  "turma": { "api_id": 7, "nome": "2026.1 Filas" },

  // Matrículas ativas copiadas no instante da publicação
  "alunos": [
    { "api_id": 12, "nome": "Ana R." },
    { "api_id": 13, "nome": "B. Lima" }
  ],

  "janela": {
    "inicio":    "2026-10-10T08:00:00Z",
    "prazo":     "2026-10-10T12:00:00Z",
    "duracao_seg": 5400,        // null = sem limite por aluno
    "pode_atrasado": true,
    "limite_atraso": "P1D"      // null = até encerramento da turma
  },

  "regras": {
    "submissoes_multiplas": false,   // false = 1 submissão, depois bloqueia
    "max_submissoes": 1,             // 0 = ilimitado
    "linguagens_permitidas": ["python"],
    "autocomplete": true
  },

  "carga": {
    "atividade": { "api_id": 12, "nome": "Prova 1", "enunciado": "..." },
    "tarefas": [
      {
        "ordem": 1,
        "tarefa_api_id": 5,
        "nome": "Dobro",
        "valor_pts": 10,
        "enunciado": "...",
        "linguagem": "python",
        "limites": { "tempo_cpu_ms": 2000, "tempo_total_ms": 5000, "memoria_mb": 256 },
        "testes": [
          { "stdin": "3", "stdout_esperado": "6", "publico": true },
          { "stdin": "",  "stdout_esperado": "",  "publico": false }
        ]
      }
    ]
  }
}
```

Resposta: `201 { "monitor_id": "atv_9f3k" }`

Regras de processamento:

- Idempotência por `api_id`: reenvio com mesmo `api_id` devolve o `monitor_id`
  já existente com status 200.
- `alunos` vazio: 422.
- Corpo além de `SENO_MONITOR_MAX_PAYLOAD_BYTES`: 413.
- Testes privados jamais saem do Monitor após a recepção.

## 3. SSO

Rota na API: `GET /v1/atribuicoes/:id/entrar`

1. A API valida sessão do aluno e matrícula ativa.
2. Emite token `st` assinado, vida curta:

```
{ "iss": "seno-api", "aud": "seno-monitor", "sub": "u_12",
  "mid": "atv_9f3k", "exp": "+5min", "nonce": "..." }
```

3. Resposta: `{ "url": "https://monitor.host/entrar?st=..." }` e o BFF redireciona.

O Monitor valida `st` (assinatura, expiração, `aud`, `mid` existe e aluno listado)
e cria sessão própria: cookie `hm_sess` (httpOnly, TTL = tempo restante da janela
do aluno mais 10 minutos, tabela `sessao`). Todas as chamadas de aluno seguintes
usam esse cookie. O Bearer da API principal não chega ao Monitor.

## 4. Esquema do banco do Monitor

```sql
CREATE TABLE atividade (
  monitor_id   TEXT PRIMARY KEY,
  api_id       INTEGER UNIQUE,
  carga        JSONB NOT NULL,      -- envelope assinado
  recebida_em  TIMESTAMPTZ NOT NULL,
  status       TEXT NOT NULL        -- ativa | encerrada | cancelada
);

CREATE TABLE atividade_aluno (
  monitor_id    TEXT NOT NULL REFERENCES atividade,
  aluno_api_id  INTEGER NOT NULL,
  UNIQUE (monitor_id, aluno_api_id)
);

CREATE TABLE sessao (
  token_hash    TEXT PRIMARY KEY,
  aluno_api_id  INTEGER NOT NULL,
  monitor_id    TEXT NOT NULL,
  expira_em     TIMESTAMPTZ NOT NULL
);

CREATE TABLE tentativa (
  monitor_id     TEXT NOT NULL REFERENCES atividade,
  aluno_api_id   INTEGER NOT NULL,
  UNIQUE (monitor_id, aluno_api_id),
  aberta_em      TIMESTAMPTZ NOT NULL,
  fim_real       TIMESTAMPTZ NOT NULL,
  revisao        INTEGER NOT NULL,
  seq_submissao  INTEGER NOT NULL DEFAULT 0,
  dados          JSONB NOT NULL
);

-- Ponto de snapshot imutável
CREATE TABLE rascunho_ponto (
  tentativa_seq  BIGSERIAL PRIMARY KEY,
  monitor_id     TEXT NOT NULL,
  aluno_api_id   INTEGER NOT NULL,
  fixado_em      TIMESTAMPTZ NOT NULL,
  dados          JSONB NOT NULL
);

CREATE TABLE execucao (
  id            BIGSERIAL PRIMARY KEY,
  monitor_id    TEXT NOT NULL,
  aluno_api_id  INTEGER NOT NULL,
  tipo          TEXT NOT NULL,      -- ensaio | pre_avaliacao
  tarefa_ordem  INTEGER NOT NULL,
  teste_ordem   INTEGER,
  publico       BOOLEAN,
  veredito      TEXT,
  stdout_obtido TEXT,
  tempo_seg     NUMERIC,
  memoria_kb    INTEGER,
  criada_em     TIMESTAMPTZ NOT NULL
);

CREATE TABLE submissao (
  id            BIGSERIAL PRIMARY KEY,
  monitor_id    TEXT NOT NULL,
  aluno_api_id  INTEGER NOT NULL,
  n_t           INTEGER NOT NULL,   -- número da submissão, 1, 2, 3...
  UNIQUE (monitor_id, aluno_api_id, n_t),
  dados         JSONB NOT NULL,     -- respostas congeladas
  entregue_em   TIMESTAMPTZ NOT NULL,
  atrasada      BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE pre_avaliacao (
  id           TEXT PRIMARY KEY,
  submissao_id BIGINT REFERENCES submissao,
  monitor_id   TEXT NOT NULL,
  aluno_api_id INTEGER NOT NULL,
  revisao_ref  INTEGER NOT NULL,    -- revisao da tentativa no momento da avaliação
  relatorio    JSONB,
  criada_em    TIMESTAMPTZ NOT NULL,
  expira_em    TIMESTAMPTZ NOT NULL,
  confirmada   BOOLEAN NOT NULL DEFAULT false,
  cancelada    BOOLEAN NOT NULL DEFAULT false
);
```

## 5. Endpoints do aluno (Monitor, cookie hm_sess)

Regras comuns a todos os endpoints: aluno não presente em `atividade_aluno`
resulta 403; janelas e prazos são recalculados no servidor a cada operação;
respostas seguem o padrão de erro da API atual (`{"erro": "..."}`).

### 5.1 Abrir atividade

`POST /a/atividade/{monitor_id}/abrir` (idempotente)

Marca o início da tentativa (primeira chamada define `aberta_em`) e calcula
`fim_real = min(inicio + duracao, prazo)`.

Resposta sanitizada, somente dados públicos:

```jsonc
{
  "atividade": {
    "nome": "Prova 1",
    "enunciado": "...",
    "tarefas": [
      {
        "ordem": 1, "nome": "Dobro", "enunciado": "...", "linguagem": "python",
        "valor_pts": 10, "limites": { "tempo_cpu_ms": 2000 },
        "testes": [ { "stdin": "3", "stdout_esperado": "6", "publico": true } ]
      }
    ]
  },
  "tentativa": {
    "fim_real": "2026-10-10T09:30:00Z",
    "segundos_restantes": 5300,
    "revisao": 0,
    "respostas": { "1": "texto já salvo" }
  }
}
```

Erros:

- Início futuro: 409 `{ "erro": "inicio", "segundos_para_inicio": 900 }`
- Janela encerrada sem entrega atrasada possível: 410
- Já submetido com submissões únicas: 403 `{ "erro": "finalizada" }`
  (tela fixa, ver 5.6)

### 5.2 Autosave

`PUT /a/atividade/{monitor_id}/tentativa`

Corpo: `{ "revisao": n, "respostas": { "1": "..." } }`

- Igual à lógica atual: `revisao` divergente resulta 409 (proteção contra duas
  abas, o cliente recarrega o estado).
- Em cada gravação, se `agora - ultimo_ponto >= SENO_SNAPSHOT_INTERVALO_S`
  (padrão 60, `.env`), registra um `rascunho_ponto` imutável antes de
  sobrescrever `dados`. O snapshot não depende do cliente.
- Corpo além de `SENO_MONITOR_MAX_SNAPSHOT_BYTES`: 413.

### 5.3 Ensaios (executar tarefa contra testes públicos)

Contrato equivalente ao atual, mudando quem executa:

- `POST /a/atividade/{monitor_id}/testar`
  `{ "ordem": 1, "linguagem": "python", "codigo": "..." }`
  Executa apenas testes públicos da tarefa, via fila assíncrona no Monitor,
  resposta `202 { "run_id": "..." }`.
- `GET /a/atividade/{monitor_id}/testar/{run_id}` devolve o resultado quando pronto.
- Permanecem o limite `SENO_ALUNO_EXEC_MAX_PER_MIN` (5 por minuto) e a retenção
  de `SENO_EXECUCAO_RETENCAO_D` dias (7) com limpeza a cada inserção.
- Exige tentativa aberta, matrícula no snapshot e janela válida, igual hoje.

### 5.4 Pré avaliação (roda antes de submeter)

`POST /a/atividade/{monitor_id}/submissao/avaliar`

Requisições sem tentativa aberta ou com janela expirada: 403. Roda todos os
testes públicos de todas as tarefas sobre as respostas correntes (batch
síncrono no Judge0, comparação exata stdin e stdout esperado). Registra
`pre_avaliacao` com `revisao_ref` da tentativa e devolve o relatório:

```jsonc
{
  "pre_id": "pv_u8xz",
  "resumo": { "acertos": 7, "total": 10 },
  "tarefas": [
    {
      "ordem": 1,
      "veredito": "wrong_answer",
      "testes": [
        { "ordem": 1, "status": "accepted", "tempo": 0.02, "memoria": 1200 },
        { "ordem": 2, "status": "wrong_answer", "stdout_obtido": "6\n" }
      ]
    }
  ],
  "expira_em": "+15min"
}
```

O relatório expira após `SENO_PRE_AVA_TTL_S` (padrão 900). A pré avaliação não
entra no histórico de ensaios do aluno.

### 5.5 Submissão (escolha do aluno)

Cancelamento, mantém a tentativa viva:

`POST /a/atividade/{monitor_id}/submissao/cancelar`
Corpo `{ "pre_id": "pv_u8xz" }`. Descarta a pré avaliação. O aluno continua
editando e pode avaliar de novo. Nada persiste além do log de auditoria.

Proceder (entrega confirmada):

`POST /a/atividade/{monitor_id}/submissao/confirmar`
Corpo `{ "pre_id": "pv_u8xz" }`

Validações:

- `pre_id` vivo, não expirado, não confirmado e não cancelado, senão 409.
- `revisao` da tentativa idêntica ao `revisao_ref` da pré avaliação, senão
  409 `{ "erro": "atualizada" }` (o cliente precisa reavaliar).

Efeito:

1. Grava `submissao` com as respostas congeladas (snapshot fechado) e
   `n_t = seq_submissao + 1`.
2. Anexa a pré avaliação à submissão.
3. Apaga a tentativa (comportamento de hoje).
4. Agenda o processamento completo (testes públicos e privados, correção) em
   fila assíncrona; vereditos completos via callback interno do Monitor.
5. Resposta:

```jsonc
{ "status": "entregue", "n_t": 2, "atrasada": false, "segundos_atraso": 0 }
```

### 5.6 Recarga e bloqueio final

- Se `submissoes_multiplas` e nova aba dentro permitida, `abrir` transporta
  automaticamente o conteúdo da última submissão para uma nova tentativa
  (`revisao` incrementada, `seq_submissao` mantém o contador). A interface
  mostra "editando a entrega N".
- Se `pode_atrasado`, o envio entre `prazo` e o limite terá `atrasada = true`,
  igual ao comportamento atual.
- Com 1 submissão, ou `n_t` na cota `max_submissoes`, todos os endpoints da
  atividade retornam `403 { "erro": "finalizada", "correcao_url": "/aluno/atividades/<api_id>" }`
  com tela fixa: "atividade encerrada para você", levando o portal da API
  principal para acompanhar a correção (rota `/v1/submissoes/:id/minha`).

## 6. Estado exposto à API principal

Rota: `GET /interna/v1/atividades/{monitor_id}/estado` (HMAC)

Resposta:

```jsonc
{
  "monitor_id": "atv_9f3k",
  "status": "ativa",
  "agregados": { "abertas": 12, "entregues": 31, "atrasos": 2, "total": 45 },
  "alunos": [
    { "aluno_api_id": 12, "estado": "entregue", "n_t": 1,
      "ultima_submissao": "...", "prazo_efetivo": "...",
      "nota_auto": 10.0, "correcao_status": "concluida" }
  ]
}
```

A listagem do aluno (`/v1/me/atribuicoes`, API principal) consome esse estado e
exibe metadados: disponibilidade, prazo, estado da realização, status de
correção e nota publicada. A listagem nunca chama o Monitor em tempo de página;
o estado é colhido por poll pós evento oujobperiódico.

## 7. Enunciados de configuração do Monitor (.env)

```
SENO_MONITOR_SECRET=...                # HMAC compartilhado com a API
SENO_MONITOR_MAX_PAYLOAD_BYTES=1048576
SENO_SNAPSHOT_INTERVALO_S=60           # intervalo default de snapshot
SENO_MONITOR_MAX_SNAPSHOT_BYTES=262144
SENO_TOKEN_SSO_TTL_S=300
SENO_PRE_AVA_TTL_S=900                 # janela de escolha após pré avaliação
SENO_ALUNO_EXEC_MAX_PER_MIN=5
SENO_EXECUCAO_RETENCAO_D=7
SENO_JUDGE0_URL=...
SENO_JUDGE0_TOKEN=...
SENO_JUDGE0_CALLBACK_SECRET=...
```

## 8. Diferencial frente ao fluxo atual

| Hoje (API principal) | Novo (Monitor) |
|---|---|
| Abrir tentativa consulta matrícula viva no banco | Matrícula vem no snapshot; revalidação por SSO de vida curta |
| Autosave com revisão | Mantém autosave e revisão e guarda pontos imutáveis a cada 60 s |
| Execução de ensaio síncrona no Judge0 | Fila assincrona no Monitor, resposta 202 com `run_id` |
| Entrega nasce correção no mesmo handler | Entrega tem pré avaliação com relatório e escolha do aluno |
| Uma submissão única por atribuição | Configurável: múltiplas submissões com recarga da última |
| Tentativa apagada na entrega | Igual: tentativa apagada ao confirmar a entrega |
| Testes privados no banco acessível à API | Teste privado nunca sai do envelope do Monitor |
| Aluno vê lista na API e executa no mesmo lugar | Lista com metadados na API, realização em ambiente próprio |
