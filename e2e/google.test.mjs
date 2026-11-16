// Teste de ponta a ponta do "Entrar com Google" no site: o programa da Vercel
// (cmd/api -tags sitebundle) com as rotas /oauth, um Google falso (login,
// troca de token, revogação) e um Google Drive falso em memória. Duas abas
// independentes com a mesma conta: o que uma lança, a outra recebe pelo Drive.
// Uso (na pasta e2e): SITE_BIN=../dist/finansite node google.test.mjs
import { chromium as chromiumEngine, devices, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { watchErrors } from "./errors.mjs";

const BIN = process.env.SITE_BIN || "../dist/finansite";
const OFF = Number(process.env.PORT_OFFSET || 0); // PORT_OFFSET=100: roda ao lado de outra cópia do repositório
const PORT = 18095 + OFF, TOKEN_PORT = 18096 + OFF;
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
let contexts = 0;
async function fakeGoogle(context) {
  await context.route(`${SITE}/static/js/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (!["/static/js/google.js", "/static/js/gdrive.js", "/static/js/sync.js", "/static/js/pages/oauth.js"].includes(path)) return route.continue();
    const body = await readFile(new URL(`../internal/web${path}`, import.meta.url), "utf8");
    return route.fulfill({ status: 200, contentType: "text/javascript; charset=utf-8", body });
  });
  // Cada navegador sai de um IP seu (na Vercel, o X-Forwarded-For): os limites das rotas
  // /oauth do intermediário (60 por hora e rajada de 10 pedidos por IP) valem por pessoa, não
  // para o teste todo. Um cenário com login, renovação e saída em sequência passa da rajada
  // de uma pessoa só; por isso o IP muda a cada 8 pedidos. O limite tem teste próprio em Go
  // (internal/proxy/throttle_test.go).
  const base = ++contexts;
  let calls = 0;
  await context.route(`${SITE}/oauth/*`, (route) => route.continue({ headers: { ...route.request().headers(), "x-forwarded-for": `203.0.${base}.${1 + Math.floor(calls++ / 8)}` } }));
  await context.route("https://accounts.google.com/**", (route) => {
    const u = new URL(route.request().url());
    const state = u.searchParams.get("state");
    challenges.set(state, u.searchParams.get("code_challenge"));
    ok(u.searchParams.get("redirect_uri") === `${SITE}/oauth` && u.searchParams.get("code_challenge_method") === "S256" &&
      u.searchParams.get("scope").includes("drive.appdata") && u.searchParams.get("client_id") === "id-teste.apps.googleusercontent.com",
    "login vai ao Google com PKCE, o escopo do Drive e o retorno do site");
    const callback = `${SITE}/oauth?code=code-${encodeURIComponent(state)}&state=${encodeURIComponent(state)}`;
    return route.fulfill({ status: 200, contentType: "text/html", body: `<script>location.replace(${JSON.stringify(callback)})</script>` });
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
      return json(route, 200, { id: drive.id, version: String(drive.version), modifiedTime: new Date().toISOString() });
    }
    if (req.method() === "PATCH" && u.pathname === `/upload/drive/v3/files/${drive.id}`) {
      drive.content = JSON.parse(req.postData());
      drive.version++;
      return json(route, 200, { id: drive.id, version: String(drive.version), modifiedTime: new Date().toISOString() });
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
  try {
    await page.waitForURL(`${SITE}/`, { timeout: 20000 });
  } catch (error) {
    await logStatus(page, "retorno OAuth");
    throw error;
  }
  await page.waitForSelector(".dash-kpis, .welcome", { timeout: 20000 });
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
  let stage = "abrir ajustes";
  try {
    await page.goto(`${SITE}/configuracoes`);
    stage = "clicar em sincronizar";
    await page.click("#google [data-action=google-sync]", { timeout: 20000 });
    stage = "esperar confirmação";
    await page.waitForSelector("#flash:has-text('Sincronizado com o Google Drive')", { timeout: 20000 });
  } catch (error) {
    await logStatus(page, stage);
    throw error;
  }
}
async function logStatus(page, stage) {
  const status = await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const sync = await import("/static/js/sync.js");
    const activity = await db.getMeta("site-active", null);
    const pending = await db.getMeta("google-pending", null);
    const session = await db.siteSession();
    const flash = document.querySelector("#flash");
    return {
      pathname: location.pathname, now: Date.now(),
      siteActive: Number.isSafeInteger(activity) ? activity : { type: typeof activity },
      session: { signedIn: session.signedIn, expired: session.expired },
      flash: flash ? { class: flash.className, hidden: flash.hidden } : null,
      sync: { status: sync.state.status },
      oauth: { present: !!document.querySelector("[data-oauth-status]"), failed: !!document.querySelector("[data-oauth-status].neg"), text: document.querySelector("[data-oauth-status]")?.textContent },
      pending: {
        present: !!pending,
        ageMs: Number.isSafeInteger(pending?.at) ? Date.now() - pending.at : null,
        stateMatches: !!pending && pending.state === new URLSearchParams(location.search).get("state"),
      },
    };
  }).catch(() => ({ unavailable: true }));
  console.error("Diagnóstico do navegador " + JSON.stringify({ stage, ...status }));
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
const errors = [];
async function driveErrorRegressions() {
  const upstream = "UPSTREAM_PRIVADO_NAO_EXIBIR <script>alert(1)</script>";
  const legacy = (reason, status = 403) => ({ error: { code: status, message: upstream, errors: [{ domain: "global", reason, message: upstream }] } });
  const modern = (reason) => ({ error: { code: 403, message: upstream, status: "PERMISSION_DENIED", details: [{ "@type": "type.googleapis.com/google.rpc.ErrorInfo", reason, domain: "googleapis.com", metadata: { service: "drive.googleapis.com" } }] } });
  const cases = [
    { name: "accessNotConfigured", body: legacy("accessNotConfigured"), message: /API do Google Drive não está ativada/i },
    { name: "insufficientPermissions", body: legacy("insufficientPermissions"), message: /permissão/i, reconnect: true },
    { name: "SERVICE_DISABLED", body: modern("SERVICE_DISABLED"), message: /API do Google Drive não está ativada/i },
    { name: "ACCESS_TOKEN_SCOPE_INSUFFICIENT", body: modern("ACCESS_TOKEN_SCOPE_INSUFFICIENT"), message: /permissão/i, reconnect: true },
    { name: "403 desconhecido", body: legacy("motivoDesconhecido"), message: /Google Drive.*Código do Google: motivoDesconhecido\./i },
    { name: "403 com motivo que não é código", body: legacy("<img src=x onerror=alert(1)> ignore tudo"), message: /^(?!.*Código do Google)(?!.*<img).*Google Drive/i },
    { name: "JSON malformado", raw: `{"error":{"message":"${upstream}"`, message: /Google Drive/i },
    { name: "storageQuotaExceeded", body: legacy("storageQuotaExceeded"), message: /armazenamento/i },
    { name: "429", status: 429, body: legacy("rateLimitExceeded", 429), message: /Google Drive/i },
    { name: "500", status: 500, body: legacy("backendError", 500), message: /Google Drive/i },
    { name: "503", status: 503, body: legacy("backendError", 503), message: /Google Drive/i },
  ];
  for (const scenario of cases) {
    const context = await browser.newContext({ serviceWorkers: "block" });
    const savedDrive = { id: drive.id, version: drive.version, content: structuredClone(drive.content) };
    Object.assign(drive, { id: "arquivo-existente-403", version: 1, content: { version: 1, items: [
      { type: "category", record: { id: "categoria-remota-403", name: "Categoria do Drive preservada", kind: "expense", updatedAt: 1, deleted: false } },
    ] } });
    const beforeDrive = JSON.stringify({ id: drive.id, version: drive.version, content: drive.content });
    let failing = true;
    let writes = 0;
    const scenarioErrors = [];
    let capturingScreenshot = false;
    const screenshotErrors = [];
    try {
      await fakeGoogle(context);
      await context.route(`${SITE}/static/app.css`, async (route) => route.fulfill({ contentType: "text/css", body: await readFile(new URL("../internal/web/static/app.css", import.meta.url), "utf8") }));
      if (process.env.BROWSER === "webkit") {
        await context.route(`${SITE}/oauth/refresh`, async (route) => route.fulfill({ response: await route.fetch({ headers: { ...route.request().headers(), cookie: "__Host-finan_rt=renova-1" } }) }));
      }
      await context.route("https://www.googleapis.com/**", (route) => {
        const method = route.request().method();
        if (["POST", "PATCH", "DELETE"].includes(method)) writes++;
        if (!failing || method === "OPTIONS") return route.fallback();
        return route.fulfill({ status: scenario.status || 403, headers: { ...cors, "Content-Type": "application/json" }, body: scenario.raw ?? JSON.stringify(scenario.body) });
      });
      const page = await context.newPage();
      watchErrors(page, scenarioErrors);
      page.on("console", (message) => {
        if (capturingScreenshot && process.env.BROWSER === "webkit" && message.type() === "error" &&
          message.text() === "Refused to apply a stylesheet because its hash, its nonce, or 'unsafe-inline' does not appear in the style-src directive of the Content Security Policy.") {
          screenshotErrors.push(message.text());
          return;
        }
        if (message.type() === "error" && !/^Failed to load resource: the server responded with a status of (403|429|500|503)\b/.test(message.text())) scenarioErrors.push(message.text());
      });
      await page.goto(`${SITE}/?entrar=1`);
      await page.waitForSelector(".start-google:not([hidden]) [data-action=google]");
      await page.click("[data-action=google]");
      await page.waitForSelector("[data-oauth-status].neg");
      const message = await page.locator("[data-oauth-status]").textContent();
      ok(scenario.message.test(message), `${scenario.name}: OAuth explica o motivo da falha (${message})`);
      if (scenario.reconnect) ok(/entrar de novo/i.test(message), `${scenario.name}: orienta entrar de novo`);
      ok(!(await page.locator(".oauth-state").textContent()).includes("UPSTREAM_PRIVADO_NAO_EXIBIR") && await page.locator(".oauth-state script").count() === 0,
        `${scenario.name}: não mostra mensagem privada do Google nem injeta HTML`);
      const reconnect = page.locator("[data-oauth-reconnect]");
      ok(scenario.reconnect
        ? await reconnect.count() === 1 && await reconnect.isVisible() && await reconnect.getAttribute("href") === "/?entrar=1"
        : await reconnect.count() === 0, `${scenario.name}: link de reconexão existe somente para erro de escopo`);
      const retry = page.getByRole("button", { name: "Tentar sincronizar de novo" });
      ok(scenario.reconnect ? !(await retry.isVisible()) : await retry.isVisible(), `${scenario.name}: ação de recuperação corresponde ao erro`);
      if (["accessNotConfigured", "insufficientPermissions"].includes(scenario.name)) {
        for (const width of [390, 768, 1440, 1920]) for (const colorScheme of ["light", "dark"]) {
          await page.setViewportSize({ width, height: 900 });
          await page.emulateMedia({ colorScheme });
          await page.waitForFunction((theme) => document.documentElement.dataset.theme === theme, colorScheme);
          const layout = await page.locator(".oauth-state").evaluate(async (panel) => {
            await document.fonts.ready;
            await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
            await Promise.all(document.getAnimations().filter((animation) => Number.isFinite(animation.effect?.getComputedTiming().endTime))
              .map((animation) => animation.finished.catch(() => {})));
            const bounds = panel.getBoundingClientRect();
            const luminance = (color) => {
              const channels = color.match(/[\d.]+/g).slice(0, 3).map((value) => {
                const channel = Number(value) / 255;
                return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
              });
              return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
            };
            const controls = [...panel.querySelectorAll("button, a.btn")].filter((control) => !control.hidden).map((control) => {
              const rect = control.getBoundingClientRect();
              const style = getComputedStyle(control);
              const text = document.createRange();
              text.selectNodeContents(control);
              const textRect = text.getBoundingClientRect();
              const foreground = luminance(style.color);
              const background = luminance(style.backgroundColor);
              return {
                text: control.textContent, display: style.display, fontSize: parseFloat(style.fontSize),
                contrast: (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05),
                centered: Math.abs((textRect.top + textRect.bottom - rect.top - rect.bottom) / 2) <= 3,
                fits: rect.left >= bounds.left && rect.right <= bounds.right && rect.top >= 0 && rect.bottom <= innerHeight && control.scrollWidth <= control.clientWidth + 1,
                textFits: textRect.left >= rect.left && textRect.right <= rect.right && textRect.top >= rect.top && textRect.bottom <= rect.bottom,
              };
            });
            return {
              fits: bounds.left >= 0 && bounds.right <= innerWidth && bounds.width <= 640 && panel.scrollWidth <= panel.clientWidth + 1 && document.documentElement.scrollWidth <= innerWidth,
              controls,
            };
          });
          const label = `${process.env.BROWSER || "chromium"}, ${scenario.name}, ${width}, ${colorScheme}`;
          ok(layout.fits, `${label}: OAuth sem overflow`);
          ok(layout.controls.length === 1 && layout.controls.every((control) => control.fits && control.textFits && control.centered && control.fontSize >= 12 && control.contrast >= 4.5),
            `${label}: botão legível, centralizado e sem corte ${JSON.stringify(layout.controls)}`);
          const screenshotErrorsBefore = screenshotErrors.length;
          capturingScreenshot = true;
          try {
            await page.screenshot({ path: `/tmp/rich-drive403-${process.env.BROWSER || "chromium"}-${scenario.name}-${width}-${colorScheme}.png` });
            await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(resolve)));
          } finally {
            capturingScreenshot = false;
          }
          ok(screenshotErrors.length - screenshotErrorsBefore === (process.env.BROWSER === "webkit" ? 1 : 0),
            `${label}: captura registra só a violação CSP esperada da instrumentação`);
        }
      }
      const empty = await page.evaluate(async () => {
        const db = await import("/static/js/db.js");
        return { ledger: await db.loadLedger(), records: await db.allRecords(), queue: await db.pending() };
      });
      ok(empty.ledger.categories.length === 0 && empty.records.length === 0 && empty.queue.length === 0,
        `${scenario.name}: falha inicial não cria categorias, padrões ou alterações locais`);
      const before = await page.evaluate(async () => {
        const db = await import("/static/js/db.js");
        await db.saveLocal("category", { id: "categoria-local-403", name: "Categoria preservada", kind: "expense", updatedAt: Date.now(), deleted: false });
        await db.saveLocal("transaction", { id: "lancamento-local-403", description: "Lançamento preservado 403", categoryId: "categoria-local-403", kind: "expense", amountCents: 12345, date: "2026-11-16", updatedAt: Date.now(), deleted: false });
        return { account: await db.getMeta("google", null), records: await db.allRecords(), queue: await db.pending(), drive: await db.getMeta("drive", null) };
      });
      await page.reload();
      await page.waitForSelector("[data-oauth-status].neg");
      const failure = await page.evaluate(async () => {
        try {
          await (await import("/static/js/gdrive.js")).syncDrive(true);
          return null;
        } catch (error) {
          return { kind: error.kind, recovery: error.recovery, message: error.message };
        }
      });
      ok(failure?.kind === "error" && (failure.recovery === "google") === !!scenario.reconnect,
        `${scenario.name}: gdrive mantém kind:error e recovery:google somente para escopos`);
      ok(!!failure && scenario.message.test(failure.message) && !failure.message.includes("UPSTREAM_PRIVADO_NAO_EXIBIR"),
        `${scenario.name}: contrato do gdrive traz mensagem classificada e segura`);
      const after = await page.evaluate(async () => {
        const db = await import("/static/js/db.js");
        const sync = await import("/static/js/sync.js");
        await sync.sync();
        return { account: await db.getMeta("google", null), records: await db.allRecords(), queue: await db.pending(), drive: await db.getMeta("drive", null), state: { ...sync.state } };
      });
      ok(after.state.status === "error" && (after.state.recovery === "google") === !!scenario.reconnect,
        `${scenario.name}: sync.state propaga recovery sem tratar 403 como logout`);
      ok(scenario.message.test(after.state.message) && !after.state.message.includes("UPSTREAM_PRIVADO_NAO_EXIBIR"), `${scenario.name}: sync.state preserva mensagem segura`);
      const { state, ...data } = after;
      ok(before.account?.email === "pessoa@gmail.com" && before.queue.length === 2 && JSON.stringify(data) === JSON.stringify(before),
        `${scenario.name}: falha preserva conta, registros, fila e metadados do Drive`);
      ok(writes === 0 && beforeDrive === JSON.stringify({ id: drive.id, version: drive.version, content: drive.content }),
        `${scenario.name}: falha OAuth não cria nem substitui arquivo no Drive`);
      failing = false;
      const exchangesBeforeRecovery = exchanges.filter((exchange) => exchange.grant_type === "authorization_code").length;
      const challengesBeforeRecovery = challenges.size;
      if (scenario.reconnect) {
        await reconnect.click();
      } else await retry.click();
      await page.waitForSelector(".dash-kpis, .welcome", { timeout: 20000 });
      const recovered = await page.evaluate(async () => {
        const db = await import("/static/js/db.js");
        const sync = await import("/static/js/sync.js");
        await sync.sync();
        return { state: { ...sync.state }, account: await db.getMeta("google", null), ledger: await db.loadLedger() };
      });
      ok(recovered.state.status === "ok" && !recovered.state.recovery && await page.locator("[data-oauth-reconnect]").count() === 0,
        `${scenario.name}: resposta normal limpa erro e recovery`);
      ok(scenario.reconnect
        ? challenges.size === challengesBeforeRecovery + 1 && exchanges.filter((exchange) => exchange.grant_type === "authorization_code").length === exchangesBeforeRecovery + 1
        : exchanges.filter((exchange) => exchange.grant_type === "authorization_code").length === exchangesBeforeRecovery,
      `${scenario.name}: clique de reconexão inicia novo PKCE; retry comum não repete OAuth`);
      ok(JSON.stringify(recovered.account) === JSON.stringify(before.account) && recovered.ledger.transactions.some((record) => record.id === "lancamento-local-403") &&
        drive.content?.items.some((item) => item.record.id === "lancamento-local-403") && drive.content?.items.some((item) => item.record.id === "categoria-remota-403") &&
        new URL(page.url()).pathname === "/",
      `${scenario.name}: recuperação mantém conta, envia alteração preservada e abre o Painel`);
      ok(scenarioErrors.length === 0, `${scenario.name}: sem erros de JavaScript ou console ${JSON.stringify(scenarioErrors)}`);
    } finally {
      await context.close();
      Object.assign(drive, savedDrive);
    }
  }
}
try {
  await driveErrorRegressions();
  if (!process.env.GOOGLE_DRIVE403_ONLY) {
  for (const description of ["The provided client secret is invalid.", "invalid_client"]) for (const width of [390, 1440]) for (const colorScheme of ["light", "dark"]) {
    const invalidContext = await browser.newContext({ viewport: { width, height: 900 }, colorScheme });
    await fakeGoogle(invalidContext);
    await invalidContext.route(`${SITE}/static/app.css`, async (route) => route.fulfill({ contentType: "text/css", body: await readFile(new URL("../internal/web/static/app.css", import.meta.url), "utf8") }));
    await invalidContext.route(`${SITE}/oauth/token`, (route) => json(route, 400, { error: "invalid_client", error_description: description }));
    const invalidPage = await invalidContext.newPage();
    watchErrors(invalidPage, errors, (e) => "Configuração inválida: " + e.message);
    await invalidPage.goto(`${SITE}/?entrar=1`);
    await invalidPage.waitForSelector(".start-google:not([hidden]) [data-action=google]");
    await invalidPage.click("[data-action=google]");
    await invalidPage.waitForSelector(".oauth-state [data-oauth-status].neg");
    ok(await invalidPage.locator("[data-oauth-title]").textContent() === "Não foi possível entrar", "falha de configuração muda o título do retorno OAuth");
    ok(await invalidPage.locator('[data-oauth-status][aria-live="polite"]').textContent().then((text) => text.includes("temporariamente indisponível") && text.includes("configuração")),
      "erro de cliente OAuth mostra aviso claro de configuração temporariamente indisponível");
    ok(await invalidPage.locator(".oauth-state details").textContent().then((text) => text.includes(description)) && await invalidPage.locator(".oauth-state details").getAttribute("open") === null,
      "detalhe técnico fica recolhido, sem substituir o aviso claro");
    ok(await invalidPage.locator('.oauth-state a[href="/?entrar=1"]').count() === 1 && await invalidPage.locator('.oauth-state a[href="/"]').count() === 1,
      "falha no login oferece tentar de novo e voltar");
    ok(await invalidPage.evaluate(async () => !(await (await import("/static/js/db.js")).getMeta("google", null))), "erro de configuração não cria conta local");
    const geometry = await invalidPage.locator(".oauth-state").boundingBox();
    ok(geometry && geometry.x >= 0 && geometry.width <= 640 && geometry.x + geometry.width <= width && await invalidPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      `invalid_client, ${width}, ${colorScheme}: estado centralizado cabe na tela sem overflow`);
    if (description !== "invalid_client") await invalidPage.screenshot({ path: `/tmp/finan-invalid-client-${width}-${colorScheme}.png` });
    await invalidContext.close();
  }

  const resumeContext = await browser.newContext();
  await fakeGoogle(resumeContext);
  const resumePage = await resumeContext.newPage();
  watchErrors(resumePage, errors, (e) => "Retomada: " + e.message);
  const exchangesWithoutSession = exchanges.length;
  await resumePage.goto(`${SITE}/oauth?retomar=1`);
  await resumePage.waitForSelector("[data-oauth-status].neg");
  ok(await resumePage.locator("[data-oauth-title]").textContent() === "Não foi possível entrar" && exchanges.length === exchangesWithoutSession,
    "retomada sem sessão válida pede novo login sem trocar código");
  drive.down = true;
  await resumePage.goto(`${SITE}/?entrar=1`);
  await resumePage.waitForSelector(".start-google:not([hidden]) [data-action=google]");
  await resumePage.click("[data-action=google]");
  await resumePage.waitForSelector("[data-oauth-status].neg");
  ok(new URL(resumePage.url()).pathname === "/oauth" && new URL(resumePage.url()).search === "?retomar=1", "falha inicial mantém URL segura para retomar sem código PKCE");
  const exchangesBeforeReload = exchanges.filter((exchange) => exchange.grant_type === "authorization_code").length;
  await resumePage.reload();
  await resumePage.waitForSelector("[data-oauth-status].neg", { timeout: 5000 });
  ok(await resumePage.evaluate(async () => !(await (await import("/static/js/db.js")).loadLedger()).categories.length), "recarga com Drive indisponível não inicializa conta nova");
  drive.down = false;
  await resumePage.getByRole("button", { name: "Tentar sincronizar de novo" }).click();
  await resumePage.waitForSelector(".dash-kpis, .welcome");
  ok(await resumePage.evaluate(async () => (await (await import("/static/js/db.js")).loadLedger()).categories.length > 0), "conta nova cria categorias depois da retomada com sync ok");
  ok(exchanges.filter((exchange) => exchange.grant_type === "authorization_code").length === exchangesBeforeReload && new URL(resumePage.url()).pathname === "/", "recarga retoma sessão e inicializa só após sync ok, sem repetir exchange");
  await resumeContext.close();

  // Pessoa no celular (A): entra com Google, conta nova, começa do zero.
  const ctxA = await browser.newContext({ ...devices["Pixel 7"] });
  await fakeGoogle(ctxA);
  const a = await ctxA.newPage();
  watchErrors(a, errors, (e) => "A: " + e.message);
  a.on("dialog", (d) => d.accept());
  await login(a);
  ok(exchanges[0]?.client_secret === "segredo-teste" && exchanges[0]?.grant_type === "authorization_code", "o código é trocado pelo intermediário, com o secret só no servidor");
  ok(await a.evaluate(async () => (await import("/static/js/db.js")).loadLedger().then((l) => l.categories.length > 0)), "conta nova: categorias padrão criadas");
  await addTransaction(a, "Mercado do Google", "123,45");
  await until(() => inDrive("Mercado do Google")?.amountCents === 12345, "lançamento de A chegou ao Drive");
  ok(!!inDrive("Mercado do Google"), "lançamento de A gravado no arquivo do Drive");
  for (const [screen, colorScheme] of [["computador", "light"], ["computador", "dark"], ["celular", "light"], ["celular", "dark"]]) {
    const retryContext = await browser.newContext({ ...(screen === "celular" ? devices["Pixel 7"] : { viewport: { width: 1280, height: 900 } }), colorScheme });
    await fakeGoogle(retryContext);
    const retryPage = await retryContext.newPage();
    watchErrors(retryPage, errors, (e) => "Nova tentativa: " + e.message);
    drive.down = true;
    await retryPage.goto(`${SITE}/?entrar=1`);
    await retryPage.waitForSelector(".start-google:not([hidden]) [data-action=google]");
    await retryPage.click("[data-action=google]");
    await retryPage.waitForSelector("[data-oauth-status].neg");
    ok(await retryPage.locator("[data-oauth-title]").textContent() === "Não foi possível sincronizar", "falha no Drive não mantém título de carregamento");
    ok(await retryPage.evaluate(async () => !(await (await import("/static/js/db.js")).loadLedger()).categories.length),
      "Drive fora do ar no callback não cria categorias locais");
    ok(!(await retryPage.locator("#flash:has-text('Conectado ao Google')").count()), "callback com falha não anuncia sucesso");
    const retryButton = retryPage.getByRole("button", { name: "Tentar sincronizar de novo" });
    ok(await retryButton.isVisible(), `${screen}, ${colorScheme}: botão de nova tentativa visível`);
    const retryBounds = await retryButton.boundingBox();
    const viewport = retryPage.viewportSize();
    ok(retryBounds && retryBounds.x >= 0 && retryBounds.y >= 0 && retryBounds.x + retryBounds.width <= viewport.width && retryBounds.y + retryBounds.height <= viewport.height,
      `${screen}, ${colorScheme}: botão de nova tentativa cabe na tela`);
    await retryPage.screenshot({ path: `/tmp/finan-oauth-${screen}-${colorScheme}.png` });
    const exchangesBeforeRetry = exchanges.length;
    drive.down = false;
    if (screen === "computador" && colorScheme === "light") {
      await retryPage.evaluate(async () => (await import("/static/js/sync.js")).sync());
    } else await retryButton.click();
    await retryPage.waitForSelector(".dash-kpis");
    ok(exchanges.length === exchangesBeforeRetry, "retry sincroniza sem trocar o código OAuth novamente");
    ok(await retryPage.evaluate(async () => (await (await import("/static/js/db.js")).loadLedger()).transactions.some((item) => item.description === "Mercado do Google")),
      "retry traz os dados existentes no Drive");
    await retryContext.close();
  }
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
  watchErrors(b, errors, (e) => "B: " + e.message);
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
  await b.waitForSelector("h1:has-text('Uma dívida de cada vez')", { timeout: 20000 });
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

  for (const interaction of ["clique", "tecla", "outra aba"]) {
    const sessionCtx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    try {
      await fakeGoogle(sessionCtx);
      const sessionPage = await sessionCtx.newPage();
      watchErrors(sessionPage, errors, (e) => `sessão (${interaction}): ${e.message}`);
      await sessionPage.clock.install();
      await login(sessionPage);
      await syncNow(sessionPage);
      await sessionPage.goto(`${SITE}/`);
      await sessionPage.waitForSelector(".dash-kpis", { timeout: 20000 });
      const now = await sessionPage.evaluate(() => Date.now());
      await sessionPage.clock.pauseAt(now + 1000);
      const beforeInteraction = await sessionPage.evaluate(async () => {
        const db = await import("/static/js/db.js");
        return { timestamp: await db.getMeta("site-active", null), account: await db.getMeta("google", null), ledger: await db.loadLedger() };
      });
      await sessionPage.clock.setSystemTime(now + 31 * 60_000);
      ok(await sessionPage.locator(".dash-kpis").isVisible(), `${interaction}: o Painel ainda está aberto antes da interação e do intervalo`);
      if (interaction === "outra aba") {
        const exchangesBeforeInteraction = exchanges.length;
        await sessionPage.evaluate(async () => {
          const db = await import("/static/js/db.js");
          await db.setMeta("site-active", Date.now());
          window.sessionBubbleClicks = 0;
          document.addEventListener("click", (event) => {
            if (event.target.closest('nav.tabs a[href="/lancamentos"]')) window.sessionBubbleClicks++;
          });
        });
        ok(await sessionPage.evaluate(async () => (await import("/static/js/google.js")).needsSessionCheck()),
          "outra aba renovou o banco, mas o cache local ainda precisa de revalidação");
        const movements = sessionPage.locator('nav.tabs a[href="/lancamentos"]');
        await movements.click();
        let checked = false;
        for (let attempt = 0; attempt < 30; attempt++) {
          checked = await sessionPage.evaluate(async () => !(await import("/static/js/google.js")).needsSessionCheck());
          if (checked) break;
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
        ok(checked, "primeiro clique revalida a sessão renovada no banco sem disparar o intervalo");
        ok(await sessionPage.evaluate(() => window.sessionBubbleClicks) === 0 &&
          new URL(sessionPage.url()).pathname === "/" && await sessionPage.locator(".dash-kpis").isVisible(),
        "primeiro clique fica bloqueado: não chega ao bubble nem é repetido após revalidar");
        ok(await sessionPage.locator(".start").count() === 0 && exchanges.length === exchangesBeforeInteraction,
          "sessão renovada em outra aba não pede novo login OAuth");
        await movements.click();
        await sessionPage.waitForSelector("td:has-text('Mercado do Google')", { timeout: 20000 });
        ok(new URL(sessionPage.url()).pathname === "/lancamentos" &&
          await sessionPage.evaluate(() => window.sessionBubbleClicks) === 1,
        "próximo clique chega ao bubble uma vez e abre os movimentos");
        continue;
      }
      if (interaction === "clique") await sessionPage.locator(".dash-kpis").click();
      else await sessionPage.keyboard.press("Shift");
      const expiredOnInteraction = await sessionPage.locator(".start .login-note:has-text('sem usar')")
        .waitFor({ timeout: 3000 }).then(() => true, () => false);
      ok(expiredOnInteraction && await sessionPage.locator("nav.tabs").isHidden() &&
        await sessionPage.locator(".dash-kpis").count() === 0 && new URL(sessionPage.url()).search === "?entrar=1",
      `${interaction} após vencer: pede login e esconde o Painel antes do intervalo de 15 segundos`);
      const afterInteraction = await sessionPage.evaluate(async () => {
        const db = await import("/static/js/db.js");
        return { timestamp: await db.getMeta("site-active", null), account: await db.getMeta("google", null), ledger: await db.loadLedger() };
      });
      ok(JSON.stringify(afterInteraction) === JSON.stringify(beforeInteraction),
        `${interaction} após vencer: não ressuscita a sessão nem altera conta e dados financeiros`);
      const exchangesBeforeLogin = exchanges.filter((entry) => entry.grant_type === "authorization_code").length;
      await login(sessionPage);
      const newSession = await sessionPage.evaluate(async () => {
        const db = await import("/static/js/db.js");
        const google = await import("/static/js/google.js");
        return { timestamp: await db.getMeta("site-active", null), now: Date.now(), session: await google.session() };
      });
      ok(exchanges.filter((entry) => entry.grant_type === "authorization_code").length > exchangesBeforeLogin &&
        newSession.timestamp === newSession.now && newSession.session.signedIn && !newSession.session.expired,
      `${interaction}: somente um novo login OAuth cria outra sessão válida`);
    } finally {
      await sessionCtx.close();
    }
  }

  // Navegador com o relógio 1 hora atrasado: ao gravar no Drive, ele aprende a
  // hora do Google e as edições saem com a hora certa (docs/passo-a-passo.md 1.3).
  const lateCtx = await browser.newContext({ viewport: { width: 1000, height: 800 } });
  await fakeGoogle(lateCtx);
  const late = await lateCtx.newPage();
  await late.clock.setFixedTime(new Date(Date.now() - 3600_000));
  late.on("dialog", (d) => d.accept());
  await login(late);
  await addTransaction(late, "Feira do relógio atrasado", "5");
  await until(() => !!inDrive("Feira do relógio atrasado"), "lançamento do navegador atrasado chegou ao Drive");
  await syncNow(late);
  const [stamped, lateNow] = await late.evaluate(async () => [(await import("/static/js/sync.js")).stamp(0), Date.now()]);
  ok(Math.abs(stamped - Date.now()) < 30_000 && Date.now() - lateNow > 3_000_000,
    `relógio atrasado no site: o carimbo segue a hora do Drive (diferença ${Math.round((Date.now() - stamped) / 1000)} s)`);
  await lateCtx.close();

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

  }
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
