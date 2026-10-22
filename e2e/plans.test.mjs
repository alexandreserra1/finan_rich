// Teste de ponta a ponta de lançamentos recorrentes, metas e avisos do painel.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node plans.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18082;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-plans-"));
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

const pad = (n) => String(n).padStart(2, "0");
const now = new Date();
const ym = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
const prev = new Date(now.getFullYear(), now.getMonth() - 2, 1);

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // Recorrente começando há 2 meses, dia 1: gera 3 lançamentos (2 atrás, 1 atrás, este).
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]", { timeout: 20000 });
  await page.selectOption("select[name=kind]", "entrada");
  await page.fill("input[name=description]", "Salário");
  await page.fill("input[name=amount]", "5.000");
  await page.fill("input[name=date]", `${ym(prev)}-01`);
  await page.check("input[name=repeat]");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("#recorrentes:has-text('Salário')");
  await page.evaluate(() => window.finanSync());
  const generated = (await records("transaction")).filter((t) => t.id.startsWith("rec-"));
  ok(generated.length === 3 && generated.every((t) => t.amountCents === 500000 && t.date.endsWith("-01")), `recorrente gerou os 3 meses (${generated.length})`);

  // Recarregar não duplica; apagar um gerado não volta.
  await page.reload();
  await page.waitForSelector("form[data-form=transaction]");
  await page.evaluate(() => window.finanSync());
  ok((await records("transaction")).filter((t) => t.id.startsWith("rec-")).length === 3, "abrir de novo não duplica os recorrentes");
  await page.click("tr:has-text('Salário') [data-delete]");
  await page.waitForSelector("tr:has-text('Salário')", { state: "detached" });
  await page.reload();
  await page.waitForSelector("form[data-form=transaction]");
  ok(await page.locator("tr:has-text('Salário')").count() === 0, "lançamento recorrente apagado não volta");

  // Pausar e apagar a recorrência.
  await page.click("[data-pause]");
  await page.waitForSelector("#recorrentes li.paused");
  ok(true, "recorrente pausado");

  // Dívida vencendo em 2 dias → aviso no painel.
  const due = new Date(now.getTime() + 2 * 864e5);
  if (due.getMonth() === now.getMonth()) {
    await page.goto(`${B}/dividas`);
    await page.waitForSelector("form[data-form=debt]");
    await page.fill("input[name=name]", "Cartão");
    await page.fill("input[name=balance]", "2.000");
    await page.fill("input[name=rate]", "10");
    await page.fill("input[name=min_payment]", "300");
    await page.fill("input[name=due_day]", String(due.getDate()));
    await page.click("form[data-form=debt] button[type=submit]");
    await page.waitForSelector("td:has-text('Cartão')");
    await page.goto(`${B}/`);
    await page.waitForSelector(".alert.warn:has-text('Cartão vence em 2 dias')", { timeout: 10000 });
    ok(true, "aviso de vencimento no painel");
  }

  // Meta com categoria de aportes.
  await page.goto(`${B}/metas`);
  await page.waitForSelector("form[data-form=goal]");
  await page.fill("input[name=name]", "Reserva");
  await page.fill("input[name=target]", "6.000");
  await page.fill("input[name=saved]", "1.000");
  await page.fill("input[name=deadline]", `${now.getFullYear() + 1}-${pad(now.getMonth() + 1)}`);
  await page.selectOption("select[name=category]", { label: "Reserva de emergência · Investimentos" });
  await page.click("form[data-form=goal] button[type=submit]");
  await page.waitForSelector(".goal:has-text('Reserva')");
  // Um aporte lançado na categoria conta para a meta.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]");
  await page.fill("input[name=description]", "Poupança");
  await page.fill("input[name=amount]", "500");
  await page.selectOption("select[name=category_id]", { label: "Reserva de emergência · Investimentos" });
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Poupança')");
  await page.goto(`${B}/`);
  await page.waitForSelector(".goal:has-text('Reserva')");
  ok((await page.textContent(".goal")).includes("R$ 1.500,00 de R$ 6.000,00 (25%)"), "meta no painel soma o já guardado e o aporte");
  await page.evaluate(() => window.finanSync());
  ok((await records("goal")).length === 1, "meta sincronizou");

  // Análise: ritmo, quanto dá para gastar por dia e onde foi o dinheiro.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]");
  await page.fill("input[name=description]", "iFood pizza");
  await page.fill("input[name=amount]", "45");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('iFood pizza')");
  await page.goto(`${B}/`);
  await page.click("a[href='/analise']");
  await page.waitForSelector(".pace-chart svg");
  const analysis = await page.textContent("main");
  ok(/Pode gastar por dia/.test(analysis) && /Projeção do mês/.test(analysis), "análise mostra quanto dá para gastar por dia e a projeção");
  ok(await page.locator(".hbars li:has-text('iFood')").count() === 1 && !/Poupança/.test(await page.textContent(".grid-2")), "gastei onde: iFood aparece; investimento não conta como gasto");
  await page.hover(".pace-chart svg", { position: { x: 20, y: 60 } });
  ok(/^Dia \d+: este mês R\$/.test(await page.textContent(".chart-tip")), "passar o dedo no gráfico mostra o dia");

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
