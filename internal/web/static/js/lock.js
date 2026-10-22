// Bloqueio do app do celular: pede a digital/rosto (ou a senha do app) ao
// abrir e ao voltar depois de 1 minuto fora do app. Só existe no app; no
// navegador do computador vale a senha de acesso pela rede.
//
// A senha do app (4 a 6 números) é a alternativa quando a biometria falha ou
// não existe. Do banco do aparelho só sai o hash dela (PBKDF2-SHA256 com sal).
// O bloqueio protege de quem pega o celular desbloqueado; ele não cifra os
// dados guardados no aparelho.
import * as db from "./db.js";
import { isApp } from "./device.js";
import * as cloud from "./cloud-ai.js";

const AWAY_MS = 60_000; // fora do app por mais que isso: pede de novo
const ITERATIONS = 600_000;
const MAX_TRIES = 5; // a cada 5 senhas erradas, espera WAIT_MS
const WAIT_MS = 30_000;

const bio = () => window.Capacitor?.Plugins?.NativeBiometric;
const config = () => db.getMeta("app-lock", null); // { salt, hash, iterations } ou null

export const validPIN = (pin) => /^[0-9]{4,6}$/.test(pin);

async function derive(pin, salt, iterations) {
  const base = await crypto.subtle.importKey("raw", new TextEncoder().encode(pin), "PBKDF2", false, ["deriveBits"]);
  return new Uint8Array(await crypto.subtle.deriveBits({ name: "PBKDF2", hash: "SHA-256", salt, iterations }, base, 256));
}

const same = (a, b) => a.length === b.length && a.reduce((diff, x, i) => diff | (x ^ b[i]), 0) === 0;

async function checkPIN(pin) {
  const cfg = await config();
  return !!cfg && same(await derive(pin, cfg.salt, cfg.iterations), cfg.hash);
}

// biometry diz se o celular tem digital ou rosto cadastrados para apps.
export async function biometry() {
  try {
    return !!(await bio()?.isAvailable({ useFallback: false }))?.isAvailable;
  } catch {
    return false;
  }
}

export async function status() {
  return { on: !!(await config()), bio: await biometry() };
}

export async function enable(pin) {
  if (!validPIN(pin)) throw new Error("a senha do app precisa ter de 4 a 6 números");
  const salt = crypto.getRandomValues(new Uint8Array(16));
  await db.setMeta("app-lock", { salt, hash: await derive(pin, salt, ITERATIONS), iterations: ITERATIONS });
  await db.setMeta("app-lock-fails", null);
}

export const disable = () => db.setMeta("app-lock", null);

// ---------- tela de bloqueio ----------

let overlay = null;
let prompting = false; // a janela da digital/rosto está aberta (o app "sai" e "volta")
let awayAt = 0;

function lock() {
  document.documentElement.classList.add("app-locked");
  if (overlay) return;
  overlay = document.createElement("div");
  overlay.className = "app-lock";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-modal", "true");
  overlay.setAttribute("aria-labelledby", "app-lock-title");
  overlay.innerHTML = `<section class="panel login">
    <p class="brand big">finan<span>_rich</span></p>
    <h2 id="app-lock-title">App bloqueado</h2>
    <button type="button" data-lock="bio" hidden>Usar digital ou rosto</button>
    <form data-lock="pin">
      <label>Senha do app <input name="pin" type="password" inputmode="numeric" autocomplete="off" maxlength="6" required></label>
      <button type="submit">Desbloquear</button>
    </form>
    <p class="lock-msg" role="alert"></p>
    <button type="button" class="secondary" data-lock="forgot">Esqueci a senha do app</button>
  </section>`;
  document.body.append(overlay);
  const form = overlay.querySelector('[data-lock="pin"]');
  const msg = overlay.querySelector(".lock-msg");
  const bioButton = overlay.querySelector('[data-lock="bio"]');

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const button = form.querySelector("button");
    button.disabled = true;
    try {
      msg.textContent = await tryPIN(form.elements.pin.value);
      if (!msg.textContent) unlock();
    } finally {
      button.disabled = false;
      form.elements.pin.value = "";
    }
  });
  bioButton.addEventListener("click", () => askBiometry(msg));
  overlay.querySelector('[data-lock="forgot"]').addEventListener("click", forgot);

  biometry().then((has) => {
    bioButton.hidden = !has;
    if (has) askBiometry(msg);
    else form.elements.pin.focus();
  });
}

function unlock() {
  overlay?.remove();
  overlay = null;
  document.documentElement.classList.remove("app-locked");
}

// tryPIN confere a senha e devolve a mensagem de erro ("" se acertou). Errar
// muitas vezes faz esperar, mesmo fechando e abrindo o app.
async function tryPIN(pin) {
  const fails = (await db.getMeta("app-lock-fails", null)) || { count: 0, until: 0 };
  const wait = fails.until - Date.now();
  if (wait > 0) return `Muitas tentativas. Espere ${Math.ceil(wait / 1000)} segundos.`;
  if (await checkPIN(pin)) {
    await db.setMeta("app-lock-fails", null);
    return "";
  }
  fails.count++;
  if (fails.count % MAX_TRIES === 0) fails.until = Date.now() + WAIT_MS;
  await db.setMeta("app-lock-fails", fails);
  return fails.until > Date.now() ? `Senha errada. Muitas tentativas: espere ${WAIT_MS / 1000} segundos.` : "Senha errada.";
}

async function askBiometry(msg) {
  if (prompting) return;
  prompting = true;
  try {
    await bio().verifyIdentity({
      reason: "Desbloquear o finan_rich", title: "Desbloquear o finan_rich",
      negativeButtonText: "Usar a senha do app", useFallback: false,
    });
    unlock();
  } catch {
    msg.textContent = "Não deu com a digital ou o rosto. Use a senha do app.";
    overlay?.querySelector('input[name="pin"]').focus();
  } finally {
    prompting = false;
    awayAt = 0;
  }
}

// Sem a senha e sem a biometria, o jeito é apagar os dados deste celular (os
// que já estão no computador continuam lá) e começar de novo.
async function forgot() {
  if (!confirm("Esqueceu a senha do app? Dá para tirar o bloqueio apagando os dados deste celular. Os que já estão no computador continuam lá e voltam ao conectar de novo. Apagar?")) return;
  await cloud.forget().catch(() => {});
  await db.wipe();
  location.reload();
}

// ---------- abrir e voltar ao app ----------

function away() {
  if (!prompting && !awayAt) awayAt = Date.now();
}
async function back() {
  const since = awayAt;
  awayAt = 0;
  if (prompting || !since || Date.now() - since < AWAY_MS) return;
  if (await config()) lock();
}

if (isApp) {
  // Nada aparece até saber se o bloqueio está ligado.
  document.documentElement.classList.add("app-locked");
  config()
    .then((cfg) => (cfg ? lock() : unlock()))
    .catch(unlock);
  document.addEventListener("visibilitychange", () => (document.hidden ? away() : back()));
  const app = window.Capacitor?.Plugins?.App;
  app?.addListener("pause", away);
  app?.addListener("resume", back);
}
