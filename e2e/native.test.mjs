// Teste do banco SQLite nativo do app (js/sqlite-native.js) sem celular: um
// plugin CapacitorSQLite falso, feito sobre o SQLite WebAssembly na memória da
// página, imita o de verdade (a primeira linha só com os nomes das colunas que o
// iPhone devolve, a conexão que sobra depois de recarregar a tela, cada chamada
// demorando um pouco como a ponte com o nativo). Confere o contrato de run(ops)
// que db-sqlite.js usa e roda o banco local de verdade (sqliteStore) em cima dele.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node native.test.mjs
import { chromium as chromiumEngine, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";

const OFF = Number(process.env.PORT_OFFSET || 0); // PORT_OFFSET=100: roda ao lado de outra cópia do repositório
const B = `http://127.0.0.1:${18104 + OFF}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-native-"));
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", dataDir, "-no-browser", "-addr", B.slice(7)], { stdio: "ignore", env: { ...process.env, FINAN_UPDATE_CHECK: "0" } });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${B}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage();
  const errors = [];
  watchErrors(page, errors);
  await page.goto(`${B}/offline`); // página do mesmo endereço, sem o app rodando
  const checks = await page.evaluate(async () => {
    const checks = [];
    const check = (cond, msg) => checks.push([!!cond, msg]);
    const json = (v) => JSON.stringify(v);
    const fails = (p) => p.then(() => null, (e) => e);

    // O plugin falso. Conexões ficam guardadas por "RW_nome", como no nativo, e
    // sobrevivem até checkConnectionsConsistency (o banco na memória é o arquivo).
    const sqlite3 = await (await import("/static/vendor/sqlite/sqlite3.js")).default();
    const mem = new sqlite3.oo1.DB(":memory:");
    const conns = new Set();
    const bridge = () => new Promise((r) => setTimeout(r, 1));
    const conn = async (o) => {
      await bridge();
      const key = `${o.readonly ? "RO" : "RW"}_${o.database}`;
      if (!conns.has(key)) throw new Error(`No available connection for ${o.database}`);
    };
    const bind = (values) => {
      if (!Array.isArray(values)) throw new Error("Must provide an Array of values");
      return values.length ? values : undefined;
    };
    const fake = {
      async checkConnectionsConsistency({ dbNames }) {
        if (!dbNames.length) conns.clear();
        return { result: false };
      },
      async createConnection({ database, readonly }) {
        const key = `${readonly ? "RO" : "RW"}_${database}`;
        if (conns.has(key)) throw new Error(`Connection ${database} already exists`);
        conns.add(key);
      },
      async open(o) { await conn(o); },
      async beginTransaction(o) { await conn(o); mem.exec("BEGIN"); return { changes: { changes: 0 } }; },
      async commitTransaction(o) { await conn(o); mem.exec("COMMIT"); return { changes: { changes: 0 } }; },
      async rollbackTransaction(o) { await conn(o); mem.exec("ROLLBACK"); return { changes: { changes: 0 } }; },
      async run(o) {
        await conn(o);
        if (o.transaction !== false) throw new Error("teste: run sem transaction: false abriria outra transação");
        const before = mem.changes(true);
        mem.exec({ sql: o.statement, bind: bind(o.values) });
        return { changes: { changes: mem.changes(true) - before, lastId: 0 } };
      },
      async query(o) {
        await conn(o);
        const rows = mem.selectObjects(o.statement, bind(o.values));
        return { values: rows.length ? [{ ios_columns: Object.keys(rows[0]) }, ...rows] : [] }; // como no iPhone
      },
    };

    const { openNative } = await import("/static/js/sqlite-native.js");
    let run = await openNative(fake);

    const first = await run([
      { sql: "CREATE TABLE t (k TEXT PRIMARY KEY, n INTEGER)" },
      { sql: "INSERT INTO t VALUES (?, ?), (?, ?)", bind: ["a", 1, "b", 2] },
      { sql: "SELECT k, n FROM t ORDER BY k", rows: true },
      { sql: "SELECT k FROM t WHERE k = 'x'", rows: true },
      { sql: "UPDATE t SET n = n + 10" },
      { sql: "DELETE FROM t WHERE k = ?", bind: ["nao-tem"] },
    ]);
    check(json(first) === json([0, 2, [{ k: "a", n: 1 }, { k: "b", n: 2 }], [], 2, 0]),
      `linhas voltam como objetos (sem os nomes de colunas do iPhone) e as contagens de mudanças batem: ${json(first)}`);

    const count = async () => (await run([{ sql: "SELECT count(*) AS n FROM t", rows: true }]))[0][0].n;
    const guard = await fails(run([
      { sql: "INSERT INTO t VALUES ('c', 3)" },
      { sql: "SELECT 1 WHERE 0", rows: true, guard: true },
      { sql: "INSERT INTO t VALUES ('d', 4)" },
    ]));
    check(guard?.code === "guard" && (await count()) === 2, `guard sem linhas desfaz a gravação anterior e devolve code "guard" (${guard?.code}, ${await count()} linhas)`);

    const bad = await fails(run([{ sql: "INSERT INTO t VALUES ('e', 5)" }, { sql: "INSERT INTO nao_existe VALUES (1)" }]));
    check(bad && bad.code !== "guard" && (await count()) === 2, `erro de SQL desfaz tudo e chega a quem chamou (${bad?.message})`);

    // Sem a fila, o segundo BEGIN cairia no meio da primeira transação e falharia.
    const many = Array.from({ length: 10 }, (_, i) => ({ sql: "INSERT INTO t VALUES (?, ?)", bind: [`m${i}`, i] }));
    const [a, b, c] = await Promise.allSettled([
      run(many),
      run([{ sql: "INSERT INTO t VALUES ('x', 0)" }, { sql: "INSERT INTO t VALUES ('a', 0)" }]), // 'a' repetido: falha
      run([{ sql: "SELECT count(*) AS n FROM t", rows: true }]),
    ]);
    check(a.status === "fulfilled" && b.status === "rejected" && c.status === "fulfilled" && c.value[0][0].n === 12,
      `duas chamadas ao mesmo tempo não se misturam e uma falha não trava a fila (${a.status}, ${b.status}, ${c.value?.[0][0].n ?? c.reason?.message})`);

    // Tela recarregada: a conexão nativa ainda existe e openNative abre de novo.
    const again = await fails(fake.createConnection({ database: "finan", version: 1, encrypted: false, mode: "no-encryption", readonly: false }));
    run = await openNative(fake);
    check(/already exists/.test(again?.message) && (await count()) === 12, `abre de novo depois de recarregar a tela, com os dados lá (${again?.message})`);

    // O banco local de verdade em cima do plugin.
    const { sqliteStore, SCHEMA } = await import("/static/js/db-sqlite.js");
    const idb = await import("/static/js/db-idb.js");
    await run(SCHEMA);
    const store = sqliteStore(run, idb);
    await store.saveLocal("transaction", { id: "t1", description: "Mercado", amountCents: 1234, updatedAt: 100 });
    const ledger = await store.loadLedger();
    check(json(ledger.transactions) === json([{ id: "t1", description: "Mercado", amountCents: 1234, updatedAt: 100 }]), `saveLocal grava e loadLedger lê: ${json(ledger.transactions)}`);
    const sent = await store.pending();
    check(sent.length === 1 && sent[0].key === "transaction:t1" && sent[0].record.description === "Mercado", `pending devolve a fila de envio: ${json(sent)}`);
    await store.saveLocal("transaction", { id: "t1", description: "Mercado e feira", amountCents: 1500, updatedAt: 200 });
    await store.clearSent(sent);
    const still = (await store.pending()).length;
    await store.clearSent(await store.pending());
    check(still === 1 && (await store.pending()).length === 0, `clearSent só tira da fila o que não mudou depois do envio (${still} depois da edição)`);

    const applied = await store.applyRemote([
      { type: "transaction", record: { id: "t1", description: "Do computador", updatedAt: 300 } },
      { type: "transaction", record: { id: "t2", description: "Novo", updatedAt: 50 } },
    ]);
    const older = await store.applyRemote([{ type: "transaction", record: { id: "t1", description: "Velho", updatedAt: 250 } }]);
    const names = (await store.loadLedger()).transactions.map((t) => t.description);
    check(applied === 2 && older === 0 && json(names) === json(["Do computador", "Novo"]), `applyRemote: o mais novo vence, o mais velho fica de fora (${applied}, ${older}, ${json(names)})`);

    await store.setMeta("bytes", new Uint8Array([1, 2, 3]));
    const bytes = await store.getMeta("bytes", null);
    check(bytes instanceof Uint8Array && bytes.join() === "1,2,3" && (await store.getMeta("nada", "padrão")) === "padrão", `setMeta/getMeta guardam bytes e devolvem o padrão (${bytes})`);
    return checks;
  });
  for (const [cond, msg] of checks) ok(cond, msg);

  // Atualização do app (native.js): avisa só quando o site publica uma versão mais nova
  // que a instalada, e fica quieto sem conexão.
  const up = await browser.newPage({ bypassCSP: true }); // a página /offline do computador só fala com ele; o app fala com o site
  watchErrors(up, errors);
  let latest = { versionName: "1.12.0", versionCode: 11200 };
  await up.route("https://finan-rich-ia.vercel.app/baixar/versao", (r) => (latest ? r.fulfill({ json: latest, headers: { "Access-Control-Allow-Origin": "*" } }) : r.abort()));
  await up.goto(`${B}/offline`);
  const newVersion = () => up.evaluate(async () => {
    document.documentElement.dataset.app = "mobile"; // como no app
    window.Capacitor = { getPlatform: () => "android", Plugins: { App: { getInfo: async () => ({ version: "1.11.0", build: "11100" }) } } };
    return (await import("/static/js/native.js")).newVersion();
  });
  ok(await newVersion() === "1.12.0", "versão nova publicada: o app avisa (1.11.0 instalada, 1.12.0 no site)");
  latest = { versionName: "1.11.0", versionCode: 11100 };
  ok(await newVersion() === null, "já na última versão: não avisa");
  latest = null;
  ok(await newVersion() === null, "sem conexão: não avisa nem dá erro");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.kill();
  rmSync(dataDir, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
