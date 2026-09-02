"use strict";

// Seno — página da tarefa: enunciado, IDE (CodeMirror) com rascunho salvo
// automaticamente em duas camadas (localStorage + servidor) e submissão.
// Utilitários em common.js.

var currentUser = sessionUser();
if (!currentUser) {
  window.location.replace("/");
  throw new Error("sem sessão");
}

var assignmentId = new URLSearchParams(window.location.search).get("id");
if (!assignmentId) {
  window.location.replace("/");
  throw new Error("tarefa não informada");
}

var isProfessor = hasRole(currentUser, "professor");
var isStudent = hasRole(currentUser, "student");
if (!isProfessor && !isStudent) {
  window.location.replace("/");
  throw new Error("acesso restrito");
}

var titleEl = document.getElementById("assignment-title");
var metaEl = document.getElementById("assignment-meta");
var statementEl = document.getElementById("assignment-statement");
var testsList = document.getElementById("tests-list");
var editorCard = document.getElementById("editor-card");
var editorLanguage = document.getElementById("editor-language");
var draftStatus = document.getElementById("draft-status");
var submitError = document.getElementById("submit-error");
var submitSuccess = document.getElementById("submit-success");
var submitButton = document.getElementById("submit-code");
var submissionsTitle = document.getElementById("submissions-title");
var submissionsList = document.getElementById("submissions-list");
var submissionsEmpty = document.getElementById("submissions-empty");
var backLink = document.getElementById("back-link");

var editor = null;
var draftKey = "seno.draft." + assignmentId;
var draftRestored = false;
var lastSyncedCode = null;

var LOCAL_SAVE_DELAY = 800;
var SERVER_SYNC_DELAY = 3000;
var POLL_INTERVAL = 3000;
var saveTimer = null;
var syncTimer = null;
var pollTimer = null;

function languageMode(language) {
  if (language === "python") {
    return "python";
  }
  if (language === "c") {
    return "text/x-csrc";
  }
  if (language === "cpp") {
    return "text/x-c++src";
  }
  return "text/plain";
}

function languageLabel(language) {
  return { python: "Python", c: "C", cpp: "C++" }[language] || language;
}

function statusLabel(status) {
  return { pending: "Pendente", passed: "Aprovado", failed: "Falhou", error: "Erro" }[status] || status;
}

function statusClass(status) {
  return "badge--" + status;
}

// parseResult decodifica o campo result (texto JSON) da submissão.
function parseResult(raw) {
  if (!raw) {
    return null;
  }
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw);
    } catch (err) {
      return null;
    }
  }
  return raw;
}

// setupEditor cria a IDE uma única vez; o conteúdo inicial vem de
// restoreDraft (rascunho do servidor ou local, o mais recente).
function setupEditor(language) {
  if (editor) {
    return;
  }

  editor = CodeMirror.fromTextArea(document.getElementById("editor"), {
    mode: languageMode(language),
    lineNumbers: true,
    indentUnit: 4,
    matchBrackets: true
  });

  editor.on("change", function () {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(saveLocalDraft, LOCAL_SAVE_DELAY);
  });
}

// Rascunho local: {code, savedAt}; o formato legado (string pura) é aceito.
function readLocalDraft() {
  var raw = localStorage.getItem(draftKey);
  if (!raw) {
    return null;
  }
  try {
    var parsed = JSON.parse(raw);
    if (parsed && typeof parsed.code === "string") {
      return parsed;
    }
  } catch (err) {
    // formato legado: string pura
  }
  return { code: raw, savedAt: 0 };
}

function writeLocalDraft(code, savedAt) {
  localStorage.setItem(draftKey, JSON.stringify({ code: code, savedAt: savedAt }));
}

// restoreDraft escolhe entre o rascunho do servidor e o local pelo mais
// recente. Se o local vencer (sync anterior falhou), agenda novo envio.
function restoreDraft(serverDraft) {
  var local = readLocalDraft();
  var server = serverDraft
    ? { code: serverDraft.source_code, savedAt: new Date(serverDraft.updated_at).getTime() }
    : null;

  var useLocal = local && (!server || local.savedAt > server.savedAt);

  if (useLocal) {
    editor.setValue(local.code);
    writeLocalDraft(local.code, local.savedAt);
    lastSyncedCode = null;
    scheduleServerSync();
  } else if (server) {
    editor.setValue(server.code);
    writeLocalDraft(server.code, server.savedAt);
    lastSyncedCode = server.code;
  }
}

function saveLocalDraft() {
  var code = editor.getValue();
  writeLocalDraft(code, Date.now());
  draftStatus.textContent = "Rascunho salvo às " + new Date().toLocaleTimeString("pt-BR");
  scheduleServerSync();
}

function scheduleServerSync() {
  clearTimeout(syncTimer);
  syncTimer = setTimeout(syncDraftToServer, SERVER_SYNC_DELAY);
}

async function syncDraftToServer() {
  if (!editor || !isStudent) {
    return;
  }
  var code = editor.getValue();
  if (code === lastSyncedCode) {
    return;
  }

  try {
    await request("/assignments/" + assignmentId + "/draft", {
      method: "PUT",
      body: JSON.stringify({ source_code: code })
    });
    lastSyncedCode = code;
    draftStatus.textContent = "Backup salvo no servidor às " + new Date().toLocaleTimeString("pt-BR");
  } catch (err) {
    draftStatus.textContent = "Offline: rascunho salvo apenas localmente";
    // a próxima edição reagendará a sincronização
  }
}

// flushDraft salva imediatamente; usado ao sair da página. O fetch com
// keepalive sobrevive ao descarregamento nos navegadores modernos.
function flushDraft() {
  if (!editor || !isStudent) {
    return;
  }
  var code = editor.getValue();
  writeLocalDraft(code, Date.now());
  if (code === lastSyncedCode) {
    return;
  }

  var token = accessToken();
  if (!token) {
    return;
  }
  fetch(API + "/assignments/" + assignmentId + "/draft", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + token
    },
    body: JSON.stringify({ source_code: code }),
    keepalive: true
  }).catch(function () { });
}

window.addEventListener("pagehide", flushDraft);

document.addEventListener("visibilitychange", function () {
  if (document.visibilityState === "hidden") {
    clearTimeout(syncTimer);
    syncDraftToServer();
  }
});

async function loadAssignment() {
  try {
    var data = await request("/assignments/" + assignmentId);
    renderAssignment(data);
  } catch (err) {
    showSubmitError(err.message);
  }
}

function renderAssignment(data) {
  titleEl.textContent = data.title;

  var meta = data.class_name + " · " + languageLabel(data.language);
  if (data.due_at) {
    meta += " · prazo: " + new Date(data.due_at).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
  }
  metaEl.textContent = meta;

  statementEl.textContent = data.statement;

  // Para o aluno, a coluna "Você obteve" reflete a saída da submissão
  // corrigida mais recente. As submissões vêm ordenadas por created_at DESC.
  var lastResult = null;
  if (!isProfessor && data.submissions && data.submissions.length) {
    var latest = null;
    data.submissions.forEach(function (s) {
      if (s.status === "pending") {
        return;
      }
      if (!latest || new Date(s.created_at) > new Date(latest.created_at)) {
        latest = s;
      }
    });
    if (latest) {
      lastResult = parseResult(latest.result);
    }
  }
  renderTests(data.tests, lastResult);

  submissionsTitle.textContent = isProfessor ? "Submissões recebidas" : "Minhas submissões";
  renderSubmissions(data.submissions);

  if (isStudent) {
    editorLanguage.textContent = "Linguagem: " + languageLabel(data.language);
    setupEditor(data.language);
    if (!draftRestored) {
      draftRestored = true;
      restoreDraft(data.draft);
    }
  } else {
    editorCard.classList.add("hidden");
  }
}

// renderTests mostra os casos de teste da tarefa. Para o aluno, a coluna
// "Você obteve" traz a saída da última submissão corrigida (por posição).
function renderTests(tests, lastResult) {
  testsList.innerHTML = "";

  if (!tests || tests.length === 0) {
    var p = document.createElement("p");
    p.className = "muted";
    p.textContent = "Sem casos de teste definidos para esta tarefa.";
    testsList.appendChild(p);
    return;
  }

  var gotByPos = {};
  if (lastResult && lastResult.tests) {
    lastResult.tests.forEach(function (r) {
      gotByPos[r.position] = r;
    });
  }

  var table = document.createElement("table");
  table.className = "table";

  var thead = document.createElement("thead");
  var headRow = document.createElement("tr");
  ["#", "Entrada", "Saída esperada", "Você obteve"].forEach(function (text) {
    var th = document.createElement("th");
    th.textContent = text;
    headRow.appendChild(th);
  });
  thead.appendChild(headRow);
  table.appendChild(thead);

  var tbody = document.createElement("tbody");
  tests.forEach(function (t, idx) {
    var tr = document.createElement("tr");
    tr.appendChild(preCell(String(idx + 1)));
    tr.appendChild(preCell(t.input));
    tr.appendChild(preCell(t.expected_output));

    var match = gotByPos[idx + 1];
    var got = "—";
    if (match) {
      got = match.got || "(sem saída)";
    }
    tr.appendChild(preCell(got));
    tbody.appendChild(tr);
  });
  table.appendChild(tbody);
  testsList.appendChild(table);
}

function preCell(text) {
  var td = document.createElement("td");
  var pre = document.createElement("pre");
  pre.className = "pre-cell";
  pre.textContent = text;
  td.appendChild(pre);
  return td;
}

function renderSubmissions(submissions) {
  submissionsList.innerHTML = "";

  if (!submissions || submissions.length === 0) {
    submissionsEmpty.textContent = isProfessor ? "Nenhuma submissão recebida." : "Você ainda não enviou submissões.";
    submissionsEmpty.classList.remove("hidden");
    return;
  }
  submissionsEmpty.classList.add("hidden");

  submissions.forEach(function (s) {
    var item = document.createElement("div");
    item.className = "submission-item";

    var meta = document.createElement("div");
    meta.className = "submission-meta";
    var left = document.createElement("span");
    left.textContent = (isProfessor ? s.student_name + " · " : "") +
      new Date(s.created_at).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
    var status = document.createElement("span");
    status.className = "badge " + statusClass(s.status);
    status.textContent = statusLabel(s.status);
    meta.appendChild(left);
    meta.appendChild(status);
    item.appendChild(meta);

    var result = parseResult(s.result);
    if (s.status === "pending") {
      var pending = document.createElement("p");
      pending.className = "muted";
      pending.textContent = "Correção automática em andamento...";
      item.appendChild(pending);
    } else if (result) {
      item.appendChild(renderResultSummary(result));
      item.appendChild(renderResultTable(result));
    }

    var details = document.createElement("details");
    var summary = document.createElement("summary");
    summary.textContent = "Ver código";
    var pre = document.createElement("pre");
    pre.className = "code-block";
    pre.textContent = s.source_code;
    details.appendChild(summary);
    details.appendChild(pre);
    item.appendChild(details);

    if (isStudent) {
      var loadBtn = document.createElement("button");
      loadBtn.type = "button";
      loadBtn.className = "btn btn--small";
      loadBtn.textContent = "Carregar no editor";
      loadBtn.addEventListener("click", function () {
        editor.setValue(s.source_code);
        writeLocalDraft(s.source_code, Date.now());
        lastSyncedCode = null;
        scheduleServerSync();
        editorCard.scrollIntoView({ behavior: "smooth" });
        editor.focus();
      });
      item.appendChild(loadBtn);
    }

    submissionsList.appendChild(item);
  });

  if (submissions.some(function (s) { return s.status === "pending"; })) {
    schedulePoll();
  } else {
    stopPoll();
  }
}

// renderResultSummary mostra o placar (ex.: 2/3 casos) logo abaixo do status.
function renderResultSummary(result) {
  var p = document.createElement("p");
  p.className = "muted";
  var summary = result.summary || {};
  p.textContent = summary.passed + "/" + summary.total + " casos aprovados";
  return p;
}

// renderResultTable detalha cada caso: entrada, esperado, obtido, duração
// e erro (stderr/timeout/exit code).
function renderResultTable(result) {
  var table = document.createElement("table");
  table.className = "table";

  var thead = document.createElement("thead");
  var headRow = document.createElement("tr");
  ["#", "Entrada", "Esperado", "Obtido", "Duração", ""].forEach(function (text) {
    var th = document.createElement("th");
    th.textContent = text;
    headRow.appendChild(th);
  });
  thead.appendChild(headRow);
  table.appendChild(thead);

  var tbody = document.createElement("tbody");
  (result.tests || []).forEach(function (t) {
    var tr = document.createElement("tr");
    tr.appendChild(preCell(String((t.position || 0))));
    tr.appendChild(preCell(t.input));
    tr.appendChild(preCell(t.expected));
    tr.appendChild(preCell(t.got || ""));
    tr.appendChild(cellText(t.duration_ms + " ms"));
    var statusTd = document.createElement("td");
    if (t.timed_out) {
      statusTd.appendChild(badge("timeout", "erro"));
    } else if (t.exit_code !== 0) {
      statusTd.appendChild(badge("exit " + t.exit_code, "erro"));
    } else if (t.passed) {
      statusTd.appendChild(badge("ok", "success"));
    } else {
      statusTd.appendChild(badge("difere", "failed"));
    }
    tr.appendChild(statusTd);
    tbody.appendChild(tr);

    if (t.stderr) {
      var errTr = document.createElement("tr");
      var errTd = document.createElement("td");
      errTd.colSpan = 6;
      errTd.className = "stderr-cell";
      errTd.textContent = "stderr: " + t.stderr;
      errTr.appendChild(errTd);
      tbody.appendChild(errTr);
    }
  });
  table.appendChild(tbody);
  return table;
}

function badge(label, mod) {
  var span = document.createElement("span");
  span.className = "badge badge--" + mod;
  span.textContent = label;
  return span;
}

function cellText(text) {
  var td = document.createElement("td");
  td.textContent = text;
  return td;
}

// schedulePoll re-carrega a página enquanto houver submissão pendente.
function schedulePoll() {
  clearTimeout(pollTimer);
  if (document.hidden) {
    return;
  }
  pollTimer = setTimeout(loadAssignment, POLL_INTERVAL);
}

function stopPoll() {
  clearTimeout(pollTimer);
}

async function handleSubmit() {
  hideMessages();

  var code = editor ? editor.getValue() : "";
  if (!code.trim()) {
    showSubmitError("Escreva seu código antes de enviar.");
    return;
  }

  submitButton.disabled = true;
  submitButton.textContent = "Enviando...";
  try {
    await request("/assignments/" + assignmentId + "/submissions", {
      method: "POST",
      body: JSON.stringify({ source_code: code })
    });
    // Submetido: o rascunho (local e servidor) passa a ser o código entregue.
    writeLocalDraft(code, Date.now());
    await syncDraftToServer();
    showSubmitSuccess("Submissão enviada com sucesso.");
    await loadAssignment();
  } catch (err) {
    showSubmitError(err.message);
  } finally {
    submitButton.disabled = false;
    submitButton.textContent = "Enviar submissão";
  }
}

function showSubmitError(message) {
  submitError.textContent = message;
  submitError.classList.remove("hidden");
}

function showSubmitSuccess(message) {
  submitSuccess.textContent = message;
  submitSuccess.classList.remove("hidden");
}

function hideMessages() {
  submitError.classList.add("hidden");
  submitSuccess.classList.add("hidden");
}

backLink.addEventListener("click", function (event) {
  event.preventDefault();
  history.back();
});

if (isStudent) {
  submitButton.addEventListener("click", handleSubmit);
}

loadAssignment();
