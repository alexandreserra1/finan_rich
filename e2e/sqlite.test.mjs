// Teste do banco SQLite da web (js/sqlite-worker.js): grava, recarrega a página
// e lê de novo, servido pelo programa do computador e pelo site. Mede quanto
// tempo o banco leva para abrir e confere que a busca por texto (FTS5) existe.
// Usa um perfil de navegador persistente: na janela temporária do Playwright o
// WebKit se comporta como a navegação anônima do Safari, sem OPFS (aí db.js
// fica no IndexedDB).
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich SITE_BIN=../dist/finansite node sqlite.test.mjs
import { chromium as chromiumEngine, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";
import assert from "node:assert/strict";

const OFF = Number(process.env.PORT_OFFSET || 0); // PORT_OFFSET=100: roda ao lado de outra cópia do repositório
const PC = `http://127.0.0.1:${18101 + OFF}`;
const SITE = `http://127.0.0.1:${18102 + OFF}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-sqlite-"));
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const servers = [
  spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", dataDir, "-no-browser", "-addr", PC.slice(7)], { stdio: "ignore", env: { ...process.env, FINAN_UPDATE_CHECK: "0" } }),
  spawn(process.env.SITE_BIN || "../dist/finansite", [], { stdio: "ignore", env: { ...process.env, PORT: String(18102 + OFF), GROQ_API_KEY: "teste", FINAN_PROXY_UPSTREAM: "http://127.0.0.1:9" } }),
];
for (const base of [PC, SITE]) {
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(`${base}/healthz`)).ok) break; } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
}

// Roda as operações no Worker do banco e devolve { out, ms } ou { error }.
const run = (page, ops) => page.evaluate((ops) => new Promise((resolve) => {
  const t0 = performance.now();
  const w = new Worker("/static/js/sqlite-worker.js", { type: "module" });
  w.onerror = (e) => resolve({ error: e.message || "erro ao carregar o Worker" });
  w.onmessage = ({ data }) => { w.terminate(); resolve({ ...data, ms: Math.round(performance.now() - t0) }); };
  w.postMessage({ id: 1, ops });
}), ops);

const profile = mkdtempSync(join(tmpdir(), "finan-e2e-sqlite-perfil-"));
const browser = await chromium.launchPersistentContext(profile, { executablePath: process.env.CHROMIUM_PATH || undefined, serviceWorkers: "block" });
try {
  for (const [name, base] of [["computador", PC], ["site", SITE]]) {
    const page = await browser.newPage();
    const errors = [];
    watchErrors(page, errors);
    await page.goto(`${base}/static/favicon.svg`); // página do mesmo endereço, sem o app rodando
    const first = await run(page, [
      { sql: "CREATE TABLE IF NOT EXISTS t (k TEXT PRIMARY KEY, v TEXT)" },
      { sql: "INSERT OR REPLACE INTO t VALUES (?, ?)", bind: ["chave", JSON.stringify({ valor: 42 })] },
      { sql: "CREATE VIRTUAL TABLE IF NOT EXISTS busca USING fts5(texto)" },
      { sql: "INSERT INTO busca VALUES ('aluguel vence dia 10')" },
      { sql: "SELECT count(*) AS n FROM busca WHERE busca MATCH 'aluguel'", rows: true },
    ]);
    ok(!first.error && first.out[4][0].n >= 1, `${name}: grava e a busca por texto (FTS5) funciona (${first.error || `abriu e gravou em ${first.ms} ms`})`);
    // Uma transação com erro não deixa nada pela metade.
    const bad = await run(page, [
      { sql: "INSERT OR REPLACE INTO t VALUES ('metade', '1')" },
      { sql: "INSERT INTO tabela_que_nao_existe VALUES (1)" },
    ]);
    await page.reload();
    const again = await run(page, [{ sql: "SELECT k, v FROM t ORDER BY k", rows: true }]);
    ok(!again.error && JSON.stringify(again.out[0]) === JSON.stringify([{ k: "chave", v: '{"valor":42}' }]),
      `${name}: depois de recarregar, o dado continua lá e a transação com erro não gravou nada (${bad.error ? "erro devolvido" : "sem erro!"}; ${JSON.stringify(again.out?.[0] ?? again.error)})`);
    ok(errors.length === 0, `${name}: sem erros de JavaScript ${JSON.stringify(errors)}`);
    await page.close();
  }

  const page = await browser.newPage();
  const errors = [];
  watchErrors(page, errors);
  await page.goto(`${PC}/static/favicon.svg`);
  await page.evaluate(async () => {
    const root = await navigator.storage.getDirectory();
    for await (const [name] of root.entries()) await root.removeEntry(name, { recursive: true });
  });
  await page.goto(`${PC}/lancamentos`);
  await page.waitForSelector(".page-head", { timeout: 20000 });
  await page.evaluate(() => window.finanSync());
  const category = await page.evaluate(async () => (await (await import("/static/js/db.js")).loadLedger()).categories[0].id);
  const second = await browser.newPage();
  watchErrors(second, errors);
  await second.goto(`${PC}/lancamentos`);
  await page.waitForSelector("#elsewhere [data-use-here]", { timeout: 10000 });
  await Promise.all([page.waitForNavigation(), page.click("#elsewhere [data-use-here]")]);
  await second.waitForSelector("#elsewhere [data-use-here]", { timeout: 10000 });
  await page.evaluate(() => window.finanSync());
  const record = {
    id: "depois-apagar", description: "Depois de apagar", kind: "saida", amountCents: 1234,
    categoryId: category, date: new Date().toISOString().slice(0, 10), updatedAt: Date.now(),
  };
  await page.evaluate(async (record) => (await import("/static/js/db.js")).saveLocal("transaction", record), record);
  const remote = await fetch(`${PC}/api/sync`, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ cursor: 0, changes: [{ type: "transaction", record: { ...record, id: "registro-anterior", description: "Registro remoto anterior" } }] }),
  });
  assert(remote.ok);

  let responseReady, releaseResponse;
  const requested = new Promise((resolve) => { responseReady = resolve; });
  const released = new Promise((resolve) => { releaseResponse = resolve; });
  await page.route("**/api/sync", async (route) => {
    const response = await route.fetch();
    responseReady(await response.json());
    await released;
    await route.fulfill({ response });
  });
  await page.evaluate(() => { window.syncBeforeWipe = window.finanSync(); });
  const response = await Promise.race([requested, page.evaluate(() => window.syncBeforeWipe).then(() => {
    throw new Error("A sincronização terminou antes de alcançar a resposta atrasada do teste.");
  })]);
  assert(response.cursor > 0);
  assert(response.changes.some((change) => change.record.id === record.id));
  assert(response.changes.some((change) => change.record.id === "registro-anterior"));
  await page.evaluate(async (record) => {
    const db = await import("/static/js/db.js");
    const wiped = db.wipe();
    const saved = db.saveLocal("transaction", { ...record, description: "Depois de apagar, versão nova" });
    await Promise.all([wiped, saved]);
  }, record);
  releaseResponse();
  await page.evaluate(() => window.syncBeforeWipe);
  await page.unrouteAll({ behavior: "wait" });
  const afterWipe = await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const { state } = await import("/static/js/sync.js");
    return {
      cursor: await db.getMeta("cursor", 0), categories: (await db.loadLedger()).categories.length,
      pending: (await db.pending()).map((item) => item.record.description),
      clock: await db.getMeta("clock-offset", null), status: state.status,
      transactions: (await db.loadLedger()).transactions.map((item) => item.description),
    };
  });
  console.log(`depois de apagar com resposta anterior pendente: ${JSON.stringify(afterWipe)}`);
  assert.equal(afterWipe.cursor, 0, "apagar o banco não pode conservar o cursor de uma sincronização anterior");
  assert.equal(afterWipe.categories, 0);
  assert.deepEqual(afterWipe.pending, ["Depois de apagar, versão nova"]);
  assert.deepEqual(afterWipe.transactions, ["Depois de apagar, versão nova"]);
  assert.equal(afterWipe.clock, null);
  assert.notEqual(afterWipe.status, "ok");
  await page.evaluate(() => window.finanSync());
  await page.waitForSelector("text=Depois de apagar", { timeout: 20000 });
  const restored = await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const idb = await import("/static/js/db-idb.js");
    return {
      categories: (await db.loadLedger()).categories.length,
      moved: await idb.getMeta("idb-moved", 0),
      backup: (await idb.loadLedger()).transactions.map((record) => record.id),
    };
  });
  assert(restored.categories > 0);
  assert(restored.moved > 0);
  assert(!restored.backup.includes("depois-apagar"));
  ok(true, "apagar com o app ativo invalida a resposta antiga sem restaurar cursor e dados nem limpar a fila nova; a próxima sincronização funciona");
  const ordered = await page.evaluate(async (record) => {
    const db = await import("/static/js/db.js");
    const key = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]);
    const original = IDBObjectStore.prototype.put;
    let wiped, saved;
    IDBObjectStore.prototype.put = function (value, name) {
      const request = original.apply(this, arguments);
      if (this.name === "meta" && name === "chave-em-curso") request.addEventListener("success", () => {
        wiped = db.wipe();
        saved = db.saveLocal("transaction", { ...record, description: "Gravado após a limpeza enfileirada" });
      }, { once: true });
      return request;
    };
    try {
      await db.setMeta("chave-em-curso", key);
      await Promise.all([wiped, saved]);
    } finally {
      IDBObjectStore.prototype.put = original;
    }
    return {
      triggered: !!wiped, meta: (await db.dump()).meta.map(([name]) => name),
      pending: (await db.pending()).map((item) => item.record.description),
    };
  }, record);
  assert(ordered.triggered);
  assert(!ordered.meta.includes("chave-em-curso"));
  assert.deepEqual(ordered.pending, ["Gravado após a limpeza enfileirada"]);
  ok(true, "wipe espera uma gravação já em curso entre IndexedDB e SQLite e preserva a alteração feita depois da limpeza");
  ok(errors.length === 0, `troca de aba e limpeza com sincronização pendente: sem erros de JavaScript ${JSON.stringify(errors)}`);
  await second.close();
  await page.close();

  for (const phase of ["busca", "download", "upload"]) {
    const drivePage = await browser.newPage();
    watchErrors(drivePage, errors);
    await drivePage.route(`${PC}/teste-drive`, (route) => route.fulfill({
      contentType: "text/html", body: '<!doctype html><html lang="pt-BR" data-site="web"><title>Teste do Drive</title></html>',
    }));
    await drivePage.route("**/oauth/refresh", (route) => route.fulfill({
      json: { access_token: "acesso-ficticio", expires_in: 3600 },
    }));
    let driveReady, releaseDrive;
    const requestedDrive = new Promise((resolve) => { driveReady = resolve; });
    const releasedDrive = new Promise((resolve) => { releaseDrive = resolve; });
    let requests = 0;
    await drivePage.route("https://www.googleapis.com/**", async (route) => {
      const headers = {
        "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Headers": "Authorization, Content-Type",
        "Access-Control-Allow-Methods": "GET, PATCH, POST, OPTIONS",
      };
      if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
      requests++;
      const url = new URL(route.request().url());
      const upload = route.request().method() === "PATCH";
      const download = url.searchParams.get("alt") === "media";
      if ((phase === "upload" && upload) || (phase === "download" && download) || phase === "busca") {
        driveReady();
        await releasedDrive;
      }
      await route.fulfill({
        headers,
        json: upload ? { version: "3", modifiedTime: "2030-01-01T00:00:00Z" }
          : download ? { items: [{ type: "transaction", record: { ...record, id: "drive-remoto-antigo" } }] }
          : { files: [{ id: "arquivo-ficticio", version: phase === "upload" ? "1" : "2" }] },
      });
    });
    await drivePage.goto(`${PC}/teste-drive`);
    await drivePage.evaluate(async (record) => {
      const db = await import("/static/js/db.js");
      await db.wipe();
      await db.setMeta("google", { email: "antiga@example.test", refresh: null });
      await db.setMeta("site-active", Date.now());
      await db.setMeta("drive", { version: "1", at: 0 });
      await db.saveLocal("transaction", record);
      const drive = await import("/static/js/gdrive.js");
      window.clockCalls = 0;
      window.driveRace = drive.syncDrive(true, () => { window.clockCalls++; }).then(
        () => "ok", (error) => error.kind,
      );
    }, record);
    await Promise.race([requestedDrive, drivePage.evaluate(() => window.driveRace).then((result) => {
      throw new Error(`O Drive terminou antes de alcançar a resposta atrasada do teste: ${result}`);
    })]);
    await drivePage.evaluate(async (record) => {
      const db = await import("/static/js/db.js");
      await db.wipe();
      await db.setMeta("google", { email: "nova@example.test", refresh: null });
      await db.setMeta("site-active", Date.now());
      await db.setMeta("drive", { version: "nova", at: 777 });
      await db.saveLocal("transaction", { ...record, description: "Fila nova do Drive" });
    }, record);
    releaseDrive();
    const driveResult = await drivePage.evaluate(async () => {
      const result = await window.driveRace;
      const db = await import("/static/js/db.js");
      return {
        result, drive: await db.getMeta("drive", null), account: await db.getMeta("google", null),
        records: (await db.allRecords()).map((item) => item.record.description),
        pending: (await db.pending()).map((item) => item.record.description), clockCalls: window.clockCalls,
      };
    });
    console.log(`Drive depois de apagar durante ${phase}: ${JSON.stringify(driveResult)}`);
    assert.equal(driveResult.result, "reset");
    assert.deepEqual(driveResult.drive, { version: "nova", at: 777 });
    assert.equal(driveResult.account.email, "nova@example.test");
    assert.deepEqual(driveResult.records, ["Fila nova do Drive"]);
    assert.deepEqual(driveResult.pending, ["Fila nova do Drive"]);
    assert.equal(driveResult.clockCalls, 0);
    assert.equal(requests, phase === "busca" ? 1 : 2);
    ok(true, `Drive: wipe durante ${phase} invalida a resposta sem restaurar dados, metadados ou relógio nem limpar a fila nova`);
    if (phase === "upload") {
      const failedReset = await drivePage.evaluate(async () => {
        const db = await import("/static/js/db.js");
        const original = IDBDatabase.prototype.transaction;
        IDBDatabase.prototype.transaction = function () { throw new Error("Falha simulada ao apagar o IndexedDB."); };
        let wipeError, readError;
        try { await db.wipe(); } catch (error) { wipeError = error.message; }
        finally { IDBDatabase.prototype.transaction = original; }
        try { await db.getMeta("cursor", 0); } catch (error) { readError = error.message; }
        const sync = await import("/static/js/sync.js");
        await sync.sync();
        return { wipeError, readError, status: sync.state.status, message: sync.state.message };
      });
      assert.equal(failedReset.wipeError, "Falha simulada ao apagar o IndexedDB.");
      assert.equal(failedReset.readError, failedReset.wipeError);
      assert.equal(failedReset.status, "error");
      assert.equal(failedReset.message, failedReset.wipeError);
      ok(true, "falha ao apagar bloqueia novos acessos até recarregar e devolve o erro, sem prender a sincronização nem marcar sucesso");
    }
    await drivePage.close();
  }
  ok(errors.length === 0, `SQLite e Drive: sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  servers.forEach((s) => s.kill());
  rmSync(dataDir, { recursive: true, force: true });
  rmSync(profile, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
