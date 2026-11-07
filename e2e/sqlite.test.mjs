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
const browser = await chromium.launchPersistentContext(profile, { executablePath: process.env.CHROMIUM_PATH || undefined });
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
