// Exportar e backup: planilha do Excel, CSV, relatório em PDF e cópia de
// segurança de tudo. Funciona sem conexão (os arquivos são gerados aqui).
import * as db from "../db.js";
import * as finance from "../finance.js";
import * as sync from "../sync.js";
import { isApp } from "../device.js";
import { exportData } from "../exporter.js";
import { html, raw } from "../ui.js";
import { pageHead } from "./dashboard.js";

const TYPE_LABEL = { category: "categorias", transaction: "lançamentos", debt: "dívidas", pref: "preferências", note: "notas", goal: "metas", recurring: "recorrentes", account: "contas" };

export const needsData = false; // o backup também serve para restaurar num aparelho vazio

const ICON = {
  up: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V3M7 8l5-5 5 5M4 15v6h16v-6"/></svg>'),
  down: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12M7 10l5 5 5-5M4 15v6h16v-6"/></svg>'),
};

export function render({ month, ledger }) {
  const count = (list) => list.filter((r) => !r.deleted).length;
  return html`
    ${pageHead("Seus dados vão com você.", null, { sub: "Exporte relatórios ou guarde uma cópia para recuperar depois." })}
    <section class="export-cols">
      <section class="panel">
        <h2 class="big-h2">Relatórios e planilhas</h2>
        <p class="muted">Gerados neste aparelho, mesmo sem internet. Escolha o período e o formato.</p>
        <form data-form="export" class="export-form">
          <div class="field-grid">
            <label class="field">Período <select name="scope">
              <option value="month">Mês escolhido</option><option value="year">Ano inteiro</option><option value="all">Todo o histórico</option>
            </select></label>
            <label class="field">Mês ou ano de referência <input type="month" name="month" value="${month}"></label>
            <label class="field wide">Tabela para o CSV <select name="dataset">
              <option value="transactions">Lançamentos</option>
              <option value="months">Resumo por mês</option>
              <option value="debts">Dívidas</option>
              <option value="notes">Notas</option>
              <option value="categories">Categorias</option>
            </select></label>
          </div>
          <button type="button" data-format="csv" class="secondary wide-btn">${ICON.up}Baixar CSV</button>
          <button type="button" data-format="xlsx" class="wide-btn">${ICON.up}Excel com abas (.xlsx)</button>
          <button type="button" data-format="pdf" class="secondary wide-btn">${ICON.up}Relatório em PDF</button>
        </form>
        <p class="faint tiny">O Excel traz abas de resumo por mês, lançamentos, gastos por categoria, divisão do salário, dívidas e notas. O CSV usa ";" e vírgula decimal (abre direto no Excel em português; no pandas: <code>read_csv(f, sep=";", decimal=",")</code>).</p>
      </section>

      <section class="panel" id="backup">
        <h2 class="big-h2">Backup não é sincronização.</h2>
        <p class="muted">A sincronização deixa seus aparelhos iguais. O backup é uma cópia guardada à parte, para voltar atrás se algo der errado.</p>
        <p class="tip info"><b>Uma cópia agora pode poupar um susto depois.</b><br>Inclui ${count(ledger.transactions)} lançamentos, ${count(ledger.debts)} dívidas, ${count(ledger.notes)} notas, metas, contas e categorias, num arquivo <code>.json</code>. Guarde no Drive, no e-mail ou num pendrive.</p>
        <button type="button" data-action="backup" class="wide-btn">${ICON.up}Baixar backup (.json)</button>
        <label class="button secondary file-button wide-btn">${ICON.down}Restaurar backup… <input type="file" name="restore" accept=".json,application/json" class="visually-hidden"></label>
        <p class="faint tiny">Restaurar traz de volta tudo o que estava no backup (inclusive o que foi apagado depois) e mantém o que você criou depois dele. Os outros aparelhos recebem na próxima sincronização.</p>
        ${isApp ? "" : html`<p class="faint tiny">Backup completo do computador, com as configurações da IA e a conversa com o consultor: <a href="/configuracoes/backup.db">baixar o banco (finan.db)</a>.</p>`}
      </section>
    </section>`;
}

export function bind(root, ctx) {
  const form = root.querySelector('[data-form="export"]');
  const options = (format) => {
    const f = form.elements;
    return { format, scope: f.scope.value, month: f.month.value || ctx.month, dataset: f.dataset.value };
  };
  const run = async (button, fn) => {
    button.disabled = true;
    try {
      const file = await fn();
      if (file) ctx.flash(`Arquivo gerado: ${file.name}`);
    } catch (err) {
      ctx.error(err);
    } finally {
      button.disabled = false;
    }
  };

  root.querySelectorAll("[data-format]").forEach((btn) => btn.addEventListener("click", () =>
    run(btn, () => exportData(ctx.ledger, options(btn.dataset.format)))));

  const backupBtn = root.querySelector('[data-action="backup"]');
  backupBtn.addEventListener("click", () => run(backupBtn, () => exportData(ctx.ledger, { format: "json" })));

  root.querySelector('input[name="restore"]').addEventListener("change", async (e) => {
    const file = e.target.files[0];
    e.target.value = "";
    if (!file) return;
    try {
      const records = finance.readBackup(await file.text());
      const counts = {};
      for (const r of records) counts[r.type] = (counts[r.type] || 0) + 1;
      const summary = Object.entries(counts).map(([t, n]) => `${n} ${TYPE_LABEL[t] || t}`).join(", ");
      if (!confirm(`Restaurar ${summary} do arquivo ${file.name}?`)) return;
      // Cada registro volta como uma alteração nova (hora de agora), para
      // valer sobre o que mudou depois do backup, aqui e nos outros aparelhos.
      const current = new Map([...listAll(ctx.ledger)].map(([type, r]) => [`${type}:${r.id}`, r]));
      const stamp = sync.stamp(0);
      const items = records.map(({ type, record }) => {
        const existing = current.get(`${type}:${record.id}`);
        const updatedAt = Math.max(stamp, (existing?.updatedAt || 0) + 1);
        return { type, record: { ...record, updatedAt, deleted: undefined } };
      });
      await db.saveManyLocal(items);
      sync.soon();
      await ctx.refresh(`Backup restaurado: ${summary}.`);
    } catch (err) {
      ctx.error(err);
    }
  });
}

function* listAll(ledger) {
  for (const [type, list] of [["category", ledger.categories], ["transaction", ledger.transactions], ["debt", ledger.debts], ["pref", ledger.prefs], ["note", ledger.notes], ["goal", ledger.goals], ["recurring", ledger.recurring], ["account", ledger.accounts]]) {
    for (const r of list) yield [type, r];
  }
}
