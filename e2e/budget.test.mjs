// Teste de ponta a ponta do primeiro uso guiado, do orçamento por categoria e
// da busca nos lançamentos.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node budget.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18085;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-budget-"));
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
const prev = new Date(now.getFullYear(), now.getMonth() - 1, 1);
const prevMonth = `${prev.getFullYear()}-${pad(prev.getMonth() + 1)}`;

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // ---------- Primeiro uso ----------
  await page.goto(`${B}/`);
  await page.waitForSelector(".welcome:has-text('Passo 1 de 4')", { timeout: 20000 });
  ok(true, "sem lançamentos, o Painel mostra o passo a passo");
  await page.fill(".welcome input[name=amount]", "4.000");
  await page.fill(".welcome input[name=day]", "1");
  await page.click(".welcome button[type=submit]");
  await page.waitForSelector(".welcome:has-text('Passo 2 de 4')");
  ok(new URL(page.url()).searchParams.get("passo") === "2", "o passo fica no endereço");
  await page.fill(".welcome input[name=necessidades]", "55");
  await page.fill(".welcome input[name=desejos]", "25");
  await page.click(".welcome button[type=submit]");
  await page.waitForSelector(".welcome:has-text('Passo 3 de 4')");
  await page.goBack();
  await page.waitForSelector(".welcome:has-text('Passo 2 de 4')");
  await page.goForward();
  await page.waitForSelector(".welcome:has-text('Passo 3 de 4')");
  ok(true, "voltar e avançar andam entre os passos");
  await page.fill(".welcome input[name=name]", "Cartão");
  await page.fill(".welcome input[name=balance]", "1.500");
  await page.fill(".welcome input[name=rate]", "10");
  await page.fill(".welcome input[name=min_payment]", "200");
  await page.click(".welcome button[type=submit]");
  await page.waitForSelector(".welcome:has-text('Passo 4 de 4')");
  await page.fill(".welcome input[name=target]", "6.000");
  await page.click(".welcome button[type=submit]");
  await page.waitForSelector(".cards.has-hero");
  ok(await page.isVisible(".flash:has-text('Pronto!')"), "ao terminar, mostra o Painel montado");
  await page.evaluate(() => window.finanSync());
  const [rec, prefs, debts, goals] = await Promise.all(["recurring", "pref", "debt", "goal"].map(records));
  const split = prefs.find((p) => p.id === "split")?.value;
  ok(rec.length === 1 && rec[0].kind === "entrada" && rec[0].amountCents === 400000 && rec[0].day === 1, "passo 1 criou o salário recorrente");
  ok(split?.necessidades === 55 && split?.desejos === 25 && split?.investimentos === 20, "passo 2 salvou a divisão do salário");
  ok(debts.length === 1 && debts[0].monthlyRateBp === 1000 && debts[0].minPaymentCents === 20000, "passo 3 cadastrou a dívida");
  ok(goals.length === 1 && goals[0].name === "Reserva de emergência" && goals[0].targetCents === 600000, "passo 4 criou a meta");
  ok(prefs.some((p) => p.id === "primeiro-uso"), "o fim do guia vale para os outros aparelhos");
  await page.reload();
  await page.waitForSelector(".cards.has-hero");
  ok(await page.locator(".welcome").count() === 0, "depois do guia, o Painel abre normal");

  // ---------- Orçamento por categoria ----------
  await page.goto(`${B}/categorias`);
  const row = page.locator("tr:has-text('Restaurantes e delivery')");
  await row.locator("input[name=budget]").fill("100");
  await row.locator("button[type=submit]").click();
  await page.waitForSelector(".flash:has-text('Orçamento de Restaurantes e delivery: R$ 100,00 por mês.')");
  await page.evaluate(() => window.finanSync());
  ok((await records("category")).find((c) => c.name === "Restaurantes e delivery")?.budgetCents === 10000, "teto salvo na categoria e sincronizado");

  const add = async (description, amount, category, date) => {
    await page.fill("form[data-form=transaction] input[name=description]", description);
    await page.fill("form[data-form=transaction] input[name=amount]", amount);
    await page.selectOption("form[data-form=transaction] select[name=category_id]", { label: category });
    if (date) await page.fill("form[data-form=transaction] input[name=date]", date);
    await page.click("form[data-form=transaction] button[type=submit]");
    await page.waitForSelector(`td:has-text('${description}')`);
  };
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]");
  await add("iFood pizza", "45", "Restaurantes e delivery · Desejos");
  await add("Padaria Açúcar", "35", "Restaurantes e delivery · Desejos");
  await add("Pão de Açúcar", "120", "Mercado · Necessidades");
  await page.goto(`${B}/`);
  await page.waitForSelector("#orcamentos");
  ok((await page.textContent("#orcamentos")).includes("R$ 80,00 de R$ 100,00"), "Painel mostra quanto já foi usado do orçamento");
  ok(await page.isVisible(".alert.warn:has-text('Orçamento de Restaurantes e delivery em 80%')"), "aviso ao chegar a 80%");
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]");
  await add("iFood lanche", "30", "Restaurantes e delivery · Desejos");
  await page.goto(`${B}/`);
  await page.waitForSelector(".alert.danger:has-text('Orçamento de Restaurantes e delivery estourou')");
  ok((await page.textContent("#orcamentos")).includes("estourou R$ 10,00"), "aviso e barra quando o orçamento estoura");
  await page.goto(`${B}/analise`);
  await page.waitForSelector(".pace-chart svg");
  ok(/Orçamentos por categoria[\s\S]*R\$ 110,00 de R\$ 100,00/.test(await page.textContent("main")), "Análise mostra os orçamentos");

  // O aviso leva aos lançamentos da categoria.
  await page.goto(`${B}/`);
  await page.click(".alert.danger:has-text('estourou')");
  await page.waitForSelector(".filter-total");
  ok(await page.locator("tbody tr").count() === 3 && (await page.textContent(".filter-total")).includes("R$ 110,00"), "o aviso abre os lançamentos filtrados pela categoria");

  // ---------- Busca e filtros ----------
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]");
  await add("Pão de Açúcar antigo", "50", "Mercado · Necessidades", `${prevMonth}-10`);
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=filter]");
  await page.fill("input[name=busca]", "ACUCAR");
  await page.press("input[name=busca]", "Enter");
  await page.waitForSelector(".filter-total");
  const descs = async () => (await page.locator("tbody td.primary").allTextContents()).map((s) => s.trim()).sort().join("|");
  ok(new URL(page.url()).searchParams.get("busca") === "ACUCAR", "a busca fica no endereço");
  ok(await descs() === "Padaria Açúcar|Pão de Açúcar" && (await page.textContent(".filter-total")).includes("R$ 155,00"),
    `busca sem acento nem maiúscula, com o total do achado (${await descs()})`);
  await page.selectOption("form[data-form=filter] select[name=categoria]", { label: "Restaurantes e delivery" });
  await page.waitForFunction(() => new URL(location.href).searchParams.get("categoria"));
  ok(await descs() === "Padaria Açúcar" && (await page.textContent(".filter-total")).includes("R$ 35,00"), "filtro por categoria");
  await page.goBack();
  await page.waitForFunction(() => !new URL(location.href).searchParams.get("categoria"));
  ok(await descs() === "Padaria Açúcar|Pão de Açúcar", "voltar desfaz o último filtro");
  await page.goForward();
  await page.waitForFunction(() => new URL(location.href).searchParams.get("categoria"));
  ok(await descs() === "Padaria Açúcar", "avançar refaz");
  await page.goto(`${B}/lancamentos?busca=acucar`);
  await page.waitForSelector("form[data-form=filter]");
  await page.check("form[data-form=filter] input[name=todos]");
  await page.waitForSelector("h2:has-text('Lançamentos de todos os meses')");
  ok(await descs() === "Padaria Açúcar|Pão de Açúcar|Pão de Açúcar antigo" && (await page.textContent(".filter-total")).includes("R$ 205,00"),
    "em todos os meses acha também o mês passado");
  await page.goto(`${B}/lancamentos?tipo=entrada&todos=1`);
  await page.waitForSelector(".filter-total");
  ok((await descs()).split("|").every((d) => d.startsWith("Salário")), "filtro por tipo (só entradas)");
  await page.goto(`${B}/lancamentos?min=40&max=50`);
  await page.waitForSelector(".filter-total");
  ok(await descs() === "iFood pizza", "faixa de valor");
  await page.goto(`${B}/lancamentos?busca=nada-disso`);
  await page.waitForSelector("text=Nada encontrado com esses filtros.");
  ok(true, "sem resultado, avisa");

  // Celular estreito: nada sai da tela.
  await page.setViewportSize({ width: 320, height: 800 });
  for (const path of ["/lancamentos?busca=acucar", "/categorias", "/"]) {
    await page.goto(`${B}${path}`);
    await page.waitForSelector("main h1");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    ok(overflow <= 0, `em 320 px, ${path} cabe na tela (${overflow})`);
  }

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
