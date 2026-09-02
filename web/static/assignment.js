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
var saveTimer = null;
var syncTimer = null;

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
    status.className = "badge";
    status.textContent = s.status === "pending" ? "pendente" : s.status;
    meta.appendChild(left);
    meta.appendChild(status);
    item.appendChild(meta);

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
