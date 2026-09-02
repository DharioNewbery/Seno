"use strict";

// Seno — página de gestão de professores (acesso restrito ao superusuário).
// Utilitários em common.js.

var currentUser = sessionUser();
if (!currentUser || !hasRole(currentUser, "super")) {
  window.location.replace("/");
  throw new Error("acesso restrito ao superusuário");
}

var professorForm = document.getElementById("professor-form");
var fullNameInput = document.getElementById("full_name");
var emailInput = document.getElementById("email");
var passwordInput = document.getElementById("password");
var professorError = document.getElementById("professor-error");
var professorSuccess = document.getElementById("professor-success");
var professorSubmit = document.getElementById("professor-submit");
var professorsBody = document.getElementById("professors-body");
var professorsEmpty = document.getElementById("professors-empty");

async function handleCreate(event) {
  event.preventDefault();
  hideProfessorMessages();

  var fullName = fullNameInput.value.trim();
  var email = emailInput.value.trim();
  var password = passwordInput.value;
  if (!fullName || !email || !password) {
    showProfessorError("Preencha todos os campos.");
    return;
  }

  professorSubmit.disabled = true;
  professorSubmit.textContent = "Cadastrando...";
  try {
    await request("/professors", {
      method: "POST",
      body: JSON.stringify({ full_name: fullName, email: email, password: password })
    });
    professorForm.reset();
    showProfessorSuccess("Professor cadastrado com sucesso.");
    await loadProfessors();
  } catch (err) {
    showProfessorError(err.message);
  } finally {
    professorSubmit.disabled = false;
    professorSubmit.textContent = "Cadastrar professor";
  }
}

async function loadProfessors() {
  try {
    var professors = await request("/professors");
    renderProfessors(professors);
  } catch (err) {
    showProfessorError(err.message);
  }
}

function renderProfessors(professors) {
  professorsBody.innerHTML = "";

  if (!professors || professors.length === 0) {
    professorsEmpty.classList.remove("hidden");
    return;
  }
  professorsEmpty.classList.add("hidden");

  professors.forEach(function (p) {
    var tr = document.createElement("tr");
    tr.appendChild(cell(p.full_name));
    tr.appendChild(cell(p.email));
    tr.appendChild(cell(new Date(p.created_at).toLocaleDateString("pt-BR")));
    professorsBody.appendChild(tr);
  });
}

function cell(text) {
  var td = document.createElement("td");
  td.textContent = text;
  return td;
}

function showProfessorError(message) {
  professorError.textContent = message;
  professorError.classList.remove("hidden");
}

function showProfessorSuccess(message) {
  professorSuccess.textContent = message;
  professorSuccess.classList.remove("hidden");
}

function hideProfessorMessages() {
  professorError.classList.add("hidden");
  professorSuccess.classList.add("hidden");
}

professorForm.addEventListener("submit", handleCreate);

loadProfessors();
