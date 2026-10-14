// Gera as capturas de tela das lojas de apps (F-Droid/IzzyOnDroid) com dados
// de exemplo, numa tela de celular. Não é teste: rode quando a interface mudar.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node screenshots.mjs
import { chromium, devices } from "playwright";
import { spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const OUT = "../fastlane/metadata/android/pt-BR/images/phoneScreenshots";
const PORT = 18079;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-shots-"));
const server = spawn(BIN, ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], { stdio: "ignore" });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${B}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}
const sync = async (changes) => (await (await fetch(`${B}/api/sync`, {
  method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cursor: 0, changes }),
})).json()).changes;

// Dados de exemplo no mês atual.
const cats = Object.fromEntries((await sync([])).filter((c) => c.type === "category").map((c) => [c.record.name, c.record.id]));
const month = new Date().toISOString().slice(0, 7);
const now = Date.now();
let n = 0;
const rec = (type, fields) => ({ type, record: { id: `demo-${type}-${++n}`, updatedAt: now, ...fields } });
const tx = (day, kind, description, reais, cat) =>
  rec("transaction", { kind, description, amountCents: Math.round(reais * 100), categoryId: cats[cat], date: `${month}-${String(day).padStart(2, "0")}` });
await sync([
  tx(5, "entrada", "Salário", 5200, "Salário"),
  tx(5, "saida", "Aluguel", 1450, "Moradia"),
  tx(6, "saida", "Supermercado", 612.4, "Mercado"),
  tx(8, "saida", "Conta de luz", 189.9, "Contas da casa (luz, água, internet)"),
  tx(9, "saida", "Uber", 38.5, "Transporte"),
  tx(10, "saida", "iFood", 74.9, "Restaurantes e delivery"),
  tx(11, "saida", "Netflix", 44.9, "Assinaturas"),
  tx(12, "saida", "Farmácia", 86.3, "Saúde"),
  tx(12, "saida", "Reserva de emergência", 400, "Investimentos"),
  rec("debt", { name: "Cartão de crédito", creditor: "Banco", balanceCents: 320000, monthlyRateBp: 1290, minPaymentCents: 48000, dueDay: 15 }),
  rec("debt", { name: "Empréstimo pessoal", creditor: "Financeira", balanceCents: 850000, monthlyRateBp: 349, minPaymentCents: 61000, dueDay: 20 }),
  rec("goal", { name: "Reserva de emergência", targetCents: 1500000, savedCents: 420000, deadline: `${Number(month.slice(0, 4)) + 1}-06`, startDate: `${month}-01` }),
  rec("goal", { name: "Viagem de férias", targetCents: 600000, savedCents: 150000, startDate: `${month}-01` }),
].filter((c) => c.type !== "transaction" || c.record.categoryId));

mkdirSync(OUT, { recursive: true });
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const ctx = await browser.newContext({ ...devices["Pixel 7"], colorScheme: "light" });
  const page = await ctx.newPage();
  // [caminho, pular o formulário de cadastro e mostrar a lista]
  const shots = [["/", false], ["/lancamentos", true], ["/dividas", true], ["/metas", true]];
  for (const [i, [path, skipForm]] of shots.entries()) {
    await page.goto(B + path);
    await page.waitForSelector("main form, main .panel", { timeout: 20000 });
    await page.waitForTimeout(800);
    if (skipForm) {
      await page.evaluate(() => {
        const panel = [...document.querySelectorAll("main .panel")].find((p) => /^(Suas|Lançamentos de)/.test(p.querySelector("h2")?.textContent || ""));
        if (panel) window.scrollTo(0, panel.getBoundingClientRect().top + window.scrollY - 76);
      });
      await page.waitForTimeout(300);
    }
    await page.screenshot({ path: `${OUT}/${i + 1}.png` });
    console.log(`${OUT}/${i + 1}.png`);
  }
} finally {
  await browser.close();
  server.kill();
  rmSync(dataDir, { recursive: true, force: true });
}
