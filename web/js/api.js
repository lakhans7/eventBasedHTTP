// Thin fetch wrapper: attaches the bearer access token, transparently
// refreshes it once on a 401 (using the httpOnly refresh cookie + the
// double-submit CSRF cookie — see docs/security.md), and redirects to
// login if the session truly can't be restored. No framework, no build step.
const API_BASE = "/api/v1";

function getCookie(name) {
  const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"));
  return match ? decodeURIComponent(match[1]) : "";
}

function getAccessToken() {
  return sessionStorage.getItem("access_token") || "";
}

function setAccessToken(token) {
  if (token) sessionStorage.setItem("access_token", token);
  else sessionStorage.removeItem("access_token");
}

async function refreshAccessToken() {
  const resp = await fetch(API_BASE + "/auth/refresh", {
    method: "POST",
    credentials: "include",
    headers: { "X-CSRF-Token": getCookie("csrf_token") },
  });
  if (!resp.ok) {
    setAccessToken("");
    return false;
  }
  const data = await resp.json();
  setAccessToken(data.access_token);
  return true;
}

async function apiFetch(path, options = {}) {
  options.headers = Object.assign({}, options.headers);
  const isFormData = options.body instanceof FormData;
  if (!isFormData && options.body && typeof options.body !== "string") {
    options.body = JSON.stringify(options.body);
    options.headers["Content-Type"] = "application/json";
  }
  const token = getAccessToken();
  if (token) options.headers["Authorization"] = "Bearer " + token;
  options.credentials = "include";

  let resp = await fetch(API_BASE + path, options);
  if (resp.status === 401 && token) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      options.headers["Authorization"] = "Bearer " + getAccessToken();
      resp = await fetch(API_BASE + path, options);
    }
  }
  if (resp.status === 401) {
    if (!location.pathname.endsWith("login.html")) {
      location.href = "/login.html";
    }
    throw new Error("Not authenticated");
  }
  return resp;
}

async function apiJSON(path, options = {}) {
  const resp = await apiFetch(path, options);
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    const message = (data.error && data.error.message) || "Request failed";
    throw new Error(message);
  }
  return data;
}

async function requireAuth() {
  if (!getAccessToken()) {
    const ok = await refreshAccessToken();
    if (!ok) {
      location.href = "/login.html";
      return null;
    }
  }
  try {
    const data = await apiJSON("/auth/me");
    return data.user;
  } catch (e) {
    location.href = "/login.html";
    return null;
  }
}

function formatCents(cents, currency) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: (currency || "usd").toUpperCase() }).format((cents || 0) / 100);
}

function escapeHTML(str) {
  const div = document.createElement("div");
  div.textContent = str == null ? "" : String(str);
  return div.innerHTML;
}
