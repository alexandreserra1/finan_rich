// Exportar e backup: planilha do Excel, CSV, relatório em PDF e cópia de
// segurança de tudo. Funciona sem conexão (os arquivos são gerados aqui).
import * as db from "../db.js";
import * as finance from "../finance.js";
import * as sync from "../sync.js";
import { isApp } from "../device.js";
import { exportData } from "../exporter.js";
import { html } from "../ui.js";
import { pageHead } from "./dashboard.js";

const TYPE_LABEL = { category: "categorias", transaction: "lançamentos", debt: "dívidas", pref: "preferências", note: "notas", goal: "metas", recurring: "recorrentes" };

export const needsData = false; // o backup também serve para restaurar num aparelho vazio

export function render({ month, ledger }) {
  const count = (list) => list.filter((r) => !r.deleted).length;
  return html`
    ${pageHead("Exportar e backup")}
    <section class="panel">
      <h2>Exportar seus dados</h2>
      <p class="muted">Leve seus dados para o Excel, Google Planilhas, pandas, ou mande o relatório para alguém. Os arquivos são gerados neste aparelho, mesmo sem internet.</p>
      <form data-form="export" class="export-form">
        <fieldset class="period">
          <legend>Período</legend>
          <label class="check"><input type="radio" name="scope" value="month" checked> Mês</label>
          <label class="check"><input type="radio" name="scope" value="year"> Ano inteiro</label>
          <label class="check"><input type="radio" name="scope" value="all"> Todo o histórico</label>
          <input type="month" name="month" value="${month}" aria-label="Mês ou ano de referência">
        </fieldset>
        <div class="export-options">
          <div class="export-card">
            <strong>Planilha do Excel</strong>
            <small class="muted">Abas: resumo por mês, lançamentos, gastos por categoria, divisão do salário, dívidas e notas. Valores e datas prontos para somar e filtrar.</small>
            <button type="button" data-format="xlsx">Baixar .xlsx</button>
          </div>
          <div class="export-card">
            <strong>Relatório em PDF</strong>
            <small class="muted">Resumo, divisão do salário com barras, gastos por categoria, lançamentos, dívidas e notas. Bom para imprimir ou enviar.</small>
            <button type="button" data-format="pdf">Baixar PDF</button>
          </div>
          <div class="export-card">
            <strong>CSV</strong>
            <small class="muted">Uma tabela por arquivo. Separador ";" e vírgula decimal (abre direto no Excel em português). No pandas: <code>read_csv(f, sep=";", decimal=",")</code>.</small>
            <select name="dataset" aria-label="Qual tabela">
              <option value="transactions">Lançamentos</option>
              <option value="months">Resumo por mês</option>
              <option value="debts">Dívidas</option>
              <option value="notes">Notas</option>
              <option value="categories">Categorias</option>
            </select>
            <button type="button" data-format="csv" class="secondary">Baixar CSV</button>
          </div>
        </div>
      </form>
    </section>

    <section class="panel" id="backup">
      <h2>Backup</h2>
      <p class="muted">Uma cópia de tudo (${count(ledger.transactions)} lançamentos, ${count(ledger.debts)} dívidas, ${count(ledger.notes)} notas e suas categorias) num arquivo <code>.json</code>. Guarde no Drive, no e-mail ou num pendrive.</p>
      <div class="row-actions">
        <button type="button" data-action="backup">Baixar backup</button>
        <label class="button secondary file-button">Restaurar backup… <input type="file" name="restore" accept=".json,application/json" class="visually-hidden"></label>
      </div>
      <p class="small muted">Restaurar traz de volta tudo o que estava no backup (inclusive o que foi apagado depois) e mantém o que você criou depois dele. Os outros aparelhos recebem na próxima sincronização.</p>
      ${isApp ? "" : html`<p class="small muted">Backup completo do computador, com as configurações da IA e a conversa com o consultor: <a href="/configuracoes/backup.db">baixar o banco (finan.db)</a>.</p>`}
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
      const stamp = Date.now();
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
  for (const [type, list] of [["category", ledger.categories], ["transaction", ledger.transactions], ["debt", ledger.debts], ["pref", ledger.prefs], ["note", ledger.notes], ["goal", ledger.goals], ["recurring", ledger.recurring]]) {
    for (const r of list) yield [type, r];
  }
}
