import { chromium as chromiumEngine, webkit } from "playwright";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { watchErrors } from "./errors.mjs";

const engine = process.env.BROWSER === "webkit" ? webkit : chromiumEngine;
const PORT = 18100 + Number(process.env.PORT_OFFSET || 0);
const BASE = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-simple-"));
const payload = '<img src=x onerror="window.__xss=1"><svg onload="window.__xss=1">';
const server = spawn(process.env.FINANRICH_BIN || "../dist/finanrich",
  ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], {
    env: { ...process.env, FINAN_PASSWORD: "", FINAN_AI_PRESET: "ollama",
      FINAN_AI_BASE_URL: "http://127.0.0.1:9", FINAN_AI_MODEL: "teste-local", FINAN_AI_API_KEY: "" },
    stdio: "ignore",
  });
let spawnError;
server.on("error", (error) => { spawnError = error; });
let browser;
let failures = 0;

async function home(page) {
  await page.goto(`${BASE}/`);
  await page.waitForSelector("main h1");
}

async function check(name, verify, withDebts = false) {
  const context = await browser.newContext({ viewport: { width: 1200, height: 900 }, serviceWorkers: "block" });
  const page = await context.newPage();
  page.setDefaultTimeout(5000);
  const errors = [];
  const aiRequests = [];
  const externalRequests = [];
  watchErrors(page, errors);
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== BASE) {
      externalRequests.push(request.url());
      return route.abort();
    }
    if (request.method() === "POST" && /\/api\/chat(?:\/|$)|\/chat\/completions|\/api\/generate/.test(url.pathname)) {
      aiRequests.push(request.postData());
      return route.fulfill({ status: 503, contentType: "application/json", body: '{"error":"IA bloqueada no teste"}' });
    }
    return route.continue();
  });
  try {
    await home(page);
    await page.evaluate(async ({ payload, withDebts }) => {
      const finance = await import("/static/js/finance.js");
      const db = await import("/static/js/db.js");
      const { today } = await import("/static/js/ui.js");
      await finance.load();
      const stamp = Date.now();
      const categories = finance.defaultCategories(stamp);
      const income = categories.find((category) => category.bucket === "renda");
      const expense = categories.find((category) => category.bucket === "desejos");
      const records = [
        ...categories.map((record) => ({ type: "category", record })),
        { type: "pref", record: { id: "primeiro-uso", value: true, updatedAt: stamp } },
        { type: "transaction", record: { id: "simple-income", kind: "entrada", description: "Salário", amountCents: 432100, categoryId: income.id, date: today(), updatedAt: stamp } },
        { type: "transaction", record: { id: "simple-expense", kind: "saida", description: payload, amountCents: 12345, categoryId: expense.id, date: today(), updatedAt: stamp } },
        { type: "recurring", record: { id: "simple-fixed", kind: "saida", description: payload, amountCents: 56789, categoryId: expense.id, day: 28, start: today().slice(0, 7), updatedAt: stamp } },
        ...Array.from({ length: 4 }, (_, index) => ({ type: "debt", record: {
          id: `simple-debt-${index}`, name: `Dívida ${index + 1}`, balanceCents: 500000,
          monthlyRateBp: 1000, minPaymentCents: 1000, updatedAt: stamp, deleted: !withDebts,
        } })),
      ];
      await db.saveManyLocal(records.map(({ type, record }) => ({ type, record: finance.validate(type, record) })));
      await window.finanSync();
    }, { payload, withDebts });
    await home(page);
    await verify(page);
    assert.deepEqual(aiRequests, [], "não deve enviar perguntas para a IA");
    assert.deepEqual(externalRequests, [], "não deve acessar serviços externos");
    assert.deepEqual(errors, [], "não deve haver erros de JavaScript");
    console.log(`OK    ${name}`);
  } catch (error) {
    failures++;
    console.error(`FALHA ${name}: ${error.message}`);
  } finally {
    await context.close();
  }
}

try {
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (spawnError) throw spawnError;
    if (server.exitCode !== null) throw new Error(`Servidor encerrou com código ${server.exitCode}`);
    try { ready = (await fetch(`${BASE}/healthz`)).ok; } catch {}
    if (ready) break;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.ok(ready, "servidor local deve iniciar");
  browser = await engine.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });

  await check("Início mostra dinheiro agora, semana e até três insights compactos", async (page) => {
    assert.ok(await page.locator('[data-section="money-now"]').isVisible(), "falta a seção Seu dinheiro agora");
    assert.match(await page.locator('[data-section="money-now"]').innerText(), /Seu dinheiro agora/i);
    assert.match(await page.locator('[data-section="week"]').innerText(), /Nesta semana/i);
    const insights = page.locator('[data-section="insights"] [data-insight]');
    assert.ok(await page.locator('[data-section="insights"]').isVisible());
    const source = await page.evaluate(async () => {
      const finance = await import("/static/js/finance.js");
      const ledger = await (await import("/static/js/db.js")).loadLedger();
      const day = (await import("/static/js/ui.js")).today();
      return { alertCount: finance.alerts(ledger, day).length, insights: finance.home(ledger, day).insights };
    });
    assert.ok(source.alertCount > 3, "os dados devem gerar mais de três achados para testar o limite");
    assert.ok(await insights.count() > 0 && await insights.count() <= 3, "deve mostrar de um a três insights com dados");
    assert.equal(await insights.count(), source.insights.length, "deve respeitar a seleção e a prioridade calculadas pelo Go");
    for (const [index, insight] of source.insights.entries()) {
      assert.equal(await insights.nth(index).locator("h3").innerText(), insight.title);
      assert.equal(await insights.nth(index).locator(".brief-main").getAttribute("href"), insight.link);
    }
    assert.equal(await page.locator('main canvas[aria-label*="Entradas e saídas por mês"], main .year-strip, main .teamcard, main .teamgrid.briefs').count(), 0,
      "Início não deve mostrar gráfico anual nem cartões grandes do time");
  }, true);

  await check("resumo usa os números do Go e explica que a estimativa não é saldo disponível", async (page) => {
    const expected = await page.evaluate(async () => {
      const finance = await import("/static/js/finance.js");
      const ledger = await (await import("/static/js/db.js")).loadLedger();
      const day = (await import("/static/js/ui.js")).today();
      const dashboard = finance.dashboard(ledger, day.slice(0, 7));
      const insights = finance.insights(ledger, day);
      const home = finance.home(ledger, day);
      return { free: finance.money(insights.freeCents),
        income: finance.money(dashboard.inCents), expense: finance.money(dashboard.outCents),
        weekIncome: finance.money(home.weekInCents), weekExpense: finance.money(home.weekOutCents) };
    });
    const normalize = (text) => text.replace(/\s/g, " ");
    const summary = normalize(await page.locator('[data-section="money-now"]').innerText());
    for (const amount of [expected.free, expected.income, expected.expense]) assert.ok(summary.includes(normalize(amount)), `resumo deve incluir ${amount}`);
    const week = normalize(await page.locator('[data-section="week"]').innerText());
    for (const amount of [expected.weekIncome, expected.weekExpense]) assert.ok(week.includes(normalize(amount)), `semana deve incluir ${amount} calculados pelo Go`);
    assert.match(summary, /estimativ/i);
    const explanation = page.locator('[data-section="money-now"] details');
    await explanation.locator("summary").click();
    assert.equal(await explanation.getAttribute("open"), "");
    assert.match(await explanation.innerText(), /n[aã]o [ée] (?:o |um |seu )?saldo dispon[ií]vel/i);
    assert.match(await explanation.innerText(), /entradas|renda/i);
    assert.match(await explanation.innerText(), /gastos|sa[ií]das|contas fixas/i);
  });

  await check("dívida ativa impede apresentar a estimativa como dinheiro livre", async (page) => {
    const summary = await page.locator('[data-section="money-now"]').innerText();
    assert.match(summary, /d[ií]vida/i);
    assert.match(summary, /confira|revis|plano|parcelas/i);
    assert.doesNotMatch(summary, /voc[eê] pode gastar|saldo dispon[ií]vel\s*R\$/i);
  }, true);

  await check("movimentos, análise, registro e importação continuam acessíveis", async (page) => {
    for (const path of ["/lancamentos", "/analise", "/importar"]) {
      const link = page.locator(`a[href="${path}"]:visible`).first();
      assert.ok(await link.isVisible(), `falta o atalho ${path}`);
      assert.ok((await link.innerText()).trim(), "atalho deve ter nome legível");
      await link.click();
      await page.waitForURL(`${BASE}${path}`);
      await page.waitForSelector("main h1");
      await home(page);
    }
    await page.locator('[data-action="new-tx"]:visible, a[href="/lancamentos?novo=1"]:visible').first().click();
    await page.waitForSelector("#tx-modal .modal-panel");
    assert.ok(await page.locator('form[data-form="transaction"]').isVisible());
  });

  await check("registro livre fica no rascunho, sem URL, persistência ou envio automático", async (page) => {
    const message = 'Paguei R$ 123,45 para Maria & João? Conta <particular> #sigilo';
    const urls = [];
    page.on("framenavigated", (frame) => { if (frame === page.mainFrame()) urls.push(frame.url()); });
    const before = await page.evaluate(async () => (await (await import("/static/js/db.js")).allRecords()));
    const input = page.locator('form[data-form="quick-entry"] input[name="message"]');
    await input.fill(message);
    await input.press("Enter");
    await page.waitForURL(`${BASE}/chat`);
    const draft = page.locator('form[data-form="ask"] textarea[name="message"]');
    await draft.waitFor();
    assert.equal(await draft.inputValue(), message);
    assert.deepEqual(urls.filter((url) => new URL(url).search || new URL(url).hash), [], "nenhuma navegação pode carregar o registro livre");
    assert.equal(await page.locator(".msg.user, .msg.pending").count(), 0, "a pessoa ainda precisa confirmar o envio");
    assert.match(await page.locator(".composer-hint").innerText(), /nada [ée] lan[cç]ado sem voc[eê] confirmar/i);
    const after = await page.evaluate(async () => ({
      records: await (await import("/static/js/db.js")).allRecords(),
      storage: JSON.stringify([Object.entries(localStorage), Object.entries(sessionStorage)]),
    }));
    assert.deepEqual(after.records, before, "preencher rascunho não deve alterar registros");
    assert.ok(!after.storage.includes(message), "rascunho não deve ir para os armazenamentos do navegador");
    const persisted = await page.evaluate(async () => {
      const database = await new Promise((resolve, reject) => {
        const request = indexedDB.open("finan_rich");
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      try {
        return JSON.stringify(await Promise.all(Array.from(database.objectStoreNames, (name) => new Promise((resolve, reject) => {
          const request = database.transaction(name).objectStore(name).getAll();
          request.onsuccess = () => resolve(request.result);
          request.onerror = () => reject(request.error);
        }))));
      } finally { database.close(); }
    });
    assert.ok(!persisted.includes(message), "rascunho não deve persistir em nenhum registro, fila ou metadado do IndexedDB");
    await page.reload();
    await draft.waitFor();
    assert.equal(await draft.inputValue(), "", "recarregar deve descartar o rascunho em memória");
  });

  await check("registro rápido não sobrescreve uma mensagem já digitada no Consultor", async (page) => {
    const original = "Ainda estou revisando minha mensagem particular";
    const replacement = "Outra mensagem que não deve apagar a anterior";
    await page.locator('nav.tabs a[href="/chat"]').click();
    const draft = page.locator('form[data-form="ask"] textarea[name="message"]');
    await draft.waitFor();
    await draft.fill(original);
    await page.locator('nav.tabs a[href="/"]:not(.side-brand)').click();
    const input = page.locator('form[data-form="quick-entry"] input[name="message"]');
    await input.fill(replacement);
    await input.press("Enter");
    await page.locator(".flash.err").waitFor();
    assert.match(await page.locator(".flash.err").innerText(), /j[aá] tem uma mensagem|em andamento/i);
    assert.equal(page.url(), `${BASE}/`, "o conflito deve manter a pessoa no Início");
    assert.equal(await input.inputValue(), replacement, "o registro recusado também não deve ser perdido");
    await page.locator('nav.tabs a[href="/chat"]').click();
    await draft.waitFor();
    assert.equal(await draft.inputValue(), original, "a mensagem digitada antes deve continuar intacta");
    assert.equal(await page.locator(".msg.user, .msg.pending").count(), 0, "nenhuma das mensagens deve ser enviada automaticamente");
    await page.reload();
    await draft.waitFor();
    assert.equal(await draft.inputValue(), "", "recarregar também deve descartar a mensagem digitada diretamente no Consultor");
  });

  await check("proposta do Consultor só grava depois da confirmação da pessoa", async (page) => {
    const startDate = await page.evaluate(async () => (await import("/static/js/ui.js")).today());
    const record = { id: "simple-confirm-goal", name: "Meta confirmada", targetCents: 12300, savedCents: 0, startDate, updatedAt: Date.now() };
    await page.route("**/api/chat*", (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      return route.fulfill({ contentType: "application/json", body: JSON.stringify({
        ready: true, local: true, model: "teste-local", history: [{ role: "assistant", agent: "tito",
          content: "Confira a meta antes de adicionar.", proposals: [{ action: "criar", type: "goal", label: "Meta confirmada", record }] }],
      }) });
    });
    await page.goto(`${BASE}/chat`);
    const confirm = page.locator("[data-propose]");
    await confirm.waitFor();
    const saved = () => page.evaluate(async () => (await (await import("/static/js/db.js")).loadLedger()).goals.filter((goal) => goal.id === "simple-confirm-goal" && !goal.deleted));
    assert.deepEqual(await saved(), [], "mostrar a proposta não deve gravar a meta");
    assert.equal(await confirm.isEnabled(), true);
    await confirm.click();
    await page.waitForFunction(() => document.querySelector("[data-propose]")?.disabled);
    const goals = await saved();
    assert.equal(goals.length, 1);
    assert.equal(goals[0].targetCents, record.targetCents);
    assert.equal(goals[0].name, record.name);
  });

  await check("dados de lançamentos e recorrências aparecem como texto, sem XSS", async (page) => {
    assert.ok((await page.locator("main").innerText()).includes(payload), "os dados escapados devem continuar legíveis");
    assert.equal(await page.locator('main img[src="x"], main svg[onload], main [onerror]').count(), 0);
    assert.equal(await page.evaluate(() => window.__xss), undefined);
  });

  for (const theme of ["claro", "escuro"]) {
    await check(`Início cabe em 320 px no tema ${theme}`, async (page) => {
      await page.setViewportSize({ width: 320, height: 740 });
      await page.evaluate((theme) => { localStorage.setItem("tema", theme); window.finanTheme.apply(); }, theme);
      await home(page);
      assert.equal(await page.locator("html").getAttribute("data-theme"), theme === "escuro" ? "dark" : "light");
      for (const section of ["money-now", "week"]) assert.ok(await page.locator(`[data-section="${section}"]`).isVisible(), `seção ${section} visível`);
      const hasInsights = await page.evaluate(async () => {
        const finance = await import("/static/js/finance.js");
        return finance.home(await (await import("/static/js/db.js")).loadLedger(), (await import("/static/js/ui.js")).today()).insights.length > 0;
      });
      assert.equal(await page.locator('[data-section="insights"]').isVisible(), hasInsights, "insights aparecem apenas quando o Go encontra algo relevante");
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "não deve haver rolagem horizontal");
      const explanation = page.locator('[data-section="money-now"] details');
      await explanation.locator("summary").click();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "explicação aberta também deve caber");
    });
  }
} catch (error) {
  failures++;
  console.error(`FALHA preparação: ${error.message}`);
} finally {
  await browser?.close();
  if (server.pid && server.exitCode === null) {
    const stopped = once(server, "exit");
    server.kill();
    await stopped;
  }
  rmSync(dataDir, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exitCode = failures ? 1 : 0;
