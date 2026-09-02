"use strict";

// Seno — página de turmas. Professor cria e gerencia; aluno ingressa por código.
// Utilitários em common.js.

var currentUser = sessionUser();
var mode = null;
if (currentUser && hasRole(currentUser, "professor")) {
  mode = "professor";
} else if (currentUser && hasRole(currentUser, "student")) {
  mode = "student";
} else {
  window.location.replace("/");
  throw new Error("acesso restrito");
}

var professorPanel = document.getElementById("professor-panel");
var studentJoin = document.getElementById("student-join");
var classesHead = document.getElementById("classes-head");
var classesBody = document.getElementById("classes-body");
var classesEmpty = document.getElementById("classes-empty");
var listTitle = document.getElementById("list-title");
var classForm = document.getElementById("class-form");
var classNameInput = document.getElementById("class-name");
var classDescriptionInput = document.getElementById("class-description");
var classError = document.getElementById("class-error");
var classSuccess = document.getElementById("class-success");
var classSubmit = document.getElementById("class-submit");
var joinForm = document.getElementById("join-form");
var joinCodeInput = document.getElementById("join-code");
var joinError = document.getElementById("join-error");
var joinSuccess = document.getElementById("join-success");
var joinSubmit = document.getElementById("join-submit");

function setupMode() {
  if (mode === "professor") {
    professorPanel.classList.remove("hidden");
    classesHead.innerHTML = "<tr><th>Nome</th><th>Código</th><th>Alunos</th><th>Tarefas</th><th>Criada em</th></tr>";
    listTitle.textContent = "Turmas criadas";
  } else {
    studentJoin.classList.remove("hidden");
    classesHead.innerHTML = "<tr><th>Turma</th><th>Professor</th><th>Ingressou em</th></tr>";
    listTitle.textContent = "Turmas em que participo";
  }
}

async function handleCreateClass(event) {
  event.preventDefault();
  hideClassMessages();

  var name = classNameInput.value.trim();
  var description = classDescriptionInput.value.trim();
  if (!name) {
    showClassError("Informe o nome da turma.");
    return;
  }

  classSubmit.disabled = true;
  classSubmit.textContent = "Criando...";
  try {
    var data = await request("/classes", {
      method: "POST",
      body: JSON.stringify({ name: name, description: description })
    });
    classForm.reset();
    showClassSuccess("Turma criada. Código de ingresso: " + data.join_code);
    await loadClasses();
  } catch (err) {
    showClassError(err.message);
  } finally {
    classSubmit.disabled = false;
    classSubmit.textContent = "Criar turma";
  }
}

async function handleJoin(event) {
  event.preventDefault();
  hideJoinMessages();

  var code = joinCodeInput.value.trim();
  if (!code) {
    showJoinError("Informe o código da turma.");
    return;
  }

  joinSubmit.disabled = true;
  joinSubmit.textContent = "Entrando...";
  try {
    var data = await request("/classes/join", {
      method: "POST",
      body: JSON.stringify({ code: code })
    });
    joinForm.reset();
    showJoinSuccess("Você ingressou na turma \"" + data.name + "\".");
    await loadClasses();
  } catch (err) {
    showJoinError(err.message);
  } finally {
    joinSubmit.disabled = false;
    joinSubmit.textContent = "Entrar na turma";
  }
}

async function loadClasses() {
  try {
    var classes = mode === "professor"
      ? await request("/classes")
      : await request("/classes/mine");
    renderClasses(classes);
  } catch (err) {
    if (mode === "professor") {
      showClassError(err.message);
    } else {
      showJoinError(err.message);
    }
  }
}

function renderClasses(classes) {
  classesBody.innerHTML = "";

  if (!classes || classes.length === 0) {
    classesEmpty.textContent = mode === "professor"
      ? "Nenhuma turma criada ainda."
      : "Você ainda não ingressou em nenhuma turma.";
    classesEmpty.classList.remove("hidden");
    return;
  }
  classesEmpty.classList.add("hidden");

  classes.forEach(function (c) {
    var tr = document.createElement("tr");
    if (mode === "professor") {
      tr.appendChild(cell(c.name));
      tr.appendChild(codeCell(c.join_code));
      tr.appendChild(cell(String(c.member_count)));
      tr.appendChild(tasksCell(c.id));
      tr.appendChild(cell(new Date(c.created_at).toLocaleDateString("pt-BR")));
    } else {
      tr.appendChild(cell(c.name));
      tr.appendChild(cell(c.professor_name));
      tr.appendChild(cell(new Date(c.joined_at).toLocaleDateString("pt-BR")));
    }
    classesBody.appendChild(tr);
  });
}

function cell(text) {
  var td = document.createElement("td");
  td.textContent = text;
  return td;
}

// tasksCell aponta o professor para as tarefas da turma.
function tasksCell(classId) {
  var td = document.createElement("td");
  var a = document.createElement("a");
  a.className = "link";
  a.href = "/assignments.html?class=" + classId;
  a.textContent = "Tarefas";
  td.appendChild(a);
  return td;
}

// codeCell exibe o código de ingresso com botão de copiar (para compartilhar
// com os alunos).
function codeCell(code) {
  var td = document.createElement("td");

  var span = document.createElement("span");
  span.className = "code";
  span.textContent = code;
  td.appendChild(span);

  var btn = document.createElement("button");
  btn.type = "button";
  btn.className = "btn btn--small";
  btn.textContent = "Copiar";
  btn.addEventListener("click", function () {
    navigator.clipboard.writeText(code).then(function () {
      btn.textContent = "Copiado!";
      setTimeout(function () { btn.textContent = "Copiar"; }, 2000);
    }, function () { /* clipboard indisponível: ignora */ });
  });
  td.appendChild(document.createTextNode(" "));
  td.appendChild(btn);

  return td;
}

function showClassError(message) {
  classError.textContent = message;
  classError.classList.remove("hidden");
}

function showClassSuccess(message) {
  classSuccess.textContent = message;
  classSuccess.classList.remove("hidden");
}

function hideClassMessages() {
  classError.classList.add("hidden");
  classSuccess.classList.add("hidden");
}

function showJoinError(message) {
  joinError.textContent = message;
  joinError.classList.remove("hidden");
}

function showJoinSuccess(message) {
  joinSuccess.textContent = message;
  joinSuccess.classList.remove("hidden");
}

function hideJoinMessages() {
  joinError.classList.add("hidden");
  joinSuccess.classList.add("hidden");
}

setupMode();
if (mode === "professor") {
  classForm.addEventListener("submit", handleCreateClass);
} else {
  joinForm.addEventListener("submit", handleJoin);
}
loadClasses();
