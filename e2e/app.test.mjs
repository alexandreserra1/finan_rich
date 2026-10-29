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
const CLOUD_PORT = 18082;
const B = `http://127.0.0.1:${PORT}`;
const APP = `http://127.0.0.1:${APP_PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-app-"));
let server = null;
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
    // Responde como um modelo com ferramentas: pede a busca ou a proposta e,
    // quando o app devolve o resultado, responde com base nele.
    let body = "";
    req.on("data", (c) => (body += c));
    req.on("end", () => {
      const { messages, tools } = JSON.parse(body);
      const last = messages.at(-1);
      const question = messages.findLast((m) => m.role === "user").content.split("</dados_financeiros>").at(-1); // sem o bloco de dados
      const reply = (message) => res.end(JSON.stringify({ message: { role: "assistant", content: "", ...message }, done: true }) + "\n");
      const ask = (name, args) => reply({ tool_calls: [{ function: { name, arguments: args } }] });
      if (last.role === "tool") {
        // Para apagar, primeiro busca; com o [id] da busca, pede a remoção.
        if (last.tool_name === "buscar_lancamentos" && /apaga/i.test(question)) return ask("apagar_lancamento", { id: last.content.match(/\[([^\]]+)\]/)[1] });
        return reply({ content: last.tool_name === "buscar_lancamentos" ? "Pela busca: " + last.content.split("\n")[0] : "Deixei pronto para você confirmar." });
      }
      if (tools && /padaria/i.test(question)) return ask("buscar_lancamentos", { texto: "padaria" });
      if (tools && /apaga/i.test(question)) return ask("buscar_lancamentos", { texto: "cinema" });
      if (tools && /meta/i.test(question)) return ask("criar_meta", { nome: "Viagem", valor: "6000", prazo: "2027-06" });
      if (tools && /lança/i.test(question)) return ask("propor_lancamento", { descricao: "Cinema", valor: "30,00", categoria: "Lazer" });
      reply({ content: "Você gastou **R$ 250,00** com mercado." });
    });
    return;
  }
  res.writeHead(404).end();
});
await new Promise((r) => ollama.listen(OLLAMA_PORT, "127.0.0.1", r));

// IA na nuvem falsa (formato OpenAI), chamada direto do celular sem computador.
// Guarda tudo que recebe, para conferir que nenhum nome real saiu do aparelho.
const cloudBodies = [];
const cloudAI = createServer((req, res) => {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Headers", "Authorization, Content-Type");
  if (req.method === "OPTIONS") return res.end();
  let body = "";
  req.on("data", (c) => (body += c));
  req.on("end", () => {
    const { messages, tools } = JSON.parse(body);
    if (req.headers.authorization === "Bearer errada") return res.writeHead(401).end('{"error":{"message":"Invalid API Key"}}');
    cloudBodies.push({ auth: req.headers.authorization, messages, tools: (tools || []).map((t) => t.function.name) });
    const last = messages.at(-1);
    const question = messages.findLast((m) => m.role === "user").content.split("</dados_financeiros>").at(-1);
    // O grátis do finan_rich chegou ao limite (o intermediário responde 429).
    if (/limite do grátis/i.test(question)) return res.writeHead(429).end('{"error":{"message":"o consultor grátis chegou ao limite deste aparelho por hoje"}}');
    // A internet cai no meio da resposta: manda um pedaço e corta a conexão.
    if (/internet caiu/i.test(question)) {
      res.write(`data: ${JSON.stringify({ choices: [{ delta: { content: "Metade da resp" } }] })}\n\n`);
      return setTimeout(() => res.socket.destroy(), 100);
    }
    const send = (delta) => res.end(`data: ${JSON.stringify({ choices: [{ delta }] })}\n\ndata: [DONE]\n\n`);
    const call = (name, args) => send({ tool_calls: [{ index: 0, id: "c" + messages.length, type: "function", function: { name, arguments: JSON.stringify(args) } }] });
    if (last.role === "tool") return send({ content: last.content.startsWith("Pronto") ? "Deixei pronto para você confirmar." : "Na " + question.match(/Loja [A-Z]{2,}/)?.[0] + " você gastou " + last.content.match(/Saídas: (R\$ [\d.,]+)/)?.[1] + "." });
    const shop = question.match(/Loja [A-Z]{2,}/)?.[0];
    if (shop) return call("buscar_lancamentos", { texto: shop });
    if (/pão/i.test(question)) return call("propor_lancamento", { descricao: "Pão", valor: "8,50", categoria: "Mercado" });
    send({ content: "Oi!" });
  });
});
await new Promise((r) => cloudAI.listen(CLOUD_PORT, "127.0.0.1", r));

const status = (page) => page.locator("#sync-status").getAttribute("data-status");
const waitStatus = (page, s, timeout = 15000) =>
  page.waitForFunction((s) => document.getElementById("sync-status")?.dataset.status === s, s, { timeout });

async function addTransaction(page, description, amount) {
  // No celular o formulário fica recolhido atrás de "+ Novo lançamento".
  await openTx(page);
  await openTx(page);
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

  // Tema: segue o aparelho (aqui claro); Escuro fica salvo ao reabrir; volta ao automático.
  await page.goto(`${APP}/configuracoes`);
  const theme = () => page.evaluate(() => [document.documentElement.dataset.theme, getComputedStyle(document.body).backgroundColor]);
  ok((await theme())[0] === "light", "tema automático segue o aparelho (claro)");
  await page.selectOption("[data-theme-pick]", "escuro");
  await page.reload();
  await page.waitForSelector("[data-theme-pick]");
  const [darkTheme, darkBg] = await theme();
  ok(darkTheme === "dark" && darkBg === "rgb(10, 13, 12)" && (await page.inputValue("[data-theme-pick]")) === "escuro", "tema escuro fica salvo ao reabrir o app");
  await page.selectOption("[data-theme-pick]", "auto");
  ok((await theme())[0] === "light", "voltar ao automático segue o aparelho de novo");

  // Usar só no celular: cria as categorias no aparelho e já dá para lançar.
  await page.goto(`${APP}/configuracoes`);
  await page.click("[data-action=solo]");
  await page.waitForSelector("form[data-form=transaction]", { state: "attached", timeout: 20000 });
  // Primeiro uso também só no celular: o Painel guia em vez de ficar vazio.
  await page.goto(`${APP}/`);
  await page.click(".welcome-empty a[href='/?passo=1']", { timeout: 20000 }); // estado vazio do Painel → começa o guia
  await page.waitForSelector(".welcome:has-text('Passo 1 de 4')", { timeout: 20000 });
  await page.fill(".welcome input[name=amount]", "3.000");
  await page.click(".welcome button[type=submit]");
  await page.waitForSelector(".welcome:has-text('Passo 2 de 4')");
  await page.click(".welcome [data-action=finish]");
  await page.waitForSelector(".cards.has-hero");
  const solo = await page.evaluate(async () => (await import("/static/js/db.js")).loadLedger());
  ok(solo.recurring.some((r) => r.amountCents === 300000 && r.categoryId === "padrao-salario") && solo.prefs.some((p) => p.id === "primeiro-uso"),
    "primeiro uso guiado funciona só no celular (salário recorrente salvo no aparelho)");
  await page.goto(`${APP}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]", { state: "attached", timeout: 20000 });
  await addTransaction(page, "Feira (só celular)", "40");
  ok(true, "sem computador, o app funciona só no celular (categorias criadas no aparelho)");

  // Consultor sem computador: IA na nuvem, chave cifrada, nomes anonimizados.
  await page.evaluate(async () => (await import("/static/js/db.js")).setMeta("cloud-ai", { preset: "groq", baseUrl: "http://127.0.0.1:18082" })); // a nuvem falsa
  await page.goto(`${APP}/configuracoes`);
  await page.waitForSelector("form[data-form=cloud-ai]", { timeout: 20000 });
  await page.selectOption("form[data-form=cloud-ai] select[name=preset]", "groq");
  ok(await page.isVisible(".key-guide li:has-text('Create API Key')") && await page.isVisible(".key-guide a.button"),
    "sem chave, mostra o passo a passo e o botão para pegar a chave");
  await page.fill("form[data-form=cloud-ai] input[name=apiKey]", "errada");
  await page.click("form[data-form=cloud-ai] button[type=submit]");
  await page.waitForSelector(".flash.err:has-text('chave da IA foi recusada')", { timeout: 20000 });
  ok(true, "chave errada: o teste na hora avisa com mensagem clara");
  await page.fill("form[data-form=cloud-ai] input[name=apiKey]", "chave-secreta-123");
  await page.click("form[data-form=cloud-ai] button[type=submit]");
  await page.waitForSelector(".flash:has-text('Chave funcionando')", { timeout: 20000 });
  const stored = await page.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const cfg = await db.getMeta("cloud-ai");
    const plain = JSON.stringify(cfg) + new TextDecoder().decode(cfg.key.data);
    return { preset: cfg.preset, cipher: cfg.key.data instanceof ArrayBuffer, leaked: plain.includes("chave-secreta-123") };
  });
  ok(stored.preset === "groq" && stored.cipher && !stored.leaked, "a chave da IA fica cifrada no aparelho (não aparece em texto puro)");
  await page.goto(`${APP}/chat`);
  await page.waitForSelector("form[data-form=ask]", { timeout: 20000 });
  await page.fill("textarea[name=message]", "Quanto gastei na Feira?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:has-text('você gastou')", { timeout: 30000 });
  const said = await page.textContent(".msg.assistant >> nth=-1");
  const sent = JSON.stringify(cloudBodies.map((b) => b.messages));
  ok(said.includes("Feira") && !said.includes("Loja") && said.includes("R$ 40,00"), `a resposta volta com o nome verdadeiro (${said.trim()})`);
  ok(!/feira/i.test(sent) && cloudBodies.every((b) => b.auth === "Bearer chave-secreta-123"), "nenhum nome real saiu do celular (a IA só viu apelidos)");
  await page.fill("textarea[name=message]", "Lança 8,50 do pão");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".proposal:has-text('Pão')", { timeout: 30000 });
  await page.click(".proposal:has-text('Pão') button");
  await page.waitForSelector(".proposal:has-text('Pão') button:has-text('Adicionado')", { timeout: 10000 });
  ok(true, "pela nuvem, a proposta também vira cartão e grava neste celular");

  // A internet cai no meio da resposta: a Jade responde com as contas do app
  // (sem IA), não fica em "Pensando…" e não guarda a resposta pela metade.
  await page.fill("textarea[name=message]", "E se a internet caiu?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:has-text('A IA não respondeu agora') .app-answer", { timeout: 30000 });
  const savedChat = JSON.stringify(await page.evaluate(async () => (await import("/static/js/cloud-ai.js")).loadChat()));
  ok(await page.locator(".msg.pending").count() === 0 && !(await page.textContent(".messages")).includes("Metade da resp") && !savedChat.includes("Metade da resp") &&
    (await page.textContent(".msg.assistant >> nth=-1")).includes("Resumo de"),
    `internet caindo no meio do chat: resposta sem IA, sem "Pensando…" e sem resposta pela metade`);

  // O grátis do finan_rich no limite: resposta sem IA e o caminho para a chave grátis da pessoa.
  await page.evaluate(async () => { // a configuração com a chave fica guardada no próprio app (a chave cifrada não passa bem pelo teste)
    const db = await import("/static/js/db.js");
    await db.setMeta("cloud-ai-antes", await db.getMeta("cloud-ai"));
    await db.setMeta("cloud-ai", { preset: "finan", model: "auto", baseUrl: "http://127.0.0.1:18082" });
  });
  await page.goto(`${APP}/chat`);
  await page.waitForSelector("form[data-form=ask]", { timeout: 20000 });
  await page.fill("textarea[name=message]", "Acabou o limite do grátis?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant .notice:has-text('chave grátis') a[href='/configuracoes#ia-nuvem']", { timeout: 30000 });
  ok((await page.textContent(".msg.assistant >> nth=-1")).includes("A IA não respondeu agora"), "grátis no limite: resposta sem IA e aviso com o caminho para a própria chave");
  await page.evaluate(async () => { const db = await import("/static/js/db.js"); await db.setMeta("cloud-ai", await db.getMeta("cloud-ai-antes")); });
  await page.goto(`${APP}/chat`);
  await page.waitForSelector("form[data-form=ask]", { timeout: 20000 });
  await page.fill("textarea[name=message]", "Oi");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:has-text('Oi!')", { timeout: 30000 });
  ok(await page.locator(".msg.error").count() === 0, "depois da queda, a próxima pergunta funciona");

  // O time: a Jade responde e, como o assunto é dívida, oferece passar para o
  // Bento; com ele, a IA recebe a voz e só as ferramentas de dívida.
  ok((await page.textContent(".msg.assistant >> nth=-1")).includes("Jade · Consultora") && await page.getAttribute(".team-member[data-agent=jade]", "aria-pressed") === "true",
    "sem escolher ninguém, quem responde é a Jade");
  await page.fill("textarea[name=message]", "Qual dívida pago primeiro?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".handoff:has-text('Isso é com Bento (Dívidas)')", { timeout: 30000 });
  await page.click("[data-handoff]");
  await page.waitForSelector(".msg.assistant >> nth=-1 >> text=Bento · Dívidas", { timeout: 30000 });
  const toBento = cloudBodies.at(-1);
  ok(toBento.messages[0].content.includes("Você é o Bento") && toBento.tools.join() === "buscar_lancamentos,criar_divida,registrar_pagamento,simular_dividas" &&
    await page.getAttribute(".team-member[data-agent=bento]", "aria-pressed") === "true" && await page.locator(".handoff").count() === 0,
    `trocar para o Bento muda a voz e as ferramentas (${toBento.tools})`);
  // Sugestão com resposta pronta: o app responde sem chamar a IA.
  const calls = cloudBodies.length;
  await page.fill("textarea[name=message]", "Quando eu fico livre das dívidas?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant >> nth=-1 >> .app-answer:has-text('Resposta do app, sem IA.')", { timeout: 30000 });
  ok(cloudBodies.length === calls, "sugestão pronta do Bento: resposta do app, sem chamar a IA");
  await page.reload();
  await page.waitForSelector(".team-member[data-agent=bento][aria-pressed=true]", { timeout: 20000 });
  ok((await page.getAttribute("textarea[name=message]", "placeholder")).includes("Bento") && (await page.textContent(".messages")).includes("Jade · Consultora"),
    "a escolha fica guardada e cada resposta antiga mostra quem respondeu");
  await page.click(".team-member[data-agent=jade]");
  await page.waitForSelector(".team-member[data-agent=jade][aria-pressed=true]");

  // IA dentro do celular, com um wllama falso no lugar do de verdade (o de
  // verdade só entra no app pelo cap sync, e o modelo tem 470 MB).
  await page.route("**/static/vendor/wllama/index.js", (route) => route.fulfill({ contentType: "text/javascript", body: `
    export class ModelManager {
      async getModels() { const url = localStorage.getItem("fake-model"); return url ? [{ url, remove: async () => localStorage.removeItem("fake-model") }] : []; }
      async downloadModel(url, { progressCallback }) {
        for (const p of [25, 50, 100]) { progressCallback({ loaded: p, total: 100 }); await new Promise((r) => setTimeout(r, 50)); }
        localStorage.setItem("fake-model", url);
      }
    }
    export class Wllama {
      async loadModel() {}
      async exit() {}
      async createChatCompletion({ messages, stream }) {
        window.localMessages = messages;
        if (JSON.stringify(messages.at(-1)).includes("IA em pane")) throw new Error("modelo travou");
        return (async function* () {
          for (const piece of ["Guarde ", "**10%** ", "do salário."]) { await new Promise((r) => setTimeout(r, 30)); yield { choices: [{ delta: { content: piece } }] }; }
        })();
      }
    }` }));
  await page.goto(`${APP}/configuracoes`);
  await page.click("[data-action=local-download]", { timeout: 20000 });
  await page.waitForSelector(".flash:has-text('Modelo baixado')", { timeout: 15000 });
  await page.goto(`${APP}/chat`);
  await page.waitForSelector(".chat:has-text('dentro deste celular')", { timeout: 20000 });
  const cloudCalls = cloudBodies.length;
  await page.fill("textarea[name=message]", "Quanto gastei na Feira?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:not(.pending):has-text('do salário.')", { timeout: 20000 });
  const sentLocal = JSON.stringify(await page.evaluate(() => window.localMessages));
  ok(cloudBodies.length === cloudCalls && sentLocal.includes("Feira") && (await page.innerHTML(".msg.assistant >> nth=-1")).includes("<strong>10%</strong>"),
    "IA dentro do celular: responde com os nomes verdadeiros e nada vai para a nuvem");
  await page.reload();
  await page.waitForSelector(".msg.assistant:has-text('do salário.')", { timeout: 20000 });
  ok(true, "a conversa com a IA dentro do celular fica salva");
  await page.fill("textarea[name=message]", "E se a IA em pane?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant >> nth=-1 >> .app-answer:has-text('modelo travou')", { timeout: 20000 });
  ok((await page.textContent(".msg.assistant >> nth=-1")).includes("A IA não respondeu agora") && cloudBodies.length === cloudCalls,
    "IA dentro do celular com erro: resposta sem IA, feita no aparelho");
  await page.goto(`${APP}/configuracoes`);
  await page.uncheck("[data-action=local-ai]", { timeout: 20000 });
  await page.waitForSelector(".flash:has-text('volta a usar')");
  await page.unroute("**/static/vendor/wllama/index.js");

  // Link de pareamento (QR code) abre Ajustes preenchido, sem conectar sozinho.
  const code = await newPairingCode();
  ok(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/.test(code || ""), `computador mostra o código de pareamento (${code})`);
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
  {
    const recs = await serverRecords();
    const cats = recs.filter((r) => r.name && r.bucket && !r.deleted);
    const feira = recs.find((r) => r.description === "Feira (só celular)");
    ok(cats.length === 15 && feira && !feira.categoryId.startsWith("padrao-") && cats.some((c) => c.id === feira.categoryId),
      `ao parear, o que foi feito só no celular vai para o computador sem duplicar categorias (${cats.length} categorias)`);
    await page.evaluate(() => window.finanSync());
    const phoneCats = await page.evaluate(async () => (await import("/static/js/db.js")).loadLedger().then((l) => l.categories.filter((c) => !c.deleted).length));
    ok(phoneCats === 15, `o celular também fica sem categorias repetidas (${phoneCats})`);
  }

  // Dados vão e voltam com o token do aparelho.
  await page.click("nav.tabs a[href='/lancamentos']");
  await page.waitForSelector("form[data-form=transaction]", { state: "attached", timeout: 20000 });
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

  // Consultor com ferramentas: busca lançamentos e propõe um lançamento, que só
  // entra depois do "Adicionar".
  await page.click("nav.tabs a[href='/chat']");
  await page.waitForSelector("form[data-form=ask]", { timeout: 15000 });
  await page.fill("textarea[name=message]", "Quanto gastei na padaria?");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".msg.assistant:has-text('Pela busca: 1 lançamentos')", { timeout: 20000 });
  ok((await page.textContent(".msg.assistant >> nth=-1")).includes("Saídas: R$ 12,50"), "consultor busca lançamentos pela ferramenta (padaria: R$ 12,50)");
  await page.fill("textarea[name=message]", "Lança 30 reais de cinema");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".proposal:has-text('Cinema')", { timeout: 20000 });
  ok(/R\$\s30,00/.test(await page.textContent(".proposal")) && !(await serverRecords()).some((r) => r.description === "Cinema"),
    "consultor propõe o lançamento, que ainda não foi gravado");
  await page.click(".proposal button");
  await page.waitForSelector(".proposal button:has-text('Adicionado')", { timeout: 10000 });
  await page.evaluate(() => window.finanSync());
  const cinema = (await serverRecords()).find((r) => r.description === "Cinema");
  ok(cinema?.amountCents === 3000 && cinema.kind === "saida", "depois do Adicionar, o lançamento entra e chega ao computador");

  // Apagar e criar meta pelo chat, também só com o toque no botão.
  await page.fill("textarea[name=message]", "Apaga o cinema");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".proposal:has-text('Apagar: ') button:has-text('Apagar')", { timeout: 20000 });
  await page.click(".proposal:has-text('Apagar: ') button");
  await page.waitForSelector(".proposal button:has-text('Apagado')", { timeout: 10000 });
  await page.fill("textarea[name=message]", "Cria uma meta de viagem");
  await page.click("form[data-form=ask] button");
  await page.waitForSelector(".proposal:has-text('Nova meta: Viagem')", { timeout: 20000 });
  await page.click(".proposal:has-text('Nova meta') button");
  await page.waitForSelector(".proposal:has-text('Nova meta') button:has-text('Adicionado')", { timeout: 10000 });
  await page.evaluate(() => window.finanSync());
  const after = await serverRecords();
  ok(after.find((r) => r.description === "Cinema")?.deleted === true && after.some((r) => r.name === "Viagem" && r.targetCents === 600000 && r.deadline === "2027-06"),
    "pelo chat, apagou o lançamento e criou a meta (depois do toque)");
  await page.reload();
  await page.waitForSelector(".proposal", { timeout: 20000 });
  const cards = await page.locator(".proposal button").allTextContents();
  ok(cards.join("|") === "Adicionado ✓|Apagado ✓|Adicionado ✓", `os cartões continuam na conversa depois de recarregar, marcados como feitos (${cards.join(" | ")})`);

  // Bloqueio do app com a senha (sem biometria no navegador de teste).
  await page.goto(`${APP}/configuracoes`);
  await page.waitForSelector("#bloqueio input[data-action=lock]:not([disabled])", { timeout: 15000 });
  await page.check("#bloqueio input[data-action=lock]");
  await page.fill("form[data-form=lock-pin] input[name=pin]", "1234");
  await page.fill("form[data-form=lock-pin] input[name=confirm]", "1235");
  await page.click("form[data-form=lock-pin] button[type=submit]");
  await page.waitForSelector(".flash.err:has-text('não são iguais')");
  await page.fill("form[data-form=lock-pin] input[name=confirm]", "1234");
  await page.click("form[data-form=lock-pin] button[type=submit]");
  await page.waitForSelector(".flash:has-text('Bloqueio ligado')");
  await page.reload();
  await page.waitForSelector(".app-lock form[data-lock=pin]", { timeout: 15000 });
  ok(await page.locator("main").evaluate((m) => getComputedStyle(m).visibility) === "hidden", "com o bloqueio ligado, o app abre bloqueado e esconde os dados");
  await page.fill(".app-lock input[name=pin]", "0000");
  await page.click(".app-lock button[type=submit]");
  await page.waitForSelector(".app-lock .lock-msg:has-text('Senha errada')");
  await page.fill(".app-lock input[name=pin]", "1234");
  await page.click(".app-lock button[type=submit]");
  await page.waitForSelector(".app-lock", { state: "detached", timeout: 15000 });
  ok(await page.locator("main").evaluate((m) => getComputedStyle(m).visibility) === "visible", "senha errada é recusada; a certa desbloqueia");
  page.once("dialog", (d) => d.accept());
  await page.uncheck("#bloqueio input[data-action=lock]");
  await page.waitForSelector(".flash:has-text('Bloqueio desligado')");
  await page.reload();
  await page.waitForSelector("#bloqueio", { timeout: 15000 });
  ok(await page.locator(".app-lock").count() === 0, "bloqueio desligado: o app abre direto");
  ok(!logs.some((l) => /Content.Security.Policy|Refused to/i.test(l)), `a política de segurança (CSP) não bloqueou nada no app ${JSON.stringify(logs.filter((l) => /Refused to/i.test(l)))}`);

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

  // O mesmo no consultor do computador (tela do navegador).
  const desk = await browser.newPage();
  desk.on("pageerror", (e) => errors.push(e.message));
  await desk.goto(`${B}/chat`);
  await desk.waitForSelector(".proposal");
  const before = await desk.locator(".proposal").count(); // os da conversa do celular também aparecem aqui
  await desk.fill("textarea[name=message]", "Lança 30 reais de cinema");
  await desk.click("#chat-form button");
  await desk.waitForFunction((n) => document.querySelectorAll(".proposal").length > n, before, { timeout: 20000 });
  const fresh = desk.locator(".proposal >> nth=-1");
  await fresh.locator("button").click();
  await fresh.locator("button:has-text('Adicionado')").waitFor({ timeout: 10000 });
  await desk.evaluate(() => window.finanSync());
  ok((await serverRecords()).filter((r) => r.description === "Cinema" && !r.deleted).length === 1, "no computador, o Adicionar da proposta também lança");
  ok(await desk.locator(".msg.assistant >> nth=-1 >> .msg-who").textContent() === "Jade · Consultora" && await desk.locator(".team-member").count() === 5,
    "no computador, o chat mostra o time e quem respondeu");
  await desk.click(".team-member[data-agent=bento]");
  await desk.fill("textarea[name=message]", "Qual dívida devo pagar primeiro?");
  await desk.click("#chat-form button");
  await desk.waitForFunction(() => { const m = [...document.querySelectorAll(".msg.assistant")].at(-1); return m && !m.classList.contains("pending") && m.querySelector(".msg-who")?.textContent.includes("Bento"); }, null, { timeout: 20000 });
  ok(await desk.locator(".msg.assistant >> nth=-1 >> .app-answer").count() === 1, "no computador, a sugestão pronta vem marcada como resposta do app");
  await desk.reload();
  await desk.waitForSelector(".team-member[data-agent=bento][aria-pressed=true]");
  ok((await desk.locator(".msg.assistant >> nth=-1 >> .msg-who").textContent()) === "Bento · Dívidas" && (await desk.getAttribute("textarea[name=message]", "placeholder")).includes("Bento"),
    "no computador, a resposta fica do Bento e a escolha é lembrada");

  // Open Finance no app sem computador: o Client Secret vai para o cofre do
  // sistema (falso aqui) e a busca sai do próprio celular direto para a Pluggy.
  const ofCtx = await browser.newContext({ ...devices["Pixel 7"] });
  await ofCtx.addInitScript(() => {
    // O cofre de verdade sobrevive à troca de tela; o falso guarda no localStorage.
    const load = () => JSON.parse(localStorage.getItem("cofre-falso") || "{}");
    const put = (s) => localStorage.setItem("cofre-falso", JSON.stringify(s));
    window.__vault = () => load();
    window.Capacitor = { Plugins: { NativeBiometric: {
      setData: async ({ key, value }) => put({ ...load(), [key]: value }),
      getData: async ({ key }) => { const s = load(); if (!(key in s)) throw new Error("vazio"); return { value: s[key] }; },
      deleteData: async ({ key }) => { const s = load(); delete s[key]; put(s); },
    }, BankNotifications: {
      status: async () => ({ enabled: true }),
      openSettings: async () => {},
      drain: async () => ({ items: localStorage.getItem("drenado") ? [] : (localStorage.setItem("drenado", "1"), [
        { id: "com.nu.production|1", time: Date.now(), text: "Compra aprovada. Compra de R$ 32,40 APROVADA em PADARIA REAL para o cartão com final 1234." },
        { id: "com.nu.production|2", time: Date.now(), text: "Aproveite! Cashback de R$ 10,00" },
      ]) }),
    } } };
  });
  const pluggyAuth = [];
  await ofCtx.route("https://api.pluggy.ai/**", (route) => {
    const req = route.request();
    const headers = { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Headers": "*", "Content-Type": "application/json" };
    if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    const url = new URL(req.url());
    if (url.pathname === "/auth") { pluggyAuth.push(req.postDataJSON()); return route.fulfill({ headers, body: '{"apiKey":"k"}' }); }
    if (url.pathname === "/accounts") return route.fulfill({ headers, body: '{"results":[{"id":"c1","type":"CREDIT","name":"Cartão","marketingName":"Nubank","number":"4321"}]}' });
    return route.fulfill({ headers, body: JSON.stringify({ next: null, results: [{ id: "n1", description: "Padaria do Zé", amount: 18.9, date: `${url.searchParams.get("dateFrom")}T09:00:00.000Z`, type: "DEBIT", status: "POSTED" }] }) });
  });
  const of = await ofCtx.newPage();
  of.on("pageerror", (e) => errors.push(e.message));
  await of.goto(`${APP}/configuracoes`);
  await of.click("[data-action=solo]");
  await of.waitForSelector("form[data-form=transaction]", { state: "attached", timeout: 20000 });
  await of.goto(`${APP}/configuracoes`);
  await of.waitForSelector("form[data-form=open-finance]");
  await of.fill("form[data-form=open-finance] input[name=client_id]", "id-do-app");
  await of.fill("form[data-form=open-finance] input[name=client_secret]", "segredo-do-app");
  await of.fill("form[data-form=open-finance] textarea[name=items]", "item-9");
  await of.click("form[data-form=open-finance] button[type=submit]");
  await of.waitForSelector("text=Open Finance salvo");
  const saved = await of.evaluate(async () => [window.__vault()["pluggy-secret"], JSON.stringify(await (await import("/static/js/db.js")).getMeta("pluggy", null))]);
  ok(saved[0] === "segredo-do-app" && !saved[1].includes("segredo"), "Client Secret no cofre do sistema, não no banco do app");
  await of.goto(`${APP}/importar`);
  await of.click("form[data-form=openfinance] button[type=submit]");
  await of.click("[data-bank='0']", { timeout: 30000 });
  await of.waitForSelector(".import-table input[value='Padaria do Zé']");
  ok(pluggyAuth[0]?.clientSecret === "segredo-do-app" && (await of.textContent(".import-table")).includes("R$ 18,90"), "no app, a busca vai do celular direto para a Pluggy");

  // Notificações do banco: a compra vira lançamento a revisar; a propaganda, não.
  await of.goto(`${APP}/importar`);
  await of.click("[data-action=bank-notes]");
  await of.waitForSelector(".import-table input[value='PADARIA REAL']");
  ok(await of.locator(".import-table tbody tr").count() === 1 && (await of.textContent(".import-table")).includes("R$ 32,40"), "notificação de compra do banco vira lançamento para revisar");
  await of.locator(".import-table select[data-field=categoryId]").selectOption({ index: 1 });
  await of.click("[data-action=import]");
  await of.waitForSelector("td:has-text('PADARIA REAL')");
  ok(JSON.stringify(await of.evaluate(async () => (await import("/static/js/db.js")).getMeta("bank-notes", null))) === "[]", "depois de importar, o texto das notificações sai do aparelho");
  // Com o cofre do sistema, o token do pareamento fica nele, não no banco do app;
  // um token antigo em texto passa para o cofre na próxima abertura.
  await of.goto(`${APP}/configuracoes?` + new URLSearchParams({ url: `127.0.0.1:${PORT}`, codigo: await newPairingCode() }));
  await of.click("form[data-form=pair] button[type=submit]");
  await of.waitForSelector("text=Conectado a", { timeout: 15000 });
  const vaulted = async () => of.evaluate(async () => [await (await import("/static/js/db.js")).getMeta("server", null), window.__vault()["server-token"]]);
  let [meta, inVault] = await vaulted();
  ok(!meta.token && meta.vault && inVault?.length > 20, "token do pareamento no cofre do sistema, não no banco do app");
  await of.evaluate(async (t) => {
    const db = await import("/static/js/db.js");
    const { vault, ...s } = await db.getMeta("server", null);
    await db.setMeta("server", { ...s, token: t }); // como guardava a versão anterior
    await window.Capacitor.Plugins.NativeBiometric.deleteData({ key: "server-token" });
  }, inVault);
  await of.reload();
  await waitStatus(of, "ok");
  [meta, inVault] = await vaulted();
  ok(!meta.token && meta.vault && inVault?.length > 20, "token antigo em texto passa para o cofre e a sincronização continua");
  await ofCtx.close();

  // Lembretes do time no celular (plugin de notificações falso): Luna avisa
  // o vencimento, Otávio toda segunda e Tito no dia do salário; o toque abre a tela.
  const remCtx = await browser.newContext({ ...devices["Pixel 7"] });
  await remCtx.addInitScript(() => {
    const load = () => JSON.parse(localStorage.getItem("notif-falsas") || "[]");
    const put = (n) => localStorage.setItem("notif-falsas", JSON.stringify(n));
    window.__notifs = load;
    window.__tap = null;
    window.Capacitor = { Plugins: { LocalNotifications: {
      checkPermissions: async () => ({ display: "granted" }),
      requestPermissions: async () => ({ display: "granted" }),
      getPending: async () => ({ notifications: load() }),
      cancel: async ({ notifications }) => put(load().filter((n) => !notifications.some((c) => c.id === n.id))),
      schedule: async ({ notifications }) => put([...load(), ...notifications]),
      addListener: (name, fn) => { if (name === "localNotificationActionPerformed") window.__tap = fn; },
    } } };
  });
  const rem = await remCtx.newPage();
  rem.on("pageerror", (e) => errors.push(e.message));
  rem.on("dialog", (d) => d.accept());
  await rem.goto(`${APP}/configuracoes`);
  await rem.click("[data-action=solo]");
  await rem.waitForSelector("main h1", { timeout: 20000 });
  await rem.evaluate(async () => {
    const db = await import("/static/js/db.js");
    const f = await import("/static/js/finance.js");
    const now = Date.now();
    const renda = (await db.loadLedger()).categories.find((c) => c.bucket === "renda");
    await db.saveLocal("debt", f.validate("debt", { id: "d-lembrete", updatedAt: now, name: "Cartão", balanceCents: 100000, monthlyRateBp: 1000, minPaymentCents: 20000, dueDay: 12 }));
    await db.saveLocal("goal", f.validate("goal", { id: "g-lembrete", updatedAt: now, name: "Viagem", targetCents: 500000, savedCents: 0, startDate: "2026-01-01" }));
    await db.saveLocal("recurring", f.validate("recurring", { id: "r-lembrete", updatedAt: now, kind: "entrada", description: "Salário", amountCents: 400000, categoryId: renda.id, day: 5, start: "2026-01" }));
  });
  await rem.reload();
  await rem.waitForFunction(() => window.__notifs().length >= 3, null, { timeout: 20000 });
  const notifs = await rem.evaluate(() => window.__notifs());
  const by = (who) => notifs.find((n) => n.title.startsWith(who + ":"));
  ok(by("Luna")?.title === "Luna: Cartão vence dia 12" && by("Luna").schedule.on.day === 10 && by("Luna").extra.link === "/dividas",
    "Luna avisa o vencimento dois dias antes");
  ok(by("Otávio")?.schedule.on.weekday === 2 && by("Otávio").schedule.on.hour === 9 && by("Otávio").extra.link === "/analise",
    "Otávio avisa toda segunda, às 9h, e leva à Análise");
  ok(by("Tito")?.schedule.on.day === 5 && by("Tito").extra.link === "/metas", "Tito faz o balanço das metas no dia do salário");
  await rem.evaluate(() => window.__tap({ notification: { extra: { link: "/metas" } } }));
  await rem.waitForFunction(() => location.pathname === "/metas");
  ok(true, "tocar na notificação abre a tela certa");
  await rem.evaluate(async () => (await import("/static/js/native.js")).setRemindersEnabled(false));
  ok((await rem.evaluate(() => window.__notifs())).length === 0, "desligar os lembretes cancela os do time também");
  await remCtx.close();

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
  await globalThis.dumpPage?.();
} finally {
  await browser.close();
  await stopServer();
  appServer.close();
  ollama.close();
  cloudAI.close();
  rmSync(dataDir, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
