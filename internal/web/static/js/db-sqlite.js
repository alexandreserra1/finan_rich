// Banco local em SQLite: o mesmo SQL no Worker da web (sqlite-worker.js) e, no
// app, no SQLite nativo do aparelho. As tabelas espelham os armazéns do
// IndexedDB (db-idb.js): records e outbox ({ key: "tipo:id", type, record }) e
// meta (chave → valor em JSON). run(ops) roda as operações numa transação só (ou
// tudo ou nada) e devolve, para cada uma, as linhas (rows) ou quantas linhas
// mudaram; uma operação guard sem linhas desfaz tudo (erro com code "guard").
// Chaves de criptografia (CryptoKey) não viram JSON: ficam no IndexedDB, que as
// guarda sem expor, e aqui fica só a marca { $idb: true }.
import { ledgerOf, SITE_IDLE_MS } from "./db-idb.js";

export const SCHEMA = [
  "CREATE TABLE IF NOT EXISTS records (key TEXT PRIMARY KEY, type TEXT NOT NULL, record TEXT NOT NULL, updated_at INTEGER NOT NULL)",
  "CREATE TABLE IF NOT EXISTS outbox (key TEXT PRIMARY KEY, type TEXT NOT NULL, record TEXT NOT NULL, updated_at INTEGER NOT NULL)",
  "CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
].map((sql) => ({ sql }));

export const MOVED = "idb-moved"; // nas duas pontas: quando os dados foram do IndexedDB para o SQLite

// Bytes passam pelo JSON como { $u8: [...] } (Uint8Array: o sal e o hash da senha
// do app) ou { $ab: [...] } (ArrayBuffer: a chave da IA cifrada no navegador).
// Outro tipo de objeto (Map, Blob...) viraria {} calado: dá erro e nada é gravado.
const enc = (v) => JSON.stringify(v ?? null, (_, x) => {
  if (x instanceof Uint8Array) return { $u8: [...x] };
  if (x instanceof ArrayBuffer) return { $ab: [...new Uint8Array(x)] };
  const proto = x !== null && typeof x === "object" && !Array.isArray(x) && Object.getPrototypeOf(x);
  if (proto && proto !== Object.prototype) throw new TypeError(`o banco não guarda ${x.constructor?.name ?? "este valor"}`);
  return x;
});
const dec = (s) => JSON.parse(s, (_, x) => (Array.isArray(x?.$u8) ? new Uint8Array(x.$u8) : Array.isArray(x?.$ab) ? new Uint8Array(x.$ab).buffer : x));
const isKey = (v) => typeof CryptoKey !== "undefined" && v instanceof CryptoKey;
export const metaOp = (key, value) => ({ sql: "INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", bind: [key, isKey(value) ? '{"$idb":true}' : enc(value)] });
export const putOp = (table, { key, type, record }) => ({
  sql: `INSERT OR REPLACE INTO ${table} (key, type, record, updated_at) VALUES (?, ?, ?, ?)`,
  bind: [key, type, JSON.stringify(record), record.updatedAt ?? 0],
});

// A sessão do site vale com uma conta ligada e o último uso dentro do prazo
// (hora estranha ou no futuro conta como vencida), como em db-idb.js.
const SESSION = `EXISTS (SELECT 1 FROM meta g JOIN meta a ON a.key = 'site-active'
  WHERE g.key = 'google' AND g.value NOT IN ('null', 'false', '0', '""')
  AND json_type(a.value) = 'integer' AND CAST(a.value AS INTEGER) > 0
  AND CAST(a.value AS INTEGER) <= ?1 AND ?1 - CAST(a.value AS INTEGER) < ?2)`;

export function sqliteStore(run, idb) {
  const keyOf = (type, id) => `${type}:${id}`;
  return {
    async loadLedger() {
      const [rows] = await run([{ sql: "SELECT type, record FROM records ORDER BY key", rows: true }]);
      return ledgerOf(rows.map(({ type, record }) => ({ type, record: JSON.parse(record) })));
    },
    async allRecords() {
      const [rows] = await run([{ sql: "SELECT type, record FROM records ORDER BY key", rows: true }]);
      return rows.map(({ type, record }) => ({ type, record: JSON.parse(record) }));
    },
    // No site, só grava com a sessão valendo, conferida na mesma transação.
    async saveManyLocal(items) {
      const site = document.documentElement.dataset.site === "web";
      const ops = site ? [{ sql: `SELECT 1 WHERE ${SESSION}`, bind: [Date.now(), SITE_IDLE_MS], rows: true, guard: true }] : [];
      for (const { type, record } of items) {
        const row = { key: keyOf(type, record.id), type, record };
        ops.push(putOp("records", row), putOp("outbox", row));
      }
      try {
        await run(ops);
      } catch (error) {
        if (error.code !== "guard") throw error;
        window.dispatchEvent(new Event("finan-session-expired"));
        throw Object.assign(new Error("A sessão venceu. Entre de novo; nenhum dado desta alteração foi gravado."), { kind: "login" });
      }
    },
    saveLocal(type, record) {
      return this.saveManyLocal([{ type, record }]);
    },
    // Vence a versão mais recente; no empate, o conteúdo maior em ordem alfabética.
    async applyRemote(changes) {
      if (!changes.length) return 0;
      const done = await run(changes.map(({ type, record }) => ({
        ...putOp("records", { key: keyOf(type, record.id), type, record }),
        sql: `INSERT INTO records (key, type, record, updated_at) VALUES (?, ?, ?, ?)
          ON CONFLICT (key) DO UPDATE SET type = excluded.type, record = excluded.record, updated_at = excluded.updated_at
          WHERE excluded.updated_at > records.updated_at OR (excluded.updated_at = records.updated_at AND excluded.record > records.record)`,
      })));
      return done.reduce((sum, n) => sum + n, 0);
    },
    async pending() {
      const [rows] = await run([{ sql: "SELECT key, type, record FROM outbox ORDER BY key", rows: true }]);
      return rows.map(({ key, type, record }) => ({ key, type, record: JSON.parse(record) }));
    },
    // dump devolve tudo: { records, outbox, meta: [[chave, valor]] }.
    async dump() {
      const row = ({ key, type, record }) => ({ key, type, record: JSON.parse(record) });
      const [records, outbox, meta] = await run(["records", "outbox"].map((t) => ({ sql: `SELECT key, type, record FROM ${t} ORDER BY key`, rows: true }))
        .concat({ sql: "SELECT key, value FROM meta ORDER BY key", rows: true }));
      return { records: records.map(row), outbox: outbox.map(row), meta: meta.map(({ key, value }) => [key, dec(value)]) };
    },
    // Tira da fila o que foi enviado, se não mudou de novo nesse meio tempo.
    async clearSent(sent) {
      if (sent.length) await run(sent.map((item) => ({ sql: "DELETE FROM outbox WHERE key = ? AND updated_at = ?", bind: [item.key, item.record.updatedAt ?? 0] })));
    },
    async getMeta(key, fallback) {
      const [rows] = await run([{ sql: "SELECT value FROM meta WHERE key = ?", bind: [key], rows: true }]);
      const value = rows.length ? dec(rows[0].value) : null;
      return value?.$idb ? idb.getMeta(key, fallback) : value ?? fallback;
    },
    async setMeta(key, value) {
      if (isKey(value)) await idb.setMeta(key, value);
      const [old] = await run([{ sql: "SELECT value FROM meta WHERE key = ?", bind: [key], rows: true }, metaOp(key, value)]);
      if (!isKey(value) && old[0]?.value === '{"$idb":true}') await idb.setMeta(key, null); // a chave antiga não fica para trás
    },
    // { signedIn, expired, expiresAt }; com renew, uma sessão ainda válida ganha a
    // hora de agora (no máximo uma gravação por minuto, salvo force).
    async siteSession(renew = false, force = false) {
      const now = Date.now();
      const ops = [{ sql: "SELECT key, value FROM meta WHERE key IN ('google', 'site-active')", rows: true }];
      if (renew) {
        ops.push({ sql: `UPDATE meta SET value = ?1 WHERE key = 'site-active' AND ${SESSION} AND (?3 OR ?1 - CAST(value AS INTEGER) >= 60000)`, bind: [now, SITE_IDLE_MS, force ? 1 : 0] });
      }
      const [rows, renewed] = await run(ops);
      const meta = Object.fromEntries(rows.map(({ key, value }) => [key, dec(value)]));
      const last = meta["site-active"];
      const invalid = !Number.isSafeInteger(last) || last <= 0 || now < last || now - last >= SITE_IDLE_MS;
      const account = !!meta.google;
      return { signedIn: account && !invalid, expired: account && invalid, expiresAt: invalid || !account ? 0 : (renewed ? now : last) + SITE_IDLE_MS };
    },
    // Apaga tudo deste aparelho (ao sair da conta), inclusive o backup do IndexedDB.
    // Só a marca da migração fica: sem ela, a próxima abertura tentaria copiar de novo.
    async wipe() {
      const [, , , mark] = await run([{ sql: "DELETE FROM records" }, { sql: "DELETE FROM outbox" },
        { sql: "DELETE FROM meta WHERE key <> ?", bind: [MOVED] }, { sql: "SELECT value FROM meta WHERE key = ?", bind: [MOVED], rows: true }]);
      await idb.wipe();
      if (mark.length) await idb.setMeta(MOVED, dec(mark[0].value));
    },
  };
}
