# Plano de implementação: Monitor — Fluxo do Aluno

Implementa a especificação de `docs/MONITOR_FLUXO_ALUNO.md`. **Escopo:** construir o
serviço Monitor (novo) de forma **autônoma e testável sem a API** (Fases 1–8) e
só depois conectá-lo a ela (Fase 9). **Fora do escopo:** a migração do fluxo de
realização da API para o Monitor — decisão registrada: a migração acontece como
esforço separado, num documento próprio, depois do Monitor funcionar e estar testado.

O monitor nunca chama a API. Toda a integração é assíncrona e por prova de
HMAC: quem tem `SENO_MONITOR_SECRET` emite, o Monitor valida. Em dev, a própria
equipe emite fixtures e tokens de teste — o fluxo completo do aluno (transferência,
prova, entrega, correção) roda com Monitor + Judge0 apenas.

Ordem pensada para cada fase terminar com algo testável end-to-end. Reaproveita os
padrões existentes da API (`internal/platform`, pgx + sqlc, fila PostgreSQL, Judge0
via batch + callback).

---

## Fase 1 — Esqueleto do serviço

**Objetivo:** Monitor sobe, tem `/healthz`, banco próprio e configuração.

1. Criar raiz `monitor/` no monorepo (mesmo padrão de `api/`) e, antes dela, o
   módulo `contract/` (contratos compartilhados — ver Fase 2.1; `go.work` na raiz):
   - `cmd/monitor/main.go` — bootstrap: config, migrações, pool, workers, Gin.
   - `internal/platform/` — copiar o mínimo de `api/internal/platform`
     (config, pool, migrate, errors); duplicar de propósito: serviços independentes.
   - `internal/httpapi/` — rotas e handlers.
2. Migração `0001_init.sql` do banco do Monitor com as 7 tabelas da spec §4
   (`atividade`, `atividade_aluno`, `sessao`, `tentativa`, `rascunho_ponto`,
   `execucao`, `submissao`, `pre_avaliacao`).
3. `.env` do Monitor com todas as chaves da spec §7 (`SENO_MONITOR_*`);
   espelhar em `deploy/.env.example`. Novas chaves na API: `SENO_MONITOR_SECRET`
   (compartilhado) e URL/segredo do SSO.
4. Dockerfile e entrada no `deploy/compose.yaml` (rede `app`) e
   `compose.dev.yaml`.

**Pronto quando:** o Monitor sobe com `docker compose up` (com ou sem o resto da
stack); `/healthz` responde; migrações aplicam no banco do Monitor.

## Fase 2 — Vermelho (HMAC) + transferência de atividade

**Objetivo:** um emissor (API em produção, fixtures em dev) consegue publicar
uma atividade no Monitor.

1. Módulo `contract/` (`github.com/seno-project/seno/contract`), módulo Go próprio
   importado por api e monitor via `go.work` (dev) e `replace` no `go.mod`
   (Docker/CI): `hmac.go` (assinatura `X-Monitor-MAC` + janela anti-replay,
   `hmac.Equal`), `st.go` (assinatura/validação do token SSO com
   `iss/aud/sub/mid/exp/jti`), `envelope.go` (structs do JSON de transferência
   spec §2 e estado spec §6) e `erro.go` (envelope `{"erro": {codigo, mensagem,
   detalhe}}` + consts dos códigos compartilhados, ex. `FINALIZADA`,
   `ATUALIZADA`, `REVISAO_OBSOLETA`). Regra do cinto: só entra o que atravessa
   a rede — regra de negócio, banco e **logger ficam de fora**
   (plataforma segue duplicada de propósito). Teste de compatibilidade garantido
   por construção: assinar num pacote, validar no outro, no mesmo teste.
2. `POST /interna/v1/atividades` no Monitor (spec §2):
   - verificação `X-Monitor-MAC` + `X-Monitor-Time` (janela anti-replay);
   - idempotência por `api_id` (reenvio devolve 200 com `monitor_id` existente);
   - validações: `alunos` vazio → 422, payload > `SENO_MONITOR_MAX_PAYLOAD_BYTES` → 413;
   - gera `monitor_id` e persiste envelope em `atividade.carga` e `atividade_aluno`.
3. Emissor de teste standalone (o Monitor não exige a API para ser exercitado):
   script/cmd de dev (`cmd/monitor-fixtures`) que assina com `SENO_MONITOR_SECRET`
   e envia o envelope de transferência — alimenta o Monitor com dados de teste
   direto, sem a API subir.
4. Lado da API: handler `POST` que monta o JSON de transferência a partir da
   Atividade/Atribuição existente (reaproveitar `internal/atividades` e
   `internal/tarefas`) e o envia assinado. Job na fila PostgreSQL
   (`tipo: monitor_transferir`) para não bloquear a publicação.

**Pronto quando:** a atividade chega ao Monitor (via qualquer emissor com o
segredo) e é persistida; reenvio é idempotente; MAC inválido → 401. Testes Go
cobrindo os 4 erros. O item 4 só é verificado depois da Fase 9.

## Fase 3 — SSO e sessão do aluno (testável sem a API)

**Objetivo:** aluno entra no Monitor com cookie próprio — e o Monitor é
testável end-to-end **sem a API rodando**.

Princípio: o Monitor **nunca chama a API**. Ele valida o token `st` apenas por
HMAC + claims, então qualquer parte que possua `SENO_MONITOR_SECRET` pode emiti-lo
em teste. Em produção quem emite é a API; em teste é um script/helper — mesmo
código de validação dos dois lados.

1. Emissor de `st` em produção (`GET /v1/atribuicoes/:id/entrar` na API):
   valida sessão + matrícula ativa e assina `{iss, aud, sub, mid, exp(+5min), jti}`.
   helper de teste (ex.: `cmd/monitor-st`) que imprime um `st` assinado com o mesmo
   segredo — só existe em dev.
2. `GET /entrar?st=...` no Monitor, validação em ordem, tudo → 401 genérico:
   MAC em tempo constante (`hmac.Equal`) → `aud` → `exp` → `mid` ativa →
   `sub ∈ atividade_aluno` → `jti` não consumido (anti-replay dentro dos 5 min).
3. Concessão do cookie `hm_sess` (httpOnly, Secure, token 32 B CSPRNG, banco
   guarda **só SHA-256**) com TTL = janela restante + 10 min; a janela válida é
   **recalculada no servidor** para cada operação — o cookie nunca vive mais que
   `tentativa.fim_real` autoriza. Sessão não é renovável por uso.
4. Middleware Gin `requireSessao`: cookie → sessão válida → `aluno_api_id` +
   `monitor_id` no contexto; 401 senão. Rate limit por IP nas tentativas de
   entrada (mesma mecânica de `api/internal/auth/ratelimit.go`).
5. BFF (`web`): redireciona para a URL retornada. CSRF do Monitor: SameSite +
   checagem de `Origin` nas mutações (o navegador fala direto com o Monitor,
   sem BFF).

## Fase 4 — Abrir atividade + autosave + snapshots

**Objetivo:** núcleo de prova funcionando.

1. Sanitização da carga: função única que de `atividade.carga` extrai **apenas
   dados públicos** (testes privados nunca saem — spec §2). Teste que varre a
   saída procurando `publico=false`.
2. `POST /a/atividade/{id}/abrir` (spec §5.1, idempotente): cria `tentativa`
   com `aberta_em`/`fim_real`, 409 início futuro, 410 encerrada, 403
   `finalizada`.
3. `PUT /a/atividade/{id}/tentativa` (spec §5.2): autosave com `revisao`
   (409 divergente), snapshot `rascunho_ponto` a cada
   `SENO_SNAPSHOT_INTERVALO_S`, 413 acima de `SENO_MONITOR_MAX_SNAPSHOT_BYTES`.
4. Reuso: a lógica de revisão é a mesma da API — extrair/portar de
   `api/internal/tentativas`.

**Pronto quando:** abrir → autosave conflitante → recarregar estado funciona;
snapshot registrado a cada intervalo. Testes com tempo congelado (inject clock).

## Fase 5 — Ensaios e pré-avaliação (Judge0)

**Objetivo:** executar código no Monitor.

1. Portar `api/internal/judge0` para o Monitor (cliente batch + callback) e
   `api/internal/testar` para fila local — mesmo desenho de fila PostgreSQL da API
   (§6 da ARQUITETURA.md), worker embutido.
2. Ensaios (spec §5.3): `POST /testar` → 202 `run_id`, só testes públicos,
   limite `SENO_ALUNO_EXEC_MAX_PER_MIN` (429 com tempo restante), retenção
   `SENO_EXECUCAO_RETENCAO_D` com limpeza na inserção; `GET /testar/{run_id}`.
3. Pré-avaliação (spec §5.4): batch **síncrono** de todos os testes públicos de
   todas as tarefas sobre as respostas correntes; grava `pre_avaliacao` com
   `revisao_ref`; expira em `SENO_PRE_AVA_TTL_S`; não entra no histórico de ensaios.
4. Endpoint de callback `POST /interna/callbacks/judge0` (HMAC no Monitor,
   dedup por `submission_token` — mesma regra da API).

**Pronto quando:** ensaio assíncrono e pré-avaliação devolvem relatório
equivalente ao do fluxo atual; rate limit e callback testados.

## Fase 6 — Submissão (cancelar/confirmar)

**Objetivo:** entrega com escolha do aluno.

1. `POST /submissao/cancelar` (spec §5.5): descarta pré-avaliação, log de
   auditoria, tentativa continua viva.
2. `POST /submissao/confirmar` (spec §5.5): valida `pre_id` vivo (409),
   `revisao == revisao_ref` (409 `atualizada`); grava `submissao` congelada com
   `n_t = seq_submissao + 1`, anexa pré-avaliação, apaga tentativa, agenda job
   assíncrono de processamento completo (públicos + privados + correção).
3. `atrasada` e `segundos_atraso` conforme janela; recarga da última submissão
   quando `submissoes_multiplas` (spec §5.6, `revisao` incrementada).
4. Bloqueio final (spec §5.6): cota atingida → todo endpoint `403 {"erro":
   "finalizada", "correcao_url": ...}`; middleware que intercepta antes dos handlers.
5. Correção após entrega: portar o caminho de correção automática
   (batch Judge0 → nota auto → `correcao_status`) para a fila do Monitor.

**Pronto quando:** fluxo completo abrir → editar → ensaiar → pré-avaliar →
cancelar → reavaliar → confirmar → bloqueio funciona; entrega atrasada marca
`atrasada=true`; múltiplas submissões recarregam a anterior.

## Fase 7 — Estado consolidado (rota pronta para a integração)

**Objetivo:** deixar pronto o contrato do qual a API, na Fase 9, dependerá —
sem mudar a API agora.

1. `GET /interna/v1/atividades/{monitor_id}/estado` (HMAC, spec §6): agregados +
   estado por aluno.
2. Teste e2e sem API: cliente de teste (script ou job no `compose.dev.yaml`)
   assina HMAC, faz poll e confere os agregados — fragmento de execução para
   provar a rota sozinha.

**Pronto quando:** a rota devolve agregados coerentes em teste e2e.

## Fase 8 — Deploy, observabilidade e endurecimento

1. Compose prod com healthcheck, volume próprio do banco do Monitor e inclusão
   no backup (§9.1 da ARQUITETURA); LGPD/retensão de 2 anos nos dados do Monitor.
2. Log de auditoria das escritas (segundo §9.3): tabela `log` no banco do Monitor.
3. Endurecimento: timeouts, tamanho de corpo global, `SameSite` do `hm_sess`,
   replay window do HMAC, teste de concorrência de duas abas (revisão).
4. Teste e2e completo do fluxo do aluno **sem a API**: fixtures (Fase 2.3)
   alimentam atividade, helper de `st` (Fase 3.1) autentica, Judge0 local cuida
   das execuções — prova a separação em prática.
5. Docs: atualizar `ARQUITETURA.md` (§1 visão geral e §8.3 compose) com o Monitor.

**Pronto quando:** Monitor e Judge0 sobem com `docker compose up -d` e o fluxo e2e
completo (transferência → prova → entrega → correção) roda **sem a API em execução**.

## Fase 9 — Integração com a API (só depois de tudo testado)

Ativa as três pontas de produção. **Não é migração** — nada da API é removido
nem substituído aqui; isso é um esforço posterior (documento separado).

1. `GET /v1/atribuicoes/:id/entrar` na API: valida sessão real + matrícula ativa
   e emite `st` de verdade (o validador já roda no Monitor desde a Fase 3).
2. Job `monitor_transferir` na fila da API: publicação de atribuição envia o
   envelope (o receptor e o formato já rodam com fixtures na Fase 2.3).
3. Job periódico na API consome `GET /interna/.../estado` e alimenta
   `/v1/me/atribuicoes` (apenas metadados de exibição).
4. BFF: rota `/entrar` redirecionando.

**Pronto quando:** aluno pela UI real (BFF) faz o fluxo completo da prova no
Monitor e a listagem na API reflete o estado.

---

## Decisões a tomar no começo (não adiáveis)

| Decisão | Recomendação |
|---|---|
| Monitor como 4º serviço Go no monorepo | Sim; raiz `monitor/`, padrões copiados de `api/` |
| Reusar código da API por imports | Apenas um módulo **contrato** compartilhado (`contract/`: HMAC, token `st`, structs de transferência/estado, **erros de rede**, importado por api e monitor); plataforma mínima (`internal/platform`, **inclui logger**) segue **duplicada**, e pacotes `judge0`/`tentativas` são portados, não importados |
| Banco do Monitor | Instância PostgreSQL própria (container + volume + backup) |
| Assinatura do `st` | HMAC já usado nas rotas internas (evita JWT + dependência) |
| Estado para a API | Poll via job periódico primeiro; pós-evento depois, se necessário |
| `st` e transferência em dev | Emissores de teste (`cmd/*`) que usam só `SENO_MONITOR_SECRET` — a API é opcional em dev |

**Sobre matrícula:** o Monitor **não** revalida matrícula viva na API (nunca a
chama). A autoridade dele é o snapshot `atividade_aluno` + regras de janela
recalculadas no servidor. Aluno removido da turma depois da publicação continua
acessível até o prazo: limitação aceita da spec, registrada aqui.

## Riscos

- **Callback do Judge0 para dois consumidores:** o Judge0 de uma prova em andamento
  falando com API e Monitor simultaneamente exige rotas de callback distintas por
  serviço — confirmar que o Judge0 aceita `callback_url` por submissão (aceita).
- **Relógio:** `fim_real`, TTLs e janela HMAC dependem de relógio consistente entre
  API/Monitor; ambos no mesmo host Docker mitiga, mas testar com clock injetado.
- **Testes privados:** o maior risco de segurança. A sanitização (Fase 4.1) precisa
  de teste que garanta zero vazamento em **todas** as respostas de aluno.
