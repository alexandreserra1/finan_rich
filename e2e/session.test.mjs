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
  if (!/^\/static\/js\/(db|google|device)\.js$/.test(req.url)) {
    res.writeHead(404);
    return res.end();
  }
  try {
    const source = await readFile(new URL(`../internal/web${req.url}`, import.meta.url));
    res.writeHead(200, { "Content-Type": "text/javascript; charset=utf-8" });
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
      await new Promise((resolve, reject) => {
        const request = indexedDB.open("finan_rich", 1);
        request.onerror = () => reject(request.error);
        request.onsuccess = () => {
          const database = request.result;
          const transaction = database.transaction("meta", "readwrite");
          transaction.objectStore("meta").delete("google");
          transaction.objectStore("meta").delete("site-active");
          transaction.oncomplete = () => { database.close(); resolve(); };
          transaction.onerror = transaction.onabort = () => { database.close(); reject(transaction.error); };
        };
      });
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
      await test(`outra aba pode ${change}: markActive(${force}) relê o IndexedDB`, async (page, context) => {
        await page.evaluate(async (now) => {
          const db = await import("/static/js/db.js");
          await db.setMeta("google", { email: "pessoa@gmail.com" });
          await db.setMeta("site-active", now - 120_000);
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
