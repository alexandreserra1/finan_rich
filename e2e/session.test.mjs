import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { chromium, webkit } from "playwright";

const PORT = 18100 + Number(process.env.PORT_OFFSET || 0);
const SITE = `http://127.0.0.1:${PORT}`;
const NOW = 1_800_000_000_000;
const engine = process.env.BROWSER === "webkit" ? webkit : chromium;
const server = createServer(async (req, res) => {
  if (req.url === "/") {
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
    return res.end('<!doctype html><html data-site="web" lang="pt-BR"><title>Teste de sessão</title></html>');
  }
  if (!/^\/static\/(js\/(db|db-idb|db-sqlite|sqlite-worker|sqlite-native|google|device)\.js|vendor\/sqlite\/sqlite3\.(js|wasm))$/.test(req.url)) {
    res.writeHead(404);
    return res.end();
  }
  try {
    const source = await readFile(new URL(`../internal/web${req.url}`, import.meta.url));
    res.writeHead(200, { "Content-Type": req.url.endsWith(".wasm") ? "application/wasm" : "text/javascript; charset=utf-8" });
    res.end(source);
  } catch {
    res.writeHead(500);
    res.end("Não foi possível ler o módulo do repositório");
  }
});

let browser;
let failures = 0;
await new Promise((resolve, reject) => {
  server.once("error", reject);
  server.listen(PORT, "127.0.0.1", resolve);
});

async function test(name, run) {
  const context = await browser.newContext();
  try {
    const page = await context.newPage();
    await page.clock.setFixedTime(NOW);
    await page.goto(SITE);
    await page.evaluate(async () => {
      const db = await import("/static/js/db.js");
      await db.setMeta("google", { email: "pessoa@gmail.com" });
      await db.setMeta("site-active", Date.now());
      await db.saveManyLocal([
        { type: "category", record: { id: "alimentacao", name: "Alimentação", updatedAt: 1 } },
        { type: "transaction", record: { id: "mercado", description: "Mercado", amountCents: 12345, date: "2026-11-05", updatedAt: 2 } },
        { type: "pref", record: { id: "tema", value: "claro", updatedAt: 3 } },
      ]);
      await db.setMeta("cursor", 42);
      await db.setMeta("drive", { id: "arquivo-teste", version: "7" });
      await db.setMeta("google", null); // sem sessão: cada teste monta a sua
      await db.setMeta("site-active", null);
    });
    await run(page, context);
    console.log(`OK    ${name}`);
  } catch (error) {
    failures++;
    console.error(`FALHA ${name}\n${error.stack}`);
  } finally {
    await context.close();
  }
}

async function touch(page, force, expected, expectedSession, expectedTimestamp) {
  const result = await page.evaluate(async ({ force }) => {
    const db = await import("/static/js/db.js");
    const google = await import("/static/js/google.js");
    const snapshot = async () => ({
      ledger: await db.loadLedger(), pending: await db.pending(),
      account: await db.getMeta("google", null), cursor: await db.getMeta("cursor", null),
      drive: await db.getMeta("drive", null),
    });
    const before = await snapshot();
    const timestamp = await db.getMeta("site-active", "ausente");
    const session = await google.session();
    const afterQuery = await db.getMeta("site-active", "ausente");
    const accepted = force ? await google.markActive(true) : await google.markActive();
    const afterTouch = await db.getMeta("site-active", "ausente");
    return {
      before, after: await snapshot(), session, accepted, timestamp, afterQuery, afterTouch,
    };
  }, { force });
  assert.deepEqual(result.after, result.before, "sessão não altera conta, lançamentos, fila nem metadados da sincronização");
  assert.deepEqual(result.session, expectedSession, "consulta reconhece o estado atual da sessão");
  assert.deepEqual(result.afterQuery, result.timestamp, "consulta não renova a sessão");
  assert.equal(result.accepted, expected, "markActive retorna um booleano de validade");
  if (expectedTimestamp === undefined) {
    assert.deepEqual(result.afterTouch, result.timestamp, "atividade recusada ou limitada não grava timestamp");
  } else {
    assert.equal(result.afterTouch, expectedTimestamp, "atividade válida renova o timestamp");
  }
}

try {
  browser = await engine.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
  await test("troca de código suspensa não permite renovação concorrente", async (page) => {
    const result = await page.evaluate(async () => {
      const db = await import("/static/js/db.js");
      const google = await import("/static/js/google.js");
      await db.setMeta("google", { email: "antiga@exemplo.test" });
      await db.setMeta("google-pending", { state: "web.teste", verifier: "falso", at: Date.now() });
      let releaseLogin, started, renewals = 0;
      const ready = new Promise((resolve) => { started = resolve; });
      const response = new Promise((resolve) => { releaseLogin = resolve; });
      window.fetch = async (url) => {
        if (url.endsWith("/oauth/token")) { started(); return response; }
        renewals++;
        return new Response("{}", { status: 401 });
      };
      const login = google.finish("codigo-falso", "web.teste").then(() => "ok", (error) => error.kind);
      await ready;
      await google.token().catch(() => {});
      releaseLogin(new Response(JSON.stringify({ access_token: "acesso-novo-falso", cookie: true })));
      return { login: await login, renewals, account: !!(await db.getMeta("google", null)), token: await google.token().catch((error) => error.kind) };
    });
    assert.deepEqual(result, { login: "ok", renewals: 0, account: true, token: "acesso-novo-falso" });
  });
  for (const action of ["login", "logout"]) {
    for (const responseStatus of [200, 400, 401]) {
      await test(`renovação atrasada ${responseStatus} não altera ${action} novo`, async (page) => {
        const result = await page.evaluate(async ({ action, responseStatus }) => {
          const db = await import("/static/js/db.js");
          const google = await import("/static/js/google.js");
          await db.setMeta("google", { email: "antiga@exemplo.test" });
          await db.setMeta("google-pending", { state: "web.teste", verifier: "verificador-falso", at: Date.now() });
          let releaseRefresh, started;
          const ready = new Promise((resolve) => { started = resolve; });
          const response = new Promise((resolve) => { releaseRefresh = resolve; });
          const requests = [];
          window.fetch = async (url) => {
            requests.push(url);
            if (url.endsWith("/oauth/refresh")) { started(); return response; }
            return new Response(JSON.stringify({ access_token: "acesso-novo-falso", cookie: true }), { status: 200 });
          };
          const old = google.token().then((token) => ({ token }), (error) => ({ kind: error.kind }));
          await ready;
          if (action === "login") await google.finish("codigo-falso", "web.teste");
          else await google.signOut();
          await db.setMeta("drive", { version: "nova" });
          releaseRefresh(new Response(JSON.stringify({ access_token: "acesso-antigo-falso" }), { status: responseStatus }));
          const stale = await old;
          const before = requests.length;
          const token = await google.token().catch((error) => error.kind);
          return { stale, token, account: !!(await db.getMeta("google", null)), drive: await db.getMeta("drive", null), requestsAfter: requests.length - before };
        }, { action, responseStatus });
        assert.deepEqual(result.stale, { kind: "google" }, "resultado antigo deve ser recusado");
        assert.equal(result.account, action === "login", "resultado antigo não muda a conta atual");
        assert.deepEqual(result.drive, { version: "nova" }, "resultado antigo não apaga metadados novos");
        assert.equal(result.token, action === "login" ? "acesso-novo-falso" : "google", "logout não ressuscita token antigo");
        assert.equal(result.requestsAfter, 0, "consulta seguinte não usa renovação para esconder token perdido");
      });
    }
  }

  for (const syncStatus of ["error", "offline", "google", "ok"]) {
    await test(`callback OAuth com sync ${syncStatus} só inicializa após sucesso`, async (page) => {
      const source = await readFile(new URL("../internal/web/static/js/pages/oauth.js", import.meta.url), "utf8");
      const result = await page.evaluate(async ({ source, syncStatus }) => {
        const calls = [];
        window.__oauthTest = {
          loadLedger: async () => { calls.push("dados"); return { categories: [] }; },
          google: { finish: async () => { calls.push("login"); } },
          sync: { state: { status: syncStatus, message: "Falha falsa de sincronização" }, sync: async () => { calls.push("sync"); }, soon: () => { calls.push("agendar"); } },
          html: () => "",
          startSolo: async () => { calls.push("iniciar"); },
        };
        const moduleSource = source.replace(/^import .*;\n/gm, "");
        const url = URL.createObjectURL(new Blob(["const { loadLedger, google, sync, html, startSolo } = window.__oauthTest;\n", moduleSource], { type: "text/javascript" }));
        try {
          const oauth = await import(url);
          document.body.innerHTML = '<p data-oauth-status></p>';
          await oauth.bind(document.body, { params: new URLSearchParams({ state: "web.teste", code: "codigo-falso" }), flash: () => { calls.push("sucesso"); } });
          const first = { calls: [...calls], error: document.querySelector("[data-oauth-status]").classList.contains("neg"), text: document.querySelector("[data-oauth-status]").textContent };
          if (syncStatus !== "ok") {
            window.__oauthTest.sync.state.status = "ok";
            document.querySelector("button").click();
            for (let attempt = 0; attempt < 20 && !calls.includes("sucesso"); attempt++) await new Promise((resolve) => setTimeout(resolve, 0));
          }
          return { ...first, recovered: calls, retryRemoved: !document.querySelector("button") };
        } finally {
          URL.revokeObjectURL(url);
          delete window.__oauthTest;
        }
      }, { source, syncStatus });
      assert.deepEqual(result.calls, syncStatus === "ok" ? ["login", "sync", "dados", "iniciar", "agendar", "sucesso"] : ["login", "sync"]);
      assert.equal(result.error, syncStatus !== "ok");
      if (syncStatus !== "ok") assert.match(result.text, /Falha falsa/);
      assert.deepEqual(result.recovered, syncStatus === "ok" ? result.calls : ["login", "sync", "sync", "dados", "iniciar", "agendar", "sucesso"], "retry não troca o código de login outra vez");
      assert.equal(result.retryRemoved, true);
    });
  }

  await test("retomada remonta após falha e compartilha conclusão concorrente", async (page) => {
    const source = await readFile(new URL("../internal/web/static/js/pages/oauth.js", import.meta.url), "utf8");
    const result = await page.evaluate(async (source) => {
      const calls = [];
      window.__oauthTest = {
        loadLedger: async () => ({ categories: [{}] }),
        google: { finish: async () => { calls.push("login"); }, session: async () => ({ signedIn: true }) },
        sync: { state: { status: "error" }, sync: async () => { calls.push("sync"); }, soon: () => {} },
        html: () => "", startSolo: async () => { calls.push("iniciar"); },
      };
      const url = URL.createObjectURL(new Blob(["const { loadLedger, google, sync, html, startSolo } = window.__oauthTest;\n", source.replace(/^import .*;\n/gm, "")], { type: "text/javascript" }));
      try {
        const oauth = await import(url);
        const mount = () => { document.body.innerHTML = '<p data-oauth-status></p>'; return document.body; };
        const ctx = { params: new URLSearchParams({ state: "web.teste", code: "falso" }), refresh: async () => { calls.push("abrir"); }, flash: () => { calls.push("sucesso"); } };
        await oauth.bind(mount(), ctx);
        window.__oauthTest.sync.state.status = "ok";
        ctx.params = new URLSearchParams({ retomar: "1" });
        await Promise.all([oauth.bind(mount(), ctx), oauth.bind(mount(), ctx)]);
        return calls;
      } finally {
        URL.revokeObjectURL(url);
        delete window.__oauthTest;
      }
    }, source);
    assert.deepEqual(result, ["login", "sync", "sync", "abrir", "sucesso"]);
  });

  await test("logout pendente bloqueia token em cache e não apaga login posterior", async (page) => {
    const result = await page.evaluate(async () => {
      const db = await import("/static/js/db.js");
      const google = await import("/static/js/google.js");
      let releaseLogout, started;
      const ready = new Promise((resolve) => { started = resolve; });
      const response = new Promise((resolve) => { releaseLogout = resolve; });
      window.fetch = async (url) => {
        if (url.endsWith("/oauth/logout")) { started(); return response; }
        return new Response(JSON.stringify({ access_token: "acesso-novo-falso", cookie: true }));
      };
      const finish = async (state) => {
        await db.setMeta("google-pending", { state, verifier: "falso", at: Date.now() });
        await google.finish("codigo-falso", state);
      };
      await finish("web.primeiro");
      const logout = google.signOut().catch((error) => error.kind);
      await ready;
      const duringLogout = await google.token().catch((error) => error.kind);
      await finish("web.segundo");
      releaseLogout(new Response("{}"));
      await logout;
      return { duringLogout, account: !!(await db.getMeta("google", null)), token: await google.token() };
    });
    assert.deepEqual(result, { duringLogout: "google", account: true, token: "acesso-novo-falso" });
  });

  await test("resposta de login anterior ao logout não reabre a sessão", async (page) => {
    const result = await page.evaluate(async () => {
      const db = await import("/static/js/db.js");
      const google = await import("/static/js/google.js");
      await db.setMeta("google-pending", { state: "web.teste", verifier: "falso", at: Date.now() });
      let releaseLogin, started;
      const ready = new Promise((resolve) => { started = resolve; });
      const response = new Promise((resolve) => { releaseLogin = resolve; });
      window.fetch = async (url) => {
        if (url.endsWith("/oauth/token")) { started(); return response; }
        return new Response("{}");
      };
      const login = google.finish("codigo-falso", "web.teste").catch((error) => error.kind);
      await ready;
      await google.signOut();
      releaseLogin(new Response(JSON.stringify({ access_token: "acesso-antigo-falso", cookie: true })));
      return { login: await login, account: await db.getMeta("google", null), token: await google.token().catch((error) => error.kind) };
    });
    assert.deepEqual(result, { login: "google", account: null, token: "google" });
  });

  for (const force of [false, true]) {
    for (const kind of ["vencida", "limite exato", "ausente", "string", "NaN", "Infinity", "futuro", "fracionário", "inteiro inseguro", "zero", "negativo", "objeto"]) {
      await test(`${kind}: markActive(${force}) recusa sem gravar`, async (page) => {
        await page.evaluate(async ({ kind, now }) => {
          const db = await import("/static/js/db.js");
          const { IDLE_MS } = await import("/static/js/google.js");
          await db.setMeta("google", { email: "pessoa@gmail.com" });
          const values = {
            vencida: now - IDLE_MS - 1, "limite exato": now - IDLE_MS,
            string: String(now - 120_000), NaN, Infinity, futuro: now + 1,
            fracionário: now - 120_000 + 0.5, "inteiro inseguro": Number.MAX_SAFE_INTEGER + 1,
            zero: 0, negativo: -1, objeto: { at: now },
          };
          if (kind !== "ausente") await db.setMeta("site-active", values[kind]);
        }, { kind, now: NOW });
        await touch(page, force, false, { signedIn: false, expired: true });
      });
    }
    await test(`sem conta: markActive(${force}) não cria sessão`, async (page) => {
      await page.evaluate(async (now) => (await import("/static/js/db.js")).setMeta("site-active", now - 120_000), NOW);
      await touch(page, force, false, { signedIn: false, expired: false });
    });
  }

  for (const age of [60_000, 30 * 60_000 - 1]) {
    await test(`válida há ${age} ms: renova antes do vencimento`, async (page) => {
      await page.evaluate(async ({ now, age }) => {
        const db = await import("/static/js/db.js");
        await db.setMeta("google", { email: "pessoa@gmail.com" });
        await db.setMeta("site-active", now - age);
      }, { now: NOW, age });
      await touch(page, false, true, { signedIn: true, expired: false }, NOW);
    });
  }

  await test("válida: o limite de um minuto retorna true sem gravar; force renova", async (page) => {
    await page.evaluate(async (now) => {
      const db = await import("/static/js/db.js");
      await db.setMeta("google", { email: "pessoa@gmail.com" });
      await db.setMeta("site-active", now - 59_999);
    }, NOW);
    await touch(page, false, true, { signedIn: true, expired: false });
    await touch(page, true, true, { signedIn: true, expired: false }, NOW);
  });

  for (const force of [false, true]) {
    await test(`sessão antes válida vence com o tempo: markActive(${force}) não ressuscita`, async (page) => {
      await page.evaluate(async (now) => {
        const db = await import("/static/js/db.js");
        await db.setMeta("google", { email: "pessoa@gmail.com" });
        await db.setMeta("site-active", now - 120_000);
      }, NOW);
      await touch(page, false, true, { signedIn: true, expired: false }, NOW);
      await page.clock.setFixedTime(NOW + 30 * 60_000);
      await touch(page, force, false, { signedIn: false, expired: true });
    });
  }

  await test("relógio recuado exige revalidação síncrona antes de consultar o banco", async (page) => {
    await page.evaluate(async (now) => {
      const db = await import("/static/js/db.js");
      await db.setMeta("google", { email: "pessoa@gmail.com" });
      await db.setMeta("site-active", now);
    }, NOW);
    const confirmed = await page.evaluate(async () => {
      const google = await import("/static/js/google.js");
      return { session: await google.session(), needsCheck: google.needsSessionCheck() };
    });
    assert.deepEqual(confirmed, { session: { signedIn: true, expired: false }, needsCheck: false });
    await page.clock.setFixedTime(NOW - 1);
    const needsCheck = await page.evaluate(async () => (await import("/static/js/google.js")).needsSessionCheck());
    assert.equal(needsCheck, true, "recuo anterior à última atividade confirmada invalida o cache sem ler IndexedDB");
    await touch(page, false, false, { signedIn: false, expired: true });
    await touch(page, true, false, { signedIn: false, expired: true });
  });

  for (const change of ["vencer", "remover conta", "renovar"]) {
    for (const force of [false, true]) {
      await test(`outra aba pode ${change}: markActive(${force}) relê o banco ou a aba antiga para`, async (page, context) => {
        await page.evaluate(async (now) => {
          const db = await import("/static/js/db.js");
          await db.setMeta("google", { email: "pessoa@gmail.com" });
          await db.setMeta("site-active", now - 120_000);
          window.__elsewhere = false;
          addEventListener("finan-db-elsewhere", () => { window.__elsewhere = true; });
        }, NOW);
        await touch(page, false, true, { signedIn: true, expired: false }, NOW);
        const other = await context.newPage();
        await other.goto(SITE);
        await other.evaluate(async ({ change, now }) => {
          const db = await import("/static/js/db.js");
          const { IDLE_MS } = await import("/static/js/google.js");
          if (change === "remover conta") await db.setMeta("google", null);
          else await db.setMeta("site-active", change === "vencer" ? now - IDLE_MS : now - 120_000);
        }, { change, now: NOW });
        // No SQLite só uma aba usa o banco: a nova assumiu, e a antiga para (nem lê nem
        // renova), então não tem como agir com dado velho. No IndexedDB, as duas abas
        // dividem o banco, e a antiga precisa reler.
        if (await page.evaluate(() => window.__elsewhere)) {
          const state = await page.evaluate(async (force) => Promise.race([
            (await import("/static/js/google.js")).markActive(force).then(() => "respondeu"),
            new Promise((resolve) => setTimeout(() => resolve("parada"), 500)),
          ]), force);
          assert.equal(state, "parada", "a aba que perdeu o banco não lê nem renova a sessão");
          return;
        }
        const valid = change === "renovar";
        await touch(page, force, valid, { signedIn: valid, expired: change === "vencer" }, valid ? NOW : undefined);
      });
    }
  }

  for (const mode of ["site válido", "site vencido", "PC sem sessão"]) {
    for (const api of ["saveLocal", "saveManyLocal"]) {
      await test(`${mode}: ${api} grava tudo ou recusa sem alterar registros e fila`, async (page) => {
        await page.evaluate(async (mode) => {
          const db = await import("/static/js/db.js");
          if (mode === "PC sem sessão") delete document.documentElement.dataset.site;
          else {
            await db.setMeta("google", { email: "pessoa@gmail.com" });
            await db.setMeta("site-active", Date.now());
          }
        }, mode);
        if (mode === "site vencido") await page.clock.setFixedTime(NOW + 31 * 60_000);
        const result = await page.evaluate(async (api) => {
          const db = await import("/static/js/db.js");
          const snapshot = async () => ({
            records: await db.allRecords(), pending: await db.pending(),
            activity: await db.getMeta("site-active", null), account: await db.getMeta("google", null),
            cursor: await db.getMeta("cursor", null), drive: await db.getMeta("drive", null),
          });
          const before = await snapshot();
          const transaction = before.records.find((item) => item.type === "transaction").record;
          const pref = before.records.find((item) => item.type === "pref").record;
          const changes = [
            { type: "transaction", record: { ...transaction, amountCents: 99999, updatedAt: 4 } },
            { type: "pref", record: { ...pref, value: "escuro", updatedAt: 4 } },
          ];
          let error = null;
          try {
            if (api === "saveLocal") await db.saveLocal(changes[0].type, changes[0].record);
            else await db.saveManyLocal(changes);
          } catch (failure) {
            error = { kind: failure.kind, message: failure.message };
          }
          return { before, after: await snapshot(), error, changes: api === "saveLocal" ? changes.slice(0, 1) : changes };
        }, api);
        if (mode === "site vencido") {
          assert.equal(result.error?.kind, "login", "sessão vencida recusa gravação com kind login");
          assert.deepEqual(result.after, result.before, "lote recusado não altera nenhum registro, fila ou metadado");
        } else {
          assert.equal(result.error, null, "sessão válida ou modo PC permite gravação");
          const expected = structuredClone(result.before);
          for (const change of result.changes) {
            expected.records.find((item) => item.type === change.type && item.record.id === change.record.id).record = change.record;
            expected.pending.find((item) => item.type === change.type && item.record.id === change.record.id).record = change.record;
          }
          assert.deepEqual(result.after, expected, "grava todos os registros e a fila sem renovar sessão ou alterar outros dados");
        }
      });
    }
  }
} finally {
  await browser?.close();
  await new Promise((resolve) => server.close(resolve));
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exitCode = failures ? 1 : 0;
