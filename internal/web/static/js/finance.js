// Carrega as regras do finan_rich (escritas em Go e compiladas para
// WebAssembly) e expõe funções JavaScript simples.
import "./wasm_exec.js";

let ready = null;

export function load() {
  ready ??= new Promise((resolve, reject) => {
    window.addEventListener("finan-ready", () => resolve(), { once: true });
    const go = new Go();
    const url = "/static/finance.wasm";
    const instantiate = WebAssembly.instantiateStreaming
      ? WebAssembly.instantiateStreaming(fetch(url), go.importObject).catch(async () =>
          WebAssembly.instantiate(await (await fetch(url)).arrayBuffer(), go.importObject))
      : fetch(url).then((r) => r.arrayBuffer()).then((b) => WebAssembly.instantiate(b, go.importObject));
    instantiate.then(({ instance }) => go.run(instance)).catch(reject);
  });
  return ready;
}

function unwrap(json) {
  const r = JSON.parse(json);
  if (r.error) throw new Error(r.error);
  return r.ok;
}

export const dashboard = (ledger, month) => unwrap(window.finan.dashboard(JSON.stringify(ledger), month));
export const debts = (ledger) => unwrap(window.finan.debts(JSON.stringify(ledger)));
// validate devolve o registro normalizado ou lança o erro (mesma regra do servidor).
export const validate = (type, record) => unwrap(window.finan.validate(type, JSON.stringify(record)));
export const parseMoney = (text) => unwrap(window.finan.parseMoney(text));
export const money = (cents) => window.finan.formatMoney(cents);
export const monthLabel = (key) => window.finan.monthLabel(key);
// exportFile gera planilha, CSV, PDF, nota ou backup: { name, mime, data (base64) }.
export const exportFile = (ledger, options) => unwrap(window.finan.export(JSON.stringify(ledger), JSON.stringify(options), Date.now()));
// readBackup confere um backup (.json) e devolve [{ type, record }] já validados.
export const readBackup = (text) => unwrap(window.finan.readBackup(text));
