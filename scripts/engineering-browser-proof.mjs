import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { once } from "node:events";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { artifacts, root, save, killGroup, stopChild } from "./engineering.mjs";

const controller = new AbortController();

async function connect(mode) {
  controller.signal.throwIfAborted();
  const child = spawn(process.execPath, [join(root, "scripts/engineering-mcp.mjs"), mode], { cwd: root, detached: process.platform !== "win32", stdio: ["pipe", "pipe", "pipe"] });
  let buffer = "";
  let serial = 0;
  let closed = false;
  const pending = new Map();
  child.stderr.on("data", () => {});
  const send = (value) => child.stdin.write(JSON.stringify(value) + "\n");
  child.stdout.on("data", (chunk) => {
    buffer += chunk;
    if (buffer.length > 8 * 1024 * 1024) { killGroup(child); return; }
    while (buffer.includes("\n")) {
      const end = buffer.indexOf("\n");
      const line = buffer.slice(0, end);
      buffer = buffer.slice(end + 1);
      if (!line.trim()) continue;
      let message;
      try { message = JSON.parse(line); } catch { killGroup(child); return; }
      if (message.method && message.id !== undefined) {
        send({ jsonrpc: "2.0", id: message.id, ...(message.method === "roots/list" ? { result: { roots: [{ uri: pathToFileURL(artifacts).href, name: "Evidências fictícias" }] } } : { error: { code: -32601, message: "Método não suportado" } }) });
      } else if (pending.has(message.id)) {
        const request = pending.get(message.id);
        pending.delete(message.id);
        clearTimeout(request.timer);
        if (message.error) request.reject(new Error(message.error.message));
        else request.resolve(message.result);
      }
    }
  });
  const fail = () => {
    closed = true;
    for (const request of pending.values()) { clearTimeout(request.timer); request.reject(new Error(`${mode}: MCP encerrou antes da resposta.`)); }
    pending.clear();
  };
  child.on("error", fail);
  child.on("exit", fail);
  child.stdin.on("error", fail);
  const abort = () => { killGroup(child); fail(); };
  controller.signal.addEventListener("abort", abort, { once: true });
  const request = (method, params) => new Promise((resolve, reject) => {
    if (closed) { reject(new Error(`${mode}: conexão encerrada.`)); return; }
    const id = ++serial;
    const timer = setTimeout(() => { pending.delete(id); reject(new Error(`${mode}: ${method} excedeu 60 segundos.`)); killGroup(child); }, 60000);
    pending.set(id, { resolve, reject, timer });
    send({ jsonrpc: "2.0", id, method, params });
  });
  const close = async () => {
    closed = true;
    child.stdin.end();
    await stopChild(child);
    controller.signal.removeEventListener("abort", abort);
  };
  try {
    await request("initialize", { protocolVersion: "2024-11-05", capabilities: { roots: { listChanged: false } }, clientInfo: { name: "rich-proof", version: "1.0.0" } });
    send({ jsonrpc: "2.0", method: "notifications/initialized" });
    const listed = await request("tools/list", {});
    save(`${mode}-tools.json`, listed);
    return { close, async call(name, args = {}) {
      assert.ok(listed.tools.some((tool) => tool.name === name), `${mode}: ferramenta ${name} existe`);
      const result = await request("tools/call", { name, arguments: args });
      if (result.isError) throw new Error(`${mode}/${name}: ${JSON.stringify(result.content).slice(0, 1500)}`);
      return result;
    } };
  } catch (error) { await close(); throw error; }
}

mkdirSync(artifacts, { recursive: true, mode: 0o700 });
assert.ok(existsSync(join(root, "dist/finansite")), "Gere o site com make e2e antes da prova MCP.");
const directory = mkdtempSync(join(tmpdir(), "rich-browser-proof-"));
const reservation = createServer();
reservation.listen(0, "127.0.0.1");
await once(reservation, "listening");
const port = reservation.address().port;
await new Promise((resolve) => reservation.close(resolve));
const url = `http://127.0.0.1:${port}`;
const server = spawn(join(root, "dist/finansite"), [], { cwd: directory, detached: process.platform !== "win32", env: { PATH: process.env.PATH, HOME: directory, PORT: String(port), GROQ_API_KEY: "fixture", FINAN_PROXY_UPSTREAM: "http://127.0.0.1:9" }, stdio: "ignore" });
const clients = [];
let cancelled = false;
const cancel = () => { cancelled = true; controller.abort(); killGroup(server); for (const client of clients) void client.close(); };
process.once("SIGINT", cancel);
process.once("SIGTERM", cancel);
const deadline = setTimeout(cancel, 240000);
try {
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (server.exitCode !== null) throw new Error("O site local encerrou antes da prova.");
    try { ready = (await fetch(url, { signal: AbortSignal.timeout(500) })).ok; } catch {}
    if (ready) break;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.ok(ready, "site local responde");
  assert.ok(!cancelled, "prova não interrompida");
  const playwright = await connect("playwright");
  clients.push(playwright);
  await playwright.call("browser_navigate", { url });
  await playwright.call("browser_wait_for", { text: "Meus gastos", time: 20 });
  const interactions = [];
  for (const mode of ["metas", "resumo", "gastos"]) {
    await playwright.call("browser_click", { target: `#lp-tab-${mode}`, element: `Aba ${mode}` });
    const result = await playwright.call("browser_evaluate", { function: `() => document.querySelector('.lp-dashboard')?.getAttribute('aria-labelledby') === 'lp-tab-${mode}'` });
    assert.ok(result.content.some((item) => item.type === "text" && /### Result\s+true/.test(item.text)), `aba ${mode} ativa`);
    interactions.push({ mode, ok: true });
  }
  save("playwright-interaction.json", interactions);
  for (const colorScheme of ["light", "dark"]) {
    await playwright.call("browser_emulate_media", { colorScheme, reducedMotion: "reduce" });
    for (const width of [390, 768, 1440, 1920]) {
      await playwright.call("browser_resize", { width, height: 1000 });
      const checked = await playwright.call("browser_evaluate", { function: "async () => { for (let attempt = 0; attempt < 100 && !document.querySelector('.lp-dashboard'); attempt++) await new Promise(resolve => setTimeout(resolve, 100)); await document.fonts.ready; return { overflow: document.documentElement.scrollWidth > innerWidth, title: document.title, demo: !!document.querySelector('.lp-dashboard'), theme: document.documentElement.getAttribute('data-theme') }; }" });
      const block = checked.content.find((item) => item.type === "text")?.text || "";
      assert.match(block, /"overflow": false/);
      assert.match(block, /"demo": true/);
      assert.ok(block.includes(`"theme": "${colorScheme}"`), `tema ${colorScheme} aplicado`);
      await playwright.call("browser_take_screenshot", { filename: join(artifacts, `mcp-${width}-${colorScheme}.png`), fullPage: true, type: "png" });
    }
  }
  const snapshot = await playwright.call("browser_snapshot");
  writeFileSync(join(artifacts, "playwright-snapshot.json"), JSON.stringify(snapshot, null, 2));
  const devtools = await connect("devtools");
  clients.push(devtools);
  assert.ok(!cancelled, "prova não interrompida");
  const opened = await devtools.call("new_page", { url });
  const pagesText = opened.content.filter((item) => item.type === "text").map((item) => item.text).join("\n");
  const selected = pagesText.match(/^\s*(\d+):[^\n]*\[selected\]/m);
  assert.ok(selected, `DevTools informa a página selecionada: ${pagesText.slice(0, 500)}`);
  const pageId = Number(selected[1]);
  const snapshotChrome = await devtools.call("take_snapshot", { pageId });
  writeFileSync(join(artifacts, "devtools-snapshot.json"), JSON.stringify(snapshotChrome, null, 2));
  await devtools.call("take_screenshot", { pageId, filePath: join(artifacts, "devtools-desktop.png"), fullPage: true });
  const consoleMessages = await devtools.call("list_console_messages", { pageId, types: ["error"] });
  const network = await devtools.call("list_network_requests", { pageId });
  assert.ok(consoleMessages.content.some((item) => item.type === "text" && item.text.includes("<no console messages found>")), "console sem erros");
  const requests = network.content.filter((item) => item.type === "text").map((item) => item.text).join("\n");
  assert.ok(!/\[(?:[45]\d\d|failed)[^\]]*\]/i.test(requests), "rede local sem respostas de erro");
  save("browser-proof.json", { at: new Date().toISOString(), local: true, widths: [390, 768, 1440, 1920], themes: ["light", "dark"], consoleMessages, network, baselinesApproved: false });
  console.log(`Dois MCPs inicializados; site real aberto; DOM, rede, console e screenshots em ${artifacts}.`);
} finally {
  clearTimeout(deadline);
  process.off("SIGINT", cancel);
  process.off("SIGTERM", cancel);
  for (const client of clients.reverse()) await client.close();
  await stopChild(server);
  rmSync(directory, { recursive: true, force: true });
}
