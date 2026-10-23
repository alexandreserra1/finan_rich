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
export const debts = (ledger, extraCents = 0) => unwrap(window.finan.debts(JSON.stringify(ledger), extraCents));
// validate devolve o registro normalizado ou lança o erro (mesma regra do servidor).
export const validate = (type, record) => unwrap(window.finan.validate(type, JSON.stringify(record)));
export const parseMoney = (text) => unwrap(window.finan.parseMoney(text));
export const money = (cents) => window.finan.formatMoney(cents);
export const monthLabel = (key) => window.finan.monthLabel(key);
export const defaultCategories = (now) => unwrap(window.finan.defaultCategories(now));
// exportFile gera planilha, CSV, PDF, nota ou backup: { name, mime, data (base64) }.
export const exportFile = (ledger, options) => unwrap(window.finan.export(JSON.stringify(ledger), JSON.stringify(options), Date.now()));
// importPreview lê um extrato (OFX/CSV em base64) e devolve as linhas para revisar.
export const importPreview = (ledger, base64, options) => unwrap(window.finan.importPreview(JSON.stringify(ledger), base64, JSON.stringify(options || {})));
// importPreviewText faz o mesmo com o texto de uma fatura em PDF.
export const importPreviewText = (ledger, text, options, day) => unwrap(window.finan.importPreviewText(JSON.stringify(ledger), text, JSON.stringify(options || {}), day));
// notificationStatement transforma notificações de banco ([{id, date, text}]) num extrato.
export const notificationStatement = (items) => unwrap(window.finan.notificationStatement(JSON.stringify(items)));
// importPreviewStatement faz o mesmo com um extrato já pronto (Open Finance).
export const importPreviewStatement = (ledger, statement, options) => unwrap(window.finan.importPreviewStatement(JSON.stringify(ledger), JSON.stringify(statement), JSON.stringify(options || {})));
// readBackup confere um backup (.json) e devolve [{ type, record }] já validados.
export const readBackup = (text) => unwrap(window.finan.readBackup(text));
// Metas, recorrências e alertas dependem do dia de hoje (do aparelho).
export const goals = (ledger, day) => unwrap(window.finan.goals(JSON.stringify(ledger), day));
export const recurringDue = (ledger, day) => unwrap(window.finan.recurringDue(JSON.stringify(ledger), day));
export const alerts = (ledger, day) => unwrap(window.finan.alerts(JSON.stringify(ledger), day));
export const insights = (ledger, day) => unwrap(window.finan.insights(JSON.stringify(ledger), day));
// canDeleteCategory lança o erro se a categoria ainda tem lançamentos, recorrentes ou metas.
export const canDeleteCategory = (ledger, id) => unwrap(window.finan.canDeleteCategory(JSON.stringify(ledger), id));
// searchTransactions filtra os lançamentos (busca da tela Lançamentos) e soma o achado.
export const searchTransactions = (ledger, filter) => unwrap(window.finan.searchTransactions(JSON.stringify(ledger), JSON.stringify(filter)));
// Contas e cartões (saldo, fatura atual e próxima), patrimônio mês a mês e compra parcelada.
export const accounts = (ledger, day) => unwrap(window.finan.accounts(JSON.stringify(ledger), day));
export const netWorth = (ledger, day) => unwrap(window.finan.netWorth(JSON.stringify(ledger), day));
export const installments = (tx, n) => unwrap(window.finan.installments(JSON.stringify(tx), n));
