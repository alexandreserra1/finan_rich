import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtempSync, writeFileSync, readFileSync, rmSync, mkdirSync, copyFileSync, chmodSync } from "node:fs";
import { spawn, spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { advance, classify, redact, root, runControlled, runCommand, stopChild } from "./engineering.mjs";

test("falhas preservam stdout e stderr sem expor credenciais", async () => {
  await assert.rejects(runCommand(process.execPath, ["-e", 'console.log("CAUSA: contrato inválido; Bearer segredo-ficticio"); console.error("AVISO: processo encerrado"); process.exit(1)']), (error) => {
    assert.match(error.message, /CAUSA: contrato inválido/);
    assert.match(error.message, /AVISO: processo encerrado/);
    assert.doesNotMatch(error.message, /segredo-ficticio/);
    return true;
  });
});

test("a primeira falha pede investigação, não afirma sucesso", () => {
  const result = advance(null, { ok: false, fingerprint: "a", issue: "sintaxe" }, 100);
  assert.equal(result.action, "repair");
  assert.equal(result.state.attempts, 1);
});

test("a mesma falha sem mudança interrompe o ciclo", () => {
  const first = advance(null, { ok: false, fingerprint: "a", issue: "sintaxe" }, 100);
  assert.equal(advance(first.state, { ok: false, fingerprint: "a", issue: "sintaxe" }, 110).action, "stop");
});

test("cinco pedidos de correção são o limite", () => {
  let state;
  for (let attempt = 0; attempt < 5; attempt++) {
    const result = advance(state, { ok: false, fingerprint: String(attempt), issue: "teste" }, 100 + attempt);
    assert.equal(result.action, "repair");
    state = result.state;
  }
  assert.equal(advance(state, { ok: false, fingerprint: "novo", issue: "teste" }, 110).action, "stop");
});

test("quinze minutos encerram a correção; sucesso posterior é verificado", () => {
  const first = advance(null, { ok: false, fingerprint: "a", issue: "teste" }, 100);
  assert.equal(advance(first.state, { ok: false, fingerprint: "b", issue: "teste" }, 900101).action, "stop");
  assert.equal(advance(first.state, { ok: true, fingerprint: "b" }, 120).action, "pass");
});

test("classifica contratos sem confundir documentação com código do app", () => {
  assert.deepEqual(classify(["docs/README.md"]), []);
  assert.deepEqual(classify(["internal/finance/accounts.go", "internal/web/static/app.css", "site/schema.sql"]), ["backend", "frontend", "db"]);
  assert.deepEqual(classify(["scripts/engineering.mjs", "openspec/config.yaml"]), ["tooling"]);
});

test("não registra credenciais comuns nos diagnósticos", () => {
  const result = redact('Bearer secreto123 GOOGLE_CLIENT_SECRET=segredo123 password="senha"');
  assert.ok(!result.includes("secreto123") && !result.includes("segredo123") && !result.includes("senha"));
});

test("falha controlada é corrigida e verificada, fora do código de produção", async () => {
  const directory = mkdtempSync(join(tmpdir(), "rich-loop-"));
  const fixture = join(directory, "valor.txt");
  const events = [];
  try {
    writeFileSync(fixture, "incorreto");
    const result = await runControlled({
      verify: async () => ({ ok: readFileSync(fixture, "utf8") === "correto", fingerprint: readFileSync(fixture, "utf8"), issue: "fixture" }),
      repair: async () => writeFileSync(fixture, "correto"),
      record: (event) => events.push(event.action),
    });
    assert.equal(result.action, "pass");
    assert.deepEqual(events, ["repair", "pass"]);
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test("exceção do verificador não é transformada em sucesso", async () => {
  await assert.rejects(runControlled({ verify: async () => { throw new Error("indisponível"); }, repair: async () => {} }), /indisponível/);
});

function repository(run) {
  const directory = mkdtempSync(join(tmpdir(), "rich-gate-"));
  const execute = (program, args, input) => spawnSync(program, args, { cwd: directory, input, encoding: "utf8", timeout: 15000 });
  const git = (...args) => execute("git", ["-c", "commit.gpgsign=false", "-c", "user.name=Teste", "-c", "user.email=teste@localhost", ...args]);
  try {
    mkdirSync(join(directory, "scripts"));
    for (const name of ["engineering.mjs", "engineering-hooks.mjs"]) copyFileSync(join(root, "scripts", name), join(directory, "scripts", name));
    assert.equal(git("init", "-q").status, 0);
    writeFileSync(join(directory, "base.txt"), "fixture\n");
    assert.equal(git("add", ".").status, 0);
    assert.equal(git("commit", "-qm", "Base da fixture").status, 0);
    run({ directory, git, execute });
  } finally { rmSync(directory, { recursive: true, force: true }); }
}

test("pre-commit examina o índice: bom preparado passa mesmo com edição pendente inválida", () => repository(({ directory, git, execute }) => {
  const file = join(directory, "fixture.mjs");
  writeFileSync(file, "export const valor = 1;\n");
  assert.equal(git("add", "fixture.mjs").status, 0);
  writeFileSync(file, "export const valor = ;\n");
  assert.equal(execute(process.execPath, ["scripts/engineering.mjs", "quick", "--staged"]).status, 0);
  assert.notEqual(execute(process.execPath, ["scripts/engineering.mjs", "quick"]).status, 0);
}));

test("hook Git real impede commit inválido sem tocar no repositório principal", () => repository(({ directory, git }) => {
  mkdirSync(join(directory, ".githooks"));
  const hook = join(directory, ".githooks/pre-commit");
  copyFileSync(join(root, ".githooks/pre-commit"), hook);
  chmodSync(hook, 0o755);
  assert.equal(git("config", "core.hooksPath", ".githooks").status, 0);
  const file = join(directory, "fixture.mjs");
  writeFileSync(file, "export const valor = ;\n");
  assert.equal(git("add", "fixture.mjs").status, 0);
  writeFileSync(file, "export const valor = 1;\n");
  const rejected = git("commit", "-qm", "Fixture inválida");
  assert.notEqual(rejected.status, 0);
  assert.match(rejected.stderr, /SyntaxError/);
  assert.equal(git("add", "fixture.mjs").status, 0);
  assert.equal(git("commit", "-qm", "Fixture corrigida").status, 0);
}));

test("remoção preparada é válida e continua visível ao gate", () => repository(({ directory, git, execute }) => {
  writeFileSync(join(directory, "fixture.mjs"), "export const valor = 1;\n");
  git("add", "fixture.mjs");
  git("commit", "-qm", "Fixture válida");
  assert.equal(git("rm", "fixture.mjs").status, 0);
  assert.equal(execute(process.execPath, ["scripts/engineering.mjs", "quick", "--staged"]).status, 0);
}));

test("hooks de sessão pedem correção e interrompem falha repetida", () => repository(({ directory, execute }) => {
  const payload = JSON.stringify({ session_id: "fixture-session" });
  const start = execute(process.execPath, ["scripts/engineering-hooks.mjs", "start"], payload);
  assert.equal(start.status, 0);
  assert.equal(JSON.parse(start.stdout).hookSpecificOutput.hookEventName, "SessionStart");
  writeFileSync(join(directory, "fixture.mjs"), "export const valor = ;\n");
  const first = execute(process.execPath, ["scripts/engineering-hooks.mjs", "stop"], payload);
  assert.equal(JSON.parse(first.stdout).decision, "block");
  const second = execute(process.execPath, ["scripts/engineering-hooks.mjs", "stop"], payload);
  assert.match(JSON.parse(second.stdout).systemMessage, /reprovada/);
  writeFileSync(join(directory, "fixture.mjs"), "export const valor = 1;\n");
  const fixed = execute(process.execPath, ["scripts/engineering-hooks.mjs", "stop"], payload);
  assert.deepEqual(JSON.parse(fixed.stdout), {});
}));

test("sincronização, CI e automação exigem os gates relevantes", () => {
  assert.deepEqual(classify(["internal/web/api.go"]), ["backend", "frontend"]);
  assert.deepEqual(classify([".github/workflows/ci.yml", "Makefile", "scripts/test-identity-db.sh"]), ["backend", "frontend", "db", "tooling"]);
});

test("alteração preparada é vista mesmo quando o arquivo volta à versão de HEAD", () => repository(({ directory, git, execute }) => {
  const file = join(directory, "fixture.mjs");
  writeFileSync(file, "export const valor = 1;\n");
  git("add", "fixture.mjs"); git("commit", "-qm", "Fixture inicial");
  writeFileSync(file, "export const valor = 2;\n"); git("add", "fixture.mjs");
  writeFileSync(file, "export const valor = 1;\n");
  const result = execute(process.execPath, ["--input-type=module", "-e", 'import {changed} from "./scripts/engineering.mjs";console.log(JSON.stringify(changed()))']);
  assert.ok(JSON.parse(result.stdout).includes("fixture.mjs"));
}));

test("timeout encerra filho e neto que ignoram SIGTERM", async () => {
  const directory = mkdtempSync(join(tmpdir(), "rich-timeout-"));
  const pidFile = join(directory, "neto.txt");
  const started = Date.now();
  const grandchild = `require("fs").writeFileSync(${JSON.stringify(pidFile)},String(process.pid));process.on("SIGTERM",()=>{});setInterval(()=>{},1000)`;
  const source = `process.on("SIGTERM",()=>{});require("child_process").spawn(process.execPath,["-e",${JSON.stringify(grandchild)}],{stdio:"inherit"});setInterval(()=>{},1000)`;
  try {
    await assert.rejects(runCommand(process.execPath, ["-e", source], { timeout: 600 }), /excedeu/);
    assert.ok(Date.now() - started < 3000);
    const pid = Number(readFileSync(pidFile, "utf8"));
    let alive = true;
    for (let attempt = 0; attempt < 100 && alive; attempt++) {
      try { process.kill(pid, 0); await new Promise((resolve) => setTimeout(resolve, 10)); }
      catch (error) { assert.equal(error.code, "ESRCH"); alive = false; }
    }
    assert.equal(alive, false, "o neto também foi encerrado");
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test("callback travado é interrompido pelo prazo do controlador", async () => {
  await assert.rejects(runControlled({ verify: async () => new Promise(() => {}), repair: async () => {}, limits: { attempts: 5, duration: 30 } }), /excedido/);
});

test("aprovação real é invalidada por nova falha com o mesmo hash", () => repository(({ directory, execute }) => {
  const external = mkdtempSync(join(tmpdir(), "rich-dependency-"));
  const marker = join(external, "indisponivel");
  try {
    const bin = join(directory, "tools/engineering/node_modules/.bin");
    mkdirSync(bin, { recursive: true });
    const stub = join(bin, "openspec");
    writeFileSync(stub, `#!/bin/sh\nif [ -f ${JSON.stringify(marker)} ]; then exit 1; fi\nexit 0\n`);
    chmodSync(stub, 0o755);
    writeFileSync(join(directory, "scripts/engineering.test.mjs"), 'import "node:test";\n');
    const first = execute(process.execPath, ["scripts/engineering.mjs", "check", "tooling"]);
    assert.equal(first.status, 0, first.stderr);
    const proof = join(directory, ".artifacts/engineering/verification.json");
    const approved = JSON.parse(readFileSync(proof, "utf8"));
    assert.equal(approved.status, "passed");
    writeFileSync(marker, "falha externa simulada");
    const second = execute(process.execPath, ["scripts/engineering.mjs", "check", "tooling"]);
    assert.notEqual(second.status, 0);
    const failed = JSON.parse(readFileSync(proof, "utf8"));
    assert.equal(failed.fingerprint, approved.fingerprint);
    assert.equal(failed.status, "failed");
    const stop = execute(process.execPath, ["scripts/engineering-hooks.mjs", "stop"], JSON.stringify({ session_id: "fixture" }));
    assert.equal(JSON.parse(stop.stdout).decision, "block");
  } finally { rmSync(external, { recursive: true, force: true }); }
}));

test("lockfile das ferramentas tem somente licenças permitidas", () => {
  const lock = JSON.parse(readFileSync(join(root, "tools/engineering/package-lock.json"), "utf8"));
  const allowed = new Set(["MIT", "ISC", "0BSD", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "MPL-2.0"]);
  for (const [name, entry] of Object.entries(lock.packages)) if (name) assert.ok(allowed.has(entry.license), `${name}: ${entry.license}`);
});

test("compactação não transforma trabalho pendente em baseline aprovado", () => repository(({ directory, execute }) => {
  const payload = JSON.stringify({ session_id: "compact-fixture", source: "startup" });
  assert.equal(execute(process.execPath, ["scripts/engineering-hooks.mjs", "start"], payload).status, 0);
  writeFileSync(join(directory, "fixture.mjs"), "export const valor = ;\n");
  const compact = JSON.stringify({ session_id: "compact-fixture", source: "compact" });
  assert.equal(execute(process.execPath, ["scripts/engineering-hooks.mjs", "start"], compact).status, 0);
  const stopped = execute(process.execPath, ["scripts/engineering-hooks.mjs", "stop"], payload);
  assert.equal(JSON.parse(stopped.stdout).decision, "block");
}));

test("hash distingue arquivo vazio de removido", () => repository(({ directory, git, execute }) => {
  const file = join(directory, "vazio.txt");
  writeFileSync(file, ""); git("add", "vazio.txt"); git("commit", "-qm", "Fixture vazia");
  const args = ["--input-type=module", "-e", 'import {fingerprint} from "./scripts/engineering.mjs";console.log(fingerprint(["vazio.txt"]))'];
  const before = execute(process.execPath, args).stdout;
  rmSync(file);
  const after = execute(process.execPath, args).stdout;
  assert.notEqual(before, after);
}));

test("líder cooperativo não deixa neto vivo ao encerrar", async () => {
  const directory = mkdtempSync(join(tmpdir(), "rich-stop-"));
  const pidFile = join(directory, "neto.txt");
  const grandchild = `require("fs").writeFileSync(${JSON.stringify(pidFile)},String(process.pid));process.on("SIGTERM",()=>{});setInterval(()=>{},1000)`;
  const source = `require("child_process").spawn(process.execPath,["-e",${JSON.stringify(grandchild)}],{stdio:"inherit"});setInterval(()=>{},1000)`;
  const child = spawn(process.execPath, ["-e", source], { detached: process.platform !== "win32", stdio: "ignore" });
  try {
    let pid;
    for (let attempt = 0; attempt < 200 && !pid; attempt++) {
      try { pid = Number(readFileSync(pidFile, "utf8")); } catch {}
      if (!pid) await new Promise((resolve) => setTimeout(resolve, 10));
    }
    assert.ok(pid, "neto iniciou");
    await stopChild(child, 200);
    let alive = true;
    for (let attempt = 0; attempt < 100 && alive; attempt++) {
      try { process.kill(pid, 0); await new Promise((resolve) => setTimeout(resolve, 10)); }
      catch (error) { assert.equal(error.code, "ESRCH"); alive = false; }
    }
    assert.equal(alive, false);
  } finally { await stopChild(child, 100); rmSync(directory, { recursive: true, force: true }); }
});
