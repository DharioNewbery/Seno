// app.js — tela de realização de atividades (Seno/Monitor).
// Isolada da interface do sistema: só conversa com os endpoints /a do
// Monitor, via cookie hm_sess. Sem frameworks, sem Monaco na v0
// (shortcut: editor simples; Monaco entra como asset quando
// necessidade for calibrada — PLANO_MONITOR Fase 8/§Front).
'use strict';

const MID = new URLSearchParams(location.search).get('anid') || '';
const WEB = new URLSearchParams(location.search).get('web') || '';
const $ = (id) => document.getElementById(id);
const conteudo = $('conteudo');
const modal = $('modal');

const estado = {
  carga: null,      // AtividadePublica sanitizada
  regras: null,     // regras com autocomplete
  revisao: 0,
  restantes: null,  // segundos
  respostas: {},    // ordem → código
  preId: null,      // pré-avaliação viva (opcional)
  suja: false,
  bloqueda: null,   // detail {correcao_url} do bloqueio final
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
    // silenciosamente ignora indisponibilidade de idb
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

// ------------------------------------------------------------- telas ----

function telaErro(titulo, msg, extraHTML = '') {
  cronometroOff();
  conteudo.innerHTML = `
    <div class="tarja fim">
      <h2>${esc(titulo)}</h2><p>${esc(msg)}</p>${extraHTML}
      <p class="mudo">Feche esta tela e volte ao portal para retomar.</p>
    </div>`;
}

function telaFinalizada(msg, det) {
  cronometroOff();
  const url = det && det.correcao_url ? linkExterno(det.correcao_url) : null;
  conteudo.innerHTML = `
    <div class="tarja fim">
      <h2>Entrega finalizada</h2>
      <p>${esc(msg || 'Sua atividade foi entregue e está fechada.')}</p>
      ${url ? `<p><a href="${esc(url)}">Ver entregas e correções no portal →</a></p>` : ''}
      <p class="mudo">Pode fechar esta tela.</p>
    </div>`;
}

function linkExterno(caminho) {
  // correcao_url é relativa ao /web: base vem da abertura servida (?web=)
  if (WEB) {
    return WEB.replace(/\/$/, '') + (caminho.startsWith('/') ? '' : '/') + caminho;
  }
  return null;
}

// ----------------------------------------------------------- abrir ------

async function abrir() {
  if (!MID) {
    telaErro('Atividade não indicada', 'Abra a atividade pelo portal do aluno.');
    return;
  }
  const r = await api('POST', '/abrir');
  switch (r.status) {
    case 200: {
      const atv = r.json.atividade, tent = r.json.tentativa;
      estado.carga = atv;
      estado.regras = r.json.regras || {};
      estado.revisao = tent.revisao;
      estado.respostas = tent.respostas || {};
      const local = await obterLocal();
      if (local && local.pendente && local.revisao >= estado.revisao) {
        estado.respostas = Object.assign({}, estado.respostas, local.respostas);
        estado.revisao = local.revisao;
      }
      cronometroOn(tent.segundos_restantes);
      renderProva(atv);
      agendarAutosave();
      return;
    }
    case 403:
      telaFinalizada(mensagem(r, 'Atividade encerrada para você.'), detalhe(r));
      return;
    case 409:
      telaErro('Avaliação ainda não abriu',
        `A janela começa em ${detalhe(r).segundos_para_inicio ?? '?'} s.`);
      return;
    case 410:
      telaEncerrada(mensagem(r, 'A janela da atividade está encerrada.'));
      return;
    case 401:
      telaErro('Sessão inválida', 'Use o link do portal do aluno para entrar de novo.');
      return;
    default:
      telaErro('Erro na abertura', mensagem(r, 'Falha inesperada no servidor.'));
  }
}

function telaEncerrada(msg) {
  cronometroOff();
  conteudo.innerHTML = `
    <div class="tarja fim">
      <h2>Janela encerrada</h2>
      <p>${esc(msg)}</p>
      <p class="mudo">Seu rascunho está preservado: fale com o professor
      (coleta manual ou nova janela).</p>
    </div>`;
}

// ----------------------------------------------------------- prova ------

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function mmss(total) {
  const s = Math.max(0, Math.floor(total));
  return `${String(Math.floor(s / 3600)).padStart(2, '0')}:${String(Math.floor((s % 3600) / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
}

let cronoTimer = null;
function cronometroOn(segundos) {
  estado.restantes = segundos;
  $('crono').hidden = false;
  clearInterval(cronoTimer);
  cronoTimer = setInterval(() => {
    if (estado.restantes === null) return;
    estado.restantes = Math.max(0, estado.restantes - 1);
    const el = $('crono');
    el.textContent = mmss(estado.restantes);
    el.classList.toggle('pouco', estado.restantes <= 120);
    if (estado.restantes === 0) {
      clearInterval(cronoTimer);
      // Janela zerada: o servidor decide (atraso permitido ou não).
      // A próxima gravação/pre-avaliação devolve 410 se nada mais vale.
      $('salvo').textContent = 'prazo encerrado';
    }
  }, 1000);
}
function cronometroOff() { clearInterval(cronoTimer); $('crono').hidden = true; }

function renderProva(atv) {
  $('titulo').textContent = atv.nome || 'Seno';
  const cards = atv.tarefas.map((t) => {
    return `
      <section class="card" data-ordem="${t.ordem}">
        <header>
          <strong>${t.ordem}. ${esc(t.nome)}</strong>
          <span class="pts">${t.valor_pts} pts</span>
          <span class="ling">${esc(t.linguagem)}</span>
        </header>
        ${t.enunciado ? `<p class="enun">${esc(t.enunciado)}</p>` : ''}
        <div class="editor-wrap" data-ordem="${t.ordem}"></div>
        <div class="aces">
          <button class="prim" data-testar="${t.ordem}" ${t.testes && t.testes.length ? '' : 'hidden'}>
            Testar (${t.testes.length} teste${t.testes.length > 1 ? 's' : ''} público${t.testes.length > 1 ? 's' : ''})
          </button>
        </div>
        <div class="tst" data-tst="${t.ordem}"></div>
      </section>`;
  }).join('');
  conteudo.innerHTML = `
    ${atv.enunciado ? `<div class="tarja"><h2>${esc(atv.nome)}</h2><p class="enun">${esc(atv.enunciado)}</p></div>` : ''}
    ${cards}
    <div class="tarja">
      <p class="mudo">Envio em duas etapas: pré-avaliação (relatório; nada
      oficial ainda) e confirmação (fecha a entrega). Salvamento automático
      a 20 s.</p>
      <div class="aces">
        <button id="btn-avaliar" class="prim">Pré-avaliar tudo</button>
      </div>
    </div>`;

  conteudo.querySelectorAll('.editor-wrap').forEach((wrap) => {
    const ordem = wrap.dataset.ordem;
    const tarefa = atv.tarefas.find((x) => String(x.ordem) === ordem);
    const lin = estado.respostas[ordem] ?? '';
    const autocomplete = Boolean(estado.regras && estado.regras.autocomplete);

    if (window.SenoEditor && window.SenoEditor.criar) {
      window.SenoEditor.criar(wrap, {
        valor: lin,
        linguagem: tarefa ? tarefa.linguagem : '',
        autocomplete,
        onchange: (novoValor) => {
          estado.respostas[ordem] = novoValor;
          estado.suja = true;
          salvarLocal(true);
        },
      });
    } else {
      const ta = document.createElement('textarea');
      ta.className = 'campo';
      ta.value = lin;
      ta.autocomplete = 'off';
      ta.spellcheck = false;
      ta.placeholder = `Escreva seu código ${tarefa ? tarefa.linguagem : ''} aqui…`;
      ta.addEventListener('input', () => {
        estado.respostas[ordem] = ta.value;
        estado.suja = true;
        salvarLocal(true);
      });
      wrap.appendChild(ta);
    }
  });

  conteudo.querySelectorAll('[data-testar]').forEach((b) => {
    b.addEventListener('click', () => testar(Number(b.dataset.testar)));
  });
  $('btn-avaliar').addEventListener('click', preAvaliar);
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
  const r = await api('PUT', '/tentativa',
    { revisao: estado.revisao, respostas: estado.respostas });
  if (r.status === 200) {
    estado.suja = false;
    estado.revisao = r.json.revisao;
    salvarLocal(false);
    $('salvo').textContent = 'salvo ' + new Date().toLocaleTimeString('pt-BR');
    return;
  }
  if (r.status === 0) {
    // Offline
    $('salvo').textContent = 'salvo localmente (offline)';
    return;
  }
  if (r.status === 409 && detalhe(r).revisao_atual !== undefined) {
    // outra aba gravou antes: alinha com a revisão vigente e tenta de novo
    const atual = detalhe(r).revisao_atual;
    const r2 = await api('PUT', '/tentativa',
      { revisao: atual, respostas: estado.respostas });
    if (r2.status === 200) {
      estado.suja = false;
      estado.revisao = r2.json.revisao;
      salvarLocal(false);
      $('salvo').textContent = 'salvo (outra aba detectada)';
    } else {
      estado.suja = true;
      $('salvo').textContent = 'conflito: outra aba salvou';
    }
    return;
  }
  if (r.status >= 400) {
    estado.suja = true;
    $('salvo').textContent = 'falhou o salvamento';
    if (r.status === 410) { cronometroZerou(); }
    if (r.status === 403) { telaFinalizada(mensagem(r, 'Encerrada.'), detalhe(r)); }
  }
}

function cronometroZerou() {
  cronometroOff();
  telaEncerrada('O tempo acabou e a entrega não foi mais possível.');
}

// ------------------------------------------------------------ ensaios ---

async function testar(ordem) {
  const t = estado.carga.tarefas.find((x) => x.ordem === ordem);
  const codigo = estado.respostas[String(ordem)] ?? '';
  const r = await api('POST', '/testar', { ordem, linguagem: t.linguagem, codigo });
  if (r.status !== 202) {
    if (r.status === 429) {
      aviso(`Limite de ensaios por minuto. Aguarde.`);
      return;
    }
    if (r.status >= 400) { aviso(mensagem(r, 'Não foi possível testar.')); }
    return;
  }
  const run = r.json.run_id;
  const alvo = conteudo.querySelector(`[data-tst="${ordem}"]`);
  aviso(`Run ${esc(run.slice(0, 8))}… executando`, alvo);
  for (let seg = 0; seg < 60; seg++) {
    await new Promise((res) => setTimeout(res, 1500));
    const q = await api('GET', `/testar/${encodeURIComponent(run)}`);
    if (q.status === 200) {
      const linhas = q.json.resultados.map((x) => `
        <tr><td>#${x.ordem}</td>
        <td class="${x.veredito === 'ok' ? 'passou' : 'falhou'}">${esc(x.veredito)}</td>
        <td>${(x.tempo ?? 0).toFixed(2)}s</td><td>${x.memoria ?? 0} KB</td></tr>`).join('');
      alvo.innerHTML = `<table>${linhas}</table>`;
      return;
    }
    if (q.status >= 400) {
      alvo.innerHTML = `<span class="falhou">Run perdido. Tente de novo.</span>`;
      return;
    }
  }
  alvo.innerHTML = `<span class="mudo">Run atrasado; o resultado aparece no histórico recente tentando de novo.</span>`;
}

function aviso(msg, alvo) {
  if (alvo) { alvo.innerHTML = `<span class="mudo">${msg}</span>`; return; }
  $('salvo').textContent = msg;
}

// --------------------------------------------------------- submissão ----

async function preAvaliar() {
  await salvar();
  mudarBotao('btn-avaliar', true, 'Pré-avaliando…');
  const r = await api('POST', '/submissao/avaliar');
  mudarBotao('btn-avaliar', false, 'Pré-avaliar tudo');
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
    <h2>Pré-avaliação</h2>
    <p>Acertos: <b>${rel.resumo.acertos}</b> de <b>${rel.resumo.total}</b> testes públicos.
       Válido por <span id="ttl">${mmss(ttlSeg)}</span>.</p>
    ${rel.tarefas.map((t) => `
      <p><b>Tarefa ${t.ordem}:</b>
         <span class="falhou">${esc(t.veredito)}</span>
         ${(t.testes || []).map((x) => `<span class="${x.status === 'ok' ? 'passou' : 'falhou'}">#${x.ordem} ${esc(x.status)}</span>`).join(' · ')}</p>
    `).join('')}
    <div class="aces">
      <button id="btn-conf" class="prim">Confirmar envio</button>
      <button id="btn-cancel">Cancelar e voltar a editar</button>
    </div>
    <p class="mudo">A confirmação é definitiva (fecha a tentativa); a
    pré-avaliação expira e pede reavaliação.</p>`;
  modal.showModal();
  let restTtl = ttlSeg;
  const ttlTimer = setInterval(() => {
    restTtl--;
    const el = $('ttl');
    if (el) el.textContent = mmss(restTtl);
    if (restTtl <= 0) {
      clearInterval(ttlTimer);
      estado.preId = null;
      aviso('Pré-avaliação expirou: avalie de novo.');
      modal.close();
    }
  }, 1000);

  $('btn-cancel').addEventListener('click', async () => {
    clearInterval(ttlTimer);
    await api('POST', '/submissao/cancelar', { pre_id: estado.preId });
    estado.preId = null;
    modal.close();
  });
  $('btn-conf').addEventListener('click', () => confirmarPre(ttlTimer));
}

function mudarBotao(id, ativo, texto) {
  const b = $(id);
  if (!b) return;
  b.disabled = ativo;
  if (texto) b.textContent = texto;
}

async function confirmarPre(ttlTimer) {
  clearInterval(ttlTimer);
  mudarBotao('btn-conf', true, 'Enviando…');
  const r = await api('POST', '/submissao/confirmar', { pre_id: estado.preId });
  estado.preId = null;
  modal.close();
  if (r.status === 200) {
    const e = r.json;
    const msg = `entrega n_t=${e.n_t}${e.atrasada ? ' (ATRASADA)' : ''} registrada`;
    // recarga: submissão múltipla reabre tentativa (transportando as respostas)
    const r2 = await api('POST', '/abrir');
    if (r2.status === 200) {
      const atv = r2.json.atividade, tent = r2.json.tentativa;
      estado.revisao = tent.revisao;
      estado.respostas = tent.respostas || {};
      renderProva(atv);
      $('salvo').textContent = msg;
      return;
    }
    telaFinalizada(mensagem(r2, 'Sua entrega foi registrada.'), detalhe(r2));
    return;
  }
  switch (r.status) {
    case 409:
      if (r.json.erro && r.json.erro.codigo === 'ATUALIZADA') {
        aviso('As respostas mudaram desde a pré-avaliação: reavalie.');
      } else {
        aviso(mensagem(r, 'Pré-avaliação expirou: reavalie.'));
      }
      return;
    case 410:
      cronometroOff();
      telaEncerrada('A janela fechou antes da confirmação. Rascunho preservado.');
      return;
    case 403:
      telaFinalizada(mensagem(r, 'Encerrada.'), detalhe(r));
      return;
    default:
      aviso(mensagem(r, 'Falha ao confirmar.'));
  }
}

// ---------------------------------------------------------------- boot --

abrir();
