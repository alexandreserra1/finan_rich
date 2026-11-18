import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { advance, artifacts, changed, classify, fingerprint, quick, redact, save } from "./engineering.mjs";

let input = "";
for await (const chunk of process.stdin) {
  input += chunk;
  if (input.length > 1024 * 1024) throw new Error("Payload de hook excedeu 1 MB.");
}
const event = JSON.parse(input || "{}");
const name = createHash("sha256").update(String(event.session_id || "local")).digest("hex").slice(0, 24);
const file = `session-${name}.json`;
const location = join(artifacts, file);
const previous = existsSync(location) ? JSON.parse(readFileSync(location, "utf8")) : {};
const hook = process.argv[2];

if (hook === "start") {
  const paths = changed();
  save(file, { ...previous, baseline: previous.baseline || fingerprint(paths), loop: previous.loop || null });
  console.log(JSON.stringify({ hookSpecificOutput: { hookEventName: "SessionStart", additionalContext: `Engenharia Rich ativa. Leia AGENTS.md, docs/engineering/README.md e os contratos relevantes em openspec/specs. Consulte a memória quando depender de histórico. Existem ${paths.length} arquivos pendentes: preserve-os. Reuse skills e personas existentes. Ao concluir, rode node scripts/engineering.mjs check affected. Não faça push, merge ou deploy automático. MCPs: rich-playwright e rich-devtools (navegador isolado), graphify (mapa do código) e ai-memory (memória longa). Novos screenshots não são baselines aprovados.` } }));
} else if (hook === "stop") {
  let report;
  const paths = changed();
  const digest = fingerprint(paths);
  try {
    if (previous.baseline !== digest) {
      quick();
      const gates = classify(paths);
      const verificationFile = join(artifacts, "verification.json");
      const verified = existsSync(verificationFile) ? JSON.parse(readFileSync(verificationFile, "utf8")) : null;
      if (gates.length && (verified?.status !== "passed" || verified.fingerprint !== digest || !gates.every((gate) => verified.gates.includes(gate)))) throw new Error("Faltam gates atuais. Execute node scripts/engineering.mjs check affected; registre causa, hipótese e resultado em docs/engineering/known-issues.md quando houver bug relevante.");
    }
    report = { ok: true, fingerprint: digest };
  } catch (error) { report = { ok: false, fingerprint: digest, issue: redact(error.message) }; }
  const result = advance(previous.loop, report);
  const history = [...(previous.history || []), { action: result.action, issue: report.issue, fingerprint: digest, at: new Date().toISOString() }].slice(-6);
  save(file, { ...previous, loop: result.state, history });
  console.log(JSON.stringify(result.action === "repair" ? { decision: "block", reason: result.reason } : result.action === "stop" ? { systemMessage: result.reason } : {}));
} else throw new Error("Hook desconhecido.");
