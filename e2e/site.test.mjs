// Teste de ponta a ponta do site: o programa da Vercel (cmd/api compilado com
// -tags sitebundle) servindo as telas do app em modo site e o intermediário da
// IA no mesmo endereço. Sem computador: os dados ficam no navegador.
// Uso (na pasta e2e): SITE_BIN=../dist/finansite node site.test.mjs
import { chromium, devices } from "playwright";
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

  // Primeira visita: a landing, sem menu, cabendo na tela do celular.
  await page.goto(`${SITE}/`);
  await page.waitForSelector(".landing .lp-hero h1", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 1 && await page.locator(".tabs").isHidden(), "primeira visita mostra a landing, sem o menu");
  ok(await page.getAttribute(".lp-hero a.lp-cta", "href") === "/?entrar=1", "Começar agora leva ao login");
  ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "landing cabe na largura do celular");
  ok(await page.locator(".lp-agent").count() === 5, "landing mostra o time");

  // Login: as boas-vindas, sem "conecte ao computador".
  await page.goto(`${SITE}/?entrar=1`);
  await page.waitForSelector(".start button[data-action=solo]", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 0, "fora da landing, o modo tela cheia sai");
  const welcome = await page.innerText("main"); // só o que aparece (sem login com Google configurado, o botão fica escondido)
  ok(await page.locator(".start-google").isHidden() && /só neste navegador/.test(welcome) && !/computador/.test(welcome.replace(/programa do computador/, "")),
    "primeira visita mostra as boas-vindas do site");
  ok((await page.textContent("#sync-status")) === "Só neste navegador", "status: só neste navegador");

  // Começar: categorias criadas no navegador; lançar e recarregar mantém.
  await page.click(".start button[data-action=solo]");
  await page.waitForSelector(".welcome, .cards", { timeout: 20000 });
  await page.goto(`${SITE}/lancamentos`);
  await openTx(page);
  await openTx(page);
  await page.fill("input[name=description]", "Padaria do site");
  await page.fill("input[name=amount]", "12,50");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Padaria do site')");
  await page.reload();
  await page.waitForSelector("td:has-text('Padaria do site')", { timeout: 20000 });
  ok(true, "lançamento fica salvo no navegador depois de recarregar");

  // Ajustes do site: sem pareamento com o computador nem Open Finance.
  await page.goto(`${SITE}/configuracoes`);
  await page.waitForSelector("#sozinho");
  ok(await page.locator("#parear, form[data-form=pair], #open-finance").count() === 0, "Ajustes do site sem pareamento e sem Open Finance");
  ok(/Ficam só neste navegador/.test(await page.textContent("#sozinho")), "Ajustes explicam onde ficam os dados");

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
