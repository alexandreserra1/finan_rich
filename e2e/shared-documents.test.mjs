import assert from "node:assert/strict";
import { chromium, webkit } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync, mkdirSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";

assert.equal(JSON.parse(readFileSync(new URL("../mobile/capacitor.config.json", import.meta.url))).loggingBehavior, "none", "a ponte não registra documentos no console nativo");

const directory = mkdtempSync(join(tmpdir(), "rich-share-"));
const port = 18131 + Number(process.env.PORT_OFFSET || 0);
const origin = `http://127.0.0.1:${port}`;
const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", directory, "-no-browser", "-addr", `127.0.0.1:${port}`], { stdio: "ignore", env: { ...process.env, FINAN_UPDATE_CHECK: "0" } });
const browser = await (process.env.BROWSER === "webkit" ? webkit : chromium).launch();
try {
  for (let attempt = 0; attempt < 100; attempt++) {
    try { if ((await fetch(origin + "/healthz")).ok) break; } catch {}
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  const errors = [];
  watchErrors(page, errors);
  await page.route("**/importar", async (route) => {
    const response = await route.fetch();
    await route.fulfill({ response, body: (await response.text()).replace('<html lang="pt-BR"', '<html lang="pt-BR" data-app="mobile"') });
  });
  await page.addInitScript(() => {
    const listeners = new Map();
    let pending = null;
    window.__share = { reads: 0, discards: 0, hold: false, delayPeek: false, peekWaiting: false, releasePeek() {}, release() {}, send(id = "primeiro") { pending = { id, name: "Extrato recebido.csv", mimeType: "text/csv" }; listeners.get("pending")?.(); } };
    window.Capacitor = { getPlatform: () => "android", Plugins: { SharedDocuments: {
      addListener: async (event, callback) => { listeners.set(event, callback); return { remove() {} }; },
      peek: async () => {
        const value = pending || {};
        if (window.__share.delayPeek) {
          window.__share.delayPeek = false;
          window.__share.peekWaiting = true;
          await new Promise((resolve) => { window.__share.releasePeek = () => { window.__share.peekWaiting = false; resolve(); }; });
        }
        return value;
      },
      read: async ({ id }) => {
        window.__share.reads++;
        if (!pending || pending.id !== id) throw new Error("Documento indisponível.");
        const file = { ...pending, base64: btoa("Data;Descrição;Valor\n16/11/2026;Mercado compartilhado;-45,90\n") };
        pending = null;
        if (window.__share.hold) await new Promise((resolve) => { window.__share.release = resolve; });
        return file;
      },
      discard: async ({ id }) => { window.__share.discards++; if (pending?.id === id) pending = null; },
    } } };
  });
  await page.goto(origin + "/importar");
  await page.waitForSelector('input[name="statement"]', { state: "attached" });
  await page.evaluate(() => window.__share.send());
  await page.getByRole("button", { name: "Revisar documento recebido" }).waitFor();
  assert.equal(await page.evaluate(() => window.__share.reads), 0);
  const screenshots = join("..", ".artifacts", "android-share", process.env.BROWSER || "chromium");
  mkdirSync(screenshots, { recursive: true });
  for (const colorScheme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme, reducedMotion: "reduce" });
    for (const width of [390, 768, 1440, 1920]) {
      await page.setViewportSize({ width, height: 1000 });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
      await page.screenshot({ path: join(screenshots, `${width}-${colorScheme}.png`), fullPage: true });
    }
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => document.documentElement.classList.add("app-locked"));
  await page.evaluate(() => document.querySelector('[data-shared-review]').click());
  assert.equal(await page.evaluate(() => window.__share.reads), 0, "bloqueio impede leitura até em clique programático");
  await page.evaluate(() => document.documentElement.classList.remove("app-locked"));
  await page.getByRole("button", { name: "Revisar documento recebido" }).click();
  await page.waitForSelector(".import-table");
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Mercado compartilhado");
  assert.equal(await page.evaluate(() => window.__share.reads), 1);
  await page.locator('[data-field="description"]').fill("Correção que deve ficar");
  await page.locator('[data-field="description"]').dispatchEvent("change");
  await page.evaluate(() => window.__share.send("segundo"));
  await page.getByRole("button", { name: "Revisar documento recebido" }).waitFor();
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Correção que deve ficar", "receber não redesenha o formulário");
  page.once("dialog", (dialog) => dialog.dismiss());
  await page.getByRole("button", { name: "Revisar documento recebido" }).click();
  assert.equal(await page.evaluate(() => window.__share.reads), 1);
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Correção que deve ficar");
  await page.getByRole("button", { name: "Dispensar documento recebido" }).click();
  await page.getByRole("button", { name: "Revisar documento recebido" }).waitFor({ state: "detached" });
  assert.equal(await page.evaluate(() => window.__share.discards), 1);
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Correção que deve ficar");
  await page.evaluate(() => { window.__share.hold = true; window.__share.send("terceiro"); });
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Revisar documento recebido" }).click();
  await page.getByRole("button", { name: "Cancelar leitura" }).waitFor();
  assert.equal(await page.locator("[data-shared-document-notice]").count(), 1);
  await page.getByRole("button", { name: "Cancelar leitura" }).click();
  await page.evaluate(() => window.__share.release());
  await page.waitForFunction(() => !document.querySelector("[data-shared-review]"), null, { timeout: 5000 }).catch(async (error) => {
    console.log(await page.evaluate(() => ({ reads: window.__share.reads, discards: window.__share.discards, notices: document.querySelectorAll("[data-shared-document-notice]").length, text: document.getElementById("app").innerText })));
    throw error;
  });
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Correção que deve ficar", "cancelar não troca a prévia anterior");
  await page.evaluate(() => window.__share.send("quarto"));
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Revisar documento recebido" }).click();
  await page.getByRole("button", { name: "Cancelar leitura" }).waitFor();
  await page.evaluate(async () => { await (await import("/static/js/shared-documents.js")).clear(); window.__share.release(); });
  await page.waitForSelector('input[name="statement"]', { state: "attached" });
  assert.equal(await page.locator(".import-table").count(), 0, "resposta anterior ao descarte não reabre revisão");
  assert.match(await page.locator("#flash").innerText(), /Leitura interrompida/);
  assert.equal(await page.evaluate(async () => (await (await import("/static/js/db.js")).loadLedger()).transactions.length), 0);
  for (const [id, action] of [["descarte-durante-peek", "clear"], ["bloqueio-durante-peek", "lock"]]) {
    await page.evaluate((id) => { window.__share.hold = false; window.__share.send(id); }, id);
    await page.getByRole("button", { name: "Revisar documento recebido" }).waitFor();
    await page.evaluate(async (id) => {
      const shared = await import("/static/js/shared-documents.js");
      window.__readResult = "aguardando";
      window.__share.delayPeek = true;
      void shared.readFile(id).then(() => { window.__readResult = "entregue"; }, () => { window.__readResult = "recusado"; });
    }, id);
    await page.waitForFunction(() => window.__share.peekWaiting);
    await page.evaluate(async (action) => {
      if (action === "clear") await (await import("/static/js/shared-documents.js")).clear();
      else document.documentElement.classList.add("app-locked");
      window.__share.releasePeek();
    }, action);
    await page.waitForFunction(() => window.__readResult !== "aguardando");
    assert.equal(await page.evaluate(() => window.__readResult), "recusado", id);
    await page.evaluate(() => document.documentElement.classList.remove("app-locked"));
  }
  await page.evaluate(() => window.__share.send("antes-de-apagar"));
  await page.getByRole("button", { name: "Revisar documento recebido" }).click();
  await page.waitForSelector(".import-table");
  await page.locator('a[href="/configuracoes"]').first().evaluate((link) => link.click());
  await page.locator('[data-action="wipe"]').waitFor();
  page.once("dialog", (dialog) => dialog.accept());
  await page.locator('[data-action="wipe"]').click();
  await page.waitForFunction(() => document.getElementById("flash")?.textContent.includes("apagados"));
  await page.locator('a[href="/importar"]').first().evaluate((link) => link.click());
  await page.waitForSelector('input[name="statement"]', { state: "attached" });
  assert.equal(await page.locator(".import-table").count(), 0, "apagar dados também elimina a prévia do documento anterior");
  assert.deepEqual(errors, []);
  console.log("OK    compartilhamento: ação explícita, bloqueio, consumo único, prévia preservada e descarte.");
} finally {
  await browser.close();
  server.kill("SIGTERM");
  await new Promise((resolve) => server.exitCode == null ? server.once("exit", resolve) : resolve());
  rmSync(directory, { recursive: true, force: true });
}
