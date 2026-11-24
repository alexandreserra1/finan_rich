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
// Sem nada guardado, vale a IA grátis da Rich: o Consultor conversa de fábrica, sem conta
// e sem chave (os nomes saem com apelidos). Desligar guarda { off: true }.
export async function config() {
  const cfg = await db.getMeta("cloud-ai", null);
  if (!cfg) return { preset: FREE, model: "auto", enabled: false };
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

// turnOff: a pessoa desligou a IA (a grátis não volta sozinha; "Ligar" nos Ajustes ou no Consultor).
export async function turnOff() {
  await forget();
  await db.setMeta("cloud-ai", { off: true });
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
export const ready = (cfg) => !cfg?.off && !!(cfg?.key || cfg?.preset === FREE);

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

// No app, o CapacitorHttp troca o fetch por um nativo que entrega a resposta inteira
// de uma vez. Os pedidos à IA na nuvem vão pelo fetch do WebView (CapacitorWebFetch),
// que lê aos poucos. Se o provedor não aceitar CORS, o WebView barra no preflight,
// antes de enviar a pergunta, e o pedido volta para o nativo. O Banco Central (rates)
// também: pelo nativo, o Content-Length chega repetido ("0, 39") e o Go recusa a resposta.
const aiBases = new Set();
function streamFromWebView(base) {
  const web = window.CapacitorWebFetch;
  if (!web || !base) return;
  aiBases.add(base);
  if (window.fetch.richAI) return;
  const native = window.fetch;
  const routed = async (res, opt) => {
    const url = String(res?.url ?? res);
    if (![...aiBases].some((b) => url.startsWith(b))) return native(res, opt);
    try {
      return await web(res, opt);
    } catch (err) {
      if (err?.name === "AbortError") throw err; // a pessoa ou o prazo cancelou: não reenvia
      return native(res, opt);
    }
  };
  routed.richAI = true;
  window.fetch = routed;
}

// rates: Selic, CDI e IPCA do Banco Central, buscados uma vez por dia (só a data
// vai no pedido, nenhum dado da pessoa). Sem internet, valem as últimas guardadas;
// sem nenhuma, o consultor segue sem taxas. O Banco Central aceita CORS.
const BCB = "https://api.bcb.gov.br";
let ratesToday = null;
let ratesRetry = 0; // depois de uma falha, só tenta de novo em 10 min (sem internet, não atrasa cada pergunta)
async function rates() {
  const day = today();
  if (ratesToday?.day === day) return ratesToday.rates;
  const saved = await db.getMeta("taxas-bcb", null).catch(() => null);
  if (saved?.day === day) return (ratesToday = saved).rates;
  if (Date.now() < ratesRetry || navigator.onLine === false) return saved?.rates || null;
  try {
    await loadAdvisor();
    streamFromWebView(BCB);
    ratesToday = { day, rates: unwrap(await window.finanAdvisor.rates()) };
    await db.setMeta("taxas-bcb", ratesToday).catch(() => {});
    return ratesToday.rates;
  } catch {
    ratesRetry = Date.now() + 10 * 60 * 1000;
    return saved?.rates || null;
  }
}

// ask pergunta à IA na nuvem: { answer, proposals }. Sempre anonimizado.
// onText recebe a resposta aos poucos, já com os nomes de volta. Pergunta de número
// volta com a resposta do app (app: true), sem chamar a IA; askAI manda para a IA mesmo assim.
export async function ask(ledger, history, question, agent = "otavio", onText = () => {}, askAI = false) {
  const cfg = await config();
  if (!ready(cfg)) throw new Error("configure a IA na nuvem em Ajustes do app");
  await loadAdvisor();
  const ai = await aiConfig(cfg);
  streamFromWebView(ai.baseUrl || (await presets()).find((p) => p.id === cfg.preset)?.baseUrl);
  const req = {
    ledger, question, today: today(),
    history: history.map(({ role, content }) => ({ role, content })),
    ai,
    anonymize: true,
    agent,
    rates: await rates(),
    askAI,
  };
  return unwrap(await window.finanAdvisor.ask(JSON.stringify(req), onText));
}

// Resposta do app, sem IA (regras em Go, advisor.wasm): { answer, proposals, app },
// com as contas do personagem quando a IA não pôde responder.
const appRequest = async (ledger, question, agent) => JSON.stringify({ ledger, question, agent, today: today(), rates: await rates() });
export async function offline(ledger, question, agent = "otavio") {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.offline(await appRequest(ledger, question, agent)));
}
// numbers: as contas do personagem para mostrar na hora, enquanto a IA escreve ("" na pergunta pronta).
export async function numbers(ledger, question, agent = "otavio") {
  await loadAdvisor();
  return unwrap(window.finanAdvisor.numbers(await appRequest(ledger, question, agent)))?.answer || "";
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
