import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { arch, cpus, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { gzipSync } from "node:zlib";
import { chromium } from "playwright";

const precache = readFileSync("../internal/web/static/sw.js", "utf8").split("const PRECACHE = [")[1].split("];", 1)[0];
const assets = [...new Set(precache.match(/\/static\/[^"\s]+/g))].map((path) => {
  const body = readFileSync(`../internal/web${path}`);
  return { path, bytes: body.length, gzipBytes: gzipSync(body).length };
});
console.log(JSON.stringify({
  arquivosPrecache: assets.length,
  bytesSemCompressao: assets.reduce((sum, asset) => sum + asset.bytes, 0),
  bytesGzipEstimados: assets.reduce((sum, asset) => sum + asset.gzipBytes, 0),
  maioresArquivos: assets.toSorted((first, second) => second.bytes - first.bytes).slice(0, 5),
  observacao: "Arquivos estáticos do precache, sem HTML, dados ou cabeçalhos. Gzip é estimativa local; a transferência real depende do servidor.",
}, null, 2));

assert.ok(assets.length > 0, "precache sem arquivos para medir");
const listener = createServer();
const port = await new Promise((resolve, reject) => {
  listener.once("error", reject);
  listener.listen(0, "127.0.0.1", () => resolve(listener.address().port));
});
await new Promise((resolve, reject) => listener.close((error) => error ? reject(error) : resolve()));
const dataDir = mkdtempSync(join(tmpdir(), "finan-performance-"));
const address = `http://127.0.0.1:${port}`;
const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${port}`], {
  stdio: ["ignore", "ignore", "pipe"],
});
let serverError = "";
server.on("error", (error) => { serverError = error.message; });
server.stderr.on("data", (chunk) => { serverError = (serverError + chunk).slice(-4000); });
let browser;
try {
  let healthy = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    assert.equal(server.exitCode, null, `o servidor encerrou: ${serverError}`);
    try { healthy = (await fetch(`${address}/healthz`)).ok; } catch {}
    if (healthy) break;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.ok(healthy, `o servidor não subiu: ${serverError}`);
  const sync = async (changes) => {
    const records = [];
    let cursor = 0;
    let more;
    do {
      const response = await fetch(`${address}/api/sync`, {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cursor, changes }),
      });
      assert.ok(response.ok, `sincronização respondeu ${response.status}`);
      const result = await response.json();
      assert.ok(!result.notices?.length, "o servidor recusou dados da medição");
      records.push(...result.changes);
      cursor = result.cursor;
      more = result.more;
      changes = [];
    } while (more);
    return records;
  };
  const category = (await sync([])).find((change) => change.type === "category" && change.record.name === "Mercado");
  assert.ok(category, "categoria Mercado não encontrada");
  const now = Date.now();
  const month = new Date().toISOString().slice(0, 7);
  const changes = Array.from({ length: 5000 }, (_, index) => ({
    type: "transaction",
    record: {
      id: `medicao-${index}`, updatedAt: now, kind: "saida", description: `Compra ${index}`,
      amountCents: 100 + index, categoryId: category.record.id, date: `${month}-${String(index % 28 + 1).padStart(2, "0")}`,
    },
  }));
  for (let offset = 0; offset < changes.length; offset += 2000) await sync(changes.slice(offset, offset + 2000));
  assert.equal((await sync([])).filter((change) => change.type === "transaction" && !change.record.deleted).length, 5000);
  browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
  const samples = [];
  for (let sample = 0; sample < 5; sample++) {
    const context = await browser.newContext({ viewport: { width: 1200, height: 900 }, serviceWorkers: "block" });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    for (const mode of ["primeira-abertura", "recarga"]) {
      await page.goto(address);
      await page.waitForFunction(async () => {
        const db = await import("/static/js/db.js");
        return (await db.loadLedger()).transactions.filter((record) => !record.deleted).length === 5000;
      });
      await page.waitForSelector(".dash-kpis");
      const result = await page.evaluate(async () => {
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
        return {
          painelMs: Math.round(performance.now()),
          recursosBytes: performance.getEntriesByType("resource").reduce((sum, entry) => sum + entry.transferSize, 0),
        };
      });
      samples.push({ sample: sample + 1, mode, ...result });
    }
    assert.deepEqual(errors, [], "erros de JavaScript durante a medição");
    await context.close();
  }
  console.log(JSON.stringify({
    ambiente: { sistema: platform(), arquitetura: arch(), processador: cpus()[0]?.model, navegador: browser.version() },
    cenario: "Computador local, 5 mil despesas no mês, sem limitação de CPU/rede. Service worker bloqueado; recarga mantém IndexedDB e cache HTTP. Não mede login/Drive nem celular real.",
    samples,
    medianas: ["primeira-abertura", "recarga"].map((mode) => ({
      mode, painelMs: samples.filter((sample) => sample.mode === mode).map((sample) => sample.painelMs).sort((first, second) => first - second)[2],
    })),
  }, null, 2));
} finally {
  await browser?.close();
  if (server.pid && server.exitCode === null) {
    const exited = new Promise((resolve) => server.once("exit", resolve));
    server.kill();
    await exited;
  }
  rmSync(dataDir, { recursive: true, force: true });
}
