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
// Abre a janela de novo lançamento (se ainda não estiver aberta).
const openTx = async (page) => {
  if (await page.locator("#tx-modal .modal-panel").isVisible()) return;
  await page.locator('[data-action="new-tx"]:visible').first().click();
  await page.waitForSelector("#tx-modal .modal-panel");
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
  await page.waitForSelector(".tx-card", { timeout: 20000 });
  await openTx(page); await page.click("[data-kind=entrada]");
  await openTx(page);
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
  await page.waitForSelector(".tx-card");
  await page.evaluate(() => window.finanSync());
  ok((await records("transaction")).filter((t) => t.id.startsWith("rec-")).length === 3, "abrir de novo não duplica os recorrentes");
  await page.click("tr:has-text('Salário') [data-delete]");
  await page.waitForSelector("tr:has-text('Salário')", { state: "detached" });
  await page.reload();
  await page.waitForSelector(".tx-card");
  ok(await page.locator("tr:has-text('Salário')").count() === 0, "lançamento recorrente apagado não volta");

  // Pausar e apagar a recorrência.
  await page.click("[data-pause]");
  await page.waitForSelector("#recorrentes li.paused");
  ok(true, "recorrente pausado");

  // Dívida vencendo em 2 dias → aviso no painel.
  const due = new Date(now.getTime() + 2 * 864e5);
  if (due.getMonth() === now.getMonth()) {
    await page.goto(`${B}/dividas`);
    await page.click('[data-action="new-debt"] >> nth=0'); // a janela de nova dívida
    await page.waitForSelector("#debt-modal .modal-panel");
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

  // Apagar dívida: some da tela, não volta ao recarregar e o apagamento chega ao computador.
  await page.goto(`${B}/dividas`);
  await page.click('[data-action="new-debt"] >> nth=0'); // a janela de nova dívida
  await page.waitForSelector("#debt-modal .modal-panel");
  await page.fill("input[name=name]", "Empréstimo");
  await page.fill("input[name=balance]", "1.000");
  await page.fill("input[name=rate]", "5");
  await page.fill("input[name=min_payment]", "100");
  await page.click("form[data-form=debt] button[type=submit]");
  await page.waitForSelector("td:has-text('Empréstimo')");
  await page.evaluate(() => window.finanSync());
  ok((await records("debt")).some((d) => d.name === "Empréstimo"), "dívida nova sincronizou");
  // Plano: uma frase com o que pagar primeiro e em quantos meses; pagar a mais encurta.
  // "em 8 meses", "em 1 ano e 6 meses", "em 2 anos": em meses.
  const toMonths = (text) => { const m = text.replace(/\s+/g, " ").match(/em (?:(\d+) anos?)?(?: e )?(?:(\d+) m[eê]s(?:es)?)? \(/); return Number(m[1] || 0) * 12 + Number(m[2] || 0); };
  const months = async () => toMonths(await page.textContent(".plan-lead"));
  const before = await months();
  ok((await page.textContent(".plan-lead")).includes("Pagando") && await page.locator(".debt-chart canvas").count() === 1,
    `plano das dívidas com frase e gráfico (${before} meses)`);
  await page.fill("form[data-form=extra] input[name=extra]", "200"); // recalcula sozinho, sem botão
  await page.waitForURL(/extra=20000/);
  await page.waitForFunction((b) => {
    const m = document.querySelector(".plan-lead")?.textContent.replace(/\s+/g, " ").match(/em (?:(\d+) anos?)?(?: e )?(?:(\d+) m[eê]s(?:es)?)? \(/);
    return m && Number(m[1] || 0) * 12 + Number(m[2] || 0) < b;
  }, before);
  ok(true, `pagando R$ 200 a mais o plano encurta (${before} → ${await months()} meses)`);
  await page.click("tr:has-text('Empréstimo') td.primary"); // abre a edição
  await page.click("#debt-modal [data-action=delete-editing]");
  await page.waitForSelector("td:has-text('Empréstimo')", { state: "detached" });
  await page.evaluate(() => window.finanSync());
  await page.reload();
  await page.waitForSelector("h1:has-text('Dívidas')");
  ok(await page.locator("td:has-text('Empréstimo')").count() === 0 && !(await records("debt")).some((d) => d.name === "Empréstimo"),
    "dívida apagada some aqui e no computador");

  // Meta com categoria de aportes.
  await page.goto(`${B}/metas`);
  await page.click("[data-action=new-goal] >> nth=0");
  await page.waitForSelector("#goal-modal .modal-panel");
  await page.fill("input[name=name]", "Reserva");
  await page.fill("input[name=target]", "6.000");
  await page.fill("input[name=saved]", "1.000");
  await page.fill("input[name=deadline]", `${now.getFullYear() + 1}-${pad(now.getMonth() + 1)}`);
  await page.selectOption("select[name=category]", { label: "Reserva de emergência · Investimentos" });
  ok((await page.textContent("[data-preview]")).includes("durante 13 meses") && (await page.textContent("[data-preview]")).includes("R$ 384,62"),
    `a janela mostra quanto guardar por mês, com a mesma conta da meta (${await page.textContent("[data-preview]")})`);
  await page.click("form[data-form=goal] button[type=submit]");
  await page.waitForSelector(".goal-card:has-text('Reserva')");
  // Guardar rápido: com categoria, vira uma saída nela (como o consultor faz).
  await page.fill(".goal-card:has-text('Reserva') [data-quick] input", "250");
  await page.press(".goal-card:has-text('Reserva') [data-quick] input", "Enter");
  await page.waitForSelector(".goal-card:has-text('Reserva') .goal-amount:has-text('R$ 1.250,00')");
  await page.evaluate(() => window.finanSync());
  ok((await records("transaction")).some((t) => t.description === "Aporte: Reserva" && t.amountCents === 25000) && (await records("goal"))[0].savedCents === 100000,
    "guardar numa meta com categoria lança o aporte na categoria");
  // Sem categoria, soma no que já está guardado.
  await page.click("[data-action=new-goal] >> nth=0");
  await page.fill("input[name=name]", "Viagem");
  await page.fill("input[name=target]", "2.000");
  await page.click("form[data-form=goal] button[type=submit]");
  await page.waitForSelector(".goal-card:has-text('Viagem')");
  await page.fill(".goal-card:has-text('Viagem') [data-quick] input", "100");
  await page.click(".goal-card:has-text('Viagem') [data-quick] button");
  await page.waitForSelector(".goal-card:has-text('Viagem') .goal-amount:has-text('R$ 100,00')");
  ok((await page.textContent(".goal-strip")).includes("Guardado nas metas") && await page.locator(".goal-card").count() === 2, "faixa do topo e um cartão por meta");
  await page.click(".goal-card:has-text('Viagem') [data-edit]");
  await page.click("#goal-modal [data-action=delete-editing]");
  await page.waitForSelector(".goal-card:has-text('Viagem')", { state: "detached" });
  // Um aporte lançado na categoria conta para a meta.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector(".tx-card");
  await openTx(page);
  await page.fill("input[name=description]", "Poupança");
  await page.fill("input[name=amount]", "500");
  await page.selectOption("select[name=category_id]", { label: "Reserva de emergência · Investimentos" });
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Poupança')");
  await page.goto(`${B}/`);
  await page.waitForSelector(".goal:has-text('Reserva')");
  ok((await page.textContent(".goal")).includes("R$ 1.750,00 de R$ 6.000,00 · 29%"), `meta no painel soma o já guardado e os aportes (${await page.textContent(".goal")})`);
  await page.evaluate(() => window.finanSync());
  ok((await records("goal")).length === 1, "meta sincronizou");

  // Análise: ritmo, quanto dá para gastar por dia e onde foi o dinheiro.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector(".tx-card");
  await openTx(page);
  await page.fill("input[name=description]", "iFood pizza");
  await page.fill("input[name=amount]", "45");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('iFood pizza')");
  await page.goto(`${B}/`);
  await page.click("a[href='/analise']");
  await page.waitForSelector(".pace-chart canvas");
  await page.waitForFunction(() => window.Chart && Object.keys(window.Chart.instances).length > 0);
  const analysis = await page.textContent("main");
  ok(/Pode gastar por dia/.test(analysis) && /Projeção do mês/.test(analysis), "análise mostra quanto dá para gastar por dia e a projeção");
  ok(await page.locator(".hbars li:has-text('iFood')").count() === 1 && !/Poupança/.test(await page.textContent("#gastei")), "gastei onde: iFood aparece; investimento não conta como gasto");
  await page.hover(".pace-chart canvas", { position: { x: 60, y: 120 } });
  await page.waitForFunction(() => /^Dia \d+\s+· Este mês R\$/.test(document.querySelector(".pace-chart .chart-strip")?.innerText || ""));
  ok(true, "passar o dedo no gráfico mostra o dia na faixa de cima");
  await page.click("[data-compare=media]");
  await page.waitForSelector("[data-compare=media][aria-checked=true]");
  ok(new URL(page.url()).searchParams.get("comparar") === "media" && (await page.textContent("main")).includes("Média de 3 meses"), "comparar com a média de 3 meses");

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
