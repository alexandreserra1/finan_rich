import { spawn, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, readFileSync, writeFileSync, mkdirSync, lstatSync, readlinkSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
export const artifacts = join(root, ".artifacts/engineering");
const toolbin = join(root, "tools/engineering/node_modules/.bin");

export function redact(value) {
  return String(value).replace(/Bearer\s+[^\s"']+/gi, "Bearer [oculto]")
    .replace(/((?:[\w]*SECRET|[\w]*TOKEN|password|api[_-]?key)\s*[=:]\s*)(?:"[^"]*"|'[^']*'|[^\s,}]+)/gi, "$1[oculto]");
}

export function classify(paths) {
  const gates = new Set();
  for (const path of paths) {
    if (/\.go$|^go\.(mod|sum)$/.test(path)) gates.add("backend");
    if (/^internal\/web\/|^internal\/store\/|^e2e\/|^mobile\/|^site\//.test(path)) gates.add("frontend");
    if (/\.sql$|^internal\/identity\//.test(path)) gates.add("db");
    if (/^scripts\/|^tools\/(?:engineering|ai-lab)\/|^openspec\/|^\.githooks\/|^\.codex\/|^\.github\/workflows\/|^\.mcp\.json$|^\.claude\/settings|^Makefile$/.test(path)) gates.add("tooling");
    if (path === "Makefile") { gates.add("backend"); gates.add("frontend"); }
    if (path === "scripts/test-identity-db.sh") gates.add("db");
  }
  return ["backend", "frontend", "db", "tooling"].filter((gate) => gates.has(gate));
}

export function advance(previous, report, now = Date.now(), limits = { attempts: 5, duration: 900000 }) {
  const state = previous || { attempts: 0, startedAt: now };
  if (report.ok) return { action: "pass", state: { attempts: 0, startedAt: now }, reason: "Verificação executada e aprovada." };
  const stalled = state.fingerprint === report.fingerprint && state.issue === report.issue;
  const exhausted = state.attempts >= limits.attempts || now - state.startedAt >= limits.duration;
  if (stalled || exhausted) return { action: "stop", state, reason: "Correção interrompida: sem progresso ou limite atingido. A tarefa continua reprovada; apresente diagnóstico e evidências." };
  return { action: "repair", state: { ...state, attempts: state.attempts + 1, fingerprint: report.fingerprint, issue: report.issue }, reason: "Investigue a causa, registre hipótese e evidência, faça uma correção incremental e verifique novamente: " + report.issue };
}

export async function runControlled({ verify, repair, record = () => {}, limits = { attempts: 5, duration: 900000 }, now = Date.now }) {
  let state;
  const deadline = now() + limits.duration;
  const bounded = async (operation, argument) => {
    const remaining = deadline - now();
    if (remaining <= 0) throw new Error("Tempo do ciclo de correção excedido.");
    const controller = new AbortController();
    let timer;
    try {
      return await Promise.race([
        operation(argument, controller.signal),
        new Promise((resolve, reject) => { timer = setTimeout(() => { controller.abort(); reject(new Error("Tempo do ciclo de correção excedido.")); }, remaining); }),
      ]);
    } finally { clearTimeout(timer); }
  };
  for (;;) {
    const report = await bounded(verify);
    const result = advance(state, report, now(), limits);
    record(result);
    if (result.action !== "repair") return result;
    state = result.state;
    await bounded(repair, result);
  }
}

export function command(program, args, options = {}) {
  const result = spawnSync(program, args, {
    cwd: root, encoding: "utf8", timeout: 60000, killSignal: "SIGKILL", maxBuffer: 8 * 1024 * 1024, detached: process.platform !== "win32",
    env: { ...process.env, OPENSPEC_TELEMETRY: "0", DO_NOT_TRACK: "1" }, ...options,
  });
  if (result.error && result.pid && process.platform !== "win32") {
    try { process.kill(-result.pid, "SIGKILL"); } catch {}
  }
  if (result.error || result.status !== 0) throw new Error(redact(`${program} falhou (${result.status ?? result.error?.code}): ${result.stderr || result.stdout || result.error?.message}`).slice(0, 4000));
  return result.stdout || "";
}

export function killGroup(child) {
  if (!child.pid) return;
  try {
    if (process.platform !== "win32") process.kill(-child.pid, "SIGKILL");
    else child.kill("SIGKILL");
  } catch (error) { if (error.code !== "ESRCH") throw error; }
}

export async function stopChild(child, grace = 1000) {
  if (child.exitCode !== null || child.signalCode !== null) { killGroup(child); return; }
  await new Promise((resolve) => {
    const timer = setTimeout(() => { killGroup(child); resolve(); }, grace);
    child.once("exit", () => { clearTimeout(timer); killGroup(child); resolve(); });
    child.kill("SIGTERM");
  });
}

export function runCommand(program, args, { timeout = 60000, cwd = root, env = process.env } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(program, args, { cwd, env: { ...env, OPENSPEC_TELEMETRY: "0", DO_NOT_TRACK: "1" }, detached: process.platform !== "win32", stdio: ["ignore", "pipe", "pipe"] });
    let output = "";
    let errors = "";
    let bytes = 0;
    const interrupted = () => fail(new Error(`${program}: verificação interrompida; grupo de processos encerrado.`));
    const cleanup = () => { clearTimeout(timer); process.off("SIGINT", interrupted); process.off("SIGTERM", interrupted); };
    const fail = (error) => { cleanup(); killGroup(child); reject(error); };
    const timer = setTimeout(() => fail(new Error(`${program} excedeu ${timeout} ms; grupo de processos encerrado.`)), timeout);
    process.once("SIGINT", interrupted);
    process.once("SIGTERM", interrupted);
    const append = (chunk, stderr) => {
      bytes += chunk.length;
      if (bytes > 8 * 1024 * 1024) { fail(new Error(`${program}: saída excedeu 8 MB.`)); return; }
      if (stderr) errors += chunk; else output += chunk;
    };
    child.stdout.on("data", (chunk) => append(chunk, false));
    child.stderr.on("data", (chunk) => append(chunk, true));
    child.on("error", fail);
    child.on("close", (code) => {
      cleanup();
      if (code !== 0) {
        const details = [output, errors].filter(Boolean).map((value) => redact(value).slice(-4000)).join("\n");
        reject(new Error(`${program} falhou (${code}): ${details}`));
      }
      else resolve(output);
    });
  });
}

export function changed(staged = false) {
  const names = command("git", ["diff", "--cached", "--name-only", "--diff-filter=ACMRDT", "-z"]);
  const extra = staged ? "" : command("git", ["diff", "--name-only", "--diff-filter=ACMRDT", "-z"]) + command("git", ["ls-files", "--others", "--exclude-standard", "-z"]);
  return [...new Set((names + extra).split("\0").filter((path) => path && !path.startsWith(".artifacts/")))].sort();
}

export function fingerprint(paths = changed()) {
  const hash = createHash("sha256");
  if (paths.length) hash.update(command("git", ["ls-files", "--stage", "-z", "--", ...paths]));
  for (const path of paths) {
    hash.update(path + "\0");
    const file = join(root, path);
    try {
      const stat = lstatSync(file);
      if (stat.isSymbolicLink()) hash.update("link:" + readlinkSync(file));
      else if (stat.isFile()) { hash.update("file:\0"); hash.update(readFileSync(file)); }
      else hash.update("other:\0");
    } catch (error) { if (error.code !== "ENOENT") throw error; hash.update("missing:\0"); }
  }
  return hash.digest("hex");
}

export function quick(staged = false) {
  const paths = changed(staged);
  command("git", ["diff", ...(staged ? ["--cached"] : []), "--check"]);
  const relevant = paths.filter((path) => !path.startsWith("docs/rich-mais-recente/"));
  if (relevant.length > 300) throw new Error("Mais de 300 arquivos alterados: divida a mudança antes de validar.");
  const deadline = Date.now() + 45000;
  for (const path of relevant) {
    if (Date.now() > deadline) throw new Error("Verificação rápida excedeu 45 segundos; divida a mudança.");
    if (staged ? spawnSync("git", ["cat-file", "-e", `:${path}`], { cwd: root, timeout: 5000 }).status !== 0 : !existsSync(join(root, path))) continue;
    if (/(^|\/)\.env(?:\.|$)|\.(db|p12|jks|key)$|^dist\/|^mobile\/www\/|finance\.wasm$/.test(path)) throw new Error(`Arquivo de dados, segredo ou gerado não permitido: ${path}`);
    if (!/\.(?:m?js|json|go|ya?ml)$/.test(path)) continue;
    if (!staged && lstatSync(join(root, path)).isSymbolicLink()) throw new Error(`Link de código exige revisão manual: ${path}`);
    const body = staged ? command("git", ["show", `:${path}`]) : readFileSync(join(root, path), "utf8");
    if (path.endsWith(".json")) JSON.parse(body);
    if (/\.ya?ml$/.test(path)) command(process.execPath, ["--input-type=module", "-e", 'import {parseDocument} from "./tools/engineering/node_modules/yaml/dist/index.js";let body="";for await(const chunk of process.stdin)body+=chunk;const doc=parseDocument(body);if(doc.errors.length)throw new Error(doc.errors.map(error=>error.message).join("\\n"));'], { input: body, timeout: 5000 });
    if (/\.m?js$/.test(path)) command(process.execPath, ["--check", "--input-type=module"], { input: body, timeout: 5000 });
    if (path.endsWith(".go") && command("gofmt", ["-d"], { input: body, timeout: 5000 }).trim()) throw new Error(`Formatação Go incorreta: ${path}`);
  }
  return paths;
}

export function save(name, value) {
  mkdirSync(artifacts, { recursive: true, mode: 0o700 });
  writeFileSync(join(artifacts, name), JSON.stringify(value, null, 2) + "\n", { mode: 0o600 });
}

async function check(scope) {
  save("verification.json", { status: "running", gates: [], evidence: [], at: new Date().toISOString() });
  const paths = quick();
  const gates = scope === "affected" ? classify(paths) : [scope];
  const allowed = ["backend", "frontend", "db", "tooling"];
  if (gates.some((gate) => !allowed.includes(gate))) throw new Error("Use check affected|backend|frontend|db|tooling.");
  const before = fingerprint(paths);
  const evidence = [];
  const startedAt = Date.now();
  save("verification.json", { status: "running", fingerprint: before, gates: [], evidence, at: new Date().toISOString() });
  const run = async (program, args) => {
    const remaining = 900000 - (Date.now() - startedAt);
    if (remaining <= 0) throw new Error("Limite total de quinze minutos atingido.");
    process.stdout.write(`Executando: ${program} ${args.join(" ")}\n`);
    const output = await runCommand(program, args, { timeout: remaining });
    process.stdout.write(redact(output));
    evidence.push({ command: [program, ...args], ok: true });
  };
  try {
    const staged = new Set(changed(true));
    const unstaged = command("git", ["diff", "--name-only", "-z"]).split("\0");
    if (unstaged.some((path) => staged.has(path))) throw new Error("Índice e arquivo de trabalho diferem no mesmo caminho. Teste a versão que será commitada antes de aprovar.");
    if (gates.includes("tooling")) {
      await run(process.execPath, ["--test", "scripts/engineering.test.mjs", ...["scripts/document-assets.test.mjs", "scripts/ios-validation.test.mjs", "scripts/ai-lab.test.mjs"].filter((file) => existsSync(join(root, file)))]);
      await run(join(toolbin, "openspec"), ["validate", "--all", "--strict"]);
    }
    if (gates.includes("backend")) await run("make", ["test"]);
    if (gates.includes("db")) await run("sh", ["scripts/test-identity-db.sh"]);
    if (gates.includes("frontend")) await run("make", ["e2e"]);
    const after = fingerprint();
    if (before !== after) throw new Error("Arquivos mudaram durante os testes; execute novamente sobre a versão final.");
    save("verification.json", { status: "passed", fingerprint: after, gates, evidence, at: new Date().toISOString() });
    console.log("Gates aprovados. Isso não substitui revisão humana, Safari real ou testes com credenciais.");
  } catch (error) {
    save("verification.json", { status: "failed", fingerprint: before, gates, evidence, issue: redact(error.message), at: new Date().toISOString() });
    throw error;
  }
}

function installHooks() {
  const current = spawnSync("git", ["config", "--get", "core.hooksPath"], { cwd: root, encoding: "utf8" });
  if (current.status === 0 && current.stdout.trim() !== ".githooks") throw new Error("Já existe core.hooksPath; integre manualmente sem sobrescrevê-lo.");
  const existing = join(root, ".git/hooks/pre-commit");
  if (!current.stdout.trim() && existsSync(existing)) throw new Error("Já existe pre-commit local; integre manualmente sem sobrescrevê-lo.");
  command("git", ["config", "--local", "core.hooksPath", ".githooks"]);
  console.log("Pre-commit local ativado. Hooks nativos de IA exigem confiança e nova sessão.");
}

async function main() {
  const [action, scope] = process.argv.slice(2);
  if (action === "quick") { quick(scope === "--staged"); console.log("Verificação rápida aprovada; testes funcionais são separados."); }
  else if (action === "check") await check(scope || "affected");
  else if (action === "install-hooks") installHooks();
  else if (action === "doctor") {
    for (const name of ["openspec", "playwright-mcp", "chrome-devtools-mcp"]) console.log(name + ": " + command(join(toolbin, name), ["--version"]).trim());
    console.log("Hooks Git: " + command("git", ["config", "--get", "core.hooksPath"]).trim());
    console.log("MCP: execute node scripts/engineering-browser-proof.mjs para testar conexão e navegador, não só configuração.");
  } else throw new Error("Use quick [--staged], check affected|backend|frontend|db|tooling, install-hooks ou doctor.");
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => { console.error(redact(error.message)); process.exitCode = 1; });
}
