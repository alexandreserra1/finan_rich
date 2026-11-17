import { spawn } from "node:child_process";
import { existsSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { root, artifacts } from "./engineering.mjs";

const mode = process.argv[2];
const definitions = {
  playwright: ["playwright-mcp", "--headless", "--isolated", "--browser", "chrome", "--allowed-origins", "http://127.0.0.1:*;http://localhost:*", "--output-dir", artifacts, "--timeout-action", "10000", "--timeout-navigation", "30000", "--idle-timeout", "300000"],
  devtools: ["chrome-devtools-mcp", "--headless", "--isolated", "--no-usage-statistics", "--no-performance-crux", "--redact-network-headers", "--workspace", artifacts],
};
if (!definitions[mode]) throw new Error("Escolha playwright ou devtools.");
const [binary, ...args] = definitions[mode];
const executable = join(root, "tools/engineering/node_modules/.bin", binary);
if (!existsSync(executable)) throw new Error("Instale as ferramentas: npm ci --prefix tools/engineering --ignore-scripts.");
mkdirSync(artifacts, { recursive: true, mode: 0o700 });
const env = Object.fromEntries(["PATH", "HOME", "TMPDIR", "USER", "LOGNAME", "SystemRoot"].filter((name) => process.env[name]).map((name) => [name, process.env[name]]));
Object.assign(env, { CHROME_DEVTOOLS_MCP_NO_UPDATE_CHECKS: "1", CHROME_DEVTOOLS_MCP_NO_USAGE_STATISTICS: "1", DO_NOT_TRACK: "1" });
const child = spawn(executable, args, { cwd: root, env, stdio: "inherit" });
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => child.kill(signal));
child.on("error", (error) => { console.error(error.message); process.exitCode = 1; });
child.on("exit", (code) => { process.exitCode = code ?? 1; });
