// Teste de ponta a ponta da importação de extratos no navegador.
//
// Envia uma fatura em CSV e um extrato OFX (os mesmos dos testes em Go),
// revisa as sugestões, importa e confere que reimportar não duplica.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node import.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const TESTDATA = "../internal/importer/testdata";
const PORT = 18087;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-import-"));
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const server = spawn(BIN, ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], { stdio: "ignore" });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${B}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}
const serverTxs = async () => (await (await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json())
  .changes.filter((c) => c.type === "transaction" && !c.record.deleted).map((c) => c.record);

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  const row = (desc) => page.locator(`.import-table tr:has(input[value="${desc}"])`);

  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]", { timeout: 20000 });
  await page.click("a[href='/importar']");
  await page.waitForSelector("input[name=statement]", { state: "attached" });

  // Fatura do cartão em CSV: positivo é gasto.
  await page.setInputFiles("input[name=statement]", `${TESTDATA}/nubank_cartao.csv`);
  await page.waitForSelector(".import-table");
  ok(await page.isChecked("[data-action=invert]"), "fatura em CSV é reconhecida (valores positivos = gastos)");
  ok((await row("Ifood *Ifood").locator("td.num").textContent()).includes("−R$ 52,30"), "iFood entra como gasto");
  ok(await row("Ifood *Ifood").locator("select option:checked").textContent() === "Restaurantes e delivery · Desejos", "iFood sugerido em Restaurantes e delivery");
  ok(await row("Drogasil 123").locator("select option:checked").textContent() === "Saúde · Necessidades", "Drogasil sugerido em Saúde");
  ok(!(await row("Pagamento recebido").locator("input[type=checkbox]").isChecked()), "pagamento da fatura vem desmarcado");
  ok(await row("Pagamento recebido").locator(".status-fatura").count() === 1, "pagamento da fatura é explicado");

  // "Supermercado Boa Compra" vira Mercado pela palavra-chave; "Amazon" em Compras.
  // Muda a categoria de um e importa.
  await row("Supermercado Boa Compra").locator("select").selectOption({ label: "Mercado · Necessidades" });
  const missing = await page.locator("select.needs").count();
  ok(missing === 0, `nenhum marcado sem categoria (${missing})`);
  await page.click("[data-action=import]");
  await page.waitForSelector(".flash.ok:has-text('4 lançamentos importados')", { timeout: 10000 });
  ok(page.url().endsWith("/lancamentos?mes=2026-10"), "depois de importar, abre os lançamentos do mês");
  await page.evaluate(() => window.finanSync());
  const txs = await serverTxs();
  ok(txs.length === 4 && txs.every((t) => t.id.startsWith("imp-")) && txs.some((t) => t.description === "Ifood *Ifood" && t.kind === "saida" && t.amountCents === 5230),
    `os 4 lançamentos chegaram ao computador (${txs.length})`);

  // Reimportar o mesmo arquivo: nada novo.
  await page.goto(`${B}/importar`);
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  await page.setInputFiles("input[name=statement]", `${TESTDATA}/nubank_cartao.csv`);
  await page.waitForSelector(".import-table");
  ok(await page.locator(".status-importado").count() === 4 && await page.isDisabled("[data-action=import]"), "reimportar o mesmo arquivo não duplica");

  // OFX com acentos em Windows-1252, salário e um lançamento que pede categoria.
  await page.click("[data-action=cancel]");
  await page.setInputFiles("input[name=statement]", `${TESTDATA}/itau.ofx`);
  await page.waitForSelector(".import-table");
  ok(await page.locator('input[value="PÃO DE AÇÚCAR 123"]').count() === 1, "OFX em Windows-1252 com acentos certos");
  ok(await row("SALARIO EMPRESA X").locator("select option:checked").textContent() === "Salário", "salário reconhecido");
  ok(await row("ALUGUEL - TED JOAO IMOVEIS").locator("select option:checked").textContent() === "Moradia · Necessidades", "aluguel sugerido em Moradia");
  ok(await page.locator("select.needs").count() === 0, "tudo marcado tem categoria sugerida");
  await page.click("[data-action=import]");
  await page.waitForSelector(".flash.ok:has-text('importados')", { timeout: 10000 });
  await page.evaluate(() => window.finanSync());
  ok((await serverTxs()).some((t) => t.description === "ALUGUEL - TED JOAO IMOVEIS" && t.amountCents === 140000), "extrato OFX importado");

  // Aprende com o histórico: o próximo "SUPERMERCADO BOA COMPRA" vem como Mercado.
  await page.goto(`${B}/importar`);
  await page.waitForSelector("input[name=statement]", { state: "attached" });
  await page.setInputFiles("input[name=statement]", { name: "novo.csv", mimeType: "text/csv", buffer: Buffer.from("Data,Valor,Descrição\n20/10/2026,-99.90,Compra no débito - SUPERMERCADO BOA COMPRA\n") });
  await page.waitForSelector(".import-table");
  const learned = row("Compra no débito - SUPERMERCADO BOA COMPRA");
  ok(await learned.locator("select option:checked").textContent() === "Mercado · Necessidades" && (await learned.textContent()).includes("pelo seu histórico"),
    "aprende com a correção anterior (histórico)");

  // Desconhecido: pede a categoria e oferece a IA; sem IA configurada, explica.
  await page.click("[data-action=cancel]");
  await page.setInputFiles("input[name=statement]", { name: "x.csv", mimeType: "text/csv", buffer: Buffer.from("Data,Valor,Descrição\n21/10/2026,-10.00,XPTO SERVICOS 123\n") });
  await page.waitForSelector(".import-table");
  ok(await page.locator("select.needs").count() === 1 && await page.isVisible("[data-action=ai]"), "lançamento sem categoria pede escolha e oferece a IA");
  await page.click("[data-action=ai]");
  await page.waitForSelector("p.neg:has-text('❌')", { timeout: 30000 });
  ok(true, "sem IA disponível, mostra o motivo");
  await page.click("[data-action=import]");
  ok(await page.isVisible(".flash.err:has-text('escolha a categoria')"), "não importa sem categoria");

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
