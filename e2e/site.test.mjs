// Teste de ponta a ponta do site: o programa da Vercel (cmd/api compilado com
// -tags sitebundle) servindo as telas do app em modo site e o intermediário da
// IA no mesmo endereço. Sem computador: os dados ficam no navegador.
// Uso (na pasta e2e): SITE_BIN=../dist/finansite node site.test.mjs
import { chromium as chromiumEngine, devices, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";

const BIN = process.env.SITE_BIN || "../dist/finansite";
const PORT = 18099;
const SITE = `http://127.0.0.1:${PORT}`;
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

const server = spawn(BIN, [], { env: { ...process.env, PORT: String(PORT), GROQ_API_KEY: "teste", FINAN_PROXY_UPSTREAM: "http://127.0.0.1:9" }, stdio: "ignore" });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${SITE}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ ...devices["Pixel 7"] });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // Antes de escolher a tela (landing ou Painel), a moldura do app não aparece.
  await page.addInitScript(() => document.addEventListener("DOMContentLoaded", () => {
    window.__shellBefore = getComputedStyle(document.querySelector("nav.tabs")).visibility;
  }));
  // Primeira visita: a landing, sem menu, cabendo na tela do celular.
  await page.goto(`${SITE}/`);
  await page.waitForSelector(".landing .lp-hero h1", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 1 && await page.locator(".tabs").isHidden(), "primeira visita mostra a landing, sem o menu");
  ok(await page.getAttribute(".lp-hero a.lp-cta", "href") === "/?entrar=1", "Começar agora leva ao login");
  ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "landing cabe na largura do celular");
  ok(await page.locator(".lp-agent").count() === 5, "landing mostra o time");
  ok(await page.evaluate(() => window.__shellBefore) === "hidden", "o menu do app não pisca antes da landing");
  ok(await page.evaluate(() => { const s = document.querySelector(".lp-faq"), w = s.querySelector(".lp-wrap"), q = s.querySelector("summary");
    return getComputedStyle(w).position === "static" && getComputedStyle(w).backgroundColor === "rgba(0, 0, 0, 0)" && getComputedStyle(q).color === "rgb(21, 32, 27)"; }),
    "estilos do app não vazam na landing (dúvidas com fundo claro e texto escuro)");

  ok(await page.locator('[data-action="solo"]').count() === 0 && !/Usar sem conta/.test(await page.innerText("main")), "a landing não oferece uso sem conta");
  await page.click(".lp-brand");
  ok(new URL(page.url()).pathname === "/" && await page.locator(".lp-hero h1").isVisible(), "o ícone da pantera, sem login, fica na página inicial");

  // Login: sem "Usar sem conta"; com o Google ainda não ligado no servidor, avisa.
  await page.goto(`${SITE}/?entrar=1`);
  await page.waitForSelector(".start [data-unavailable]:not([hidden])", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 0, "fora da landing, o modo tela cheia sai");
  const welcome = await page.innerText("main");
  ok(await page.locator(".start-google").isHidden() && /abre em breve/.test(welcome) && !/Usar sem conta|Começar a usar/.test(welcome),
    "o login sem Google ligado avisa e não deixa usar sem conta");
  await page.click(".start .brand");
  await page.waitForSelector(".lp-hero h1");
  ok(new URL(page.url()).search === "", "o ícone da pantera na tela de login volta para a landing");

  // Barreira: nenhum endereço do app abre sem login, nem digitado, nem com dados antigos no navegador.
  await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const f = await import("/static/js/finance.js");
    await f.load();
    await db.saveManyLocal(f.defaultCategories(Date.now()).map((record) => ({ type: "category", record })));
  });
  let leaked = [];
  for (const path of ["/", "/dividas", "/lancamentos", "/metas", "/analise", "/contas", "/chat", "/configuracoes", "/exportar"]) {
    await page.goto(`${SITE}${path}`);
    await page.waitForSelector(".landing, .start", { timeout: 20000 });
    const url = new URL(page.url());
    const visible = await page.evaluate(() => ["nav.tabs", "header.top", "#sync-status"].filter((q) => document.querySelector(q)?.checkVisibility({ visibilityProperty: true })));
    if (visible.length || !/^\/(\?entrar=1)?$/.test(url.pathname + url.search) || await page.locator(".cards, .tx-card, table").count()) leaked.push(path);
  }
  ok(leaked.length === 0, `sem login, nenhum endereço do app abre, mesmo com dados no navegador (vazou: ${leaked})`);

  // Ajustes do site (conferidos com login em google.test): aqui só a política de privacidade.
  // Política de privacidade (o Google exige o link).
  const privacy = await page.request.get(`${SITE}/privacidade.html`);
  ok(privacy.ok() && /drive\.appdata/.test(await privacy.text()), "política de privacidade publicada");

  // Segurança: a página do site só fala https (nada de http para a rede local).
  const csp = await page.getAttribute('meta[http-equiv="Content-Security-Policy"]', "content");
  ok(/connect-src 'self' https:;/.test(csp) && !/ http:/.test(csp), `CSP do site só https (${csp.match(/connect-src[^;]*/)?.[0]})`);

  // No mesmo endereço, o intermediário da IA continua respondendo.
  ok(await (await fetch(`${SITE}/healthz`)).text() === "ok", "intermediário da IA no mesmo endereço (/healthz)");
  const chat = await fetch(`${SITE}/chat/completions`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"messages":[{"role":"user","content":"oi"}]}' });
  ok(chat.status === 403, `pedido fora do formato do app continua recusado pelo intermediário (${chat.status})`);
  ok((await fetch(`${SITE}/static/nao-existe.js`)).status === 404, "arquivo que não existe dá 404, não a página do app");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.kill();
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
