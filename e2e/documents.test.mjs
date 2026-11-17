import assert from "node:assert/strict";
import { chromium, webkit } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";
import { xlsxFixture, imagePDF } from "./document-fixtures.mjs";

const directory = mkdtempSync(join(tmpdir(), "rich-documents-"));
const port = 18130 + Number(process.env.PORT_OFFSET || 0);
const origin = `http://127.0.0.1:${port}`;
const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich", ["-data", directory, "-no-browser", "-addr", `127.0.0.1:${port}`], { stdio: "ignore", env: { ...process.env, FINAN_AI_BASE_URL: "http://127.0.0.1:1" } });
const browser = await (process.env.BROWSER === "webkit" ? webkit : chromium).launch();
try {
  for (let attempt = 0; attempt < 100; attempt++) {
    try { if ((await fetch(origin + "/healthz")).ok) break; } catch {}
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  const errors = [];
  const remote = [];
  watchErrors(page, errors);
  page.on("request", (request) => { if (!request.url().startsWith(origin) && /^https?:/.test(request.url())) remote.push(request.url()); });
  await page.goto(origin + "/importar");
  await page.waitForSelector("input[name=statement]", { state: "attached", timeout: 20000 });
  await page.route("**/vendor/pdfjs/pdf.min.js", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 100));
    await route.continue();
  });
  const abortedPDF = await page.evaluate(async () => {
    const { pdfText } = await import("/static/js/pdftext.js");
    const controller = new AbortController();
    const reading = pdfText(new Uint8Array(), () => null, { signal: controller.signal });
    controller.abort();
    try { await reading; return false; } catch (error) { return error.name === "AbortError"; }
  });
  assert.equal(abortedPDF, true, "cancelamento durante a carga do leitor não inicia um PDF");
  await page.unroute("**/vendor/pdfjs/pdf.min.js");
  await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    await db.saveManyLocal(["a", "b"].map((suffix) => ({ type: "account", record: { id: "docs-" + suffix, updatedAt: Date.now(), name: "Conta de teste " + suffix, kind: "corrente", initialCents: 0 } })));
  });
  await page.reload();
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  await page.setInputFiles("input[name=statement]", { name: "extrato.xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer: xlsxFixture() });
  await page.waitForSelector(".import-table");
  assert.equal(await page.locator('.import-table input[data-field="description"]').first().inputValue(), "Mercado da planilha");
  assert.match(await page.locator('.import-table input[data-field="amountCents"]').first().inputValue(), /45,90/);
  await page.locator('[data-field="description"]').fill("Mercado corrigido");
  await page.locator('[data-field="description"]').dispatchEvent("change");
  await page.locator('[data-field="date"]').fill("2026-11-14");
  await page.locator('[data-field="date"]').dispatchEvent("change");
  await page.locator('[data-field="amountCents"]').fill("40,00");
  await page.locator('[data-field="amountCents"]').dispatchEvent("change");
  await page.locator('[data-field="amountCents"]').fill("90071992547409,93");
  await page.locator('[data-field="amountCents"]').dispatchEvent("change");
  assert.match(await page.locator('[data-field="amountCents"]').inputValue(), /40,00/, "valor sem precisão segura não altera a revisão");
  await page.locator('.import-table [data-field="selected"]').uncheck();
  await page.locator('[data-action="account"]').selectOption("docs-b");
  assert.equal(await page.locator('[data-field="description"]').inputValue(), "Mercado corrigido");
  assert.equal(await page.locator('[data-field="date"]').inputValue(), "2026-11-14");
  assert.match(await page.locator('[data-field="amountCents"]').inputValue(), /40,00/);
  assert.equal(await page.locator('.import-table [data-field="selected"]').isChecked(), false);
  assert.equal(await page.locator(".import-table tr.off td").evaluateAll((cells) => cells.every((cell) => getComputedStyle(cell).opacity === "1")), true, "desmarcar não reduz a legibilidade dos valores");
  const screenshots = join("..", ".artifacts", "document-import", process.env.BROWSER || "chromium");
  mkdirSync(screenshots, { recursive: true });
  for (const colorScheme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme, reducedMotion: "reduce" });
    for (const width of [390, 768, 1440, 1920]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.evaluate(() => document.fonts.ready);
      await page.evaluate(() => window.scrollTo(0, 0));
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${width}/${colorScheme}`);
      assert.equal(await page.locator(".import-table").evaluate((table) => {
        const frame = table.closest(".table-wrap");
        return frame.scrollWidth > frame.clientWidth + 1;
      }), width > 720 && width < 1000, `campos completos ou rolagem explícita: ${width}/${colorScheme}`);
      await page.screenshot({ path: join(screenshots, `${width}-${colorScheme}.png`), fullPage: true });
    }
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator('[data-action="cancel"]').click();
  const fixture = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 1600; canvas.height = 460;
    const context = canvas.getContext("2d");
    context.fillStyle = "white"; context.fillRect(0, 0, canvas.width, canvas.height);
    context.fillStyle = "black"; context.font = "46px Arial";
    context.fillText("EXTRATO DA CONTA CORRENTE", 60, 100);
    context.fillText("15/11/2026 MERCADO -45,90", 60, 220);
    const rgba = context.getImageData(0, 0, canvas.width, canvas.height).data;
    let rgb = "";
    for (let offset = 0; offset < rgba.length; offset += 4) rgb += String.fromCharCode(rgba[offset], rgba[offset + 1], rgba[offset + 2]);
    return { png: canvas.toDataURL("image/png").split(",")[1], jpeg: canvas.toDataURL("image/jpeg").split(",")[1], webp: canvas.toDataURL("image/webp"), rgb: btoa(rgb), width: canvas.width, height: canvas.height };
  });
  await page.setInputFiles("input[name=statement]", { name: "extrato.png", mimeType: "image/png", buffer: Buffer.from(fixture.png, "base64") });
  await page.waitForSelector(".import-table", { timeout: 90000 });
  assert.match(await page.locator("main").innerText(), /reconhecimento de imagem/);
  assert.equal(await page.locator('.import-table tbody input[data-field="selected"]:checked').count(), 0);
  assert.equal(await page.locator('.import-table input[data-field="date"]').first().inputValue(), "2026-11-15");
  assert.match(await page.locator('.import-table input[data-field="amountCents"]').first().inputValue(), /45,90/);
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  const before = await (await fetch(origin + "/api/sync", { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json();
  assert.equal(before.changes.filter((item) => item.type === "transaction").length, 0);
  await page.locator('[data-action="cancel"]').click();
  for (const [extension, mimeType, encoded] of [["jpg", "image/jpeg", fixture.jpeg], ...(fixture.webp.startsWith("data:image/webp;") ? [["webp", "image/webp", fixture.webp.split(",")[1]]] : [])]) {
    await page.setInputFiles("input[name=statement]", { name: `extrato.${extension}`, mimeType, buffer: Buffer.from(encoded, "base64") });
    await page.waitForSelector(".import-table", { timeout: 90000 });
    assert.equal(await page.locator('[data-field="date"]').inputValue(), "2026-11-15");
    assert.match(await page.locator('[data-field="amountCents"]').inputValue(), /45,90/);
    assert.equal(await page.locator('.import-table tbody input[data-field="selected"]:checked').count(), 0);
    await page.locator('[data-action="cancel"]').click();
  }
  await page.setInputFiles("input[name=statement]", { name: "digitalizado.pdf", mimeType: "application/pdf", buffer: imagePDF(Buffer.from(fixture.rgb, "base64"), fixture.width, fixture.height, true) });
  await page.waitForSelector(".import-table", { timeout: 90000 });
  assert.match(await page.locator("main").innerText(), /PDF com reconhecimento de imagem/);
  assert.equal(await page.locator('.import-table input[data-field="date"]').first().inputValue(), "2026-11-15");
  assert.equal(await page.locator('.import-table tbody input[data-field="selected"]:checked').count(), 0);
  await page.locator('.import-table [data-field="selected"]').check();
  const category = page.locator('[data-field="categoryId"]').first();
  if (!await category.inputValue()) {
    const value = await category.locator('option[value]:not([value=""])').first().getAttribute("value");
    await category.selectOption(value);
  }
  await page.locator('[data-action="import"]').click();
  await page.waitForURL(/\/lancamentos/);
  await page.goto(origin + "/?mes=2026-11");
  await page.waitForSelector(".dash-kpis");
  assert.match(await page.locator(".dash-kpis").innerText(), /45,90/, "o painel inclui o gasto confirmado");
  await page.goto(origin + "/importar");
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  await page.setInputFiles("input[name=statement]", { name: "mesma-compra.png", mimeType: "image/png", buffer: Buffer.from(fixture.png, "base64") });
  await page.waitForSelector(".import-table", { timeout: 90000 });
  assert.equal(await page.locator(".status-importado").count(), 1, "PDF e imagem da mesma compra não duplicam");
  await page.locator('[data-action="cancel"]').click();
  await page.setInputFiles("input[name=statement]", { name: "danificado.png", mimeType: "image/png", buffer: Buffer.from([0x89, 0x50, 0x4e, 0x47]) });
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  assert.equal(await page.locator(".import-table").count(), 0);
  const dimensions = await page.evaluate(async () => {
    const { imageDimensions, imageType } = await import("/static/js/document-reader.js");
    const bytes = new Uint8Array(24);
    bytes.set([0x89, 0x50, 0x4e, 0x47]);
    new DataView(bytes.buffer).setUint32(16, 100000);
    new DataView(bytes.buffer).setUint32(20, 100000);
    try { imageDimensions(bytes, imageType(bytes)); return false; } catch { return true; }
  });
  assert.equal(dimensions, true, "dimensões abusivas recusadas antes de decodificar");
  await page.setInputFiles("input[name=statement]", { name: "extrato.png", mimeType: "image/png", buffer: Buffer.from(fixture.png, "base64") });
  await page.getByRole("button", { name: "Cancelar leitura" }).click();
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  assert.equal(await page.locator(".import-table").count(), 0);
  assert.deepEqual(remote, [], "documentos não saem para serviços externos");
  assert.deepEqual(errors, [], "sem erros globais na leitura e no cancelamento");
  console.log("OK    XLSX, imagem, PDF digitalizado, OCR real local, revisão obrigatória, valor/data, privacidade, cancelamento e celular.");
} finally {
  await browser.close();
  server.kill();
  rmSync(directory, { recursive: true, force: true });
}
