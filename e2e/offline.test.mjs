// Testes de ponta a ponta do modo offline e da sincronização entre aparelhos.
//
// Sobe o finan_rich (binário já compilado) com um banco temporário e usa um
// Chromium de verdade. Uso (na pasta e2e):
//   npm install && npx playwright install chromium
//   FINANRICH_BIN=../dist/finanrich npm test
// Para usar um Chromium já instalado: CHROMIUM_PATH=/caminho/do/chrome
import { chromium, devices } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18099;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-"));
let server = null;
let failures = 0;

const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function startServer() {
  server = spawn(BIN, ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], { stdio: "ignore" });
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(`${B}/healthz`)).ok) return; } catch {}
    await sleep(100);
  }
  throw new Error("o servidor não subiu");
}
async function stopServer() {
  if (!server) return;
  const exited = new Promise((r) => server.once("exit", r));
  server.kill();
  await exited;
  server = null;
}
async function serverRecords() {
  const out = [];
  for (let cursor = 0; ;) { // o servidor responde em páginas ("more")
    const res = await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cursor }) });
    const body = await res.json();
    out.push(...body.changes.map((c) => c.record));
    cursor = body.cursor;
    if (!body.more) return out;
  }
}
const syncStatus = (page) => page.locator("#sync-status").getAttribute("data-status");
const waitStatus = (page, status, timeout = 15000) =>
  page.waitForFunction((s) => document.getElementById("sync-status")?.dataset.status === s, status, { timeout });

async function addTransaction(page, description, amount) {
  await page.fill("input[name=description]", description);
  await page.fill("input[name=amount]", amount);
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector(`td:has-text('${description}')`);
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  await startServer();
  const pc = await browser.newContext({ viewport: { width: 1100, height: 900 } });
  const page = await pc.newPage();
  const errors = [];
  const logs = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => logs.push(`[${m.type()}] ${m.text()}`));
  page.on("requestfailed", (r) => logs.push(`[falhou] ${r.url()} ${r.failure()?.errorText}`));
  globalThis.dumpPage = async () => {
    console.log("--- diagnóstico ---");
    console.log("url:", page.url(), "| status:", await page.textContent("#sync-status").catch(() => "?"));
    console.log("tela:", (await page.innerText("body").catch(() => "")).slice(0, 500));
    console.log(logs.slice(-15).join("\n"));
  };

  // Online: baixa as categorias, lança, calcula o painel pelo WebAssembly.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]", { timeout: 20000 });
  await page.selectOption("select[name=kind]", "entrada");
  await addTransaction(page, "Salário", "4.500");
  await addTransaction(page, "Aluguel", "1.400");
  await waitStatus(page, "ok");
  await page.click("nav.tabs a[href='/']");
  await page.waitForSelector("text=Divisão do salário");
  ok((await page.textContent(".cards")).includes("R$ 3.100,00"), "painel calculado no aparelho (saldo R$ 3.100,00)");

  // Instala o app (service worker) e fecha o servidor.
  await page.evaluate(() => navigator.serviceWorker.ready);
  await page.reload();
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null);
  await page.evaluate(() => window.finanSync()); // garante que nada ficou pendente antes de desligar
  ok((await page.textContent("#sync-status")) === "Sincronizado", "antes de desligar, tudo foi enviado ao computador");
  await stopServer();

  // Offline: abre do cache, mostra os dados, aceita lançamentos novos.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("td:has-text('Aluguel')", { timeout: 20000 });
  ok(true, "sem o servidor, o app abre e mostra os dados guardados");
  await addTransaction(page, "Feira (offline)", "87,50");
  await waitStatus(page, "offline");
  await page.waitForFunction(() => /1 pendente/.test(document.getElementById("sync-status").textContent), null, { timeout: 5000 })
    .then(() => ok(true, "indicador mostra 1 alteração pendente"), async () => ok(false, "indicador mostra 1 alteração pendente: " + await page.textContent("#sync-status")));
  await page.goto(`${B}/`);
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("td:has-text('Feira (offline)')", { timeout: 20000 });
  ok(true, "recarregando sem servidor, nada se perde");

  // O servidor volta: sincroniza sozinho.
  await startServer();
  await page.evaluate(() => window.dispatchEvent(new Event("online")));
  await waitStatus(page, "ok", 20000);
  ok((await serverRecords()).some((r) => r.description === "Feira (offline)" && r.amountCents === 8750),
    "ao voltar a conexão, o servidor recebeu o lançamento feito offline");

  // Outro aparelho recebe; e o que ele lança chega ao primeiro.
  const phone = await browser.newContext({ ...devices["iPhone 13"] });
  const other = await phone.newPage();
  // Aparelho lento: as regras (WebAssembly) chegam depois da primeira sincronização.
  // Antes da correção, a tela ficava presa em "Baixando seus dados…".
  await phone.route("**/static/finance.wasm", async (route) => { await sleep(3000); await route.continue(); });
  await other.goto(`${B}/lancamentos`);
  await other.waitForSelector("td:has-text('Feira (offline)')", { timeout: 20000 });
  ok(true, "segundo aparelho recebeu o lançamento feito offline");
  await addTransaction(other, "Uber (do celular)", "23");
  await other.evaluate(() => window.finanSync());
  await page.evaluate(() => window.finanSync());
  await page.waitForSelector("td:has-text('Uber (do celular)')", { timeout: 15000 });
  ok(true, "o primeiro aparelho recebeu o lançamento do segundo sem recarregar");

  // Apagar também sincroniza.
  page.once("dialog", (d) => d.accept());
  await page.click("tr:has-text('Uber (do celular)') [data-delete]");
  await page.evaluate(() => window.finanSync());
  await other.evaluate(() => window.finanSync());
  await other.waitForSelector("td:has-text('Uber (do celular)')", { state: "detached", timeout: 15000 });
  ok(true, "apagar num aparelho apaga no outro");

  // Fila grande (ex.: importação de anos de extrato): vai em lotes e chega inteira.
  const before = (await serverRecords()).length;
  await page.evaluate(async () => {
    const db = await import("/static/js/db.js"), fin = await import("/static/js/finance.js");
    const cat = (await db.loadLedger()).categories[0].id;
    await db.saveManyLocal(Array.from({ length: 4500 }, (_, i) => ({ type: "transaction",
      record: fin.validate("transaction", { id: "lote-" + i, updatedAt: Date.now(), kind: "saida", description: "Lote " + i, amountCents: 100 + i, categoryId: cat, date: "2026-09-15" }) })));
    await window.finanSync();
  });
  ok((await serverRecords()).length - before === 4500 && (await page.evaluate(async () => (await (await import("/static/js/db.js")).pending()).length)) === 0,
    "4.500 lançamentos pendentes chegam inteiros ao computador (em lotes)");

  ok(await syncStatus(page) === "ok", "tudo sincronizado no fim");
  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
  await globalThis.dumpPage?.();
} finally {
  await browser.close();
  await stopServer();
  rmSync(dataDir, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
