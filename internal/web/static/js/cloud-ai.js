// Consultor sem computador: o celular chama uma IA na nuvem (Groq, Gemini...)
// direto, com os nomes de lojas, dívidas e metas trocados por apelidos antes de
// sair (regras em Go, no advisor.wasm, que só o app do celular leva).
//
// A chave da IA nunca fica em texto puro no banco do app, no backup do
// finan_rich nem na sincronização:
// - no app do celular, fica no cofre do sistema (Android Keystore / iOS
//   Keychain, pelo plugin NativeBiometric). Quem copiar a pasta do app só leva
//   a chave cifrada; a chave que decifra fica no Keystore/Keychain.
// - no navegador (sem o app), fica cifrada (AES-GCM) com uma chave deste
//   aparelho que o navegador não deixa exportar, guardada no mesmo banco: quem
//   tiver acesso total ao aparelho ainda consegue usá-la.
import * as db from "./db.js";
import "./wasm_exec.js";
import { today } from "./ui.js";

let advisorLoaded = null;

// Se a internet cai no meio da resposta, o Go (net/http no WebAssembly) fecha a
// resposta quebrada com stream.cancel() e não trata a promessa. O erro já chega
// ao ask(); aqui só evitamos o "Uncaught (in promise)".
window.addEventListener("unhandledrejection", (e) => {
  if (e.reason instanceof TypeError && e.reason.message === "network error") e.preventDefault();
});

export function loadAdvisor() {
  advisorLoaded ??= new Promise((resolve, reject) => {
    window.addEventListener("finan-advisor-ready", () => resolve(), { once: true });
    const go = new Go();
    fetch("/static/advisor.wasm")
      .then((r) => {
        if (!r.ok) throw new Error("o consultor sem computador só existe no app do celular");
        return r.arrayBuffer();
      })
      .then((b) => WebAssembly.instantiate(b, go.importObject))
      .then(({ instance }) => go.run(instance))
      .catch((err) => { advisorLoaded = null; reject(err); });
  });
  return advisorLoaded;
}

function unwrap(json) {
  const r = JSON.parse(json);
  if (r.error) throw new Error(r.error);
  return r.ok;
}

async function deviceKey() {
  let key = await db.getMeta("cloud-ai-key", null);
  if (!key) {
    key = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]);
    await db.setMeta("cloud-ai-key", key);
  }
  return key;
}

const SECURE_KEY = "cloud-ai-key";
const vault = () => window.Capacitor?.Plugins?.NativeBiometric;
// inVault: a chave da IA vai para o cofre do sistema (só no app do celular).
export const inVault = () => !!vault();

// config: { preset, model, enabled, key } (a chave nunca em texto puro). key é
// { vault: true } (no cofre do sistema) ou { iv, data } (cifrada no navegador).
export async function config() {
  const cfg = await db.getMeta("cloud-ai", null);
  return cfg?.key?.data && vault() ? toVault(cfg) : cfg;
}

// toVault move a chave do formato antigo (cifrada no banco do app) para o
// cofre do sistema e apaga a antiga. Se algo falhar, fica como estava.
async function toVault(cfg) {
  try {
    await vault().setData({ key: SECURE_KEY, value: await decrypt(cfg.key) });
    const moved = { ...cfg, key: { vault: true } };
    await db.setMeta("cloud-ai", moved);
    await db.setMeta("cloud-ai-key", null);
    return moved;
  } catch {
    return cfg;
  }
}

async function encrypt(apiKey) {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const data = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, await deviceKey(), new TextEncoder().encode(apiKey));
  return { iv, data };
}

async function decrypt(key) {
  const plain = await crypto.subtle.decrypt({ name: "AES-GCM", iv: key.iv }, await deviceKey(), key.data);
  return new TextDecoder().decode(plain);
}

export async function saveConfig({ preset, model, enabled, apiKey }) {
  const old = await config();
  const same = old?.preset === preset;
  const cfg = { preset, model, enabled, key: same ? old.key : null, baseUrl: same ? old.baseUrl : undefined };
  if (apiKey && vault()) {
    await vault().setData({ key: SECURE_KEY, value: apiKey.trim() });
    cfg.key = { vault: true };
  } else if (apiKey) {
    cfg.key = await encrypt(apiKey.trim());
  }
  if (!cfg.key && old?.key?.vault) await vault()?.deleteData({ key: SECURE_KEY }).catch(() => {});
  await db.setMeta("cloud-ai", cfg);
  return cfg;
}

export async function forget() {
  if (vault()) await vault().deleteData({ key: SECURE_KEY }).catch(() => {});
  await db.setMeta("cloud-ai", null);
  await db.setMeta("cloud-chat", []);
}

async function apiKey(cfg) {
  if (!cfg.key.vault) return decrypt(cfg.key);
  try {
    return (await vault().getData({ key: SECURE_KEY })).value;
  } catch {
    throw new Error("não consegui ler a chave da IA guardada neste celular; cole a chave de novo em Ajustes");
  }
}

// No grátis do finan_rich não há chave: vai uma identificação aleatória deste
// aparelho, só para o limite de perguntas por aparelho.
const FREE = "finan";
async function deviceId() {
  let id = await db.getMeta("device-id", null);
  if (!id) {
    id = crypto.randomUUID();
    await db.setMeta("device-id", id);
  }
  return id;
}

// ready: a IA na nuvem está configurada (com chave, ou a grátis).
export const ready = (cfg) => !!(cfg?.key || cfg?.preset === FREE);

const aiConfig = async (cfg) => ({
  preset: cfg.preset, model: cfg.model, baseUrl: cfg.baseUrl || "",
  apiKey: cfg.preset === FREE ? await deviceId() : await apiKey(cfg),
});

// check testa a chave guardada com uma pergunta mínima, sem dados da pessoa.
export async function check() {
  const cfg = await config();
  await loadAdvisor();
  return unwrap(await window.finanAdvisor.checkKey(JSON.stringify(await aiConfig(cfg))));
}

export async function presets() {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.cloudPresets());
}

// ask pergunta à IA na nuvem: { answer, proposals }. Sempre anonimizado.
export async function ask(ledger, history, question, agent = "jade") {
  const cfg = await config();
  if (!ready(cfg)) throw new Error("configure a IA na nuvem em Ajustes do app");
  await loadAdvisor();
  const req = {
    ledger, question, today: today(),
    history: history.map(({ role, content }) => ({ role, content })),
    ai: await aiConfig(cfg),
    anonymize: true,
    agent,
  };
  return unwrap(await window.finanAdvisor.ask(JSON.stringify(req)));
}

// Respostas do app, sem IA (regras em Go, advisor.wasm): { answer, proposals, app }.
// prepared: a resposta pronta da sugestão, ou null se a pergunta precisa da IA.
// offline: a resposta com as contas do personagem quando a IA não pôde responder.
const appRequest = (ledger, question, agent) => JSON.stringify({ ledger, question, agent, today: today() });
export async function prepared(ledger, question, agent = "jade") {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.prepared(appRequest(ledger, question, agent))) || null;
}
export async function offline(ledger, question, agent = "jade") {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.offline(appRequest(ledger, question, agent)));
}

// categorize: "IA sugere" da importação, com a IA na nuvem deste celular.
export async function categorize(categories, items) {
  const cfg = await config();
  if (!ready(cfg)) throw new Error("conecte ao computador ou configure a IA na nuvem em Ajustes para usar a IA");
  await loadAdvisor();
  return unwrap(await window.finanAdvisor.categorize(JSON.stringify({ categories, items, ai: await aiConfig(cfg) })));
}

export async function applied(ledger, proposal) {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.proposalApplied(JSON.stringify(ledger), JSON.stringify(proposal)));
}

// A conversa com a IA na nuvem fica só neste celular.
export const loadChat = () => db.getMeta("cloud-chat", []);
export const saveChat = (history) => db.setMeta("cloud-chat", history);
