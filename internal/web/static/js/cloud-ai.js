// Consultor sem computador: o celular chama uma IA na nuvem (Groq, Gemini...)
// direto, com os nomes de lojas, dívidas e metas trocados por apelidos antes de
// sair (regras em Go, no advisor.wasm, que só o app do celular leva).
//
// A chave da IA fica cifrada (AES-GCM) com uma chave deste aparelho que o
// navegador não deixa exportar: não aparece em texto puro no banco do app, no
// backup do finan_rich nem na sincronização. Quem tiver acesso total ao
// aparelho (root, cópia da pasta do app) ainda consegue usá-la.
import * as db from "./db.js";
import "./wasm_exec.js";
import { today } from "./ui.js";

let advisorLoaded = null;

function loadAdvisor() {
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

// config: { preset, model, enabled, key: {iv, data} } (a chave nunca em texto puro).
export const config = () => db.getMeta("cloud-ai", null);

export async function saveConfig({ preset, model, enabled, apiKey }) {
  const old = await config();
  const same = old?.preset === preset;
  const cfg = { preset, model, enabled, key: same ? old.key : null, baseUrl: same ? old.baseUrl : undefined };
  if (apiKey) {
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const data = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, await deviceKey(), new TextEncoder().encode(apiKey.trim()));
    cfg.key = { iv, data };
  }
  await db.setMeta("cloud-ai", cfg);
  return cfg;
}

export async function forget() {
  await db.setMeta("cloud-ai", null);
  await db.setMeta("cloud-chat", []);
}

async function apiKey(cfg) {
  const plain = await crypto.subtle.decrypt({ name: "AES-GCM", iv: cfg.key.iv }, await deviceKey(), cfg.key.data);
  return new TextDecoder().decode(plain);
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
export async function ask(ledger, history, question) {
  const cfg = await config();
  if (!ready(cfg)) throw new Error("configure a IA na nuvem em Ajustes do app");
  await loadAdvisor();
  const req = {
    ledger, question, today: today(),
    history: history.map(({ role, content }) => ({ role, content })),
    ai: await aiConfig(cfg),
    anonymize: true,
  };
  return unwrap(await window.finanAdvisor.ask(JSON.stringify(req)));
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
