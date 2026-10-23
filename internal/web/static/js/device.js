// Onde este aparelho sincroniza.
//
// No navegador, o app é servido pelo próprio computador: sincroniza com o
// mesmo endereço da página, usando o cookie de login. No app do celular
// (Capacitor), as telas vêm de dentro do app e o computador fica em outro
// endereço, guardado no pareamento junto com o token do aparelho.

import { getMeta, setMeta } from "./db.js";

export const isApp = document.documentElement.dataset.app === "mobile";
// isSite: as mesmas telas publicadas na internet (site/). Os dados ficam no
// navegador; não há pareamento (uma página https não alcança o computador por http).
export const isSite = document.documentElement.dataset.site === "web";
export const here = isSite ? "neste navegador" : "neste celular";

// O token do aparelho vale como senha para sincronizar: no app ele fica no
// cofre do sistema (o mesmo da chave da IA), não no banco do app.
const TOKEN = "server-token";
const vault = () => window.Capacitor?.Plugins?.NativeBiometric;
let cached; // { ...server, token }: o cofre é lido uma vez por abertura

// server devolve { url, token, device, pairedAt } ou null se não pareado.
export async function server() {
  if (cached !== undefined) return cached;
  const s = await getMeta("server", null);
  if (s?.token && vault()) {
    // Formato antigo (token em texto no banco do app): passa para o cofre.
    try {
      await vault().setData({ key: TOKEN, value: s.token });
      const { token, ...rest } = s;
      await setMeta("server", { ...rest, vault: true });
    } catch {}
    return (cached = s);
  }
  if (s?.vault) {
    try {
      return (cached = { ...s, token: (await vault().getData({ key: TOKEN })).value });
    } catch {
      return { ...s, token: "" }; // sem o token o computador recusa e a tela pede para parear de novo
    }
  }
  return (cached = s);
}

// endpoint monta a chamada para uma rota da API, ou null se o app não foi pareado.
export async function endpoint(path) {
  if (!isApp) return { url: path, init: { credentials: "same-origin", headers: {} } };
  const s = await server();
  if (!s) return null;
  return { url: s.url + path, init: { headers: { Authorization: `Bearer ${s.token}` } } };
}

// normalizeURL aceita "192.168.0.10:8080" e devolve "http://192.168.0.10:8080".
export function normalizeURL(text) {
  let s = String(text || "").trim().replace(/\/+$/, "");
  if (!s) throw new Error("informe o endereço do computador");
  if (!/^https?:\/\//i.test(s)) s = "http://" + s;
  const u = new URL(s);
  return u.origin;
}

// deviceName dá um nome legível ao aparelho na lista do computador.
export function deviceName() {
  const ua = navigator.userAgent;
  const android = ua.match(/Android [\d.]+; ([^;)]+?)(?: Build|\))/);
  if (android) return android[1].trim();
  if (/iPhone/.test(ua)) return "iPhone";
  if (/iPad/.test(ua)) return "iPad";
  if (/Android/.test(ua)) return "Android";
  return "Celular";
}

// pair troca o código mostrado no computador por um token deste aparelho.
export async function pair(address, code) {
  const url = normalizeURL(address);
  let res;
  try {
    res = await fetch(url + "/api/parear", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code, name: deviceName() }),
    });
  } catch {
    throw new Error(`não consegui falar com ${url}. Confira se o celular está no mesmo Wi-Fi, se o finan_rich está aberto no computador e se o acesso pelo celular está ligado.`);
  }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `o computador respondeu ${res.status}`);
  const paired = { url, device: body.device, pairedAt: Date.now() };
  if (vault()) {
    await vault().setData({ key: TOKEN, value: body.token });
    await setMeta("server", { ...paired, vault: true });
  } else {
    await setMeta("server", { ...paired, token: body.token });
  }
  cached = { ...paired, token: body.token };
  // Outro computador (ou o mesmo, depois de desparear) tem outra numeração de
  // revisões: baixa tudo de novo. O que está na fila deste aparelho é enviado.
  await setMeta("cursor", 0);
  return url;
}

export async function unpair() {
  await vault()?.deleteData({ key: TOKEN }).catch(() => {});
  cached = null;
  await setMeta("server", null);
}

// pairingLink lê finanrich://parear?url=...&codigo=... (QR code do computador).
export function pairingLink(link) {
  try {
    const u = new URL(link);
    if (u.protocol !== "finanrich:" || !(u.host === "parear" || u.pathname.replace(/^\/+/, "") === "parear")) return null;
    return { url: u.searchParams.get("url") || "", code: u.searchParams.get("codigo") || "" };
  } catch {
    return null;
  }
}
