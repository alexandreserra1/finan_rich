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
const screenshotDir = mkdtempSync(join(tmpdir(), "finan-front-telas-"));
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

async function capture(page, name) {
  await page.waitForFunction(() => [...document.querySelectorAll("#app .chart canvas")]
    .every((canvas) => window.Chart?.getChart(canvas) || canvas.closest(".chart-box.failed")));
  await page.evaluate(() => {
    document.querySelectorAll("#app .chart canvas").forEach((canvas) => {
      const chart = window.Chart?.getChart(canvas);
      chart?.stop();
      chart?.update("none");
    });
  });
  await page.screenshot({ path: join(screenshotDir, name), fullPage: true, animations: "disabled" });
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
    console.error(`FALHA ${name} [${page.url()}, ${page.viewportSize()?.width} px]: ${error.message}`);
    try {
      const diagnostics = await page.evaluate(() => {
        const selector = (element) => {
          const parts = [];
          while (element) {
            if (element.id) {
              parts.unshift(`#${CSS.escape(element.id)}`);
              break;
            }
            const position = element.parentElement ? Array.from(element.parentElement.children).indexOf(element) + 1 : 1;
            parts.unshift(`${element.localName}:nth-child(${position})`);
            element = element.parentElement;
          }
          return parts.join(" > ");
        };
        const viewport = document.documentElement.clientWidth;
        const overflowing = Array.from(document.body.querySelectorAll("*")).flatMap((element) => {
          const bounds = element.getBoundingClientRect();
          if (!element.getClientRects().length || bounds.width === 0 || bounds.height === 0) return [];
          if (bounds.left >= 0 && bounds.right <= viewport) return [];
          return [{ selector: selector(element), width: bounds.width, scrollWidth: element.scrollWidth,
            left: bounds.left, right: bounds.right }];
        });
        return { path: location.pathname, title: document.title, viewport, scrollWidth: document.documentElement.scrollWidth,
          focused: selector(document.activeElement),
          headings: Array.from(document.querySelectorAll("#app h1")).map((heading) => ({
            text: heading.textContent.trim(), tabindex: heading.getAttribute("tabindex"), focused: heading === document.activeElement,
          })),
          overflowCount: overflowing.length,
          overflowCandidates: overflowing.sort((first, second) => second.right - first.right || first.left - second.left).slice(0, 20) };
      });
      console.error(`DIAGNÓSTICO ${name}: ${JSON.stringify(diagnostics, null, 2)}`);
    } catch (diagnosticError) {
      console.error(`Não foi possível coletar o diagnóstico: ${diagnosticError.message}`);
    }
    try {
      const captureDir = mkdtempSync(join("/tmp", "finan-e2e-simple-falha-"));
      const scenario = name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").replace(/[^a-zA-Z0-9]+/g, "-").slice(0, 100);
      const capturePath = join(captureDir, `${scenario}-${page.viewportSize()?.width}px.png`);
      await page.screenshot({ path: capturePath, fullPage: true, timeout: 5000 });
      console.error(`CAPTURA ${name}: ${capturePath}`);
    } catch (captureError) {
      console.error(`Não foi possível salvar a captura: ${captureError.message}`);
    }
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

  for (const [width, theme] of [390, 768, 1440, 1920].flatMap((width) => [[width, "claro"], [width, "escuro"]])) {
    await check(`links SPA levam foco e título à tela, sem estouro em ${width} px no tema ${theme}`, async (page) => {
      await page.setViewportSize({ width, height: 900 });
      await page.evaluate((theme) => { localStorage.setItem("tema", theme); window.finanTheme.apply(); }, theme);
      await capture(page, `${width}-${theme}-inicio.png`);
      await page.evaluate(() => { window.__spaDocument = true; });
      for (const path of ["/lancamentos", "/contas", "/dividas", "/metas", "/categorias", "/analise", "/chat", "/notas", "/importar", "/exportar", "/"]) {
        console.log(`ROTA links SPA: ${path} em ${width} px`);
        const link = page.locator(`nav.tabs a[href="${path}"]:not(.side-brand)`);
        if (!(await link.isVisible())) await page.locator("[data-menu]:visible").click();
        await link.click();
        await page.waitForURL(`${BASE}${path}`);
        await page.waitForFunction(() => {
          const heading = document.querySelector("#app h1");
          return heading && document.activeElement === heading && document.title === `${heading.textContent.trim()} — Rich`;
        });
        assert.equal(await page.locator("#app h1").getAttribute("tabindex"), "-1", `${path}: título deve receber foco por código`);
        assert.equal(await link.getAttribute("aria-current"), "page", `${path}: menu deve indicar a tela atual`);
        assert.equal(await page.evaluate(() => window.__spaDocument), true, `${path}: navegação não deve recarregar o documento`);
        await page.evaluate(() => document.fonts.ready);
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth),
          `${path}: não deve haver rolagem horizontal em ${width} px`);
        await capture(page, `${width}-${theme}-${path.slice(1) || "inicio"}.png`);
      }
      const settings = page.locator('nav.tabs a[href="/configuracoes"]');
      console.log(`ROTA Ajustes do servidor: /configuracoes em ${width} px`);
      if (!(await settings.isVisible())) await page.locator("[data-menu]:visible").click();
      await settings.click();
      await page.waitForURL(`${BASE}/configuracoes`);
      await page.waitForSelector("main h1");
      assert.equal(await page.locator("main h1").innerText(), "Do seu jeito. Mesmo.");
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth),
        `Ajustes do servidor: não deve haver rolagem horizontal em ${width} px`);
      await capture(page, `${width}-${theme}-configuracoes.png`);
    });
  }

  await check("voltar e avançar no histórico SPA restauram foco e título", async (page) => {
    await page.evaluate(() => { window.__spaDocument = true; });
    for (const path of ["/contas", "/metas"]) {
      await page.locator(`nav.tabs a[href="${path}"]`).click();
      await page.waitForURL(`${BASE}${path}`);
      await page.waitForSelector("#app h1");
    }
    for (const [direction, path] of [["goBack", "/contas"], ["goForward", "/metas"]]) {
      console.log(`ROTA histórico ${direction}: ${path}`);
      await page[direction]();
      await page.waitForURL(`${BASE}${path}`);
      await page.waitForFunction(() => {
        const heading = document.querySelector("#app h1");
        return heading && document.activeElement === heading && document.title === `${heading.textContent.trim()} — Rich`;
      });
      assert.equal(await page.locator("#app h1").getAttribute("tabindex"), "-1");
      assert.equal(await page.evaluate(() => window.__spaDocument), true, "histórico não deve recarregar o documento");
    }
  });

  await check("focusPage exportado foca o título e atualiza o nome da aba", async (page) => {
    const result = await page.evaluate(async () => {
      const { focusPage } = await import("/static/js/ui.js");
      const root = document.getElementById("app");
      const heading = root.querySelector("h1");
      document.querySelector('nav.tabs a[href="/contas"]').focus();
      document.title = "Título anterior";
      await focusPage(root);
      return { focused: document.activeElement === heading, tabindex: heading.getAttribute("tabindex"),
        title: document.title, expectedTitle: `${heading.textContent.trim()} — Rich` };
    });
    assert.equal(result.focused, true);
    assert.equal(result.tabindex, "-1");
    assert.equal(result.title, result.expectedTitle);
  });

  await check("focusPage preserva um campo que a tela já colocou em foco", async (page) => {
    assert.equal(await page.evaluate(async () => {
      const { focusPage } = await import("/static/js/ui.js");
      const root = document.getElementById("app");
      const field = document.createElement("input");
      root.append(field);
      field.focus();
      focusPage(root);
      return document.activeElement === field;
    }), true);
  });

  for (const [path, action, modalId] of [
    ["/lancamentos", "new-tx", "tx-modal"],
    ["/contas", "new-account", "account-modal"],
    ["/dividas", "new-debt", "debt-modal"],
    ["/metas", "new-goal", "goal-modal"],
  ]) {
    await check(`${modalId}: Tab e Shift+Tab ficam no modal; Escape devolve foco`, async (page) => {
      await page.locator(`nav.tabs a[href="${path}"]`).click();
      const trigger = page.locator(`[data-action="${action}"]:visible`).first();
      await trigger.click();
      const modal = page.locator(`#${modalId}`);
      await modal.locator(".modal-panel").waitFor();
      assert.ok(await modal.evaluate((element) => element.contains(document.activeElement)), "abrir deve levar foco ao modal");
      const focusable = modal.locator('a[href], button, input:not([type="hidden"]), select, textarea, [tabindex]');
      const count = await focusable.evaluateAll((elements) => elements.filter((element) =>
        !element.disabled && element.tabIndex >= 0 && element.getClientRects().length > 0).length);
      assert.ok(count > 1, "modal deve ter controles para exercitar a navegação");
      if (modalId === "account-modal") {
        const selected = modal.locator('input[type="radio"][name="kind"]:checked');
        await selected.focus();
        await page.keyboard.press("Tab");
        assert.ok(await modal.evaluate((element) => element.contains(document.activeElement)), "Tab deve continuar dentro da janela");
        assert.equal(await selected.evaluate((element) => document.activeElement.name === element.name), false,
          "Tab deve sair do grupo de opções; setas continuam escolhendo o tipo da conta");
      }
      for (const key of ["Tab", "Shift+Tab"]) {
        await focusable.evaluateAll((elements, key) => {
          const controls = elements.filter((element) => !element.disabled && element.tabIndex >= 0 && element.getClientRects().length > 0);
          (key === "Tab" ? controls[controls.length - 1] : controls[0]).focus();
        }, key);
        for (let step = 0; step <= count; step++) {
          await page.keyboard.press(key);
          assert.ok(await modal.evaluate((element) => element.contains(document.activeElement)), `${key} não deve sair do modal`);
          if (step === 0) {
            assert.ok(await focusable.evaluateAll((elements, key) => {
              const controls = elements.filter((element) => !element.disabled && element.tabIndex >= 0 && element.getClientRects().length > 0);
              return document.activeElement === (key === "Tab" ? controls[0] : controls[controls.length - 1]);
            }, key), `${key} deve circular entre o primeiro e o último controle`);
          }
        }
      }
      await page.keyboard.press("Escape");
      await modal.waitFor({ state: "hidden" });
      assert.ok(await trigger.evaluate((element) => document.activeElement === element), "Escape deve devolver foco ao disparador");
    });
  }

  await check("voltar com modal aberto libera o teclado e a rolagem", async (page) => {
    await page.locator('nav.tabs a[href="/contas"]').click();
    await page.locator('nav.tabs a[href="/lancamentos"]').click();
    await page.locator('[data-action="new-tx"]:visible').first().click();
    await page.locator("#tx-modal .modal-panel").waitFor();
    await page.goBack();
    await page.waitForURL(`${BASE}/contas`);
    assert.equal(await page.locator("body").evaluate((element) => element.classList.contains("modal-open")), false,
      "modal removido não deve manter o bloqueio de rolagem");
    assert.equal(await page.locator("#app h1").evaluate((element) => element.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Tab", bubbles: true, cancelable: true,
    }))), true, "o listener do modal anterior não deve cancelar Tab");
    await page.keyboard.press("Tab");
  });

  await check("focusPage não rouba foco de modal já aberto", async (page) => {
    await page.locator('nav.tabs a[href="/lancamentos"]').click();
    const trigger = page.locator('[data-action="new-tx"]:visible').first();
    await trigger.click();
    await page.locator("#tx-modal .modal-panel").waitFor();
    const keptFocus = await page.evaluate(async () => {
      const { focusPage } = await import("/static/js/ui.js");
      const active = document.activeElement;
      await focusPage(document.getElementById("app"));
      return document.querySelector("#tx-modal").contains(active) && document.activeElement === active;
    });
    assert.equal(keptFocus, true, "o controle ativo do modal deve continuar com foco");
    await page.keyboard.press("Escape");
    await page.locator("#tx-modal").waitFor({ state: "hidden" });
    assert.ok(await trigger.evaluate((element) => document.activeElement === element));
  });

  await check("movimento reduzido não inicia Web Animations na navegação SPA", async (page) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.evaluate(() => {
      window.__navigationAnimations = 0;
      const animate = Element.prototype.animate;
      Element.prototype.animate = function (...args) {
        window.__navigationAnimations++;
        return animate.apply(this, args);
      };
    });
    for (const path of ["/contas", "/metas"]) {
      await page.locator(`nav.tabs a[href="${path}"]`).click();
      await page.waitForURL(`${BASE}${path}`);
      await page.waitForSelector("#app h1");
    }
    await page.goBack();
    await page.waitForURL(`${BASE}/contas`);
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    assert.equal(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches), true);
    assert.equal(await page.evaluate(() => window.__navigationAnimations), 0, "links e histórico devem respeitar menos movimento");
  });

  for (const [theme, bg, text] of [["claro", "#f7f6f0", "#252627"], ["escuro", "#17191d", "#f5f2de"]]) {
    await check(`tokens de movimento existem sem mudar as cores do tema ${theme}`, async (page) => {
      await page.evaluate((theme) => { localStorage.setItem("tema", theme); window.finanTheme.apply(); }, theme);
      const styles = await page.evaluate(() => {
        const root = getComputedStyle(document.documentElement);
        return Object.fromEntries(["--motion-duration-enter", "--motion-duration-feedback", "--motion-ease",
          "--bg", "--text", "--primary", "--yellow"].map((name) => [name, root.getPropertyValue(name).trim()]));
      });
      for (const token of ["--motion-duration-enter", "--motion-duration-feedback", "--motion-ease"]) {
        assert.ok(styles[token], `${token} deve existir e não ser vazio`);
      }
      assert.equal(styles["--bg"], bg);
      assert.equal(styles["--text"], text);
      assert.equal(styles["--primary"], "#233cec");
      assert.equal(styles["--yellow"], "#f5efab");
    });
  }

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
    const persisted = await page.evaluate(async () => JSON.stringify(await (await import("/static/js/db.js")).dump()));
    assert.ok(!persisted.includes(message), "rascunho não deve persistir em nenhum registro, fila ou metadado do banco local");
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
console.log(`Capturas das telas com dados fictícios: ${screenshotDir}`);
process.exitCode = failures ? 1 : 0;
