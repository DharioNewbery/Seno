"use strict";

// Seno — página de login, autocadastro de aluno e sessão (MVP).
// Utilitários em common.js.

var viewLogin = document.getElementById("view-login");
var viewRegister = document.getElementById("view-register");
var viewMe = document.getElementById("view-me");
var loginForm = document.getElementById("login-form");
var loginInput = document.getElementById("login");
var passwordInput = document.getElementById("password");
var loginError = document.getElementById("login-error");
var loginNotice = document.getElementById("login-notice");
var loginSubmit = document.getElementById("login-submit");
var logoutButton = document.getElementById("logout");
var linkProfessors = document.getElementById("link-professors");
var changePasswordForm = document.getElementById("change-password-form");
var currentPasswordInput = document.getElementById("current-password");
var newPasswordInput = document.getElementById("new-password");
var changePasswordError = document.getElementById("change-password-error");
var changePasswordSubmit = document.getElementById("change-password-submit");
var registerForm = document.getElementById("register-form");
var regFullNameInput = document.getElementById("reg-full-name");
var regEmailInput = document.getElementById("reg-email");
var regPasswordInput = document.getElementById("reg-password");
var registerError = document.getElementById("register-error");
var registerSubmit = document.getElementById("register-submit");
var showRegisterLink = document.getElementById("show-register");
var showLoginLink = document.getElementById("show-login");

async function handleLogin(event) {
  event.preventDefault();
  hideLoginMessages();

  var login = loginInput.value.trim();
  var password = passwordInput.value;
  if (!login || !password) {
    showLoginError("Informe login e senha.");
    return;
  }

  setLoading(true);
  try {
    var data = await request("/auth/login", {
      method: "POST",
      body: JSON.stringify({ login: login, password: password })
    });
    localStorage.setItem(ACCESS_KEY, data.tokens.access_token);
    localStorage.setItem(REFRESH_KEY, data.tokens.refresh_token);
    await showMe();
  } catch (err) {
    showLoginError(err.message);
  } finally {
    setLoading(false);
  }
}

async function handleRegister(event) {
  event.preventDefault();
  registerError.classList.add("hidden");

  var fullName = regFullNameInput.value.trim();
  var email = regEmailInput.value.trim();
  var password = regPasswordInput.value;
  if (!fullName || !email || !password) {
    showRegisterError("Preencha todos os campos.");
    return;
  }

  registerSubmit.disabled = true;
  registerSubmit.textContent = "Criando...";
  try {
    await request("/auth/register", {
      method: "POST",
      body: JSON.stringify({ full_name: fullName, email: email, password: password })
    });
    registerForm.reset();
    showLoginView();
    loginInput.value = email;
    passwordInput.focus();
    showLoginNotice("Conta criada com sucesso. Entre com seu email e senha.");
  } catch (err) {
    showRegisterError(err.message);
  } finally {
    registerSubmit.disabled = false;
    registerSubmit.textContent = "Criar conta";
  }
}

async function showMe() {
  try {
    var data = await request("/auth/me");
    var user = {
      full_name: data.user.full_name,
      email: data.user.email,
      username: data.user.username || null,
      roles: (data.roles || []).map(function (role) { return role.name; })
    };
    saveUser(user);
    renderMe(user);
    viewLogin.classList.add("hidden");
    viewRegister.classList.add("hidden");
    viewMe.classList.remove("hidden");
  } catch (err) {
    clearSession();
    showLoginView();
  }
}

function renderMe(user) {
  document.getElementById("me-name").textContent = user.full_name;
  document.getElementById("me-email").textContent = user.email;
  document.getElementById("me-username").textContent = user.username || "—";

  var roles = document.getElementById("me-roles");
  roles.innerHTML = "";
  (user.roles || []).forEach(function (name) {
    var badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = name;
    roles.appendChild(badge);
  });

  linkProfessors.classList.toggle("hidden", !hasRole(user, "super"));
}

function showLoginView() {
  viewMe.classList.add("hidden");
  viewRegister.classList.add("hidden");
  viewLogin.classList.remove("hidden");
}

function showRegisterView() {
  viewLogin.classList.add("hidden");
  viewRegister.classList.remove("hidden");
  hideLoginMessages();
}

function showLoginError(message) {
  loginError.textContent = message;
  loginError.classList.remove("hidden");
}

function showLoginNotice(message) {
  loginNotice.textContent = message;
  loginNotice.classList.remove("hidden");
}

function hideLoginMessages() {
  loginError.classList.add("hidden");
  loginNotice.classList.add("hidden");
}

function showRegisterError(message) {
  registerError.textContent = message;
  registerError.classList.remove("hidden");
}

function setLoading(loading) {
  loginSubmit.disabled = loading;
  loginSubmit.textContent = loading ? "Entrando..." : "Entrar";
}

async function handleChangePassword(event) {
  event.preventDefault();
  changePasswordError.classList.add("hidden");

  var currentPassword = currentPasswordInput.value;
  var newPassword = newPasswordInput.value;
  if (!currentPassword || !newPassword) {
    showChangePasswordError("Preencha os dois campos.");
    return;
  }

  changePasswordSubmit.disabled = true;
  try {
    await request("/auth/change-password", {
      method: "POST",
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword
      })
    });
    // O servidor revogou os refresh tokens: encerra a sessão para
    // entrar novamente com a nova senha.
    clearSession();
    changePasswordForm.reset();
    showLoginView();
    showLoginNotice("Senha alterada com sucesso. Entre com a nova senha.");
  } catch (err) {
    showChangePasswordError(err.message);
  } finally {
    changePasswordSubmit.disabled = false;
  }
}

function showChangePasswordError(message) {
  changePasswordError.textContent = message;
  changePasswordError.classList.remove("hidden");
}

loginForm.addEventListener("submit", handleLogin);
registerForm.addEventListener("submit", handleRegister);
changePasswordForm.addEventListener("submit", handleChangePassword);

showRegisterLink.addEventListener("click", function (event) {
  event.preventDefault();
  showRegisterView();
});

showLoginLink.addEventListener("click", function (event) {
  event.preventDefault();
  showLoginView();
});

logoutButton.addEventListener("click", function () {
  clearSession();
  showLoginView();
  passwordInput.value = "";
});

// Restaura sessão existente (se o token ainda for válido)
if (accessToken()) {
  showMe();
} else {
  showLoginView();
}
