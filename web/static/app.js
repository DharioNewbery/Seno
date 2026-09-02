"use strict";

// Seno — comportamento da página de login (MVP).
// Convenções em web/STYLEGUIDE.md.

var API = "/api/v1";
var ACCESS_KEY = "seno.access";
var REFRESH_KEY = "seno.refresh";

var viewLogin = document.getElementById("view-login");
var viewMe = document.getElementById("view-me");
var loginForm = document.getElementById("login-form");
var loginInput = document.getElementById("login");
var passwordInput = document.getElementById("password");
var loginError = document.getElementById("login-error");
var loginSubmit = document.getElementById("login-submit");
var logoutButton = document.getElementById("logout");

function accessToken() {
  return localStorage.getItem(ACCESS_KEY);
}

function clearSession() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

// request injeta o Bearer, interpreta o envelope padrao da API e repete a
// chamada uma unica vez caso o access token tenha expirado (refresh).
async function request(path, options, retried) {
  options = options || {};
  var headers = { "Content-Type": "application/json" };
  if (accessToken()) {
    headers["Authorization"] = "Bearer " + accessToken();
  }

  var resp = await fetch(API + path, {
    method: options.method || "GET",
    headers: headers,
    body: options.body
  });
  var payload = await resp.json().catch(function () { return null; });

  if (resp.status === 401 && !retried && path !== "/auth/login" && path !== "/auth/refresh") {
    if (await tryRefresh()) {
      return request(path, options, true);
    }
  }

  if (!resp.ok) {
    throw new Error((payload && payload.error) || "Falha de comunicação com o servidor");
  }
  return payload && payload.data;
}

async function tryRefresh() {
  var refreshToken = localStorage.getItem(REFRESH_KEY);
  if (!refreshToken) {
    return false;
  }

  var resp = await fetch(API + "/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken })
  });
  if (!resp.ok) {
    clearSession();
    return false;
  }

  var payload = await resp.json();
  localStorage.setItem(ACCESS_KEY, payload.data.access_token);
  localStorage.setItem(REFRESH_KEY, payload.data.refresh_token);
  return true;
}

async function handleLogin(event) {
  event.preventDefault();
  hideLoginError();

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

async function showMe() {
  try {
    var data = await request("/auth/me");
    document.getElementById("me-name").textContent = data.user.full_name;
    document.getElementById("me-email").textContent = data.user.email;
    document.getElementById("me-username").textContent = data.user.username || "—";

    var roles = document.getElementById("me-roles");
    roles.innerHTML = "";
    (data.roles || []).forEach(function (role) {
      var badge = document.createElement("span");
      badge.className = "badge";
      badge.textContent = role.name;
      roles.appendChild(badge);
    });

    viewLogin.classList.add("hidden");
    viewMe.classList.remove("hidden");
  } catch (err) {
    clearSession();
    showLoginView();
  }
}

function showLoginView() {
  viewMe.classList.add("hidden");
  viewLogin.classList.remove("hidden");
}

function showLoginError(message) {
  loginError.textContent = message;
  loginError.classList.remove("hidden");
}

function hideLoginError() {
  loginError.classList.add("hidden");
}

function setLoading(loading) {
  loginSubmit.disabled = loading;
  loginSubmit.textContent = loading ? "Entrando..." : "Entrar";
}

loginForm.addEventListener("submit", handleLogin);

logoutButton.addEventListener("click", function () {
  clearSession();
  showLoginView();
  passwordInput.value = "";
});

// Restaura sessao existente (se o token ainda for valido)
if (accessToken()) {
  showMe();
} else {
  showLoginView();
}
