"use strict";

// Seno — utilitários compartilhados entre páginas: sessão e chamadas à API.
// Convenções em web/STYLEGUIDE.md. Deve ser incluído antes do script da página.

var API = "/api/v1";
var ACCESS_KEY = "seno.access";
var REFRESH_KEY = "seno.refresh";
var USER_KEY = "seno.user";

function accessToken() {
  return localStorage.getItem(ACCESS_KEY);
}

function sessionUser() {
  var raw = localStorage.getItem(USER_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch (err) {
    return null;
  }
}

function saveUser(user) {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

function hasRole(user, role) {
  return !!(user && user.roles && user.roles.indexOf(role) !== -1);
}

function clearSession() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
  localStorage.removeItem(USER_KEY);
}

// request injeta o Bearer, interpreta o envelope padrão da API e repete a
// chamada uma única vez caso o access token tenha expirado (refresh).
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
