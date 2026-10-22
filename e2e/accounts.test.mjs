// Teste de ponta a ponta de contas e cartões: cadastrar, lançar parcelado no
// cartão, ver a fatura, o saldo das contas e o patrimônio.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node accounts.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18083;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-accounts-"));
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
const records = async (type) => (await (await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json())
  .changes.filter((c) => c.type === type && !c.record.deleted).map((c) => c.record);

const MESES = ["janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"];
const pad = (n) => String(n).padStart(2, "0");
const now = new Date();
// Cartão fecha dia 10 e vence dia 20: a compra de hoje cai na fatura deste mês
// até o dia 10; depois, na do mês seguinte.
const inv = new Date(now.getFullYear(), now.getMonth() + (now.getDate() <= 10 ? 0 : 1), 1);
const invLabel = `${MESES[inv.getMonth()]} de ${inv.getFullYear()}`;

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // Conta corrente e cartão.
  await page.goto(`${B}/contas`);
  await page.waitForSelector("form[data-form=account]", { timeout: 20000 });
  ok(await page.locator("[data-card]").first().isHidden(), "fechamento e vencimento só aparecem no cartão");
  await page.fill("input[name=name]", "Itaú");
  await page.fill("input[name=initial]", "1.000");
  await page.click("form[data-form=account] button[type=submit]");
  await page.waitForSelector(".account:has-text('Itaú')");
  await page.fill("input[name=name]", "Cartão Nubank");
  await page.selectOption("select[name=kind]", "cartao");
  await page.fill("input[name=closing_day]", "10");
  await page.fill("input[name=due_day]", "20");
  await page.click("form[data-form=account] button[type=submit]");
  await page.waitForSelector(".account:has-text('Cartão Nubank') .invoice");
  await page.evaluate(() => window.finanSync());
  const accounts = await records("account");
  ok(accounts.length === 2 && accounts.some((a) => a.kind === "cartao" && a.closingDay === 10 && a.dueDay === 20), `contas sincronizaram (${accounts.length})`);
  const cardId = accounts.find((a) => a.kind === "cartao").id;

  // Compra parcelada em 3x no cartão: 1.000,00 vira 333,34 + 333,33 + 333,33.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction] select[name=account_id]");
  await page.fill("input[name=description]", "Geladeira");
  await page.fill("input[name=amount]", "1.000");
  await page.selectOption("select[name=account_id]", { label: "Cartão Nubank" });
  await page.fill("input[name=installments]", "3");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Geladeira (1/3)')");
  // Uma entrada na conta corrente.
  await page.selectOption("select[name=kind]", "entrada");
  await page.fill("input[name=description]", "Freela");
  await page.fill("input[name=amount]", "500");
  await page.selectOption("select[name=account_id]", { label: "Itaú" });
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Freela')");
  await page.evaluate(() => window.finanSync());
  const parts = (await records("transaction")).filter((t) => t.description.startsWith("Geladeira"));
  ok(parts.length === 3 && parts.reduce((s, t) => s + t.amountCents, 0) === 100000 && parts.every((t) => t.accountId === cardId && t.id.startsWith("parc-")),
    `parcelas somam o total, no cartão (${parts.map((t) => t.amountCents)})`);
  const months = parts.map((t) => t.date.slice(0, 7)).sort();
  ok(new Set(months).size === 3, `uma parcela por mês (${months})`);

  // Fatura: a primeira parcela está na fatura certa, com o vencimento dia 20.
  await page.goto(`${B}/contas`);
  await page.waitForSelector(".account:has-text('Cartão Nubank') .invoice");
  const invoice = page.locator(`.invoice:has-text('${invLabel}')`);
  const invText = await invoice.first().textContent();
  ok(/R\$ 333,34/.test(invText) && invText.includes(`vence 20/${pad(inv.getMonth() + 1)}`), `fatura de ${invLabel} tem a 1ª parcela e vence dia 20: ${invText.replace(/\s+/g, " ")}`);
  ok(await page.locator(".invoice").count() === 2, "mostra a fatura atual e a próxima");
  ok((await page.textContent(".account:has-text('Itaú')")).includes("R$ 1.500,00"), "saldo da conta: inicial + entrada");
  ok((await page.textContent(".account:has-text('Cartão Nubank') > .row")).includes("R$ 333,34 em aberto"), "cartão: só a parcela de hoje está em aberto");

  // Patrimônio na Análise: 1.500 − 333,34.
  await page.goto(`${B}/analise`);
  await page.waitForSelector(".networth");
  ok((await page.textContent(".networth .chart-head")).includes("R$ 1.166,66"), "patrimônio = contas − cartão em aberto");
  ok(await page.locator(".networth .yearchart .col").count() === 12, "patrimônio dos últimos 12 meses");

  // Celular (320 px): sem rolagem para o lado.
  await page.setViewportSize({ width: 320, height: 700 });
  await page.goto(`${B}/contas`);
  await page.waitForSelector(".invoice");
  ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "tela de contas cabe em 320 px");

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
