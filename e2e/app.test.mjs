// Teste de ponta a ponta do app do celular (Capacitor) no navegador.
//
// As telas do app (mobile/www, geradas por "go run ./tools/mobilebuild") são
// servidas num endereço próprio, como no celular, onde elas vêm de dentro do
// app e o computador fica em outro endereço. Cobre: pareamento por código,
// sincronização com token, uso offline e aparelho desconectado no computador.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich APP_WWW=../mobile/www node app.test.mjs
import { chromium, devices } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { extname, join, normalize } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const WWW = process.env.APP_WWW || "../mobile/www";
const PORT = 18098;
const APP_PORT = 18097;
const OLLAMA_PORT = 18083;
const B = `http://127.0.0.1:${PORT}`;
const APP = `http://127.0.0.1:${APP_PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-app-"));
let server = null;
let failures = 0;

const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitHealthy() {
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(`${B}/healthz`)).ok) return; } catch {}
    await sleep(100);
  }
  throw new Error("o servidor não subiu");
}
async function startServer() {
  server = spawn(BIN, ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], {
    stdio: "ignore",
    env: { ...process.env, FINAN_AI_PRESET: "ollama", FINAN_AI_MODEL: "qwen3.5:9b", FINAN_AI_BASE_URL: `http://127.0.0.1:${OLLAMA_PORT}` },
  });
  await waitHealthy();
}
async function stopServer() {
  if (!server) return;
  const exited = new Promise((r) => server.once("exit", r));
  server.kill();
  await exited;
  server = null;
}
// "computador": chamadas feitas deste computador (sem login, como o navegador local).
const form = (path, data) => fetch(B + path, {
  method: "POST", redirect: "manual",
  headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: new URLSearchParams(data),
});
async function serverRecords() {
  const res = await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' });
  return (await res.json()).changes.map((c) => c.record);
}
async function newPairingCode() {
  await form("/configuracoes/parear", {});
  const page = await (await fetch(`${B}/configuracoes`)).text();
  return page.match(/class="pair-code">([A-Z0-9-]+)</)?.[1];
}

// Servidor das telas do app: arquivos de mobile/www e, para caminhos sem
// extensão, o index.html (como o Capacitor faz).
const TYPES = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".wasm": "application/wasm", ".png": "image/png", ".svg": "image/svg+xml" };
const appServer = createServer((req, res) => {
  let path = normalize(decodeURIComponent(new URL(req.url, APP).pathname)).replace(/^(\.\.[/\\])+/, "");
  if (!extname(path)) path = "/index.html";
  try {
    const file = join(WWW, path);
    statSync(file);
    res.writeHead(200, { "Content-Type": TYPES[extname(file)] || "application/octet-stream" });
    res.end(readFileSync(file));
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise((r) => appServer.listen(APP_PORT, "127.0.0.1", r));

// Ollama falso: responde como o modelo, para testar o consultor no app.
const ollama = createServer((req, res) => {
  if (req.url === "/api/tags") return res.end(JSON.stringify({ models: [{ name: "qwen3.5:9b" }] }));
  if (req.url === "/api/chat") {
    res.write(JSON.stringify({ message: { content: "Você gastou **R$ 250,00** com mercado." }, done: false }) + "\n");
    return res.end(JSON.stringify({ message: { content: "" }, done: true }) + "\n");
  }
  res.writeHead(404).end();
});
await new Promise((r) => ollama.listen(OLLAMA_PORT, "127.0.0.1", r));

const status = (page) => page.locator("#sync-status").getAttribute("data-status");
const waitStatus = (page, s, timeout = 15000) =>
  page.waitForFunction((s) => document.getElementById("sync-status")?.dataset.status === s, s, { timeout });

async function addTransaction(page, description, amount) {
  await page.fill("input[name=description]", description);
  await page.fill("input[name=amount]", amount);
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector(`td:has-text('${description}')`);
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  await startServer();
  // Liga o acesso pela rede (é por ele que o celular chega ao computador).
  await form("/configuracoes/acesso", { acao: "ligar", senha: "senha-forte-1", confirmar: "senha-forte-1" });
  await sleep(800);
  await waitHealthy();

  const ctx = await browser.newContext({ ...devices["Pixel 7"] });
  const page = await ctx.newPage();
  const errors = [];
  const logs = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => logs.push(`[${m.type()}] ${m.text()}`));
  page.on("requestfailed", (r) => logs.push(`[falhou] ${r.url()} ${r.failure()?.errorText}`));
  globalThis.dumpPage = async () => {
    console.log("--- diagnóstico ---");
    console.log("url:", page.url(), "| status:", await page.textContent("#sync-status").catch(() => "?"));
    console.log("tela:", (await page.innerText("body").catch(() => "")).slice(0, 600));
    console.log(logs.slice(-15).join("\n"));
  };

  // Recém-instalado: pede para conectar.
  await page.goto(`${APP}/`);
  await page.waitForSelector("text=ainda não está conectado", { timeout: 20000 });
  await waitStatus(page, "unpaired");
  ok(true, "app novo pede para conectar ao computador");

  // Link de pareamento (QR code) abre Ajustes preenchido, sem conectar sozinho.
  const code = await newPairingCode();
  ok(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/.test(code || ""), `computador mostra o código de pareamento (${code})`);
  await page.click("text=Conectar agora");
  await page.goto(`${APP}/configuracoes?` + new URLSearchParams({ url: `127.0.0.1:${PORT}`, codigo: "ERRADO22" }));
  await page.waitForSelector("form[data-form=pair]");
  ok(await page.inputValue("input[name=url]") === `127.0.0.1:${PORT}`, "link de pareamento preenche o endereço");
  ok(await page.isVisible("text=Confira se o endereço"), "pede para conferir o endereço antes de conectar");
  await page.click("form[data-form=pair] button");
  await page.waitForSelector(".flash.err");
  ok((await page.textContent(".flash.err")).includes("código errado"), "código errado é recusado com mensagem clara");

  await page.fill("input[name=code]", code.toLowerCase());
  await page.click("form[data-form=pair] button");
  await page.waitForSelector("text=Conectado a", { timeout: 15000 });
  await waitStatus(page, "ok");
  ok(true, "pareou com o código e sincronizou");
  ok((await (await fetch(`${B}/configuracoes`)).text()).includes("Pixel 7"), "computador lista o aparelho pareado pelo nome");

  // Dados vão e voltam com o token do aparelho.
  await page.click("nav.tabs a[href='/lancamentos']");
  await page.waitForSelector("form[data-form=transaction]", { timeout: 20000 });
  await addTransaction(page, "Mercado (app)", "250");
  await page.evaluate(() => window.finanSync());
  ok((await serverRecords()).some((r) => r.description === "Mercado (app)" && r.amountCents === 25000), "lançamento do app chegou ao computador");

  // Offline: o computador desliga; o app continua e envia depois.
  await stopServer();
  await addTransaction(page, "Padaria (offline)", "12,50");
  await waitStatus(page, "offline");
  await page.reload();
  await page.waitForSelector("td:has-text('Padaria (offline)')", { timeout: 20000 });
  ok(true, "sem o computador, o app abre com os dados e guarda o que é novo");
  await startServer();
  await page.evaluate(() => window.finanSync());
  await waitStatus(page, "ok");
  ok((await serverRecords()).some((r) => r.description === "Padaria (offline)"), "ao reconectar, o lançamento offline chegou ao computador");

  // Consultor dentro do app: fala com a IA do computador pelo token.
  await page.click("nav.tabs a[href='/chat']");
  await page.waitForSelector("form[data-form=ask]", { timeout: 15000 });
  await page.fill("textarea[name=message]", "Quanto gastei com mercado?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:has-text('R$ 250,00')", { timeout: 20000 });
  ok(true, "consultor responde dentro do app (IA do computador, com o token)");
  await page.click(".msg.assistant [data-save]");
  await page.waitForSelector(".flash.ok:has-text('Resposta salva em Notas')");
  await page.goto(`${APP}/notas`);
  await page.waitForSelector(".note:has-text('Consultor: Quanto gastei com mercado?')", { timeout: 10000 });
  ok(true, "resposta do consultor salva como nota");

  // Desconectado no computador: o app avisa para parear de novo.
  await form("/configuracoes/aparelhos/remover", { id: "1" });
  await page.evaluate(() => window.finanSync());
  await waitStatus(page, "login");
  ok((await page.textContent("#sync-status")) === "Pareie de novo", "aparelho removido no computador pede novo pareamento");

  // Pareia de novo lendo o QR code com a câmera (jsQR). A "câmera" é um
  // canvas com a imagem do QR que o computador mostra.
  await form("/configuracoes/parear", {});
  const qr = (await (await fetch(`${B}/configuracoes`)).text()).match(/<img src="(data:image\/png;base64,[^"]+)" alt="QR code para parear o app"/)?.[1]?.replaceAll("&#43;", "+").replaceAll("&#x2b;", "+");
  ok(!!qr, "computador mostra o QR code de pareamento");
  await page.addInitScript((src) => {
    navigator.mediaDevices.getUserMedia = async () => {
      const img = new Image();
      img.src = src;
      await img.decode();
      const canvas = Object.assign(document.createElement("canvas"), { width: 480, height: 480 });
      const g = canvas.getContext("2d");
      const draw = () => { g.fillStyle = "#fff"; g.fillRect(0, 0, 480, 480); g.drawImage(img, 60, 60, 360, 360); };
      draw();
      setInterval(draw, 100);
      return canvas.captureStream(10);
    };
  }, qr);
  await page.goto(`${APP}/configuracoes`);
  await page.click("summary:has-text('Conectar a outro computador')");
  await page.click("[data-action=scan]");
  await page.waitForSelector("text=Conectado a", { timeout: 15000 });
  await waitStatus(page, "ok");
  ok(await page.locator(".qr-overlay").count() === 0, "leu o QR code pela câmera, fechou a câmera e pareou de novo");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
  await globalThis.dumpPage?.();
} finally {
  await browser.close();
  await stopServer();
  appServer.close();
  ollama.close();
  rmSync(dataDir, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
