// Banco local no IndexedDB: o de antes do SQLite (db.js escolhe). Continua em uso
// onde o SQLite não abre (navegação anônima, app do celular até o motor nativo),
// guarda o backup de antes da migração e as chaves de criptografia (CryptoKey),
// que só o IndexedDB guarda sem expor. Tem uma cópia de todos os registros, a
// fila de alterações ainda não enviadas ao computador (outbox) e o cursor da
// sincronização.

const DB_NAME = "finan_rich";
const DB_VERSION = 1;
export const SITE_IDLE_MS = 30 * 60_000; // site: sem uso por esse tempo, pede o login de novo

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
  return ledgerOf(await all(db.transaction("records").objectStore("records")));
}

// ledgerOf monta o domain.Ledger com linhas { type, record }.
export function ledgerOf(rows) {
  const ledger = { categories: [], transactions: [], debts: [], prefs: [], notes: [], goals: [], recurring: [], accounts: [], transfers: [] };
  const lists = {
    category: ledger.categories, transaction: ledger.transactions, debt: ledger.debts, pref: ledger.prefs,
    note: ledger.notes, goal: ledger.goals, recurring: ledger.recurring,
    account: ledger.accounts, transfer: ledger.transfers,
  };
  for (const row of rows) lists[row.type]?.push(row.record);
  return ledger;
}

// clearMeta apaga do meta o que keep(chave, valor) não mandar ficar.
export async function clearMeta(keep) {
  const db = await open();
  const tx = db.transaction("meta", "readwrite");
  const req = tx.objectStore("meta").openCursor();
  req.onsuccess = () => {
    const cursor = req.result;
    if (!cursor) return;
    if (!keep(cursor.key, cursor.value)) cursor.delete();
    cursor.continue();
  };
  return done(tx);
}

// dump devolve tudo (a migração ao SQLite e os testes usam): { records, outbox, meta: [[chave, valor]] }.
export async function dump() {
  const db = await open();
  const tx = db.transaction(["records", "outbox", "meta"]);
  const keys = new Promise((resolve, reject) => {
    const req = tx.objectStore("meta").getAllKeys();
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
  const [records, outbox, metaKeys, metaValues] = await Promise.all([all(tx.objectStore("records")), all(tx.objectStore("outbox")), keys, all(tx.objectStore("meta"))]);
  return { records, outbox, meta: metaKeys.map((k, i) => [k, metaValues[i]]) };
}

// allRecords devolve todos os registros com o tipo, inclusive os apagados (é o
// que vai para o arquivo do Google Drive).
export async function allRecords() {
  const db = await open();
  return (await all(db.transaction("records").objectStore("records"))).map(({ type, record }) => ({ type, record }));
}

// saveManyLocal grava várias alterações de uma vez (ex.: restaurar um backup).
// No site, só com a sessão valendo, conferida na mesma transação: vencida, nada é gravado.
export async function saveManyLocal(items) {
  const db = await open();
  const site = document.documentElement.dataset.site === "web";
  const tx = db.transaction(site ? ["records", "outbox", "meta"] : ["records", "outbox"], "readwrite");
  let authorizationError = null;
  const save = () => {
    for (const { type, record } of items) {
      const key = keyOf(type, record.id);
      tx.objectStore("records").put({ key, type, record });
      tx.objectStore("outbox").put({ key, type, record });
    }
  };
  if (site) {
    readSiteSession(tx.objectStore("meta"), (state) => {
      if (state.signedIn) save();
      else {
        authorizationError = Object.assign(new Error("A sessão venceu. Entre de novo; nenhum dado desta alteração foi gravado."), { kind: "login" });
        tx.abort();
      }
    });
  } else save();
  try {
    await done(tx);
  } catch (error) {
    if (authorizationError) {
      window.dispatchEvent(new Event("finan-session-expired"));
      throw authorizationError;
    }
    throw error;
  }
}

// saveLocal grava uma alteração feita neste aparelho e a coloca na fila de envio.
export async function saveLocal(type, record) {
  return saveManyLocal([{ type, record }]);
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

// readSiteSession lê a sessão do site dentro de uma transação já aberta, para
// quem chama decidir e gravar nela mesma (outra aba não muda nada no meio).
// Hora do último uso estranha ou no futuro (relógio mexido) conta como vencida.
function readSiteSession(store, receive) {
  const account = store.get("google");
  const activity = store.get("site-active");
  activity.onsuccess = () => {
    const now = Date.now();
    const last = activity.result;
    const invalid = !Number.isSafeInteger(last) || last <= 0 || now < last || now - last >= SITE_IDLE_MS;
    receive({ signedIn: !!account.result && !invalid, expired: !!account.result && invalid, expiresAt: invalid || !account.result ? 0 : last + SITE_IDLE_MS }, now, last);
  };
}

// siteSession devolve { signedIn, expired, expiresAt }; com renew, uma sessão
// ainda válida ganha a hora de agora (no máximo uma gravação por minuto, salvo force).
export async function siteSession(renew = false, force = false) {
  const db = await open();
  const transaction = db.transaction("meta", renew ? "readwrite" : "readonly");
  const store = transaction.objectStore("meta");
  let result;
  readSiteSession(store, (state, now, last) => {
    result = state;
    if (renew && state.signedIn && (force || now - last >= 60_000)) {
      store.put(now, "site-active");
      result.expiresAt = now + SITE_IDLE_MS;
    }
  });
  await done(transaction);
  return result;
}

// wipe apaga tudo deste aparelho (ao sair da conta).
export async function wipe() {
  const db = await open();
  const tx = db.transaction(["records", "outbox", "meta"], "readwrite");
  for (const name of ["records", "outbox", "meta"]) tx.objectStore(name).clear();
  return done(tx);
}
