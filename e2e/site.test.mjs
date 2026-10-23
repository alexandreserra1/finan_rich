// Teste de ponta a ponta do site: as telas do app publicadas na internet
// (site/www, geradas por "go run ./tools/mobilebuild -site site/www"), servidas
// como a Vercel serve (arquivo existente ou index.html). Sem computador: os
// dados ficam no navegador.
// Uso (na pasta e2e): SITE_WWW=../site/www node site.test.mjs
import { chromium, devices } from "playwright";
import { createServer } from "node:http";
import { readFileSync, statSync } from "node:fs";
import { extname, join, normalize } from "node:path";

const WWW = process.env.SITE_WWW || "../site/www";
const PORT = 18099;
const SITE = `http://127.0.0.1:${PORT}`;
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const TYPES = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".wasm": "application/wasm", ".png": "image/png", ".svg": "image/svg+xml" };
const server = createServer((req, res) => {
  const path = normalize(decodeURIComponent(new URL(req.url, SITE).pathname)).replace(/^(\.\.[/\\])+/, "");
  let file = join(WWW, path);
  try {
    if (!statSync(file).isFile()) throw new Error();
  } catch {
    file = join(WWW, "index.html"); // como o "rewrites" do site/vercel.json
  }
  res.writeHead(200, { "Content-Type": TYPES[extname(file)] || "application/octet-stream" });
  res.end(readFileSync(file));
});
await new Promise((r) => server.listen(PORT, "127.0.0.1", r));

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ ...devices["Pixel 7"] });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // Primeira visita: boas-vindas, sem "conecte ao computador".
  await page.goto(`${SITE}/`);
  await page.waitForSelector(".start button[data-action=solo]", { timeout: 20000 });
  const welcome = await page.textContent("main");
  ok(/só neste navegador/.test(welcome) && !/computador/.test(welcome.replace(/programa do computador/, "")),
    "primeira visita mostra as boas-vindas do site");
  ok((await page.textContent("#sync-status")) === "Só neste navegador", "status: só neste navegador");

  // Começar: categorias criadas no navegador; lançar e recarregar mantém.
  await page.click(".start button[data-action=solo]");
  await page.waitForSelector(".welcome, .cards", { timeout: 20000 });
  await page.goto(`${SITE}/lancamentos`);
  await page.locator("details.add[data-add=transaction]:not([open]) > summary").click({ timeout: 2000 }).catch(() => {});
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

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.close();
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
