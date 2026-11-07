// Teste da troca do IndexedDB pelo SQLite na web (js/db.js): os dados de antes
// aparecem iguais (inclusive bytes e a chave de criptografia), a fila de envio
// chega ao computador, uma falha na cópia não perde nada (fica no IndexedDB e
// copia na próxima abertura) e só uma aba usa o banco por vez ("Usar aqui").
// Usa perfis de navegador persistentes: na janela temporária do Playwright o
// WebKit não tem OPFS, como a navegação anônima do Safari.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node migrate.test.mjs
import { chromium as chromiumEngine, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";

const OFF = Number(process.env.PORT_OFFSET || 0); // PORT_OFFSET=100: roda ao lado de outra cópia do repositório
const B = `http://127.0.0.1:${18103 + OFF}`;
const dirs = [];
const tmp = (name) => { const d = mkdtempSync(join(tmpdir(), name)); dirs.push(d); return d; };
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", tmp("finan-e2e-migrate-"), "-no-browser", "-addr", B.slice(7)], { stdio: "ignore", env: { ...process.env, FINAN_UPDATE_CHECK: "0" } });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${B}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}
const serverChanges = async () => (await (await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json()).changes;
const category = (await serverChanges()).find((c) => c.type === "category").record.id;
const tx = (id, description) => ({ id, description, updatedAt: Date.now(), kind: "saida", amountCents: 1234, categoryId: category, date: new Date().toISOString().slice(0, 10) });

// Começa sem banco SQLite: no WebKit do Playwright (Mac) o OPFS é por endereço, não
// por perfil, e sobraria o de outro cenário ou de outra rodada.
const clearOPFS = (page) => page.evaluate(async () => {
  const root = await navigator.storage.getDirectory();
  for await (const [name] of root.entries()) await root.removeEntry(name, { recursive: true });
});

// Dados como a versão anterior deixava: tudo no IndexedDB, com a fila de envio.
// bad: um valor que o JSON não guarda (o IndexedDB guarda), para a cópia falhar.
const oldData = (page, bad) => page.evaluate(async ({ record, bad }) => {
  const idb = await import("/static/js/db-idb.js");
  await idb.saveManyLocal([{ type: "transaction", record }]);
  await idb.setMeta("bytes", new Uint8Array([1, 2, 3]));
  await idb.setMeta("buffer", { data: new Uint8Array([4, 5]).buffer }); // como a chave da IA cifrada
  await idb.setMeta("cloud-ai-key", await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]));
  if (bad) await idb.setMeta("ruim", 10n);
}, { record: tx(`antigo-${bad ? 2 : 1}`, bad ? "Antigo com valor ruim" : "Migrado do IndexedDB"), bad });

// Onde cada coisa está: no banco em uso (db.js) e no IndexedDB (o backup).
const state = (page) => page.evaluate(async () => {
  const db = await import("/static/js/db.js");
  const idb = await import("/static/js/db-idb.js");
  const bytes = await db.getMeta("bytes", null);
  const buffer = (await db.getMeta("buffer", null))?.data;
  const key = await db.getMeta("cloud-ai-key", null);
  let usable = false;
  try { usable = !!(await crypto.subtle.encrypt({ name: "AES-GCM", iv: new Uint8Array(12) }, key, new Uint8Array([7]))); } catch {}
  return {
    tx: (await db.loadLedger()).transactions.map((t) => t.description).sort(),
    backup: (await idb.loadLedger()).transactions.map((t) => t.description).sort(),
    bytes: bytes instanceof Uint8Array ? [...bytes].join() : String(bytes), key: usable,
    buffer: buffer instanceof ArrayBuffer ? [...new Uint8Array(buffer)].join() : String(buffer),
    moved: (await idb.getMeta("idb-moved", 0)) > 0, bad: typeof (await db.getMeta("ruim", null)),
    backupMeta: (await idb.dump()).meta.map(([key]) => key).join(),
  };
});
const save = (page, record) => page.evaluate(async (record) => (await import("/static/js/db.js")).saveLocal("transaction", record), record);

// Uma aba ainda na versão anterior: abre o IndexedDB na versão 1, como ela fazia, e
// fica com a conexão aberta (a versão anterior nunca fecha).
const oldTab = (page) => page.evaluate(() => new Promise((resolve, reject) => {
  const req = indexedDB.open("finan_rich", 1);
  req.onupgradeneeded = () => {
    for (const name of ["records", "outbox"]) req.result.createObjectStore(name, { keyPath: "key" });
    req.result.createObjectStore("meta");
  };
  req.onsuccess = () => { window.oldDb = req.result; resolve(); };
  req.onerror = () => reject(req.error);
}));
const oldSave = (page, record) => page.evaluate((record) => new Promise((resolve, reject) => {
  const t = window.oldDb.transaction(["records", "outbox"], "readwrite");
  const row = { key: `transaction:${record.id}`, type: "transaction", record };
  t.objectStore("records").put(row);
  t.objectStore("outbox").put(row);
  t.oncomplete = resolve;
  t.onerror = t.onabort = () => reject(t.error);
}), record);

const browserOpts = { executablePath: process.env.CHROMIUM_PATH || undefined };
const errors = [];
let ctx;
try {
  // 1. Migração: tudo aparece, a chave de criptografia continua usável e a fila sobe.
  ctx = await chromium.launchPersistentContext(tmp("finan-e2e-migrate-perfil-"), browserOpts);
  const page = ctx.pages()[0] || await ctx.newPage();
  watchErrors(page, errors);
  await page.goto(`${B}/offline`);
  await clearOPFS(page);
  await oldData(page, false);
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("text=Migrado do IndexedDB", { timeout: 20000 });
  let s = await state(page);
  ok(s.backupMeta === "cloud-ai-key,idb-moved", `depois da cópia, o meta do backup fica só com a marca e a chave de criptografia (tokens e senha saem): ${s.backupMeta}`);
  ok(s.moved && s.bytes === "1,2,3" && s.buffer === "4,5" && s.key, `dados da versão anterior aparecem iguais, com bytes e a chave de criptografia usável (${JSON.stringify(s)})`);
  await save(page, tx("novo-sqlite", "Gravado no SQLite"));
  s = await state(page);
  ok(s.tx.includes("Gravado no SQLite") && !s.backup.includes("Gravado no SQLite") && s.backup.includes("Migrado do IndexedDB"),
    "depois da cópia, o novo vai para o SQLite e o IndexedDB fica intacto como backup");
  let synced = false;
  for (let i = 0; i < 100 && !synced; i++) {
    synced = (await serverChanges()).some((c) => c.record.description === "Migrado do IndexedDB");
    if (!synced) await new Promise((r) => setTimeout(r, 200));
  }
  ok(synced, "a fila de envio copiada do IndexedDB chega ao computador");

  // 2. Uma aba de cada vez: a nova assume, a antiga avisa e "Usar aqui" volta o banco para ela.
  const second = await ctx.newPage();
  watchErrors(second, errors);
  await second.goto(`${B}/lancamentos`);
  await second.waitForSelector("text=Gravado no SQLite", { timeout: 20000 });
  await page.waitForSelector("#elsewhere [data-use-here]", { timeout: 10000 });
  ok(true, "abrir a Rich em outra aba: a antiga avisa que está aberta em outra aba");
  await save(second, tx("segunda-aba", "Feito na segunda aba"));
  await page.click("#elsewhere [data-use-here]");
  await page.waitForSelector("text=Feito na segunda aba", { timeout: 20000 });
  await second.waitForSelector("#elsewhere [data-use-here]", { timeout: 10000 });
  ok(true, "Usar aqui: a aba antiga volta a usar o banco, vê o que a outra gravou, e a outra avisa");
  // Apagar os dados do aparelho (como "Esquecer este aparelho") e gravar de novo.
  await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    await db.wipe();
    await db.setMeta("pluggy", null);
  });
  await save(page, tx("depois-apagar", "Depois de apagar"));
  await page.reload();
  await page.waitForSelector("text=Depois de apagar", { timeout: 20000 });
  s = await state(page);
  ok(s.moved && s.tx.includes("Depois de apagar") && !s.backup.includes("Depois de apagar"),
    `apagar os dados do aparelho e gravar de novo: continua no SQLite, sem voltar para o IndexedDB (${JSON.stringify(s)})`);
  await ctx.close();

  // 3. Falha na cópia: continua no IndexedDB sem perder nada e copia na próxima abertura.
  ctx = await chromium.launchPersistentContext(tmp("finan-e2e-migrate-perfil-"), browserOpts);
  const other = ctx.pages()[0] || await ctx.newPage();
  watchErrors(other, errors);
  await other.goto(`${B}/offline`);
  await clearOPFS(other);
  await oldData(other, true);
  await other.goto(`${B}/lancamentos`);
  await other.waitForSelector("text=Antigo com valor ruim", { timeout: 20000 });
  s = await state(other);
  ok(!s.moved && s.bad === "bigint", `cópia que falha não marca nada e o app segue no IndexedDB (${JSON.stringify(s)})`);
  await save(other, tx("no-idb", "Gravado no IndexedDB"));
  await other.evaluate(async () => (await import("/static/js/db.js")).setMeta("ruim", null));
  await other.reload();
  await other.waitForSelector("text=Gravado no IndexedDB", { timeout: 20000 });
  s = await state(other);
  ok(s.moved && s.tx.includes("Antigo com valor ruim") && s.tx.includes("Gravado no IndexedDB") && s.key,
    `na abertura seguinte a cópia dá certo e leva também o que foi gravado no IndexedDB (${JSON.stringify(s)})`);

  // Depois da cópia, se o SQLite não abrir, avisa em vez de usar o IndexedDB desatualizado.
  const broken = await ctx.newPage();
  await broken.addInitScript(() => { window.Worker = class { constructor() { throw new Error("sem Worker"); } }; });
  await broken.goto(`${B}/offline`);
  const refused = await broken.evaluate(() => import("/static/js/db.js").then((db) => db.getMeta("cursor", null)).then(() => "abriu", (e) => e.message));
  ok(refused.includes("não consegui abrir o banco"), `se o SQLite não abrir depois da cópia, avisa em vez de usar o backup (${refused})`);
  await ctx.close();

  // 4. Aba ainda na versão anterior durante a atualização: a nova espera ela sair antes de
  // copiar, e depois a versão anterior não abre mais o banco (nada fica para trás).
  ctx = await chromium.launchPersistentContext(tmp("finan-e2e-migrate-perfil-"), browserOpts);
  const old = ctx.pages()[0] || await ctx.newPage();
  await old.goto(`${B}/offline`);
  await clearOPFS(old);
  await oldTab(old);
  await oldSave(old, tx("antes-troca", "Antes da troca"));
  const updated = await ctx.newPage();
  watchErrors(updated, errors);
  await updated.goto(`${B}/lancamentos`);
  await updated.waitForSelector("#db-wait .modal-panel", { timeout: 20000 });
  ok(true, "com uma aba na versão anterior aberta, a aba nova espera e pede para recarregar ou fechar a outra");
  await oldSave(old, tx("aba-antiga", "Gravado na aba antiga"));
  await old.reload(); // fecha a conexão da versão anterior
  await updated.waitForSelector("text=Gravado na aba antiga", { timeout: 20000 });
  s = await state(updated);
  ok(s.moved && s.tx.includes("Antes da troca") && s.tx.includes("Gravado na aba antiga") && !(await updated.$("#db-wait")),
    `a cópia leva também o que a aba antiga gravou enquanto a nova esperava (${JSON.stringify(s)})`);
  const reopened = await old.evaluate(() => new Promise((resolve) => {
    const req = indexedDB.open("finan_rich", 1);
    req.onsuccess = () => resolve("abriu");
    req.onerror = () => resolve(req.error.name);
  }));
  ok(reopened === "VersionError", `depois da cópia, a versão anterior não abre mais o banco antigo (${reopened})`);

  // 5. O Worker da aba anterior ainda segura os arquivos do banco (recarregar, ou fechar e
  // abrir logo): a aba nova tenta de novo, sem desistir e sem apagar o banco.
  await updated.close();
  await old.evaluate(() => new Promise((resolve, reject) => {
    window.holder = new Worker("/static/js/sqlite-worker.js", { type: "module" });
    window.holder.onmessage = ({ data }) => (data.error ? reject(new Error(data.error)) : resolve());
    window.holder.postMessage({ id: 1, ops: [] });
  }));
  const next = await ctx.newPage();
  watchErrors(next, errors);
  await next.goto(`${B}/lancamentos`);
  await new Promise((r) => setTimeout(r, 2000)); // a aba nova já tentou abrir e achou o banco preso
  await old.evaluate(() => new Promise((resolve) => {
    window.holder.onmessage = resolve;
    window.holder.postMessage({ id: 2, close: true });
  }));
  await next.waitForSelector("text=Gravado na aba antiga", { timeout: 20000 });
  ok(true, "banco ainda preso pelo Worker da aba anterior: a aba nova espera ele soltar e abre com tudo");

  // 6. Aba suspensa (no celular, em segundo plano): não responde ao pedido do banco e
  // continua com a trava. Aqui, a aba fica ocupada (o JavaScript dela não roda), o mesmo
  // efeito nos dois motores. A nova espera com o aviso e abre quando a suspensa volta e
  // solta o banco, ou quando ela é fechada (o celular descarta a aba).
  const suspend = (p, ms) => { p.evaluate((ms) => { const end = Date.now() + ms; while (Date.now() < end); }, ms).catch(() => {}); };
  suspend(next, 8000);
  const third = await ctx.newPage();
  watchErrors(third, errors);
  await third.goto(`${B}/lancamentos`);
  await third.waitForSelector("#db-wait .modal-panel", { timeout: 20000 });
  await third.waitForSelector("text=Gravado na aba antiga", { timeout: 20000 });
  await next.waitForSelector("#elsewhere [data-use-here]", { timeout: 10000 });
  ok(!(await third.$("#db-wait .modal-panel")), "aba suspensa: a nova espera com o aviso e abre quando a suspensa volta e solta o banco");
  suspend(third, 20000); // se fechar demorar (o WebKit espera a aba ocupada responder), ela volta e solta
  const fourth = await ctx.newPage();
  watchErrors(fourth, errors);
  await fourth.goto(`${B}/lancamentos`);
  await fourth.waitForSelector("#db-wait .modal-panel", { timeout: 20000 });
  await third.close(); // o WebKit só fecha a aba quando o laço acaba (até 16 s): a nova continua esperando
  await fourth.waitForSelector("text=Gravado na aba antiga", { timeout: 45000 });
  ok(!(await fourth.$("#db-wait .modal-panel")), "aba suspensa e fechada (o celular descartou a aba): a nova abre com tudo");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, `${err.message} ${JSON.stringify(errors)}`);
} finally {
  await ctx?.close().catch(() => {});
  server.kill();
  dirs.forEach((d) => rmSync(d, { recursive: true, force: true }));
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
