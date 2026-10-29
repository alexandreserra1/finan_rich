// Entrar com Google: os dados vão para a pasta de dados do app no Google Drive
// da própria pessoa (gdrive.js; ver docs/GOOGLE.md). Login com código + PKCE:
// o verificador fica neste aparelho e a troca do código e a renovação do token
// passam pelo intermediário (cmd/api), que guarda o client secret do Google.
// O token de renovação fica no cofre do sistema no app e no banco do navegador
// no site; o token de acesso (1 hora) só na memória.
import { getMeta, setMeta } from "./db.js";
import * as device from "./device.js";

// Mesmo endereço de llm.FreeProxyURL (um teste em Go confere).
const SERVER = "https://finan-rich-ia.vercel.app";
// No site, o próprio endereço (é o mesmo programa); no app, o servidor.
const base = () => (device.isSite ? location.origin : SERVER);
const SCOPE = "openid email https://www.googleapis.com/auth/drive.appdata";
const VAULT_KEY = "google-refresh";
const vault = () => window.Capacitor?.Plugins?.NativeBiometric;

let clientId; // undefined: ainda não perguntou ao servidor; "": login não configurado lá
let access = null; // { token, exp }

// available diz se o login com Google está ligado no servidor (as variáveis
// GOOGLE_CLIENT_ID e GOOGLE_CLIENT_SECRET na Vercel). Só no app e no site.
export async function available() {
  // shortcut: só no site por enquanto; no app Android a volta (finanrich://google) já existe,
  // mas precisa de teste num celular de verdade antes de ligar.
  if (!device.isSite) return false;
  if (clientId === undefined) {
    try {
      const res = await fetch(base() + "/oauth/config");
      clientId = res.ok ? (await res.json()).clientId || "" : "";
    } catch {
      return false; // sem internet: pergunta de novo na próxima vez
    }
  }
  return !!clientId;
}

// ---- sessão do site ----
// O site pode estar num computador de outra pessoa: sem uso por IDLE_MS, ele
// pede para entrar de novo (main.js mostra o login no lugar das telas).
export const IDLE_MS = 30 * 60_000;
let touched = 0;

// markActive guarda a hora do último uso (no máximo uma gravação por minuto).
export async function markActive(force = false) {
  const now = Date.now();
  if (!force && now - touched < 60_000) return;
  touched = now;
  await setMeta("site-active", now);
}

// session diz se há conta ligada e se ela ainda vale: { signedIn, expired }.
export async function session() {
  if (!(await getMeta("google", null))) return { signedIn: false, expired: false };
  const last = await getMeta("site-active", 0);
  const expired = !last || Date.now() - last > IDLE_MS;
  return { signedIn: !expired, expired };
}

// account devolve { email } da conta ligada, ou null.
export async function account() {
  const a = await getMeta("google", null);
  return a ? { email: a.email } : null;
}

const b64url = (bytes) => btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
const random = (n) => b64url(crypto.getRandomValues(new Uint8Array(n)));

// signIn abre a página de login do Google. kind diz para onde a página /oauth
// manda a volta: "web" (este site) ou "app" (o app Android, pelo navegador do
// sistema: o Google não deixa entrar dentro do app).
export async function signIn(kind) {
  if (!(await available())) throw new Error("o login com Google não está disponível agora");
  const verifier = random(48);
  const challenge = b64url(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier))));
  const state = `${kind}.${random(18)}`;
  await setMeta("google-pending", { verifier, state, at: Date.now() });
  const url = "https://accounts.google.com/o/oauth2/v2/auth?" + new URLSearchParams({
    client_id: clientId, redirect_uri: base() + "/oauth", response_type: "code", scope: SCOPE,
    code_challenge: challenge, code_challenge_method: "S256", state,
    access_type: "offline", prompt: "select_account consent", // consent garante o token de renovação; select_account faz escolher a conta (num computador compartilhado, o Google não entra sozinho na conta de quem estava lá)
  });
  if (kind === "app") window.open(url, "_system");
  else location.assign(url);
}

async function post(path, body) {
  const res = await fetch(base() + path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  return { ok: res.ok, status: res.status, data };
}

// email lê o e-mail do id_token (só para mostrar qual conta está ligada).
function email(idToken) {
  try {
    return JSON.parse(atob(idToken.split(".")[1].replace(/-/g, "+").replace(/_/g, "/"))).email || "";
  } catch {
    return "";
  }
}

// finish termina o login com o código que o Google devolveu à página /oauth.
export async function finish(code, state) {
  const pending = await getMeta("google-pending", null);
  if (!pending || pending.state !== state || Date.now() - pending.at > 15 * 60_000) {
    throw new Error("o login expirou ou não começou neste aparelho; tente de novo");
  }
  await setMeta("google-pending", null);
  const { ok, data } = await post("/oauth/token", { code, verifier: pending.verifier, redirectUri: base() + "/oauth" });
  if (!ok || !data.access_token) throw new Error("o Google recusou o login (" + (data.error_description || data.error || "tente de novo") + ")");
  if (!data.refresh_token) throw new Error("o Google não mandou a permissão para manter você conectado; tente de novo");
  access = { token: data.access_token, exp: Date.now() + (data.expires_in || 3600) * 1000 };
  await saveRefresh(data.refresh_token);
  await setMeta("google", { email: email(data.id_token || ""), refresh: vault() ? null : data.refresh_token });
  await markActive(true); // a sessão do site começa agora
}

async function saveRefresh(token) {
  if (vault()) await vault().setData({ key: VAULT_KEY, value: token });
}

async function refreshToken() {
  const a = await getMeta("google", null);
  if (!a) return "";
  if (a.refresh) return a.refresh;
  try {
    return (await vault().getData({ key: VAULT_KEY })).value;
  } catch {
    return "";
  }
}

const loginError = () => Object.assign(new Error("entre de novo com Google"), { kind: "google" });

// token devolve um token de acesso válido, renovando quando passa de 1 hora.
export async function token() {
  if (access && access.exp > Date.now() + 60_000) return access.token;
  const refresh = await refreshToken();
  if (!refresh) throw loginError();
  const { ok, status, data } = await post("/oauth/refresh", { refreshToken: refresh });
  if (status === 400 || status === 401) { // a pessoa tirou o acesso na conta Google
    await forget();
    throw loginError();
  }
  if (!ok || !data.access_token) throw new Error("não foi possível falar com o Google agora");
  access = { token: data.access_token, exp: Date.now() + (data.expires_in || 3600) * 1000 };
  return access.token;
}

// expire descarta o token de acesso (o Drive respondeu 401).
export function expire() {
  access = null;
}

async function forget() {
  access = null;
  await setMeta("google", null);
  await setMeta("drive", null);
  if (vault()) await vault().deleteData({ key: VAULT_KEY }).catch(() => {});
}

// signOut desliga a conta: revoga o acesso no Google e apaga o token daqui.
// Os dados continuam neste aparelho e no Drive.
export async function signOut() {
  const refresh = await refreshToken();
  if (refresh) {
    fetch("https://oauth2.googleapis.com/revoke", {
      method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: "token=" + encodeURIComponent(refresh),
    }).catch(() => {});
  }
  await forget();
}
