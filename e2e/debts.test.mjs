// Simulações de dívida do começo ao fim, na tela de Dívidas: a frase do plano,
// os anos, o calendário mês a mês, as contagens e as quitações batem com a
// conta das regras em Go (finance.Debts pelo WebAssembly). Cada cenário imprime
// um resumo.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node debts.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18089;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-debts-"));
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
const sync = (changes) => fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cursor: 0, changes }) });

const MESES = ["janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"];
const now = new Date();
const monthAhead = (n) => { const d = new Date(now.getFullYear(), now.getMonth() + n, 1); return { label: `${MESES[d.getMonth()]} de ${d.getFullYear()}`, year: d.getFullYear() }; };
const duration = (n) => {
  const y = Math.floor(n / 12), m = n % 12, p = (k, a, b) => `${k} ${k === 1 ? a : b}`;
  return !y ? p(m, "mês", "meses") : m ? `${p(y, "ano", "anos")} e ${p(m, "mês", "meses")}` : p(y, "ano", "anos");
};
const brl = (c) => `R$ ${(c / 100).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

let seq = 0, current = [];
// setDebts troca as dívidas do cenário (apaga as do anterior).
async function setDebts(debts) {
  const t = Date.now() + seq++;
  const gone = current.map((d) => ({ type: "debt", record: { ...d, updatedAt: t, deleted: true } }));
  current = debts.map((d, i) => ({ id: `cen-${seq}-${i}`, updatedAt: t, creditor: "", dueDay: 0, ...d }));
  await sync([...gone, ...current.map((record) => ({ type: "debt", record }))]);
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));

  // open abre Dívidas (com o valor a mais) e devolve a conta do Go e o que a tela mostra.
  async function open(extraCents = 0) {
    await page.goto(`${B}/dividas${extraCents ? `?extra=${extraCents}` : ""}`);
    await page.waitForSelector(".debt-plan", { timeout: 20000 });
    await page.waitForFunction((n) => document.querySelectorAll("tbody tr td.primary, .cards-mobile tbody tr").length >= n, current.length);
    return page.evaluate(async (extra) => {
      const finance = await import("/static/js/finance.js");
      const ledger = await (await import("/static/js/db.js")).loadLedger();
      const v = finance.debts(ledger, extra);
      const cal = document.querySelector(".calendar");
      cal?.setAttribute("open", "");
      return {
        v,
        lead: document.querySelector(".plan-lead")?.textContent.replace(/\s+/g, " ").trim() || "",
        notices: [...document.querySelectorAll(".debt-plan .notice")].map((n) => n.textContent.replace(/\s+/g, " ").trim()),
        counts: document.querySelector(".debt-plan .small.muted")?.textContent.replace(/\s+/g, " ").trim() || "",
        order: [...document.querySelectorAll(".paid-off li")].map((li) => li.textContent.replace(/\s+/g, " ").trim()),
        months: cal ? cal.querySelectorAll("tbody tr:not(.year-row)").length : 0,
        years: cal ? [...cal.querySelectorAll("tbody tr.year-row")].map((r) => Number(r.textContent)) : [],
        firstRow: cal ? [...cal.querySelectorAll("tbody tr:not(.year-row)")[0].querySelectorAll("td")].map((td) => td.textContent.trim()) : [],
        quitadas: cal ? cal.querySelectorAll(".tag.ok").length : 0,
      };
    }, extraCents);
  }
  const summary = (name, s) => console.log(`      ${name}: ${s.v.plan.ok ? `${duration(s.v.plan.months)}, juros ${brl(s.v.plan.interestCents)}` : "não acaba"} | só o mínimo: ${s.v.minOnly.ok ? `${duration(s.v.minOnly.months)}, juros ${brl(s.v.minOnly.interestCents)}` : "não acaba"}`);

  // 1. Três dívidas comuns: cartão (caro), empréstimo e cheque especial.
  await setDebts([
    { name: "Cartão", balanceCents: 320000, monthlyRateBp: 1290, minPaymentCents: 48000 },
    { name: "Empréstimo", balanceCents: 850000, monthlyRateBp: 349, minPaymentCents: 61000 },
    { name: "Cheque especial", balanceCents: 150000, monthlyRateBp: 800, minPaymentCents: 15000 },
  ]);
  let s = await open();
  summary("três dívidas, sem dinheiro a mais", s);
  const end = monthAhead(s.v.plan.months);
  ok(s.v.plan.ok && s.lead.includes("Pague primeiro Cartão") && s.lead.includes(duration(s.v.plan.months)) && s.lead.includes(`(${end.label})`) && s.lead.includes(brl(s.v.plan.interestCents)),
    `frase do plano: ${s.lead}`);
  ok(s.counts === `3 dívidas: a primeira acaba em ${monthAhead(s.v.plan.paidOff[0].month).label}, a última em ${end.label}.`, `contagem: ${s.counts}`);
  ok(s.order.length === 3 && s.order[0].startsWith("Cartão") && s.order.every((o, i) => o.endsWith(monthAhead(s.v.plan.paidOff[i].month).label)), `ordem para quitar com o mês de cada uma: ${s.order.join(" / ")}`);
  ok(s.months === s.v.plan.months && s.quitadas === 3, `calendário com ${s.months} meses e 3 quitações marcadas`);
  const years = [...new Set(Array.from({ length: s.v.plan.months }, (_, i) => monthAhead(i + 1).year))];
  ok(JSON.stringify(s.years) === JSON.stringify(years), `anos no calendário: ${s.years.join(", ")}`);
  ok(s.firstRow[0] === monthAhead(1).label.replace(/ de \d{4}$/, "") && s.firstRow[4].includes(brl(s.v.totalMinCents)), `1º mês do calendário (${s.firstRow[0]}) paga ${s.firstRow[4]} (a soma dos mínimos)`);
  const saved = s.v.minOnly.interestCents - s.v.plan.interestCents;
  ok(saved > 0 && s.notices[0].includes(brl(saved)), `economia contra só o mínimo: ${s.notices[0]}`);

  // 2. O mesmo com R$ 500 a mais por mês: acaba antes e paga menos juros.
  const before = s.v.plan;
  s = await open(50000);
  summary("três dívidas, R$ 500 a mais", s);
  ok(s.v.plan.months < before.months && s.v.plan.interestCents < before.interestCents && s.firstRow[4].includes(brl(s.v.totalMinCents + 50000)),
    `R$ 500 a mais: ${duration(before.months)} → ${duration(s.v.plan.months)}, juros ${brl(before.interestCents)} → ${brl(s.v.plan.interestCents)}`);
  ok(s.v.minOnly.months > s.v.plan.months && s.notices[0].includes(`sai ${duration(s.v.minOnly.months - s.v.plan.months)} antes`), `quanto antes sai: ${s.notices[0]}`);

  // 3. Rotativo em que o mínimo não cobre os juros: só no mínimo nunca acaba.
  await setDebts([{ name: "Rotativo", balanceCents: 300000, monthlyRateBp: 1250, minPaymentCents: 10000 }]);
  s = await open();
  summary("rotativo, mínimo não cobre os juros", s);
  ok(!s.v.plan.ok && s.notices.some((n) => n.includes("não acabam")) && s.months === 0, "sem dinheiro a mais: avisa que não acaba e não mostra calendário");
  s = await open(40000);
  summary("rotativo, R$ 400 a mais", s);
  ok(s.v.plan.ok && s.notices.some((n) => n.includes("Só no mínimo as dívidas não acabam")) && s.months === s.v.plan.months, `com R$ 400 a mais acaba em ${duration(s.v.plan.months)}`);

  // 4. Financiamento longo: 10 anos e 1 mês (conta da tabela Price), um ano por grupo no calendário.
  await setDebts([{ name: "Financiamento", balanceCents: 5000000, monthlyRateBp: 150, minPaymentCents: 90000 }]);
  s = await open();
  summary("financiamento de R$ 50 mil a 1,5%", s);
  ok(s.v.plan.months === 121 && s.lead.includes("10 anos e 1 mês") && s.months === 121 && s.years.length === new Set(Array.from({ length: 121 }, (_, i) => monthAhead(i + 1).year)).size,
    `financiamento: ${s.lead.match(/em [^(]+/)?.[0]} em ${s.years.length} anos de calendário`);

  // 5. Dívida sem juros que acaba no 1º mês.
  await setDebts([{ name: "Parcela da loja", balanceCents: 20000, monthlyRateBp: 0, minPaymentCents: 20000 }]);
  s = await open();
  ok(s.v.plan.months === 1 && s.lead.startsWith("Pagando R$ 200,00 por mês, você quita Parcela da loja em 1 mês") && s.months === 1 && s.v.plan.interestCents === 0, `uma dívida só, acaba no 1º mês, sem juros: ${s.lead}`);

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
