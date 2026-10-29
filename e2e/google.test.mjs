// Teste de ponta a ponta do "Entrar com Google" no site: o programa da Vercel
// (cmd/api -tags sitebundle) com as rotas /oauth, um Google falso (login,
// troca de token, revogação) e um Google Drive falso em memória. Duas abas
// independentes com a mesma conta: o que uma lança, a outra recebe pelo Drive.
// Uso (na pasta e2e): SITE_BIN=../dist/finansite node google.test.mjs
import { chromium, devices } from "playwright";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer } from "node:http";

const BIN = process.env.SITE_BIN || "../dist/finansite";
const PORT = 18095, TOKEN_PORT = 18096;
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
const b64url = (buf) => buf.toString("base64").replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

// ---- Google falso: troca de token (o intermediário chama aqui) ----
const challenges = new Map(); // state -> code_challenge (do endereço de login)
const exchanges = [];
const tokenServer = createServer((req, res) => {
  let body = "";
  req.on("data", (c) => (body += c));
  req.on("end", () => {
    const f = Object.fromEntries(new URLSearchParams(body));
    if (req.url === "/revoke") { drive.revoked.push(body); return res.end(); } // o intermediário revoga ao sair
    exchanges.push(f);
    const idToken = ["x", b64url(Buffer.from(JSON.stringify({ email: "pessoa@gmail.com" }))), "x"].join(".");
    res.writeHead(200, { "Content-Type": "application/json" });
    if (f.grant_type === "authorization_code") {
      const state = f.code.replace(/^code-/, "");
      const pkce = b64url(createHash("sha256").update(f.code_verifier).digest()) === challenges.get(state);
      res.end(JSON.stringify(pkce ? { access_token: "acesso-1", expires_in: 3600, refresh_token: "renova-1", id_token: idToken } : { error: "invalid_grant" }));
    } else {
      res.end(JSON.stringify({ access_token: "acesso-2", expires_in: 3600 }));
    }
  });
});
await new Promise((r) => tokenServer.listen(TOKEN_PORT, "127.0.0.1", r));

const server = spawn(BIN, [], {
  env: { ...process.env, PORT: String(PORT), GROQ_API_KEY: "teste", FINAN_PROXY_UPSTREAM: "http://127.0.0.1:9",
    GOOGLE_CLIENT_ID: "id-teste.apps.googleusercontent.com", GOOGLE_CLIENT_SECRET: "segredo-teste", GOOGLE_TOKEN_URL: `http://127.0.0.1:${TOKEN_PORT}/token`, GOOGLE_REVOKE_URL: `http://127.0.0.1:${TOKEN_PORT}/revoke` },
  stdio: "ignore",
});
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${SITE}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}

// ---- Drive falso (um arquivo por conta; aqui, uma conta só) ----
const drive = { id: null, version: 0, content: null, revoked: [], spaces: new Set(), down: false };
const cors = { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PATCH, OPTIONS" };
const json = (route, status, data) => route.fulfill({ status, headers: { ...cors, "Content-Type": "application/json" }, body: JSON.stringify(data) });
async function fakeGoogle(context) {
  await context.route("https://accounts.google.com/**", (route) => {
    const u = new URL(route.request().url());
    const state = u.searchParams.get("state");
    challenges.set(state, u.searchParams.get("code_challenge"));
    ok(u.searchParams.get("redirect_uri") === `${SITE}/oauth` && u.searchParams.get("code_challenge_method") === "S256" &&
      u.searchParams.get("scope").includes("drive.appdata") && u.searchParams.get("client_id") === "id-teste.apps.googleusercontent.com",
    "login vai ao Google com PKCE, o escopo do Drive e o retorno do site");
    return route.fulfill({ status: 302, headers: { Location: `${SITE}/oauth?code=code-${encodeURIComponent(state)}&state=${encodeURIComponent(state)}` } });
  });
  await context.route("https://oauth2.googleapis.com/revoke", async (route) => {
    drive.revoked.push(route.request().postData());
    return route.fulfill({ status: 200, headers: cors, body: "" });
  });
  await context.route("https://www.googleapis.com/**", async (route) => {
    const req = route.request();
    if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors });
    if (!/^Bearer acesso-\d$/.test(req.headers().authorization || "")) return json(route, 401, { error: "sem token" });
    if (drive.down) return json(route, 503, { error: "Drive fora do ar" });
    const u = new URL(req.url());
    if (req.method() === "GET" && u.pathname === "/drive/v3/files") {
      drive.spaces.add(u.searchParams.get("spaces"));
      return json(route, 200, { files: drive.id ? [{ id: drive.id, version: String(drive.version) }] : [] });
    }
    if (req.method() === "GET" && u.searchParams.get("alt") === "media") return json(route, 200, drive.content);
    if (req.method() === "POST" && u.pathname === "/upload/drive/v3/files") {
      const body = req.postData();
      const parts = body.split(/--finanrich\w+/).filter((p) => p.includes("{"));
      const meta = JSON.parse(parts[0].slice(parts[0].indexOf("{")));
      ok(meta.name === "finan_rich.json" && meta.parents[0] === "appDataFolder", "arquivo criado na pasta de dados do app");
      drive.id = "arquivo-1";
      drive.content = JSON.parse(parts[1].slice(parts[1].indexOf("{")));
      drive.version++;
      return json(route, 200, { id: drive.id, version: String(drive.version) });
    }
    if (req.method() === "PATCH" && u.pathname === `/upload/drive/v3/files/${drive.id}`) {
      drive.content = JSON.parse(req.postData());
      drive.version++;
      return json(route, 200, { id: drive.id, version: String(drive.version) });
    }
    return json(route, 404, { error: "rota do Drive falso não existe: " + u.pathname });
  });
}
const inDrive = (desc) => (drive.content?.items || []).find((it) => it.type === "transaction" && it.record.description === desc)?.record;
async function until(cond, msg, ms = 15000) {
  for (let t = 0; t < ms; t += 200) {
    if (cond()) return true;
    await new Promise((r) => setTimeout(r, 200));
  }
  ok(false, msg);
  return false;
}

async function login(page) {
  await page.goto(`${SITE}/?entrar=1`); // "Começar agora" da landing
  ok(!(await page.locator('[data-action="solo"]').count()), "no login do site não existe \"usar sem conta\"");
  await page.waitForSelector(".start-google:not([hidden]) [data-action=google]", { timeout: 20000 });
  await page.click("[data-action=google]");
  await page.waitForURL(`${SITE}/`, { timeout: 20000 });
  await page.waitForSelector(".cards, .welcome", { timeout: 20000 });
}
async function addTransaction(page, desc, amount) {
  await page.goto(`${SITE}/lancamentos`);
  await openTx(page);
  await openTx(page);
  await page.fill("input[name=description]", desc);
  await page.fill("input[name=amount]", amount);
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector(`td:has-text('${desc}')`);
}
async function syncNow(page) {
  await page.goto(`${SITE}/configuracoes`);
  await page.click("#google [data-action=google-sync]", { timeout: 20000 });
  await page.waitForSelector("#flash:has-text('Sincronizado com o Google Drive')", { timeout: 20000 });
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
const errors = [];
try {
  // Pessoa no celular (A): entra com Google, conta nova, começa do zero.
  const ctxA = await browser.newContext({ ...devices["Pixel 7"] });
  await fakeGoogle(ctxA);
  const a = await ctxA.newPage();
  a.on("pageerror", (e) => errors.push("A: " + e.message));
  a.on("dialog", (d) => d.accept());
  await login(a);
  ok(exchanges[0]?.client_secret === "segredo-teste" && exchanges[0]?.grant_type === "authorization_code", "o código é trocado pelo intermediário, com o secret só no servidor");
  ok(await a.evaluate(async () => (await import("/static/js/db.js")).loadLedger().then((l) => l.categories.length > 0)), "conta nova: categorias padrão criadas");
  await addTransaction(a, "Mercado do Google", "123,45");
  await until(() => inDrive("Mercado do Google")?.amountCents === 12345, "lançamento de A chegou ao Drive");
  ok(!!inDrive("Mercado do Google"), "lançamento de A gravado no arquivo do Drive");
  const meta = await a.evaluate(async () => (await import("/static/js/db.js")).getMeta("google", null));
  const dump = await a.evaluate(async () => JSON.stringify(await (await import("/static/js/db.js")).getMeta("google", null)) + document.cookie);
  const rt = (await ctxA.cookies()).find((c) => c.name === "__Host-finan_rt");
  ok(meta?.email === "pessoa@gmail.com" && !meta?.refresh && !dump.includes("renova-1"), "conta ligada mostra o e-mail; o token de renovação não fica no navegador ao alcance do JavaScript");
  ok(rt?.value === "renova-1" && rt.httpOnly && rt.secure && rt.sameSite === "Strict", `o token de renovação fica num cookie HttpOnly (${JSON.stringify(rt && { httpOnly: rt.httpOnly, secure: rt.secure, sameSite: rt.sameSite })})`);
  await a.goto(`${SITE}/configuracoes`);
  await a.waitForSelector("#google:has-text('pessoa@gmail.com')", { timeout: 20000 });
  ok(true, "Ajustes mostram a conta Google ligada");

  // Outro navegador (B), mesma conta: os dados vêm do Drive.
  const ctxB = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await fakeGoogle(ctxB);
  const b = await ctxB.newPage();
  b.on("pageerror", (e) => errors.push("B: " + e.message));
  b.on("dialog", (d) => d.accept());
  await login(b);
  await b.goto(`${SITE}/lancamentos`);
  await b.waitForSelector("td:has-text('Mercado do Google')", { timeout: 20000 });
  ok(true, "outro navegador com a mesma conta recebe os dados pelo Drive");
  const catsB = await b.evaluate(async () => (await import("/static/js/db.js")).loadLedger().then((l) => l.categories.filter((c) => !c.deleted).length));
  const catsA = drive.content.items.filter((it) => it.type === "category" && !it.record.deleted).length;
  ok(catsB === catsA, `categorias não duplicam no segundo aparelho (${catsB} = ${catsA})`);

  // B lança; A sincroniza e recebe. A apaga; B recebe o apagamento.
  await addTransaction(b, "Farmácia do B", "30");
  await until(() => !!inDrive("Farmácia do B"), "lançamento de B chegou ao Drive");
  await syncNow(a);
  await a.goto(`${SITE}/lancamentos`);
  await a.waitForSelector("td:has-text('Farmácia do B')", { timeout: 20000 });
  ok(true, "A recebe o lançamento de B");
  // No celular a lixeira fica na janela de edição: toca na linha e em Apagar.
  await a.click("tr:has-text('Farmácia do B') td.primary");
  await a.click("#tx-modal [data-action=delete-editing]");
  await a.waitForSelector("td:has-text('Farmácia do B')", { state: "detached" });
  await until(() => inDrive("Farmácia do B")?.deleted === true, "apagamento de A chegou ao Drive");
  await syncNow(b);
  await b.goto(`${SITE}/lancamentos`);
  await b.waitForSelector("td:has-text('Mercado do Google')");
  ok(await b.locator("td:has-text('Farmácia do B')").count() === 0, "o que A apagou some em B");

  // Logado: o ícone da pantera leva ao Painel, e endereço direto de dentro do app abre.
  await b.goto(`${SITE}/lancamentos`);
  await b.waitForSelector("td:has-text('Mercado do Google')", { timeout: 20000 });
  await b.click("nav.tabs .side-brand, header.top .brand >> visible=true");
  await b.waitForSelector(".dash-kpis", { timeout: 20000 });
  ok(new URL(b.url()).pathname === "/", "logado, o ícone da pantera volta ao Painel");

  // Sessão do site: parado por mais de 30 minutos, o site pede para entrar de novo (e nenhuma tela do app aparece).
  await b.evaluate(async () => (await import("/static/js/db.js")).setMeta("site-active", Date.now() - 31 * 60_000));
  await b.goto(`${SITE}/dividas`);
  await b.waitForSelector(".start .login-note:has-text('sem usar')", { timeout: 20000 });
  ok(await b.locator("nav.tabs").isHidden() && new URL(b.url()).search === "?entrar=1", "sessão parada vence: volta ao login com o aviso, sem menu");
  ok(await b.evaluate(async () => !!(await (await import("/static/js/db.js")).getMeta("google", null))), "a sessão vencida não apaga os dados nem a conta; só pede o login de novo");
  // Com o relógio de uso dentro do prazo, abrir o site entra direto e renova a contagem.
  await b.evaluate(async () => (await import("/static/js/db.js")).setMeta("site-active", Date.now() - 20 * 60_000));
  await b.goto(`${SITE}/dividas`);
  await b.waitForSelector("h1:has-text('Dívidas')", { timeout: 20000 });
  ok(await b.evaluate(async () => Date.now() - (await (await import("/static/js/db.js")).getMeta("site-active", 0)) < 60_000), "abrir o site dentro do prazo renova a contagem de uso");

  // Com a página aberta e parada, a sessão vence sozinha e o Painel some.
  await b.goto(`${SITE}/`);
  await b.waitForSelector(".dash-kpis", { timeout: 20000 });
  await b.evaluate(async () => (await import("/static/js/db.js")).setMeta("site-active", Date.now() - 31 * 60_000));
  await b.waitForSelector(".start .login-note:has-text('sem usar')", { timeout: 30000 });
  ok(await b.locator(".dash-kpis").count() === 0, "página aberta e parada: a sessão vence sozinha e o Painel some");
  await b.evaluate(async () => (await import("/static/js/db.js")).setMeta("site-active", Date.now()));
  await b.goto(`${SITE}/`);
  await b.waitForSelector(".dash-kpis", { timeout: 20000 });

  // Token de acesso vencido: renova pelo intermediário, sem pedir login de novo.
  await b.evaluate(async () => { (await import("/static/js/google.js")).expire(); });
  await syncNow(b);
  ok(exchanges.some((f) => f.grant_type === "refresh_token" && f.refresh_token === "renova-1"), "token vencido é renovado sozinho");

  // Sair no site (computador que pode ser de outra pessoa): os dados vão
  // para o Drive antes e saem do navegador. Sem conseguir salvar, não sai.
  drive.down = true;
  await addTransaction(b, "Último do B", "9");
  await b.goto(`${SITE}/configuracoes`);
  await b.click("#google [data-action=google-out]");
  await b.waitForSelector(".flash.err:has-text('Drive')", { timeout: 20000 });
  ok(await b.evaluate(async () => !!(await (await import("/static/js/db.js")).getMeta("google", null))) && !inDrive("Último do B"),
    "sem conseguir salvar no Drive, não sai nem apaga nada");
  drive.down = false;
  await b.click("#google [data-action=google-out]");
  await b.waitForSelector(".lp-hero h1", { timeout: 20000 });
  const after = await b.evaluate(async () => {
    const db = await import("/static/js/db.js");
    return { google: await db.getMeta("google", null), txs: (await db.loadLedger()).transactions.length };
  });
  ok(after.google === null && after.txs === 0 && drive.revoked.some((p) => p.includes("renova-1")) && !!inDrive("Último do B"),
    `sair salva no Drive, revoga o acesso e tira os dados do navegador (${after.txs} lançamentos ficaram)`);
  await b.reload();
  await b.waitForSelector(".lp-hero h1", { timeout: 20000 });
  ok(await b.locator("nav.tabs").isHidden(), "depois de sair, recarregar mostra a landing, não o Painel");
  await login(b);
  await b.goto(`${SITE}/lancamentos`);
  await b.waitForSelector("td:has-text('Último do B')", { timeout: 20000 });
  ok(true, "entrar de novo traz os dados de volta do Drive");
  ok(drive.spaces.size === 1 && drive.spaces.has("appDataFolder"), "o Drive só é consultado na pasta de dados do app");

  // A página /oauth só repassa para 127.0.0.1 (programa do computador) com porta numérica.
  await b.goto(`${SITE}/oauth?code=x&state=pc.evil.com.abc`);
  await b.waitForSelector("[data-oauth-status].neg", { timeout: 20000 });
  ok(/inválido/.test(await b.textContent("[data-oauth-status]")) && b.url().startsWith(SITE), "retorno para o computador com porta inválida é recusado");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.kill();
  tokenServer.close();
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
