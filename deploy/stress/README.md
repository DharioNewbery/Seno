# Teste de Estresse — 100 Alunos Simultâneos em Prova (Grafana k6)

Esta pasta contém uma suíte completa baseada no **Grafana k6** para simular **100 alunos conectados simultaneamente** realizando uma atividade de programação criada por um professor, enquanto o professor acompanha as entregas e estatísticas da turma em tempo real.

## Por que Grafana k6?

1. **Dashboard em tempo real nativo (`http://localhost:5665`)**: exibe gráficos ao vivo de usuários ativos (VUs), requisições/s, latências `p90`/`p95`/`p99` por etapa (`login`, `entrar_monitor`, `autosave`, `ensaio`, `preavaliacao`, `confirmar_entrega`), taxa de falhas e avisos de saturação do Judge0.
2. **Duplo relatório final em HTML + JSON**:
   - `resultados/relatorio-seno.html`: relatório executivo em português específico para a arquitetura do Seno (SLOs, percentis por rota e diagnóstico de gargalos entre API, Monitor e Judge0).
   - `resultados/dashboard-tempo-real-<timestamp>.html`: exportação interativa completa com todos os gráficos de séries temporais da execução.
   - `resultados/resumo.json`: métricas brutas para comparação histórica entre versões/hardwares.
3. **Zero instalação obrigatória**: os scripts usam o binário `k6` local se estiver instalado (`winget install k6`) ou caem automaticamente para a imagem oficial `grafana/k6:latest` via Docker.

---

## Como executar

### 1. Suba o ambiente completo
Com Docker Compose (PostgreSQL + Judge0 + Monitor + API + Web):
```powershell
docker compose -f deploy/compose.dev.yaml up -d
```

### 2. Dispare o teste de estresse
No **Windows (PowerShell)**:
```powershell
.\deploy\stress\rodar-estresse.ps1 -AbrirRelatorio
```

No **Linux / macOS**:
```bash
chmod +x deploy/stress/rodar-estresse.sh
./deploy/stress/rodar-estresse.sh
```

Assim que o teste começar, abra **`http://localhost:5665`** no navegador para acompanhar os gráficos em tempo real.

---

## Parâmetros Principais

| Parâmetro (PS1 / Env) | Padrão | Descrição |
| :--- | :---: | :--- |
| `-TotalAlunos` / `TOTAL_ALUNOS` | `100` | Quantidade de alunos simultâneos matriculados e em prova |
| `-RampUpSeg` / `RAMP_UP_S` | `25` | Janela (segundos) em que os alunos entram gradualmente na prova |
| `-DuracaoProvaSeg` / `DURACAO_PROVA_S` | `120` | Tempo de realização com autosaves contínuos e botão "Testar" |
| `-JanelaEntregaSeg` / `JANELA_ENTREGA_S` | `20` | Janela final ("rush") em que os 100 alunos pré-avaliam e entregam |
| `-ExecutarEnsaios` / `EXECUTAR_ENSAIOS` | `true` | Se `false`, testa apenas API + Monitor + Banco sem disparar "Testar" durante a prova |

Exemplo de prova mais longa (10 minutos) com 150 alunos:
```powershell
.\deploy\stress\rodar-estresse.ps1 -TotalAlunos 150 -DuracaoProvaSeg 600 -AbrirRelatorio
```

---

## Ajuste de Capacidade do Judge0 para 100+ Alunos

Por padrão no Judge0, `MAX_QUEUE_SIZE=100`. Quando 100 alunos clicam em **Pré-avaliar / Entregar** ao mesmo tempo numa prova com 2 tarefas e 3 testes cada, são geradas centenas de submissões simultâneas no Judge0:

- **Com `judge0.conf` padrão**: você verá `Degradações Judge0 > 0` no relatório, comprovando que o Monitor **não trava nem perde submissões** (a entrega prossegue e as correções são reconciliadas em background).
- **Para absorver o pico sem fila cheia em produção**: adicione em `deploy/judge0/judge0.conf`:
  ```ini
  MAX_QUEUE_SIZE=2000
  COUNT_OF_WORKERS=8
  ```
  ou escale os containers de workers:
  ```bash
  docker compose -f deploy/compose.dev.yaml up -d --scale workers=4
  ```
