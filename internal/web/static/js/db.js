// Banco local do aparelho. Guarda uma cópia de todos os registros, a fila de
// alterações ainda não enviadas ao computador (outbox) e o cursor da
// sincronização. É o que permite usar o app sem conexão.
//
// Na web, é SQLite (db-sqlite.js, no Worker sqlite-worker.js). Na primeira vez,
// os dados do IndexedDB (db-idb.js) são copiados numa transação só, conferidos e
// marcados; o IndexedDB fica intacto, como backup. Se a cópia falhar, continua
// no IndexedDB e tenta de novo na próxima abertura. Onde o SQLite não abre
// (navegação anônima) fica no IndexedDB; mas, depois da cópia, nunca volta para
// ele (estaria desatualizado). No app do celular é o SQLite nativo
// (sqlite-native.js), com a mesma cópia; sem o plugin no app, fica no IndexedDB.
// A cópia só começa depois que as abas da versão anterior (só IndexedDB) fecham:
// veja DB_VERSION em db-idb.js.
//
// Só uma aba usa o banco SQLite: a nova pede (Web Locks e BroadcastChannel), a
// antiga termina o que estava gravando, solta o banco e avisa a tela
// ("finan-db-elsewhere"); dali em diante as chamadas dela ficam paradas.
import * as idb from "./db-idb.js";
import { metaOp, MOVED, putOp, SCHEMA, sqliteStore } from "./db-sqlite.js";
import { openNative } from "./sqlite-native.js";

export { SITE_IDLE_MS } from "./db-idb.js";

const LOCK = "finan-rich-db";
let backend = null;

const store = () => (backend ??= choose());

async function choose() {
  const app = window.Capacitor?.isNativePlatform?.();
  const plugin = window.Capacitor?.Plugins?.CapacitorSQLite;
  if (app ? !plugin : typeof Worker === "undefined") return fallback();
  if (!app) await takeLock(); // o app tem uma tela só
  let worker;
  try {
    let run;
    if (app) run = await openNative(plugin);
    else {
      worker = new Worker("/static/js/sqlite-worker.js", { type: "module" });
      run = connect(worker);
    }
    // O Worker da página anterior pode demorar a soltar o arquivo (sqlite-worker.js espera).
    const slow = setTimeout(() => waitScreen(true), 3000);
    const started = Date.now();
    try {
      await run(SCHEMA);
    } finally {
      clearTimeout(slow);
      waitScreen(false);
      if (Date.now() - started > 3000) console.warn(`banco: abrir levou ${Date.now() - started} ms`);
    }
    const sql = sqliteStore(run, idb);
    await migrate(run);
    return sql;
  } catch (error) {
    console.warn("banco: o SQLite não abriu:", error?.message || error); // o motivo, para quem investiga
    worker?.terminate();
    giveUp?.();
    giveUp = null;
    return fallback(error);
  }
}

// fallback é o IndexedDB, só se os dados nunca foram para o SQLite: depois da
// cópia, ele está desatualizado, e usar ele esconderia o que foi gravado depois.
async function fallback(cause) {
  if (await idb.getMeta(MOVED, null)) throw new Error("não consegui abrir o banco deste aparelho; recarregue (no navegador, feche antes as outras abas da Rich)", { cause });
  return idb;
}

// connect devolve run(ops) para o Worker. Depois que outra aba assume o banco,
// as chamadas ficam paradas (nada é gravado aqui).
let elsewhere = false;
function connect(worker) {
  let seq = 0;
  const waiting = new Map();
  worker.onmessage = ({ data }) => {
    const p = waiting.get(data.id);
    waiting.delete(data.id);
    if (data.error) p?.reject(Object.assign(new Error(data.error), { code: data.code }));
    else p?.resolve(data.out);
  };
  worker.onerror = (e) => {
    e.preventDefault();
    for (const p of waiting.values()) p.reject(new Error(e.message || "o banco deste navegador não abriu"));
    waiting.clear();
  };
  const send = (msg) => new Promise((resolve, reject) => {
    const id = ++seq;
    waiting.set(id, { resolve, reject });
    worker.postMessage({ id, ...msg });
  });
  channel?.addEventListener("message", async ({ data }) => {
    if (data !== "take" || !giveUp || elsewhere) return;
    elsewhere = true; // o que chegar daqui em diante fica parado
    await send({ close: true }).catch(() => {}); // o Worker termina antes o que já estava na fila
    worker.terminate();
    giveUp();
    giveUp = null;
    window.dispatchEvent(new Event("finan-db-elsewhere"));
  });
  return (ops) => (elsewhere ? new Promise(() => {}) : send({ ops }));
}

// migrate copia o IndexedDB para o SQLite uma vez, numa transação só, e confere
// as contagens dentro dela (diferença desfaz tudo). O IndexedDB fica com os
// registros e a fila, como backup; do meta dele sai tudo (tokens, a senha do
// app), menos a marca e as chaves de criptografia, que só existem lá.
async function migrate(run) {
  const [moved] = await run([{ sql: "SELECT value FROM meta WHERE key = ?", bind: [MOVED], rows: true }]);
  if (moved.length) {
    if (!(await idb.getMeta(MOVED, null))) await idb.setMeta(MOVED, JSON.parse(moved[0].value)); // a aba fechou logo depois da cópia
    return;
  }
  const { records, outbox, meta } = await idb.dump();
  const count = (table, n) => ({ sql: `SELECT 1 WHERE (SELECT count(*) FROM ${table}) = ?`, bind: [n], rows: true, guard: true });
  const at = Date.now();
  await run([ // um valor que não vira JSON falha aqui, antes de gravar
    ...records.map((row) => putOp("records", row)),
    ...outbox.map((row) => putOp("outbox", row)),
    ...meta.map(([key, value]) => metaOp(key, value)),
    count("records", records.length),
    count("outbox", outbox.length),
    count("meta", meta.length),
    metaOp(MOVED, at),
  ]);
  await idb.setMeta(MOVED, at);
  await idb.clearMeta((key, value) => key === MOVED || (typeof CryptoKey !== "undefined" && value instanceof CryptoKey));
}

// Uma aba de cada vez com o banco SQLite.
const channel = typeof BroadcastChannel === "undefined" ? null : new BroadcastChannel(LOCK);
let giveUp = null;
const hold = (lock) => lock && new Promise((resolve) => { giveUp = resolve; });
const waitScreen = (on) => window.dispatchEvent(new CustomEvent("finan-db-wait", { detail: on }));

async function takeLock() {
  if (!navigator.locks) return;
  const free = await new Promise((resolve) => navigator.locks.request(LOCK, { ifAvailable: true }, (lock) => { resolve(!!lock); return hold(lock); }).catch(() => resolve(true)));
  if (free) return;
  // Repete o pedido: a outra aba pode ainda estar abrindo o banco e perder o primeiro.
  const ask = () => channel?.postMessage("take");
  ask();
  const again = setInterval(ask, 1000);
  // A outra aba pode estar suspensa (no celular, em segundo plano): a tela explica.
  const slow = setTimeout(() => waitScreen(true), 3000);
  await new Promise((resolve) => navigator.locks.request(LOCK, (lock) => { resolve(); return hold(lock); }).catch(resolve));
  clearInterval(again);
  clearTimeout(slow);
  waitScreen(false);
}

export const loadLedger = () => store().then((b) => b.loadLedger());
export const allRecords = () => store().then((b) => b.allRecords());
export const saveManyLocal = (items) => store().then((b) => b.saveManyLocal(items));
export const saveLocal = (type, record) => store().then((b) => b.saveLocal(type, record));
export const applyRemote = (changes) => store().then((b) => b.applyRemote(changes));
export const pending = () => store().then((b) => b.pending());
export const clearSent = (sent) => store().then((b) => b.clearSent(sent));
export const getMeta = (key, fallback) => store().then((b) => b.getMeta(key, fallback));
export const setMeta = (key, value) => store().then((b) => b.setMeta(key, value));
export const siteSession = (renew = false, force = false) => store().then((b) => b.siteSession(renew, force));
export const wipe = () => store().then((b) => b.wipe());
export const dump = () => store().then((b) => b.dump());
