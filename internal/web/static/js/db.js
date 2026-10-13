// Banco local do aparelho (IndexedDB). Guarda uma cópia de todos os registros,
// a fila de alterações ainda não enviadas ao computador (outbox) e o cursor da
// sincronização. É o que permite usar o app sem conexão.

const DB_NAME = "finan_rich";
const DB_VERSION = 1;
export const TYPES = ["category", "transaction", "debt", "pref", "note"];

let dbPromise = null;

function open() {
  dbPromise ??= new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      db.createObjectStore("records", { keyPath: "key" }); // {key: "tipo:id", type, record}
      db.createObjectStore("outbox", { keyPath: "key" });  // {key, type, record}: última versão pendente
      db.createObjectStore("meta");                         // cursor etc.
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
  return dbPromise;
}

const keyOf = (type, id) => `${type}:${id}`;

function done(tx) {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = tx.onabort = () => reject(tx.error);
  });
}

function all(store) {
  return new Promise((resolve, reject) => {
    const req = store.getAll();
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

// loadLedger devolve todos os registros no formato do domain.Ledger do Go.
export async function loadLedger() {
  const db = await open();
  const rows = await all(db.transaction("records").objectStore("records"));
  const ledger = { categories: [], transactions: [], debts: [], prefs: [], notes: [] };
  const lists = { category: ledger.categories, transaction: ledger.transactions, debt: ledger.debts, pref: ledger.prefs, note: ledger.notes };
  for (const row of rows) lists[row.type]?.push(row.record);
  return ledger;
}

// saveManyLocal grava várias alterações de uma vez (ex.: restaurar um backup).
export async function saveManyLocal(items) {
  const db = await open();
  const tx = db.transaction(["records", "outbox"], "readwrite");
  for (const { type, record } of items) {
    const key = keyOf(type, record.id);
    tx.objectStore("records").put({ key, type, record });
    tx.objectStore("outbox").put({ key, type, record });
  }
  return done(tx);
}

// saveLocal grava uma alteração feita neste aparelho e a coloca na fila de envio.
export async function saveLocal(type, record) {
  const db = await open();
  const tx = db.transaction(["records", "outbox"], "readwrite");
  const key = keyOf(type, record.id);
  tx.objectStore("records").put({ key, type, record });
  tx.objectStore("outbox").put({ key, type, record });
  return done(tx);
}

// applyRemote grava registros vindos do computador. Vence a versão mais
// recente, como no servidor: uma edição local ainda não enviada não é perdida.
export async function applyRemote(changes) {
  if (!changes.length) return 0;
  const db = await open();
  const tx = db.transaction("records", "readwrite");
  const store = tx.objectStore("records");
  let applied = 0;
  for (const { type, record } of changes) {
    const key = keyOf(type, record.id);
    const req = store.get(key);
    req.onsuccess = () => {
      const cur = req.result?.record;
      if (!cur || record.updatedAt > cur.updatedAt ||
          (record.updatedAt === cur.updatedAt && JSON.stringify(record) > JSON.stringify(cur))) {
        store.put({ key, type, record });
        applied++;
      }
    };
  }
  await done(tx);
  return applied;
}

export async function pending() {
  const db = await open();
  return all(db.transaction("outbox").objectStore("outbox"));
}

// clearSent remove da fila o que foi enviado, se não mudou de novo nesse meio tempo.
export async function clearSent(sent) {
  const db = await open();
  const tx = db.transaction("outbox", "readwrite");
  const store = tx.objectStore("outbox");
  for (const item of sent) {
    const req = store.get(item.key);
    req.onsuccess = () => {
      if (req.result && req.result.record.updatedAt === item.record.updatedAt) store.delete(item.key);
    };
  }
  return done(tx);
}

export async function getMeta(key, fallback) {
  const db = await open();
  return new Promise((resolve, reject) => {
    const req = db.transaction("meta").objectStore("meta").get(key);
    req.onsuccess = () => resolve(req.result ?? fallback);
    req.onerror = () => reject(req.error);
  });
}

export async function setMeta(key, value) {
  const db = await open();
  const tx = db.transaction("meta", "readwrite");
  tx.objectStore("meta").put(value, key);
  return done(tx);
}

// wipe apaga tudo deste aparelho (ao sair da conta).
export async function wipe() {
  const db = await open();
  const tx = db.transaction(["records", "outbox", "meta"], "readwrite");
  for (const name of ["records", "outbox", "meta"]) tx.objectStore(name).clear();
  return done(tx);
}
