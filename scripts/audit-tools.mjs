// Auditoria das ferramentas de desenvolvimento (tools/engineering): falha em toda
// vulnerabilidade alta ou crítica, menos as liberadas em ALLOWED. Cada liberação tem
// motivo e prazo; vencido o prazo, o CI volta a falhar e alguém precisa olhar de novo.
// Não vale para o app: mobile/ continua com "npm audit" puro no CI.
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const ALLOWED = [
  // picomatch <= 4.0.7 (ReDoS com extglob repetido). Em 17/11/2026 não existe versão
  // corrigida (4.0.7 é a mais nova). Só o OpenSpec usa, no CI e na máquina de quem
  // desenvolve, com padrões do próprio repositório; nada disso vai para o app ou o site.
  { id: "GHSA-fc5q-39xc-vjpf", until: "2026-12-17" },
];

const SERIOUS = ["high", "critical"];
const idOf = (via) => via.url?.split("/").pop() || via.title || String(via.source);

// blocking devolve o que ainda barra (pacote: aviso) e o que passou por estar liberado.
// O "via" de um pacote traz o aviso em si (objeto) ou o nome de outro pacote (texto):
// no segundo caso, o aviso aparece no próprio pacote de origem e é contado lá.
export function blocking(report, today, allowed = ALLOWED) {
  const block = [];
  const excused = [];
  for (const [name, item] of Object.entries(report.vulnerabilities || {})) {
    for (const via of item.via || []) {
      if (typeof via === "string" || !SERIOUS.includes(via.severity)) continue;
      const id = idOf(via);
      (allowed.some((a) => a.id === id && today <= a.until) ? excused : block).push(`${name}: ${id}`);
    }
  }
  return { block, excused };
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const run = spawnSync("npm", ["audit", "--json", "--prefix", "tools/engineering"], { encoding: "utf8" });
  let report;
  try {
    report = JSON.parse(run.stdout);
  } catch {
    console.error("npm audit não respondeu com JSON:", run.stderr.trim());
    process.exit(1);
  }
  if (report.error) {
    console.error("npm audit falhou:", report.error.summary || report.error);
    process.exit(1);
  }
  const { block, excused } = blocking(report, new Date().toISOString().slice(0, 10));
  for (const line of excused) console.log(`liberado com prazo (scripts/audit-tools.mjs): ${line}`);
  if (block.length) {
    console.error(`Vulnerabilidades altas ou críticas sem liberação:\n${block.join("\n")}`);
    process.exit(1);
  }
  console.log("Ferramentas sem vulnerabilidade alta ou crítica pendente.");
}
