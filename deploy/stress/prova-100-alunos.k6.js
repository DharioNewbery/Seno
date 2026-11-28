// deploy/stress/prova-100-alunos.k6.js
// Simulação ponta a ponta de 100 alunos conectados simultaneamente realizando
// uma atividade de programação de um professor no Seno (API <-> Monitor <-> Judge0).
//
// Visualização em tempo real:
//   K6_WEB_DASHBOARD=true K6_WEB_DASHBOARD_EXPORT=deploy/stress/resultados/dashboard-k6.html k6 run deploy/stress/prova-100-alunos.k6.js
//   Painel ao vivo: http://localhost:5665

import http from 'k6/http';
import { check, group, sleep, fail } from 'k6';
import { Trend, Counter, Rate, Gauge } from 'k6/metrics';
import exec from 'k6/execution';

// ============================================================================
// Parâmetros configuráveis via variáveis de ambiente (__ENV)
// ============================================================================
const API_URL = (__ENV.API_URL || 'http://localhost:8080').replace(/\/$/, '');
const MONITOR_URL = (__ENV.MONITOR_URL || 'http://localhost:8081').replace(/\/$/, '');
const SENHA_PADRAO = __ENV.SENHA_PADRAO || '102938';

const TOTAL_ALUNOS = parseInt(__ENV.TOTAL_ALUNOS || '100', 10);
const RAMP_UP_S = parseInt(__ENV.RAMP_UP_S || '25', 10);
const DURACAO_PROVA_S = parseInt(__ENV.DURACAO_PROVA_S || '120', 10);
const JANELA_ENTREGA_S = parseInt(__ENV.JANELA_ENTREGA_S || '20', 10);
const AUTOSAVE_INTERVALO_S = parseFloat(__ENV.AUTOSAVE_INTERVALO_S || '10');
const ENSAIOS_POR_MINUTO = parseFloat(__ENV.ENSAIOS_POR_MINUTO || '5');
const EXECUTAR_ENSAIOS = (__ENV.EXECUTAR_ENSAIOS || 'true').toLowerCase() !== 'false';
const RESULTADOS_DIR = (__ENV.RESULTADOS_DIR || 'deploy/stress/resultados').replace(/\/$/, '');

function extrairOrigem(urlStr) {
  const m = urlStr.match(/^(https?:\/\/[^/]+)/i);
  return m ? m[1] : urlStr;
}
const MONITOR_ORIGIN = extrairOrigem(MONITOR_URL);

// ============================================================================
// Métricas customizadas (visíveis ao vivo no Web Dashboard e no relatório final)
// ============================================================================
const loginDuration = new Trend('seno_login_ms', true);
const emitirSTDuration = new Trend('seno_emitir_st_ms', true);
const entrarMonitorDuration = new Trend('seno_entrar_monitor_ms', true);
const abrirProvaDuration = new Trend('seno_abrir_prova_ms', true);
const autosaveDuration = new Trend('seno_autosave_ms', true);
const ensaioRoundtripDuration = new Trend('seno_ensaio_roundtrip_ms', true);
const preavaliacaoDuration = new Trend('seno_preavaliacao_ms', true);
const confirmarEntregaDuration = new Trend('seno_confirmar_entrega_ms', true);
const painelProfessorDuration = new Trend('seno_painel_professor_ms', true);

const alunosEmProvaGauge = new Gauge('seno_alunos_em_prova');
const autosavesOkCounter = new Counter('seno_autosaves_ok');
const ensaiosConcluidosCounter = new Counter('seno_ensaios_concluidos');
const avaliadorDegradadoCounter = new Counter('seno_avaliador_degradado_avisos');
const entregasConfirmadasCounter = new Counter('seno_entregas_confirmadas');
const erros5xxCounter = new Counter('seno_erros_5xx');
const sucessoFluxoCompletoRate = new Rate('seno_fluxo_completo_sucesso');

const duracaoTotalProfessorS = RAMP_UP_S + DURACAO_PROVA_S + JANELA_ENTREGA_S + 30;

export const options = {
  setupTimeout: '120s',
  teardownTimeout: '90s',
  scenarios: {
    alunos_em_prova: {
      executor: 'per-vu-iterations',
      vus: TOTAL_ALUNOS,
      iterations: 1,
      maxDuration: `${duracaoTotalProfessorS + 120}s`,
      exec: 'fluxoAluno',
    },
    professor_acompanhando: {
      executor: 'constant-vus',
      vus: 1,
      duration: `${duracaoTotalProfessorS}s`,
      exec: 'fluxoProfessor',
    },
  },
  thresholds: {
    seno_erros_5xx: ['count==0'],
    seno_fluxo_completo_sucesso: ['rate>=0.98'],
    seno_login_ms: ['p(95)<2500'],
    seno_abrir_prova_ms: ['p(95)<1000'],
    seno_autosave_ms: ['p(95)<600', 'p(99)<1500'],
    seno_confirmar_entrega_ms: ['p(95)<3500'],
    seno_painel_professor_ms: ['p(95)<1000'],
  },
};

function registrarStatus(res) {
  if (res && res.status >= 500 && res.status !== 503) {
    erros5xxCounter.add(1);
  }
}

function pad3(n) {
  return String(n).padStart(3, '0');
}

function detectarWebOrigin() {
  if (__ENV.WEB_ORIGIN) {
    return __ENV.WEB_ORIGIN;
  }
  const candidatos = ['http://localhost:3000', 'http://localhost:5173'];
  for (const origin of candidatos) {
    const res = http.post(
      `${API_URL}/v1/auth/login`,
      JSON.stringify({ email: 'prof.stress@seno.dev', senha: SENHA_PADRAO }),
      { headers: { 'Content-Type': 'application/json', Origin: origin } }
    );
    if (res.status !== 403) {
      return origin;
    }
  }
  return 'http://localhost:3000';
}

// ============================================================================
// FASE 0 — Setup: Professor cria tarefas, atividade e atribui para os 100 alunos
// ============================================================================
export function setup() {
  const healthApi = http.get(`${API_URL}/healthz`);
  if (healthApi.status !== 200) {
    fail(`API indisponível em ${API_URL}/healthz (status ${healthApi.status}).`);
  }
  const healthMon = http.get(`${MONITOR_URL}/healthz`);
  if (healthMon.status !== 200) {
    fail(`Monitor indisponível em ${MONITOR_URL}/healthz (status ${healthMon.status}).`);
  }

  const webOrigin = detectarWebOrigin();
  const baseHeaders = {
    'Content-Type': 'application/json',
    Origin: webOrigin,
  };

  const loginProf = http.post(
    `${API_URL}/v1/auth/login`,
    JSON.stringify({ email: 'prof.stress@seno.dev', senha: SENHA_PADRAO }),
    { headers: baseHeaders }
  );
  if (loginProf.status !== 200) {
    fail(
      `Falha no login de prof.stress@seno.dev (status ${loginProf.status}: ${loginProf.body}). Execute seed-stress.sql antes.`
    );
  }
  const profToken = loginProf.json('token');
  const profHeaders = Object.assign({}, baseHeaders, {
    Authorization: `Bearer ${profToken}`,
  });

  // Localiza a turma T-STRESS-100 criada pelo seed
  const resTurmas = http.get(`${API_URL}/v1/turmas?por=100`, { headers: profHeaders });
  if (resTurmas.status !== 200) {
    fail(`Falha ao listar turmas do professor: ${resTurmas.status} ${resTurmas.body}`);
  }
  const turmas = resTurmas.json('turmas') || [];
  const turma = turmas.find((t) => t.titulo === 'T-STRESS-100');
  if (!turma) {
    fail('Turma T-STRESS-100 não encontrada. Rode deploy/stress/seed-stress.sql antes do teste.');
  }
  if (turma.total_alunos < TOTAL_ALUNOS) {
    fail(
      `Turma T-STRESS-100 possui ${turma.total_alunos} alunos, mas TOTAL_ALUNOS=${TOTAL_ALUNOS}. Rode o seed com total_alunos=${TOTAL_ALUNOS}.`
    );
  }

  // Cria 2 tarefas de programação (públicos + privados)
  const sufixo = new Date().toISOString().slice(11, 19);
  const resT1 = http.post(
    `${API_URL}/v1/tarefas`,
    JSON.stringify({
      nome: `Dobro [${sufixo}]`,
      enunciado: 'Leia um inteiro N da entrada padrão e imprima 2*N.',
      tempo_cpu_ms: 2000,
      tempo_total_ms: 5000,
      memoria_mb: 256,
      testes: [
        { stdin: '4\n', stdout_esperado: '8\n', publico: true },
        { stdin: '-7\n', stdout_esperado: '-14\n', publico: false },
        { stdin: '0\n', stdout_esperado: '0\n', publico: false },
      ],
    }),
    { headers: profHeaders }
  );
  if (resT1.status !== 201) {
    fail(`Falha ao criar Tarefa 1: ${resT1.status} ${resT1.body}`);
  }
  const idT1 = resT1.json('id');

  const resT2 = http.post(
    `${API_URL}/v1/tarefas`,
    JSON.stringify({
      nome: `Soma Simples [${sufixo}]`,
      enunciado: 'Leia dois inteiros A e B separados por espaço e imprima A + B.',
      tempo_cpu_ms: 2000,
      tempo_total_ms: 5000,
      memoria_mb: 256,
      testes: [
        { stdin: '3 5\n', stdout_esperado: '8\n', publico: true },
        { stdin: '10 -4\n', stdout_esperado: '6\n', publico: false },
        { stdin: '100 250\n', stdout_esperado: '350\n', publico: false },
      ],
    }),
    { headers: profHeaders }
  );
  if (resT2.status !== 201) {
    fail(`Falha ao criar Tarefa 2: ${resT2.status} ${resT2.body}`);
  }
  const idT2 = resT2.json('id');

  // Cria a atividade combinando as 2 tarefas
  const resAtv = http.post(
    `${API_URL}/v1/atividades`,
    JSON.stringify({
      nome: `Prova Estresse (${TOTAL_ALUNOS} Alunos) — ${sufixo}`,
      conteudo: {
        schema_version: 1,
        components: [
          { type: 'large_text', text: 'Avaliação Prática de Programação' },
          { type: 'task_default', id: idT1, value_pts: 5, language: 'python' },
          { type: 'task_default', id: idT2, value_pts: 5, language: 'python' },
        ],
      },
      tarefas: [
        { tarefa_id: idT1, ordem: 1, valor_pts: 5, linguagem: 'python' },
        { tarefa_id: idT2, ordem: 2, valor_pts: 5, linguagem: 'python' },
      ],
    }),
    { headers: profHeaders }
  );
  if (resAtv.status !== 201) {
    fail(`Falha ao criar Atividade: ${resAtv.status} ${resAtv.body}`);
  }
  const idAtv = resAtv.json('id');

  // Publica a atribuição na turma (transfere via HMAC para o Monitor)
  const prazoISO = new Date(Date.now() + 4 * 3600 * 1000).toISOString();
  const resAtrib = http.post(
    `${API_URL}/v1/atribuicoes`,
    JSON.stringify({
      turma_id: turma.id,
      atividade_id: idAtv,
      prazo: prazoISO,
      duracao_seg: 7200,
      pode_atrasado: true,
      autocomplete: false,
    }),
    { headers: profHeaders }
  );
  if (resAtrib.status !== 201) {
    fail(`Falha ao criar Atribuição: ${resAtrib.status} ${resAtrib.body}`);
  }

  return {
    webOrigin,
    profToken,
    turmaId: turma.id,
    atribuicaoId: resAtrib.json('id'),
    inicioEpochMs: Date.now(),
  };
}

// ============================================================================
// CENÁRIO 1 — Fluxo completo de cada um dos 100 alunos
// ============================================================================
export function fluxoAluno(data) {
  const indiceAluno = (exec.scenario.iterationInTest % TOTAL_ALUNOS) + 1;
  const email = `stress.aluno${pad3(indiceAluno)}@seno.dev`;
  const jar = new http.CookieJar();

  // Ramp-up escalonado (simula os 100 alunos entrando na sala nos primeiros RAMP_UP_S)
  const atrasoEntrada = ((indiceAluno - 1) / Math.max(1, TOTAL_ALUNOS)) * RAMP_UP_S + Math.random() * 1.5;
  sleep(atrasoEntrada);

  let tokenAluno = '';
  let monitorId = '';
  let revisao = 0;
  let entregou = false;

  const apiHeaders = {
    'Content-Type': 'application/json',
    Origin: data.webOrigin,
  };
  const monHeaders = {
    'Content-Type': 'application/json',
    Origin: MONITOR_ORIGIN,
  };

  // --- Etapa 1: Login + Entrada SSO no Monitor + Abertura da Prova ---
  group('01_entrada_na_prova', () => {
    const resLogin = http.post(
      `${API_URL}/v1/auth/login`,
      JSON.stringify({ email, senha: SENHA_PADRAO }),
      { headers: apiHeaders, tags: { etapa: 'login' } }
    );
    registrarStatus(resLogin);
    loginDuration.add(resLogin.timings.duration);

    const okLogin = check(resLogin, {
      'login aluno 200': (r) => r.status === 200 && !!r.json('token'),
    });
    if (!okLogin) {
      sucessoFluxoCompletoRate.add(false);
      return;
    }
    tokenAluno = resLogin.json('token');

    const resST = http.get(`${API_URL}/v1/atribuicoes/${data.atribuicaoId}/entrar`, {
      headers: Object.assign({}, apiHeaders, { Authorization: `Bearer ${tokenAluno}` }),
      tags: { etapa: 'emitir_st' },
    });
    registrarStatus(resST);
    emitirSTDuration.add(resST.timings.duration);

    const urlEntrada = resST.status === 200 ? resST.json('url') : '';
    const matchST = urlEntrada ? urlEntrada.match(/[?&]st=([^&]+)/) : null;
    const okST = check(resST, {
      'token SSO (st) emitido 200': (r) => r.status === 200 && !!matchST,
    });
    if (!okST) {
      sucessoFluxoCompletoRate.add(false);
      return;
    }

    const stToken = matchST[1];
    const resEntrarMon = http.get(`${MONITOR_URL}/entrar?st=${stToken}`, {
      jar,
      redirects: 0,
      headers: { Accept: 'application/json' },
      tags: { etapa: 'entrar_monitor' },
    });
    registrarStatus(resEntrarMon);
    entrarMonitorDuration.add(resEntrarMon.timings.duration);

    const okMon = check(resEntrarMon, {
      'sessão hm_sess criada no Monitor 200': (r) => r.status === 200 && !!r.json('monitor_id'),
    });
    if (!okMon) {
      sucessoFluxoCompletoRate.add(false);
      return;
    }
    monitorId = resEntrarMon.json('monitor_id');

    const resAbrir = http.post(`${MONITOR_URL}/a/atividade/${monitorId}/abrir`, '{}', {
      jar,
      headers: monHeaders,
      tags: { etapa: 'abrir_prova' },
    });
    registrarStatus(resAbrir);
    abrirProvaDuration.add(resAbrir.timings.duration);

    const okAbrir = check(resAbrir, {
      'atividade aberta no Monitor 200': (r) => r.status === 200 && r.json('tentativa') !== null,
    });
    if (!okAbrir) {
      monitorId = '';
      sucessoFluxoCompletoRate.add(false);
      return;
    }
    revisao = resAbrir.json('tentativa.revisao') || 0;
  });

  if (!monitorId) {
    return;
  }

  alunosEmProvaGauge.add(1);

  // --- Etapa 2: Realização da Prova (Autosaves contínuos + botão "Testar" no Judge0) ---
  const fimRealizacaoMs =
    data.inicioEpochMs + (RAMP_UP_S + DURACAO_PROVA_S) * 1000 + Math.random() * JANELA_ENTREGA_S * 1000;

  // 85% dos alunos resolvem as duas questões corretamente; 15% erram a 2ª (realismo de notas)
  const acertaSegunda = indiceAluno % 7 !== 0;
  const estagiosCodigo = [
    {
      '1': '# lendo entrada\nn = int(input())\n',
      '2': '# rascunho inicial\n',
    },
    {
      '1': 'n = int(input().strip())\nprint(n * 2)\n',
      '2': 'partes = input().split()\n',
    },
    {
      '1': 'n = int(input().strip())\nprint(n * 2)\n',
      '2': acertaSegunda
        ? 'a, b = map(int, input().split())\nprint(a + b)\n'
        : 'a, b = map(int, input().split())\nprint(a - b)\n',
    },
  ];

  let ciclo = 0;
  const probEnsaioPorCiclo = Math.min(1, (ENSAIOS_POR_MINUTO / 60) * AUTOSAVE_INTERVALO_S);

  group('02_realizacao_autosave_e_ensaios', () => {
    while (Date.now() < fimRealizacaoMs) {
      const inicioCicloMs = Date.now();
      const respostasAtuais = estagiosCodigo[Math.min(ciclo, estagiosCodigo.length - 1)];
      const resSave = http.put(
        `${MONITOR_URL}/a/atividade/${monitorId}/tentativa`,
        JSON.stringify({ revisao, respostas: respostasAtuais }),
        { jar, headers: monHeaders, tags: { etapa: 'autosave' } }
      );
      registrarStatus(resSave);
      autosaveDuration.add(resSave.timings.duration);

      if (resSave.status === 200) {
        revisao = resSave.json('revisao');
        autosavesOkCounter.add(1);
      } else if (resSave.status === 409 && resSave.json('erro.detalhes.revisao_atual') !== undefined) {
        revisao = resSave.json('erro.detalhes.revisao_atual');
      }

      check(resSave, {
        'autosave tentativa 200': (r) => r.status === 200,
      });

      // Testes aleatórios (~5 por aluno a cada 1 minuto), alternando entre as tarefas 1 e 2
      if (EXECUTAR_ENSAIOS && Math.random() < probEnsaioPorCiclo) {
        const ordem = Math.random() < 0.5 ? 1 : 2;
        const t0Ensaio = Date.now();
        const resEnsaio = http.post(
          `${MONITOR_URL}/a/atividade/${monitorId}/testar`,
          JSON.stringify({
            ordem,
            linguagem: 'python',
            codigo: respostasAtuais[String(ordem)],
          }),
          { jar, headers: monHeaders, tags: { etapa: 'ensaio_agendar' } }
        );
        registrarStatus(resEnsaio);

        if (resEnsaio.status === 202 && resEnsaio.json('run_id')) {
          const runId = resEnsaio.json('run_id');
          let pronto = false;
          for (let tent = 0; tent < 15 && !pronto; tent++) {
            sleep(1.0);
            const resPoll = http.get(`${MONITOR_URL}/a/atividade/${monitorId}/testar/${runId}`, {
              jar,
              tags: { etapa: 'ensaio_poll' },
            });
            registrarStatus(resPoll);
            if (resPoll.status === 200 && resPoll.json('pronto') === true) {
              pronto = true;
              ensaioRoundtripDuration.add(Date.now() - t0Ensaio);
              ensaiosConcluidosCounter.add(1);
            } else if (resPoll.status === 503) {
              // Degradação limpa sob saturação do Judge0 (503 AVALIADOR_INDISPONIVEL)
              avaliadorDegradadoCounter.add(1);
              break;
            }
          }
        }
      }

      ciclo++;
      const restanteMs = fimRealizacaoMs - Date.now();
      if (restanteMs <= 0) break;
      const esperaCicloS = Math.max(0.5, AUTOSAVE_INTERVALO_S - (Date.now() - inicioCicloMs) / 1000);
      sleep(Math.min(restanteMs / 1000, esperaCicloS));
    }

    // Garante que a versão final do código esteja salva antes de pré-avaliar
    const codigoFinal = estagiosCodigo[estagiosCodigo.length - 1];
    const resSaveFinal = http.put(
      `${MONITOR_URL}/a/atividade/${monitorId}/tentativa`,
      JSON.stringify({ revisao, respostas: codigoFinal }),
      { jar, headers: monHeaders, tags: { etapa: 'autosave' } }
    );
    registrarStatus(resSaveFinal);
    if (resSaveFinal.status === 200) {
      revisao = resSaveFinal.json('revisao');
      autosavesOkCounter.add(1);
    }
  });

  // --- Etapa 3: Pré-avaliação + Confirmação da Entrega Final ---
  group('03_entrega_final', () => {
    const resPre = http.post(`${MONITOR_URL}/a/atividade/${monitorId}/submissao/avaliar`, '{}', {
      jar,
      headers: monHeaders,
      timeout: '40s',
      tags: { etapa: 'preavaliacao' },
    });
    registrarStatus(resPre);
    preavaliacaoDuration.add(resPre.timings.duration);

    const preId = resPre.status === 200 ? resPre.json('pre_id') : '';
    if (resPre.status === 200 && resPre.json('aviso_avaliador')) {
      avaliadorDegradadoCounter.add(1);
    }

    const okPre = check(resPre, {
      'pré-avaliação retornou pre_id 200': (r) => r.status === 200 && !!preId,
    });
    if (!okPre) {
      sucessoFluxoCompletoRate.add(false);
      return;
    }

    sleep(0.5 + Math.random() * 1.5); // Tempo de leitura do modal de confirmação pelo aluno

    const resConf = http.post(
      `${MONITOR_URL}/a/atividade/${monitorId}/submissao/confirmar`,
      JSON.stringify({ pre_id: preId }),
      { jar, headers: monHeaders, timeout: '30s', tags: { etapa: 'confirmar_entrega' } }
    );
    registrarStatus(resConf);
    confirmarEntregaDuration.add(resConf.timings.duration);

    entregou = check(resConf, {
      'submissão confirmada com sucesso 200': (r) => r.status === 200 && r.json('n_t') >= 1,
    });
    if (entregou) {
      entregasConfirmadasCounter.add(1);
    }
    sucessoFluxoCompletoRate.add(entregou);
  });
}

// ============================================================================
// CENÁRIO 2 — Professor acompanhando entregas e analytics em tempo real
// ============================================================================
export function fluxoProfessor(data) {
  const headers = {
    Origin: data.webOrigin,
    Authorization: `Bearer ${data.profToken}`,
  };

  const t0 = Date.now();
  const resSubs = http.get(`${API_URL}/v1/atribuicoes/${data.atribuicaoId}/submissoes`, {
    headers,
    tags: { etapa: 'painel_professor' },
  });
  registrarStatus(resSubs);

  const resAnalytics = http.get(`${API_URL}/v1/atribuicoes/${data.atribuicaoId}/analytics`, {
    headers,
    tags: { etapa: 'painel_professor' },
  });
  registrarStatus(resAnalytics);
  painelProfessorDuration.add(Date.now() - t0);

  check(resSubs, {
    'professor lista submissões 200': (r) => r.status === 200,
  });
  check(resAnalytics, {
    'professor consulta analytics 200': (r) => r.status === 200,
  });

  sleep(8);
}

// ============================================================================
// TEARDOWN — Verifica sincronização Monitor -> API (zero perda de entregas)
// ============================================================================
export function teardown(data) {
  if (!data || !data.atribuicaoId) return;
  const headers = {
    Origin: data.webOrigin,
    Authorization: `Bearer ${data.profToken}`,
  };

  let sincronizadas = 0;
  for (let i = 0; i < 12; i++) {
    const res = http.get(`${API_URL}/v1/atribuicoes/${data.atribuicaoId}/submissoes`, { headers });
    if (res.status === 200) {
      sincronizadas = res.json('total') || 0;
      if (sincronizadas >= TOTAL_ALUNOS) break;
    }
    sleep(3);
  }
  console.log(
    `[Reconciliação Final API] Atribuição #${data.atribuicaoId}: ${sincronizadas}/${TOTAL_ALUNOS} submissões já ingeridas na API.`
  );
}

// ============================================================================
// RELATÓRIO FINAL (HTML executivo + JSON completo + resumo no console)
// ============================================================================
function fmtNum(v, casas = 1) {
  if (v === undefined || v === null || Number.isNaN(v)) return '—';
  return Number(v).toFixed(casas);
}

function extrairTrend(metrics, nome) {
  const m = metrics[nome];
  if (!m || !m.values) return null;
  return {
    avg: fmtNum(m.values.avg),
    med: fmtNum(m.values.med),
    p90: fmtNum(m.values['p(90)']),
    p95: fmtNum(m.values['p(95)']),
    p99: fmtNum(m.values['p(99)']),
    max: fmtNum(m.values.max),
  };
}

function valCounter(metrics, nome) {
  return metrics[nome] && metrics[nome].values ? metrics[nome].values.count || 0 : 0;
}

function gerarHTMLRelatorio(data) {
  const m = data.metrics || {};
  const totalReqs = valCounter(m, 'http_reqs');
  const duracaoSeg = (data.state && data.state.testRunDurationMs ? data.state.testRunDurationMs : 0) / 1000;
  const rpsMedio = duracaoSeg > 0 ? (totalReqs / duracaoSeg).toFixed(1) : '0';
  const entregas = valCounter(m, 'seno_entregas_confirmadas');
  const autosaves = valCounter(m, 'seno_autosaves_ok');
  const ensaios = valCounter(m, 'seno_ensaios_concluidos');
  const degradacoes = valCounter(m, 'seno_avaliador_degradado_avisos');
  const erros5xx = valCounter(m, 'seno_erros_5xx');
  const taxaSucesso =
    m.seno_fluxo_completo_sucesso && m.seno_fluxo_completo_sucesso.values
      ? (m.seno_fluxo_completo_sucesso.values.rate * 100).toFixed(1)
      : '0.0';

  const etapas = [
    ['Login na API (POST /v1/auth/login)', 'seno_login_ms'],
    ['Emissão Token SSO (GET /v1/atribuicoes/:id/entrar)', 'seno_emitir_st_ms'],
    ['Entrada no Monitor (GET /entrar?st=...)', 'seno_entrar_monitor_ms'],
    ['Abertura da Prova (POST /a/atividade/:id/abrir)', 'seno_abrir_prova_ms'],
    ['Autosave de Código (PUT /a/atividade/:id/tentativa)', 'seno_autosave_ms'],
    ['Botão "Testar" Roundtrip (Judge0 + Callback)', 'seno_ensaio_roundtrip_ms'],
    ['Pré-avaliação Síncrona (POST .../submissao/avaliar)', 'seno_preavaliacao_ms'],
    ['Confirmação de Entrega (POST .../submissao/confirmar)', 'seno_confirmar_entrega_ms'],
    ['Painel do Professor (Submissões + Analytics)', 'seno_painel_professor_ms'],
  ];

  const linhasTabela = etapas
    .map(([titulo, chave]) => {
      const t = extrairTrend(m, chave);
      if (!t) {
        return `<tr><td>${titulo}</td><td colspan="6" style="text-align:center;color:#64748b">Não executado</td></tr>`;
      }
      return `<tr>
        <td><strong>${titulo}</strong></td>
        <td>${t.avg} ms</td>
        <td>${t.med} ms</td>
        <td>${t.p90} ms</td>
        <td><strong>${t.p95} ms</strong></td>
        <td>${t.p99} ms</td>
        <td>${t.max} ms</td>
      </tr>`;
    })
    .join('\n');

  const slos = [];
  for (const [metricName, metricObj] of Object.entries(m)) {
    if (metricObj.thresholds) {
      for (const [regra, res] of Object.entries(metricObj.thresholds)) {
        slos.push({ metricName, regra, ok: res.ok });
      }
    }
  }
  const todosSlosOk = slos.every((s) => s.ok) && erros5xx === 0;

  const linhasSlo = slos
    .map(
      (s) => `<tr>
      <td><code>${s.metricName}</code></td>
      <td><code>${s.regra}</code></td>
      <td><span class="badge ${s.ok ? 'ok' : 'fail'}">${s.ok ? 'APROVADO' : 'VIOLADO'}</span></td>
    </tr>`
    )
    .join('\n');

  return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8" />
  <title>Relatório de Estresse — Seno (${TOTAL_ALUNOS} Alunos)</title>
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; background: #f8fafc; color: #0f172a; margin: 0; padding: 32px; }
    .container { max-width: 1120px; margin: 0 auto; }
    header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; border-bottom: 2px solid #e2e8f0; padding-bottom: 16px; }
    h1 { margin: 0; font-size: 1.65rem; }
    .sub { color: #475569; font-size: 0.95rem; margin-top: 4px; }
    .status-banner { padding: 8px 16px; border-radius: 999px; font-weight: 700; font-size: 0.9rem; }
    .status-ok { background: #dcfce7; color: #166534; }
    .status-fail { background: #fee2e2; color: #991b1b; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(175px, 1fr)); gap: 14px; margin-bottom: 28px; }
    .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 16px; box-shadow: 0 1px 2px rgba(0,0,0,0.03); }
    .card .label { font-size: 0.8rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.04em; }
    .card .value { font-size: 1.6rem; font-weight: 700; margin-top: 6px; }
    h2 { font-size: 1.2rem; margin: 28px 0 12px; }
    table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; overflow: hidden; }
    th, td { padding: 11px 14px; text-align: left; border-bottom: 1px solid #f1f5f9; font-size: 0.92rem; }
    th { background: #f1f5f9; font-weight: 600; color: #334155; }
    .badge { padding: 3px 10px; border-radius: 999px; font-size: 0.78rem; font-weight: 700; }
    .badge.ok { background: #dcfce7; color: #166534; }
    .badge.fail { background: #fee2e2; color: #991b1b; }
    .note { margin-top: 24px; padding: 16px; background: #eff6ff; border-left: 4px solid #2563eb; border-radius: 6px; font-size: 0.92rem; line-height: 1.5; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <h1>Relatório de Teste de Estresse — Seno</h1>
        <div class="sub">Cenário: ${TOTAL_ALUNOS} alunos simultâneos + 1 professor acompanhando | Gerado em ${new Date().toLocaleString('pt-BR')}</div>
      </div>
      <div class="status-banner ${todosSlosOk ? 'status-ok' : 'status-fail'}">
        ${todosSlosOk ? 'TODOS OS SLOs APROVADOS' : 'ATENÇÃO: SLO VIOLADO'}
      </div>
    </header>

    <div class="grid">
      <div class="card"><div class="label">Alunos Simulados</div><div class="value">${TOTAL_ALUNOS}</div></div>
      <div class="card"><div class="label">Entregas Confirmadas</div><div class="value">${entregas} / ${TOTAL_ALUNOS}</div></div>
      <div class="card"><div class="label">Sucesso Ponta a Ponta</div><div class="value">${taxaSucesso}%</div></div>
      <div class="card"><div class="label">Autosaves Gravados</div><div class="value">${autosaves}</div></div>
      <div class="card"><div class="label">Ensaios no Judge0</div><div class="value">${ensaios}</div></div>
      <div class="card"><div class="label">Degradações Judge0</div><div class="value">${degradacoes}</div></div>
      <div class="card"><div class="label">Requisições (${rpsMedio} req/s)</div><div class="value">${totalReqs}</div></div>
      <div class="card"><div class="label">Erros Internos (5xx)</div><div class="value" style="color:${erros5xx > 0 ? '#dc2626' : '#16a34a'}">${erros5xx}</div></div>
    </div>

    <h2>Latência por Etapa do Fluxo (API ↔ Monitor ↔ Judge0)</h2>
    <table>
      <thead>
        <tr>
          <th>Operação</th>
          <th>Média</th>
          <th>Mediana (p50)</th>
          <th>p90</th>
          <th>p95</th>
          <th>p99</th>
          <th>Máx</th>
        </tr>
      </thead>
      <tbody>
        ${linhasTabela}
      </tbody>
    </table>

    <h2>Critérios de Aceite (Thresholds / SLOs)</h2>
    <table>
      <thead>
        <tr><th>Métrica</th><th>Critério</th><th>Resultado</th></tr>
      </thead>
      <tbody>
        ${linhasSlo}
      </tbody>
    </table>

    <div class="note">
      <strong>Interpretação de Gargalos:</strong><br/>
      • Se <strong>Degradações Judge0 &gt; 0</strong> ou <code>seno_preavaliacao_ms</code> tiver p95 alto com <code>seno_autosave_ms</code> baixo, a API e o Monitor aguentaram folgados e o gargalo é exclusivamente a fila/workers do Judge0 (ajuste <code>MAX_QUEUE_SIZE=2000</code> em <code>judge0.conf</code> e escale <code>--scale workers=4</code>).<br/>
      • Se <code>seno_login_ms</code> subir muito no início, é o custo esperado de CPU do hash de senha sob login simultâneo.<br/>
      • Verifique também o dashboard interativo completo de séries temporais exportado pelo k6 em <code>dashboard-tempo-real.html</code>.
    </div>
  </div>
</body>
</html>`;
}

export function handleSummary(data) {
  const entregas = valCounter(data.metrics || {}, 'seno_entregas_confirmadas');
  const autosaves = valCounter(data.metrics || {}, 'seno_autosaves_ok');
  const ensaios = valCounter(data.metrics || {}, 'seno_ensaios_concluidos');
  const degradacoes = valCounter(data.metrics || {}, 'seno_avaliador_degradado_avisos');
  const erros5xx = valCounter(data.metrics || {}, 'seno_erros_5xx');

  const resumoTxt = [
    '',
    '========================================================================',
    ` RESUMO DO TESTE DE ESTRESSE SENO (${TOTAL_ALUNOS} ALUNOS + 1 PROFESSOR)`,
    '========================================================================',
    `  Entregas confirmadas : ${entregas} / ${TOTAL_ALUNOS}`,
    `  Autosaves concluídos : ${autosaves}`,
    `  Ensaios Judge0 OK    : ${ensaios}`,
    `  Avisos Judge0 cheio  : ${degradacoes} (degradação graciosa sem perda)`,
    `  Erros internos (5xx) : ${erros5xx}`,
    `  Relatório Executivo  : ${RESULTADOS_DIR}/relatorio-seno.html`,
    `  Dados Brutos (JSON)  : ${RESULTADOS_DIR}/resumo.json`,
    '========================================================================',
    '',
  ].join('\n');

  const saidas = {
    stdout: resumoTxt,
  };
  saidas[`${RESULTADOS_DIR}/relatorio-seno.html`] = gerarHTMLRelatorio(data);
  saidas[`${RESULTADOS_DIR}/resumo.json`] = JSON.stringify(data, null, 2);
  return saidas;
}
