// Consultor sem computador: o celular chama uma IA na nuvem (Groq, Gemini...)
// direto, com os nomes de lojas, dívidas e metas trocados por apelidos antes de
// sair (regras em Go, no advisor.wasm, que só o app do celular leva).
//
// A chave da IA fica cifrada (AES-GCM) com uma chave deste aparelho que o
// navegador não deixa exportar: quem copiar o banco do app não lê a chave.
import * as db from "./db.js";
import "./wasm_exec.js";
import { today } from "./ui.js";

let ready = null;

function loadAdvisor() {
  ready ??= new Promise((resolve, reject) => {
    window.addEventListener("finan-advisor-ready", () => resolve(), { once: true });
    const go = new Go();
    fetch("/static/advisor.wasm")
      .then((r) => {
        if (!r.ok) throw new Error("o consultor sem computador só existe no app do celular");
        return r.arrayBuffer();
      })
      .then((b) => WebAssembly.instantiate(b, go.importObject))
      .then(({ instance }) => go.run(instance))
      .catch((err) => { ready = null; reject(err); });
  });
  return ready;
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
  const cfg = { preset, model, enabled, key: old?.preset === preset ? old.key : null };
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

export async function presets() {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.cloudPresets());
}

// ask pergunta à IA na nuvem: { answer, proposals }. Sempre anonimizado.
export async function ask(ledger, history, question) {
  const cfg = await config();
  if (!cfg?.key) throw new Error("configure a IA na nuvem em Ajustes do app");
  await loadAdvisor();
  const req = {
    ledger, question, today: today(),
    history: history.map(({ role, content }) => ({ role, content })),
    ai: { preset: cfg.preset, model: cfg.model, apiKey: await apiKey(cfg), baseUrl: cfg.baseUrl || "" },
    anonymize: true,
  };
  return unwrap(await window.finanAdvisor.ask(JSON.stringify(req)));
}

export async function applied(ledger, proposal) {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.proposalApplied(JSON.stringify(ledger), JSON.stringify(proposal)));
}

// A conversa com a IA na nuvem fica só neste celular.
export const loadChat = () => db.getMeta("cloud-chat", []);
export const saveChat = (history) => db.setMeta("cloud-chat", history);
