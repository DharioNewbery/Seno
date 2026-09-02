"use strict";

// Seno — listagem de tarefas. Professor: tarefas de uma turma (?class=ID) +
// publicação. Aluno: feed de tarefas das turmas ingressadas.
// Utilitários em common.js.

var currentUser = sessionUser();
if (!currentUser) {
  window.location.replace("/");
  throw new Error("sem sessão");
}

var classId = new URLSearchParams(window.location.search).get("class");
var mode = null;
if (classId && hasRole(currentUser, "professor")) {
  mode = "professor";
} else if (!classId && hasRole(currentUser, "student")) {
  mode = "student";
} else if (hasRole(currentUser, "professor")) {
  window.location.replace("/classes.html");
  throw new Error("redirect");
} else if (hasRole(currentUser, "student")) {
  window.location.replace("/assignments.html");
  throw new Error("redirect");
} else {
  window.location.replace("/");
  throw new Error("redirect");
}

var pageTitle = document.getElementById("page-title");
var listTitle = document.getElementById("list-title");
var newAssignmentCard = document.getElementById("new-assignment");
var assignmentsHead = document.getElementById("assignments-head");
var assignmentsBody = document.getElementById("assignments-body");
var assignmentsEmpty = document.getElementById("assignments-empty");
var assignmentForm = document.getElementById("assignment-form");
var titleInput = document.getElementById("as-title");
var statementInput = document.getElementById("as-statement");
var languageSelect = document.getElementById("as-language");
var dueInput = document.getElementById("as-due");
var asError = document.getElementById("as-error");
var asSuccess = document.getElementById("as-success");
var asSubmit = document.getElementById("as-submit");

function setupMode() {
  if (mode === "professor") {
    pageTitle.textContent = "Tarefas da turma";
    listTitle.textContent = "Tarefas publicadas";
    newAssignmentCard.classList.remove("hidden");
    assignmentsHead.innerHTML = "<tr><th>Título</th><th>Linguagem</th><th>Prazo</th><th>Publicada em</th><th></th></tr>";
  } else {
    pageTitle.textContent = "Minhas tarefas";
    listTitle.textContent = "Tarefas das suas turmas";
    assignmentsHead.innerHTML = "<tr><th>Tarefa</th><th>Turma</th><th>Linguagem</th><th>Prazo</th><th></th></tr>";
  }
}

async function handleCreate(event) {
  event.preventDefault();
  hideMessages();

  var title = titleInput.value.trim();
  var statement = statementInput.value;
  var language = languageSelect.value;
  if (!title || !statement.trim()) {
    showError("Preencha o título e o enunciado.");
    return;
  }

  var body = { title: title, statement: statement, language: language };
  if (dueInput.value) {
    body.due_at = new Date(dueInput.value).toISOString();
  }

  asSubmit.disabled = true;
  asSubmit.textContent = "Publicando...";
  try {
    await request("/classes/" + classId + "/assignments", {
      method: "POST",
      body: JSON.stringify(body)
    });
    assignmentForm.reset();
    showSuccess("Tarefa publicada com sucesso.");
    await loadAssignments();
  } catch (err) {
    showError(err.message);
  } finally {
    asSubmit.disabled = false;
    asSubmit.textContent = "Publicar tarefa";
  }
}

async function loadAssignments() {
  try {
    var assignments = mode === "professor"
      ? await request("/classes/" + classId + "/assignments")
      : await request("/assignments/mine");
    renderAssignments(assignments);
  } catch (err) {
    showError(err.message);
  }
}

function renderAssignments(assignments) {
  assignmentsBody.innerHTML = "";

  if (!assignments || assignments.length === 0) {
    assignmentsEmpty.textContent = mode === "professor"
      ? "Nenhuma tarefa publicada nesta turma."
      : "Nenhuma tarefa nas suas turmas ainda.";
    assignmentsEmpty.classList.remove("hidden");
    return;
  }
  assignmentsEmpty.classList.add("hidden");

  assignments.forEach(function (a) {
    var tr = document.createElement("tr");
    tr.appendChild(cell(a.title));
    if (mode === "student") {
      tr.appendChild(cell(a.class_name));
    }
    tr.appendChild(cell(languageLabel(a.language)));
    tr.appendChild(cell(formatDue(a.due_at)));
    if (mode === "professor") {
      tr.appendChild(cell(formatDate(a.created_at)));
    }
    tr.appendChild(linkCell("/assignment.html?id=" + a.id, "Abrir"));
    assignmentsBody.appendChild(tr);
  });
}

function languageLabel(language) {
  return { python: "Python", c: "C", cpp: "C++" }[language] || language;
}

function formatDate(value) {
  return new Date(value).toLocaleDateString("pt-BR");
}

function formatDue(value) {
  if (!value) {
    return "—";
  }
  return new Date(value).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

function cell(text) {
  var td = document.createElement("td");
  td.textContent = text;
  return td;
}

function linkCell(href, label) {
  var td = document.createElement("td");
  var a = document.createElement("a");
  a.className = "link";
  a.href = href;
  a.textContent = label;
  td.appendChild(a);
  return td;
}

function showError(message) {
  asError.textContent = message;
  asError.classList.remove("hidden");
}

function showSuccess(message) {
  asSuccess.textContent = message;
  asSuccess.classList.remove("hidden");
}

function hideMessages() {
  asError.classList.add("hidden");
  asSuccess.classList.add("hidden");
}

setupMode();
if (mode === "professor") {
  assignmentForm.addEventListener("submit", handleCreate);
}
loadAssignments();
