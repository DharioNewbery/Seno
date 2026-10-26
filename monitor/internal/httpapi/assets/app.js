// app.js — Tela aprimorada de realização de atividades (Seno / Monitor).
// Isolada da interface do sistema: comunica-se apenas com /a via cookie hm_sess.
'use strict';

const MID = new URLSearchParams(location.search).get('anid') || '';
const WEB = new URLSearchParams(location.search).get('web') || '';
const $ = (id) => document.getElementById(id);
const conteudo = $('conteudo');
const modal = $('modal');
const barRodape = $('bar-rodape');

const estado = {
  carga: null,           // AtividadePublica sanitizada
  regras: null,          // regras (autocomplete, etc.)
  revisao: 0,
  restantes: null,       // segundos restantes
  respostas: {},         // ordem (string/number) → código
  preId: null,           // pré-avaliação ativa
  suja: false,
  bloqueada: null,       // detail de bloqueio
  tarefaAtiva: 1,        // ordem da tarefa ativa
  testeAtivo: 1,         // número/ordem do teste selecionado na aba do console
  resultados: {},        // ordem -> { status: 'vazio'|'executando'|'pronto'|'erro', run_id, resultados: [...] }
  instanciaEditor: null, // editor CodeMirror ativo
  consoleMinimizado: false,
  executandoTeste: false,
};

// -------------------------------------------------------- local autosave ----

const DB_NAME = 'seno_monitor_autosave';
const DB_VERSION = 1;
const STORE_NAME = 'rascunhos';

function abrirIDB() {
  return new Promise((resolve, reject) => {
    if (!('indexedDB' in window)) return reject(new Error('IndexedDB indisponível'));
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, { keyPath: 'mid' });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function salvarLocal(pendente) {
  if (!MID) return;
  try {
    const db = await abrirIDB();
    const tx = db.transaction(STORE_NAME, 'readwrite');
    tx.objectStore(STORE_NAME).put({
      mid: MID,
      revisao: estado.revisao,
      respostas: estado.respostas,
      gravado_em: new Date().toISOString(),
      pendente: Boolean(pendente),
    });
  } catch {
    // silenciosamente ignora indisponibilidade
  }
}

async function obterLocal() {
  if (!MID) return null;
  try {
    const db = await abrirIDB();
    return new Promise((resolve) => {
      const tx = db.transaction(STORE_NAME, 'readonly');
      const req = tx.objectStore(STORE_NAME).get(MID);
      req.onsuccess = () => resolve(req.result || null);
      req.onerror = () => resolve(null);
    });
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------- api ----

async function api(metodo, caminho, corpo) {
  const init = { method: metodo, headers: {} };
  if (corpo !== undefined) {
    init.body = JSON.stringify(corpo);
    init.headers['Content-Type'] = 'application/json';
  }
  let res;
  try {
    res = await fetch(`/a/atividade/${encodeURIComponent(MID)}${caminho}`, init);
  } catch {
    return { status: 0, json: { erro: { codigo: 'RETE', mensagem: 'Sem conexão com o servidor.' } } };
  }
  let json = null;
  try { json = await res.json(); } catch { /* corpo vazio */ }
  return { status: res.status, json: json ?? {} };
}

const detalhe = (r) => (r.json.erro && r.json.erro.detalhe) || {};
const mensagem = (r, padrao) => (r.json.erro && r.json.erro.mensagem) || padrao;

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function mmss(total) {
  const s = Math.max(0, Math.floor(total));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const seg = s % 60;
  if (h > 0) {
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(seg).padStart(2, '0')}`;
  }
  return `${String(m).padStart(2, '0')}:${String(seg).padStart(2, '0')}`;
}

function linkExterno(caminho) {
  if (WEB) {
    return WEB.replace(/\/$/, '') + (caminho.startsWith('/') ? '' : '/') + caminho;
  }
  return null;
}

// ------------------------------------------------------------- telas especiais ----

function telaErro(titulo, msg, extraHTML = '') {
  cronometroOff();
  if (barRodape) barRodape.hidden = true;
  conteudo.innerHTML = `
    <div class="tarja-especial fim">
      <h2>${esc(titulo)}</h2>
      <p>${esc(msg)}</p>
      ${extraHTML}
      <p class="mudo">Feche esta tela e volte ao portal para retomar.</p>
    </div>`;
}

function telaFinalizada(msg, det) {
  cronometroOff();
  if (barRodape) barRodape.hidden = true;
  const url = det && det.correcao_url ? linkExterno(det.correcao_url) : null;
  conteudo.innerHTML = `
    <div class="tarja-especial">
      <h2>Entrega Finalizada</h2>
      <p>${esc(msg || 'Sua atividade foi entregue com sucesso e está encerrada.')}</p>
      ${url ? `<p><a href="${esc(url)}" class="btn btn-primario">Ver entregas e correções no portal →</a></p>` : ''}
      <p class="mudo">Você já pode fechar esta tela com segurança.</p>
    </div>`;
}

function telaEncerrada(msg) {
  cronometroOff();
  if (barRodape) barRodape.hidden = true;
  conteudo.innerHTML = `
    <div class="tarja-especial fim">
      <h2>Janela Encerrada</h2>
      <p>${esc(msg)}</p>
      <p class="mudo">Seu rascunho está preservado. Entre em contato com seu professor.</p>
    </div>`;
}

// ----------------------------------------------------------- cronômetro ----

let cronoTimer = null;
function cronometroOn(segundos) {
  estado.restantes = segundos;
  const el = $('crono');
  if (!el) return;
  el.hidden = false;
  clearInterval(cronoTimer);

  const atualizaVisual = () => {
    el.innerHTML = `<span aria-hidden="true">⏱</span> ${mmss(estado.restantes)}`;
    el.classList.toggle('pouco', estado.restantes <= 180);
  };

  atualizaVisual();
  cronoTimer = setInterval(() => {
    if (estado.restantes === null) return;
    estado.restantes = Math.max(0, estado.restantes - 1);
    atualizaVisual();
    if (estado.restantes === 0) {
      clearInterval(cronoTimer);
      atualizarBadgeSalvo('aviso', 'prazo encerrado');
    }
  }, 1000);
}

function cronometroOff() {
  clearInterval(cronoTimer);
  const el = $('crono');
  if (el) el.hidden = true;
}

// ------------------------------------------------------------- status e vereditos ----

function decodificaVeredito(v) {
  if (v === 'accepted') return { cls: 'passou', curto: 'Passou', texto: 'Passou (Correto)' };
  const m = /^judge0:(\d+)$/.exec(String(v));
  const n = m ? Number(m[1]) : 4;
  const T = {
    4: ['Resposta incorreta', 'Resposta incorreta — a saída obtida foi diferente da esperada'],
    5: ['Tempo esgotado', 'Tempo esgotado — a execução ultrapassou o limite de CPU'],
    6: ['Erro de compilação', 'Erro de compilação — o código não pôde ser compilado'],
    7: ['Erro de execução', 'Erro de execução — Falha de segmentação (SIGSEGV)'],
    8: ['Erro de execução', 'Erro de execução — Limite de arquivo excedido (SIGXFSZ)'],
    9: ['Memória esgotada', 'Erro de execução — Limite de memória excedido (SIGKILL)'],
    10: ['Erro de runtime', 'Erro de execução — O programa terminou com código de erro'],
    11: ['Falha interna', 'Falha temporária no avaliador. Tente novamente.'],
    12: ['Formato inválido', 'Erro de execução — Formato de executável inválido'],
    13: ['Falha interna', 'Falha temporária no avaliador. Tente novamente.'],
    14: ['Erro de runtime', 'Erro de execução — O programa abortou durante execução'],
  };
  const [curto, texto] = T[n] || ['Falhou', 'Falha na avaliação'];
  return { cls: 'falhou', curto, texto };
}

function atualizarBadgeSalvo(tipo, texto) {
  const badge = $('salvo-badge');
  const txt = $('salvo');
  const icone = $('salvo-icone');
  if (!badge || !txt) return;

  badge.className = 'salvo-badge';
  txt.textContent = texto;

  if (tipo === 'salvando') {
    badge.classList.add('salvando');
    icone.textContent = '🔄';
  } else if (tipo === 'sucesso') {
    badge.classList.add('sucesso');
    icone.textContent = '☁';
  } else if (tipo === 'offline') {
    badge.classList.add('offline');
    icone.textContent = '🟡';
  } else if (tipo === 'erro') {
    badge.classList.add('erro');
    icone.textContent = '⚠️';
  } else {
    icone.textContent = '☁';
  }
}

// ----------------------------------------------------------- abertura ----

async function abrir() {
  if (!MID) {
    telaErro('Atividade não informada', 'Abra a atividade a partir do portal do aluno.');
    return;
  }
  const r = await api('POST', '/abrir');
  switch (r.status) {
    case 200: {
      const atv = r.json.atividade;
      const tent = r.json.tentativa;
      estado.carga = atv;
      estado.regras = r.json.regras || {};
      estado.revisao = tent.revisao;
      estado.respostas = tent.respostas || {};

      const local = await obterLocal();
      if (local && local.pendente && local.revisao >= estado.revisao) {
        estado.respostas = Object.assign({}, estado.respostas, local.respostas);
        estado.revisao = local.revisao;
      }

      if (atv.tarefas && atv.tarefas.length > 0) {
        estado.tarefaAtiva = atv.tarefas[0].ordem;
      }

      cronometroOn(tent.segundos_restantes);
      atualizarBadgeSalvo('sucesso', 'conectado');

      if (barRodape) barRodape.hidden = false;
      renderizarTudo();
      agendarAutosave();
      carregarHistoricoGlobal();
      return;
    }
    case 403:
      telaFinalizada(mensagem(r, 'Atividade encerrada para você.'), detalhe(r));
      return;
    case 409:
      telaErro('Avaliação ainda não iniciada',
        `A janela de avaliação começa em ${detalhe(r).segundos_para_inicio ?? '?'} segundos.`);
      return;
    case 410:
      telaEncerrada(mensagem(r, 'A janela desta atividade está encerrada.'));
      return;
    case 401:
      telaErro('Sessão expirada ou inválida', 'Retorne ao portal do aluno para entrar novamente.');
      return;
    default:
      telaErro('Erro ao carregar', mensagem(r, 'Falha inesperada no servidor.'));
  }
}

// ---------------------------------------------------------- autosave ----

let autosaver = null;
function agendarAutosave() {
  clearInterval(autosaver);
  autosaver = setInterval(salvar, 20000);
  window.addEventListener('beforeunload', salvar);
  window.addEventListener('online', salvar);
}

async function salvar() {
  if (!estado.suja) return;
  salvarLocal(true);
  atualizarBadgeSalvo('salvando', 'salvando…');

  const r = await api('PUT', '/tentativa', {
    revisao: estado.revisao,
    respostas: estado.respostas,
  });

  if (r.status === 200) {
    estado.suja = false;
    estado.revisao = r.json.revisao;
    salvarLocal(false);
    atualizarBadgeSalvo('sucesso', `salvo às ${new Date().toLocaleTimeString('pt-BR')}`);
    atualizarProgressoGlobal();
    return;
  }
  if (r.status === 0) {
    atualizarBadgeSalvo('offline', 'salvo localmente (offline)');
    return;
  }
  if (r.status === 409 && detalhe(r).revisao_atual !== undefined) {
    const atual = detalhe(r).revisao_atual;
    const r2 = await api('PUT', '/tentativa', { revisao: atual, respostas: estado.respostas });
    if (r2.status === 200) {
      estado.suja = false;
      estado.revisao = r2.json.revisao;
      salvarLocal(false);
      atualizarBadgeSalvo('sucesso', 'salvo (sincronizado)');
    } else {
      estado.suja = true;
      atualizarBadgeSalvo('erro', 'conflito entre abas');
    }
    return;
  }
  if (r.status >= 400) {
    estado.suja = true;
    atualizarBadgeSalvo('erro', 'falha ao salvar');
    if (r.status === 410) { cronometroZerou(); }
    if (r.status === 403) { telaFinalizada(mensagem(r, 'Atividade encerrada.'), detalhe(r)); }
  }
}

function cronometroZerou() {
  cronometroOff();
  telaEncerrada('O tempo limite encerrou e a submissão não está mais disponível.');
}

// ---------------------------------------------------------- renderização ----

function obterTarefaAtual() {
  if (!estado.carga || !estado.carga.tarefas) return null;
  return estado.carga.tarefas.find((t) => t.ordem === estado.tarefaAtiva) || estado.carga.tarefas[0];
}

function renderizarTudo() {
  const atv = estado.carga;
  if (!atv) return;

  $('titulo').textContent = atv.nome || 'Seno';

  // Renderiza a navegação de abas no header
  renderizarAbasHeader();

  // Renderiza a estrutura Split View dentro de #conteudo
  conteudo.innerHTML = `
    <div class="split-view">
      <div id="painel-esquerdo" class="painel-esquerdo"></div>
      <div id="painel-direito" class="painel-direito">
        <div class="editor-container">
          <div class="editor-barra-topo">
            <span id="editor-ling-badge" class="editor-ling-badge"></span>
            <button id="btn-historico" type="button" class="btn-link" title="Consultar testes já realizados nesta tarefa">
              ↺ Ensaios anteriores
            </button>
          </div>
          <div id="editor-wrap" class="editor-wrap"></div>
        </div>
        <div id="console-container" class="console-container">
          <div class="console-topo">
            <div id="console-abas" class="console-abas"></div>
            <div class="console-direita">
              <span id="console-status-resumo" class="console-status-resumo"></span>
              <button id="btn-console-toggle" type="button" class="btn-icone-min" title="Minimizar / Expandir console">▾</button>
            </div>
          </div>
          <div id="console-corpo" class="console-corpo"></div>
        </div>
      </div>
    </div>
  `;

  $('btn-historico').addEventListener('click', () => abrirHistorico(estado.tarefaAtiva));
  $('btn-console-toggle').addEventListener('click', toggleConsole);

  // Botões do Rodapé
  const btnTestar = $('btn-rodape-testar');
  const btnAvaliar = $('btn-rodape-avaliar');
  if (btnTestar) {
    btnTestar.onclick = () => testar(estado.tarefaAtiva);
  }
  if (btnAvaliar) {
    btnAvaliar.onclick = preAvaliar;
  }

  // Renderiza o conteúdo da tarefa atual
  trocarParaTarefa(estado.tarefaAtiva);
  atualizarProgressoGlobal();
}

function renderizarAbasHeader() {
  const nav = $('tarefas-nav');
  if (!nav || !estado.carga) return;

  nav.innerHTML = estado.carga.tarefas.map((t) => {
    const ativa = t.ordem === estado.tarefaAtiva ? ' ativa' : '';
    const dotClass = obterClasseStatusDot(t.ordem);
    return `
      <button type="button" class="aba-tarefa${ativa}" data-tarefa="${t.ordem}" title="Ir para tarefa ${t.ordem}: ${esc(t.nome)}">
        <span class="status-dot ${dotClass}"></span>
        <span>${t.ordem}. ${esc(t.nome)}</span>
      </button>
    `;
  }).join('');

  nav.querySelectorAll('[data-tarefa]').forEach((btn) => {
    btn.onclick = () => {
      const ordem = Number(btn.dataset.tarefa);
      if (ordem !== estado.tarefaAtiva) {
        trocarParaTarefa(ordem);
      }
    };
  });
}

function obterClasseStatusDot(ordem) {
  const res = estado.resultados[ordem];
  if (res && res.status === 'pronto') {
    const total = res.resultados.length;
    const acertos = res.resultados.filter((x) => x.veredito === 'accepted').length;
    return acertos === total ? 'passou' : 'falhou';
  }
  const codigo = (estado.respostas[String(ordem)] || '').trim();
  return codigo.length > 0 ? 'editando' : 'vazio';
}

function trocarParaTarefa(ordem) {
  // Salva o código da tarefa anterior se o editor estiver montado
  if (estado.instanciaEditor) {
    estado.respostas[String(estado.tarefaAtiva)] = estado.instanciaEditor.getValor();
    estado.instanciaEditor.destruir();
    estado.instanciaEditor = null;
  }

  estado.tarefaAtiva = ordem;
  const tarefa = obterTarefaAtual();
  if (!tarefa) return;

  // Atualiza abas no header
  const nav = $('tarefas-nav');
  if (nav) {
    nav.querySelectorAll('.aba-tarefa').forEach((btn) => {
      btn.classList.toggle('ativa', Number(btn.dataset.tarefa) === ordem);
    });
  }

  // 1. Painel Esquerdo (Enunciado, Limites e Exemplos)
  const painelEsq = $('painel-esquerdo');
  if (painelEsq) {
    const atv = estado.carga;
    const cpu = tarefa.limites?.tempo_cpu_ms ? `${tarefa.limites.tempo_cpu_ms} ms CPU` : '2000 ms CPU';
    const ram = tarefa.limites?.memoria_mb ? `${tarefa.limites.memoria_mb} MB RAM` : '256 MB RAM';

    painelEsq.innerHTML = `
      ${atv.enunciado ? `
        <details class="card-instrucoes-gerais">
          <summary>Instruções Gerais da Atividade</summary>
          <div class="corpo-instrucoes">${esc(atv.enunciado)}</div>
        </details>` : ''}
      <div class="tarefa-cabecalho">
        <h2 class="tarefa-titulo">${tarefa.ordem}. ${esc(tarefa.nome)}</h2>
        <div class="tarefa-meta">
          <span class="tag tag-pts">⭐ ${tarefa.valor_pts} pts</span>
          <span class="tag tag-ling">🐍 ${esc(tarefa.linguagem)}</span>
          <span class="tag">⏱ ${cpu}</span>
          <span class="tag">💾 ${ram}</span>
        </div>
      </div>
      <div class="enunciado-tarefa">${esc(tarefa.enunciado || 'Sem enunciado cadastrado.')}</div>
      ${renderizarExemplos(tarefa)}
    `;
  }

  // 2. Editor de Código
  $('editor-ling-badge').textContent = tarefa.linguagem || 'Código';
  const wrap = $('editor-wrap');
  wrap.innerHTML = '';

  const codigoSalvo = estado.respostas[String(ordem)] ?? '';
  const autocomplete = Boolean(estado.regras && estado.regras.autocomplete);

  if (window.SenoEditor && window.SenoEditor.criar) {
    estado.instanciaEditor = window.SenoEditor.criar(wrap, {
      valor: codigoSalvo,
      linguagem: tarefa.linguagem,
      autocomplete,
      onchange: (novoValor) => {
        estado.respostas[String(ordem)] = novoValor;
        estado.suja = true;
        salvarLocal(true);
        atualizarProgressoGlobal();
      },
    });
  } else {
    const ta = document.createElement('textarea');
    ta.className = 'campo-fallback';
    ta.value = codigoSalvo;
    ta.autocomplete = 'off';
    ta.spellcheck = false;
    ta.placeholder = `Escreva seu código ${tarefa.linguagem} aqui…`;
    ta.addEventListener('input', () => {
      estado.respostas[String(ordem)] = ta.value;
      estado.suja = true;
      salvarLocal(true);
      atualizarProgressoGlobal();
    });
    wrap.appendChild(ta);
    estado.instanciaEditor = {
      getValor: () => ta.value,
      setValor: (v) => { ta.value = v; },
      destruir: () => {},
    };
  }

  // 3. Reset da aba de teste ativo
  if (tarefa.testes && tarefa.testes.length > 0) {
    const existe = tarefa.testes.some((te) => te.ordem === estado.testeAtivo);
    if (!existe) estado.testeAtivo = tarefa.testes[0].ordem;
  }

  // 4. Renderiza Console de Testes
  renderizarConsoleTestes();
}

function renderizarExemplos(tarefa) {
  if (!tarefa.testes || !tarefa.testes.length) return '';
  return `
    <div class="secao-testes-exemplos">
      <h3 class="secao-titulo">Casos de Teste Públicos (Exemplos)</h3>
      ${tarefa.testes.map((te) => `
        <div class="exemplo-card">
          <div class="exemplo-card-topo">Caso #${te.ordem}</div>
          <div class="exemplo-grid">
            <div class="exemplo-bloco">
              <span class="exemplo-rotulo">Entrada</span>
              <pre>${esc(te.stdin || '(vazio)')}</pre>
            </div>
            <div class="exemplo-bloco">
              <span class="exemplo-rotulo">Saída esperada</span>
              <pre>${esc(te.stdout_esperado || '(vazio)')}</pre>
            </div>
          </div>
        </div>
      `).join('')}
    </div>
  `;
}

// ----------------------------------------------------- console de testes ----

function toggleConsole() {
  const c = $('console-container');
  const btn = $('btn-console-toggle');
  if (!c || !btn) return;
  estado.consoleMinimizado = !estado.consoleMinimizado;
  c.classList.toggle('minimizado', estado.consoleMinimizado);
  btn.textContent = estado.consoleMinimizado ? '▴' : '▾';
}

function renderizarConsoleTestes() {
  const tarefa = obterTarefaAtual();
  const abasEl = $('console-abas');
  const corpoEl = $('console-corpo');
  const resumoEl = $('console-status-resumo');
  if (!tarefa || !abasEl || !corpoEl) return;

  const testes = tarefa.testes || [];
  const dadosRun = estado.resultados[tarefa.ordem];

  // Resumo
  if (resumoEl) {
    if (estado.executandoTeste && estado.tarefaAtiva === tarefa.ordem) {
      resumoEl.innerHTML = `<span style="color:var(--primario)">Executando no Judge0…</span>`;
    } else if (dadosRun && dadosRun.status === 'pronto') {
      const acertos = dadosRun.resultados.filter((x) => x.veredito === 'accepted').length;
      const total = dadosRun.resultados.length;
      const cls = acertos === total ? 'passou' : 'falhou';
      resumoEl.innerHTML = `<span class="${cls}">${acertos}/${total} passaram</span>`;
    } else {
      resumoEl.textContent = testes.length ? `${testes.length} teste(s) disponível(is)` : 'Sem testes';
    }
  }

  // Abas de Casos de Teste
  if (!testes.length) {
    abasEl.innerHTML = '<span class="aba-teste ativa">Sem testes públicos</span>';
    corpoEl.innerHTML = '<div class="resultado-vazio">Esta tarefa não possui testes públicos cadastrados.</div>';
    return;
  }

  const resultadosPorOrdem = {};
  if (dadosRun && dadosRun.resultados) {
    dadosRun.resultados.forEach((r) => { resultadosPorOrdem[r.ordem] = r; });
  }

  abasEl.innerHTML = testes.map((te) => {
    const ativa = te.ordem === estado.testeAtivo ? ' ativa' : '';
    const r = resultadosPorOrdem[te.ordem];
    let tag = '';
    if (r) {
      const d = decodificaVeredito(r.veredito);
      tag = `<span class="aba-teste-tag ${d.cls}">${d.cls === 'passou' ? '✓' : '✗'}</span>`;
    }
    return `
      <button type="button" class="aba-teste${ativa}" data-teste="${te.ordem}">
        ${tag} Teste #${te.ordem}
      </button>
    `;
  }).join('');

  abasEl.querySelectorAll('[data-teste]').forEach((btn) => {
    btn.onclick = () => {
      estado.testeAtivo = Number(btn.dataset.teste);
      renderizarConsoleTestes();
    };
  });

  // Corpo do Teste Selecionado
  const testeAtual = testes.find((t) => t.ordem === estado.testeAtivo) || testes[0];
  const resultadoAtual = resultadosPorOrdem[testeAtual.ordem];

  if (estado.executandoTeste && estado.tarefaAtiva === tarefa.ordem) {
    corpoEl.innerHTML = `
      <div class="resultado-executando">
        <div class="spinner"></div>
        <span>Compilando e executando testes no Judge0…</span>
      </div>`;
    return;
  }

  if (!resultadoAtual) {
    corpoEl.innerHTML = `
      <div class="comparador-grid">
        <div class="comparador-bloco">
          <span class="comparador-rotulo">Entrada (stdin)</span>
          <pre class="comparador-pre">${esc(testeAtual.stdin || '(vazio)')}</pre>
        </div>
        <div class="comparador-bloco">
          <span class="comparador-rotulo">Saída esperada (stdout)</span>
          <pre class="comparador-pre">${esc(testeAtual.stdout_esperado || '(vazio)')}</pre>
        </div>
      </div>
      <p class="mudo" style="margin-top:12px;font-size:12px;">Clique em "Testar Código" para avaliar seu algoritmo contra este caso.</p>
    `;
    return;
  }

  const d = decodificaVeredito(resultadoAtual.veredito);
  const tempo = Number(resultadoAtual.tempo ?? 0).toFixed(2);
  const mem = resultadoAtual.memoria ?? 0;

  let detalheErroHTML = '';
  if (resultadoAtual.compilacao) {
    detalheErroHTML = `
      <div class="compilador-aviso">
        <b>Mensagem do Compilador:</b>
        <pre>${esc(resultadoAtual.compilacao)}</pre>
      </div>`;
  } else if (resultadoAtual.stderr) {
    detalheErroHTML = `
      <div class="compilador-aviso">
        <b>Saída de Erro (stderr):</b>
        <pre>${esc(resultadoAtual.stderr)}</pre>
      </div>`;
  }

  corpoEl.innerHTML = `
    <div class="veredito-banner ${d.cls}">
      <span>${d.cls === 'passou' ? '✓' : '✗'} ${esc(d.texto)}</span>
      <span class="veredito-detalhes">${tempo}s · ${mem} KB</span>
    </div>
    <div class="comparador-grid">
      <div class="comparador-bloco">
        <span class="comparador-rotulo">Entrada</span>
        <pre class="comparador-pre">${esc(testeAtual.stdin || '(vazio)')}</pre>
      </div>
      <div class="comparador-bloco">
        <span class="comparador-rotulo">Saída esperada</span>
        <pre class="comparador-pre">${esc(testeAtual.stdout_esperado || '(vazio)')}</pre>
      </div>
      <div class="comparador-bloco">
        <span class="comparador-rotulo">Sua Saída</span>
        <pre class="comparador-pre ${d.cls === 'falhou' ? 'erro-saida' : ''}">${esc(resultadoAtual.stdout_obtido || '(vazio)')}</pre>
      </div>
    </div>
    ${detalheErroHTML}
  `;
}

// ------------------------------------------------------------- ensaios ----

async function testar(ordem) {
  if (estado.executandoTeste) return;
  const t = estado.carga.tarefas.find((x) => x.ordem === ordem);
  if (!t) return;

  // Garante que o editor salvou o valor recente no estado
  if (estado.instanciaEditor && estado.tarefaAtiva === ordem) {
    estado.respostas[String(ordem)] = estado.instanciaEditor.getValor();
  }

  const codigo = estado.respostas[String(ordem)] ?? '';
  estado.executandoTeste = true;
  const btnTestar = $('btn-rodape-testar');
  if (btnTestar) {
    btnTestar.disabled = true;
    btnTestar.innerHTML = `<span class="spinner" style="width:14px;height:14px;border-width:2px;"></span> Executando…`;
  }

  // Abre o console se estiver minimizado
  if (estado.consoleMinimizado) toggleConsole();
  renderizarConsoleTestes();

  const r = await api('POST', '/testar', { ordem, linguagem: t.linguagem, codigo });
  if (r.status !== 202) {
    estado.executandoTeste = false;
    if (btnTestar) {
      btnTestar.disabled = false;
      btnTestar.innerHTML = `<span class="icone-btn">▶</span> Testar Código <kbd class="atalho-teclado">Ctrl+Enter</kbd>`;
    }
    const msg = r.status === 429
      ? 'Limite de 5 ensaios por minuto atingido. Aguarde alguns segundos.'
      : mensagem(r, 'Não foi possível enviar o ensaio.');
    aviso(msg);
    renderizarConsoleTestes();
    return;
  }

  const run = r.json.run_id;

  // Polling pelo resultado
  for (let seg = 0; seg < 60; seg++) {
    await new Promise((res) => setTimeout(res, 1200));
    const q = await api('GET', `/testar/${encodeURIComponent(run)}`);
    if (q.status === 200) {
      estado.executandoTeste = false;
      if (btnTestar) {
        btnTestar.disabled = false;
        btnTestar.innerHTML = `<span class="icone-btn">▶</span> Testar Código <kbd class="atalho-teclado">Ctrl+Enter</kbd>`;
      }
      estado.resultados[ordem] = {
        status: 'pronto',
        run_id: run,
        resultados: q.json.resultados,
      };

      // Foca automaticamente no primeiro teste com falha (se houver)
      const primeiroErro = q.json.resultados.find((x) => x.veredito !== 'accepted');
      if (primeiroErro) {
        estado.testeAtivo = primeiroErro.ordem;
      }

      renderizarAbasHeader();
      renderizarConsoleTestes();
      atualizarProgressoGlobal();
      return;
    }
    if (q.status >= 400) {
      estado.executandoTeste = false;
      if (btnTestar) {
        btnTestar.disabled = false;
        btnTestar.innerHTML = `<span class="icone-btn">▶</span> Testar Código <kbd class="atalho-teclado">Ctrl+Enter</kbd>`;
      }
      aviso('Não foi possível obter o resultado do teste.');
      renderizarConsoleTestes();
      return;
    }
  }

  estado.executandoTeste = false;
  if (btnTestar) {
    btnTestar.disabled = false;
    btnTestar.innerHTML = `<span class="icone-btn">▶</span> Testar Código <kbd class="atalho-teclado">Ctrl+Enter</kbd>`;
  }
  aviso('Execução demorada. O resultado aparecerá no histórico em instantes.');
  renderizarConsoleTestes();
}

// ------------------------------------------------------------- histórico ----

async function abrirHistorico(ordem) {
  const r = await api('GET', '/testar');
  if (r.status !== 200) {
    aviso('Não foi possível carregar o histórico de ensaios.');
    return;
  }
  const runs = (r.json.runs || []).filter((x) => x.ordem === Number(ordem));
  modal.innerHTML = `
    <h2>Ensaios Anteriores — Tarefa ${ordem}</h2>
    ${runs.length ? runs.map((rh, idx) => {
      const dataHora = new Date(rh.criada_em).toLocaleString('pt-BR');
      const passados = rh.resultados.filter((x) => x.veredito === 'accepted').length;
      const total = rh.resultados.length;
      return `
        <div class="hist-item">
          <div class="hist-item-topo">
            <b>${esc(dataHora)}</b>
            <span class="${passados === total ? 'passou' : 'falhou'}">${passados}/${total} passaram</span>
          </div>
          <div class="modal-testes-chips">
            ${rh.resultados.map((x) => `
              <span class="chip-teste ${x.veredito === 'accepted' ? 'passou' : 'falhou'}">
                #${x.ordem} ${esc(decodificaVeredito(x.veredito).curto)}
              </span>
            `).join('')}
          </div>
          <pre>${esc(rh.codigo || '(código vazio)')}</pre>
          <div style="margin-top:8px;text-align:right;">
            <button type="button" class="btn btn-secundario" data-restaurar="${idx}" style="font-size:11.5px;padding:4px 8px;">
              Restaurar no editor
            </button>
          </div>
        </div>`;
    }).join('') : '<p class="mudo">Nenhum ensaio registrado para esta tarefa ainda.</p>'}
    <div class="modal-acoes">
      <button id="btn-hist-fechar" type="button" class="btn btn-secundario">Fechar</button>
    </div>
  `;

  runs.forEach((rh, idx) => {
    const btn = modal.querySelector(`[data-restaurar="${idx}"]`);
    if (btn) {
      btn.onclick = () => {
        if (estado.instanciaEditor && estado.tarefaAtiva === ordem) {
          estado.instanciaEditor.setValor(rh.codigo || '');
          estado.respostas[String(ordem)] = rh.codigo || '';
          estado.suja = true;
          salvarLocal(true);
          atualizarProgressoGlobal();
        }
        modal.close();
      };
    }
  });

  $('btn-hist-fechar').onclick = () => modal.close();
  modal.showModal();
}

async function carregarHistoricoGlobal() {
  const r = await api('GET', '/testar');
  if (r.status !== 200) return;
  // Pré-alimenta o status com a última execução válida de cada tarefa
  const porTarefa = {};
  (r.json.runs || []).forEach((run) => {
    if (!porTarefa[run.ordem] || new Date(run.criada_em) > new Date(porTarefa[run.ordem].criada_em)) {
      porTarefa[run.ordem] = run;
    }
  });
  Object.keys(porTarefa).forEach((ordemStr) => {
    const ordem = Number(ordemStr);
    if (!estado.resultados[ordem]) {
      estado.resultados[ordem] = {
        status: 'pronto',
        run_id: porTarefa[ordem].run_id || '',
        resultados: porTarefa[ordem].resultados,
      };
    }
  });
  renderizarAbasHeader();
  renderizarConsoleTestes();
}

// ------------------------------------------------------------- submissão ----

async function preAvaliar() {
  if (estado.instanciaEditor) {
    estado.respostas[String(estado.tarefaAtiva)] = estado.instanciaEditor.getValor();
  }
  await salvar();

  const btnAvaliar = $('btn-rodape-avaliar');
  if (btnAvaliar) {
    btnAvaliar.disabled = true;
    btnAvaliar.textContent = 'Avaliando…';
  }

  const r = await api('POST', '/submissao/avaliar');
  if (btnAvaliar) {
    btnAvaliar.disabled = false;
    btnAvaliar.textContent = 'Pré-avaliar e Entregar';
  }

  if (r.status !== 200) {
    if (r.status === 403) { telaFinalizada(mensagem(r, 'Encerrada.'), detalhe(r)); return; }
    if (r.status === 410) { telaEncerrada(mensagem(r, 'A janela está encerrada.')); return; }
    aviso(mensagem(r, 'Falha na pré-avaliação.'));
    return;
  }

  const rel = r.json;
  estado.preId = rel.pre_id;
  const ttlSeg = parseInt(String(rel.expira_em || '0'), 10);

  modal.innerHTML = `
    <h2>Relatório de Pré-Avaliação</h2>
    <p>
      Acertos: <b>${rel.resumo.acertos}</b> de <b>${rel.resumo.total}</b> testes públicos.
      Válido por <span id="ttl" class="modal-ttl">${mmss(ttlSeg)}</span>
    </p>
    <div class="modal-tarefas-lista">
      ${rel.tarefas.map((t) => `
        <div class="modal-tarefa-card">
          <div class="modal-tarefa-card-topo">
            <span>Tarefa ${t.ordem}</span>
            <span class="${t.veredito === 'accepted' ? 'passou' : 'falhou'}">
              ${t.veredito === 'accepted' ? '✓ Todos os testes passaram' : '✗ Falhas encontradas'}
            </span>
          </div>
          <div class="modal-testes-chips">
            ${(t.testes || []).map((x) => `
              <span class="chip-teste ${x.status === 'accepted' ? 'passou' : 'falhou'}">
                #${x.ordem} ${esc(decodificaVeredito(x.status).curto)}
              </span>
            `).join('')}
          </div>
        </div>
      `).join('')}
    </div>
    <p class="mudo" style="font-size:12px;margin-bottom:0;">
      Atenção: A confirmação encerra sua tentativa nesta atividade. Se desejar corrigir algo, clique em "Voltar e editar".
    </p>
    <div class="modal-acoes">
      <button id="btn-cancel" type="button" class="btn btn-secundario">Voltar e editar</button>
      <button id="btn-conf" type="button" class="btn btn-primario">Confirmar Envio Definitivo</button>
    </div>
  `;
  modal.showModal();

  let restTtl = ttlSeg;
  const ttlTimer = setInterval(() => {
    restTtl--;
    const el = $('ttl');
    if (el) el.textContent = mmss(restTtl);
    if (restTtl <= 0) {
      clearInterval(ttlTimer);
      estado.preId = null;
      aviso('A pré-avaliação expirou. Avalie novamente antes de enviar.');
      modal.close();
    }
  }, 1000);

  $('btn-cancel').onclick = async () => {
    clearInterval(ttlTimer);
    await api('POST', '/submissao/cancelar', { pre_id: estado.preId });
    estado.preId = null;
    modal.close();
  };

  $('btn-conf').onclick = () => confirmarPre(ttlTimer);
}

async function confirmarPre(ttlTimer) {
  clearInterval(ttlTimer);
  const btnConf = $('btn-conf');
  if (btnConf) {
    btnConf.disabled = true;
    btnConf.textContent = 'Enviando…';
  }

  const r = await api('POST', '/submissao/confirmar', { pre_id: estado.preId });
  estado.preId = null;
  modal.close();

  if (r.status === 200) {
    const r2 = await api('POST', '/abrir');
    if (r2.status === 200) {
      const atv = r2.json.atividade;
      const tent = r2.json.tentativa;
      estado.revisao = tent.revisao;
      estado.respostas = tent.respostas || {};
      renderizarTudo();
      aviso('Entrega registrada com sucesso!');
      return;
    }
    telaFinalizada(mensagem(r2, 'Sua entrega foi confirmada com sucesso.'), detalhe(r2));
    return;
  }

  switch (r.status) {
    case 409:
      if (r.json.erro && r.json.erro.codigo === 'ATUALIZADA') {
        aviso('Suas respostas mudaram desde a pré-avaliação. Avalie novamente.');
      } else {
        aviso(mensagem(r, 'Pré-avaliação expirou. Reavalie para confirmar.'));
      }
      return;
    case 410:
      cronometroOff();
      telaEncerrada('A janela fechou antes da confirmação. Seu rascunho foi salvo.');
      return;
    case 403:
      telaFinalizada(mensagem(r, 'Encerrada.'), detalhe(r));
      return;
    default:
      aviso(mensagem(r, 'Falha ao confirmar envio.'));
  }
}

// ------------------------------------------------------------- utilitários ----

function atualizarProgressoGlobal() {
  if (!estado.carga || !estado.carga.tarefas) return;
  const tarefas = estado.carga.tarefas;
  const total = tarefas.length;
  const preenchidas = tarefas.filter((t) => (estado.respostas[String(t.ordem)] || '').trim().length > 0).length;

  const txt = $('progresso-texto');
  const bar = $('progresso-barra');
  if (txt) txt.textContent = `Progresso: ${preenchidas} de ${total} tarefas`;
  if (bar) {
    const pct = total > 0 ? Math.round((preenchidas / total) * 100) : 0;
    bar.style.width = `${pct}%`;
  }
  renderizarAbasHeader();
}

function aviso(msg) {
  atualizarBadgeSalvo('aviso', msg);
}

// ------------------------------------------------------------ atalhos teclado ----

window.addEventListener('keydown', (e) => {
  // Ctrl+Enter ou Cmd+Enter para testar código da tarefa ativa
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault();
    testar(estado.tarefaAtiva);
  }
  // Ctrl+S ou Cmd+S para forçar autosave
  if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
    e.preventDefault();
    salvar();
  }
});

// ---------------------------------------------------------------- boot ----

abrir();
