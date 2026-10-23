// Importar extrato (OFX ou CSV do banco): o arquivo é lido neste aparelho
// pelas regras em Go, que sugerem a categoria de cada lançamento (pelo seu
// histórico e por palavras-chave) e marcam duplicados. A pessoa revisa e
// importa. "Sugerir com IA" é opcional e manda só as descrições ao computador.
import * as db from "../db.js";
import * as finance from "../finance.js";
import * as sync from "../sync.js";
import { endpoint } from "../device.js";
import { isPDF, pdfText } from "../pdftext.js";
import * as openFinance from "../openfinance.js";
import * as bankNotes from "../banknotes.js";
import { BUCKETS, html, shiftMonth, today } from "../ui.js";
import { pageHead } from "./dashboard.js";

export const needsData = false;

// O extrato em revisão fica aqui (e não na tela): a tela é redesenhada quando
// chegam dados da sincronização, e a revisão não pode se perder.
let state = null; // { fileName, base64, text (PDF) ou statement (Open Finance), preview, aiMessage, accountId } ou { bank: [{ name, statement }] }

const liveAccounts = (ledger) => ledger.accounts.filter((a) => !a.deleted).sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));

// guessAccount: a conta já usada com este arquivo (pelo número no OFX) ou,
// numa fatura, o único cartão cadastrado.
async function guessAccount(ledger, preview) {
  const accounts = liveAccounts(ledger);
  const known = preview.account ? (await db.getMeta("import-accounts", {}))[preview.account] : "";
  if (known && accounts.some((a) => a.id === known)) return known;
  const cards = accounts.filter((a) => a.kind === "cartao");
  return preview.cardBill && cards.length === 1 ? cards[0].id : "";
}

// buildPreview lê o arquivo guardado no estado (OFX/CSV em base64, ou o texto
// já extraído do PDF) com as opções da tela.
function buildPreview(ledger, s, options) {
  if (s.statement) return finance.importPreviewStatement(ledger, s.statement, options);
  return s.text != null
    ? finance.importPreviewText(ledger, s.text, options, today())
    : finance.importPreview(ledger, s.base64, options);
}

const STATUS = {
  novo: "",
  importado: "já importado",
  duplicado: "parece já lançado",
  fatura: "pagamento de fatura",
};
const REASON = { "histórico": "pelo seu histórico", "palavra-chave": "pela descrição", arquivo: "do arquivo", ia: "sugerido pela IA" };

const fmtDate = (d) => `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}`;

function categoryOptions(ledger, kind, selected) {
  const income = kind === "entrada";
  const cats = ledger.categories.filter((c) => !c.deleted && (c.bucket === "renda") === income)
    .sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
  return html`<option value="">Escolha…</option>${cats.map((c) => html`<option value="${c.id}" ${c.id === selected ? "selected" : ""}>${c.name}${income ? "" : ` · ${BUCKETS[c.bucket]}`}</option>`)}`;
}

export function render({ ledger }) {
  if (!state) return renderUpload();
  if (state.bank) return renderBank();
  const { preview } = state;
  const items = preview.items;
  const selected = items.filter((i) => i.selected);
  const missing = selected.filter((i) => !i.categoryId).length;
  const counts = {};
  for (const it of items) counts[it.status] = (counts[it.status] || 0) + 1;
  const m = finance.money;
  return html`
    ${pageHead("Importar extrato")}
    <section class="panel">
      <div class="panel-head">
        <h2>${state.fileName}</h2>
        <button type="button" class="secondary" data-action="cancel">Escolher outro arquivo</button>
      </div>
      <p class="muted">${items.length} lançamentos de ${fmtDate(preview.from)} a ${fmtDate(preview.to)}${counts.importado ? ` · ${counts.importado} já importados antes` : ""}${counts.duplicado ? ` · ${counts.duplicado} parecem já lançados à mão` : ""}${counts.fatura ? ` · ${counts.fatura} pagamento de fatura` : ""}.
        Os desmarcados não entram; revise e marque se quiser.</p>
      ${preview.cardBill ? html`<p class="notice">${preview.inverted ? "Parece uma fatura de cartão: os valores positivos foram tratados como gastos." : "Este arquivo parece uma fatura de cartão."} Os pagamentos da fatura vêm desmarcados, porque os gastos já aparecem um a um.</p>` : ""}
      <label class="check"><input type="checkbox" data-action="invert" ${preview.inverted ? "checked" : ""}> Inverter entradas e saídas (use se os gastos aparecerem como entradas)</label>
      ${ledger.accounts.some((a) => !a.deleted) ? html`<label class="import-account">De qual conta ou cartão é este arquivo?
        <select data-action="account">
          <option value="">Nenhuma</option>
          ${liveAccounts(ledger).map((a) => html`<option value="${a.id}" ${a.id === state.accountId ? "selected" : ""}>${a.name}</option>`)}
        </select></label>` : ""}

      <div class="table-wrap"><table class="import-table cards-mobile">
        <thead><tr><th><input type="checkbox" data-action="all" aria-label="Marcar todos" ${selected.length && selected.length === items.filter((i) => i.status !== "importado").length ? "checked" : ""}></th>
          <th>Data</th><th>Descrição</th><th class="num">Valor</th><th>Categoria</th></tr></thead>
        <tbody>
          ${items.map((it, i) => html`<tr class="${it.selected ? "" : "off"}">
            <td><input type="checkbox" data-row="${i}" data-field="selected" ${it.selected ? "checked" : ""} ${it.status === "importado" ? "disabled" : ""} aria-label="Importar este lançamento"></td>
            <td data-label="Data" class="nowrap">${fmtDate(it.date)}</td>
            <td class="primary"><input data-row="${i}" data-field="description" value="${it.description}" aria-label="Descrição">
              ${STATUS[it.status] ? html`<small class="tag status-${it.status}">${STATUS[it.status]}</small>` : ""}</td>
            <td data-label="Valor" class="num nowrap ${it.kind === "entrada" ? "pos" : "neg"}">${it.kind === "entrada" ? "+" : "−"}${m(it.amountCents)}</td>
            <td data-label="Categoria"><select data-row="${i}" data-field="categoryId" class="${it.selected && !it.categoryId ? "needs" : ""}" aria-label="Categoria">${categoryOptions(ledger, it.kind, it.categoryId)}</select>
              ${it.reason && it.categoryId ? html`<small class="muted">${REASON[it.reason] || it.reason}</small>` : ""}</td>
          </tr>`)}
        </tbody>
      </table></div>

      ${state.aiMessage ? html`<p class="small ${state.aiMessage.startsWith("❌") ? "neg" : "muted"}">${state.aiMessage}</p>` : ""}
      <div class="row-actions">
        <button type="button" data-action="import" ${selected.length ? "" : "disabled"}>Importar ${selected.length} lançamento${selected.length === 1 ? "" : "s"}</button>
        ${missing ? html`<button type="button" class="secondary" data-action="ai">Sugerir com IA (${missing} sem categoria)</button>` : ""}
      </div>
      ${missing ? html`<p class="small muted">${missing} marcado${missing === 1 ? "" : "s"} sem categoria: escolha na lista ou peça à IA. A IA roda no computador e recebe só as descrições, não os valores.</p>` : ""}
    </section>`;
}

// renderBank: as contas que vieram do Open Finance; escolhe uma para revisar.
function renderBank() {
  return html`
    ${pageHead("Importar extrato")}
    <section class="panel">
      <div class="panel-head">
        <h2>Do banco (Open Finance)</h2>
        <button type="button" class="secondary" data-action="cancel">Voltar</button>
      </div>
      ${state.bank.length
        ? html`<p class="muted">Escolha a conta ou o cartão para revisar os lançamentos:</p>
          <div class="bank-accounts">${state.bank.map((a, i) => html`<button type="button" class="secondary" data-bank="${i}">${a.name} <small class="muted">${a.statement.Rows?.length || 0} lançamentos</small></button>`)}</div>`
        : html`<p class="empty">Nenhuma conta veio da Pluggy. Confira os Item IDs em Ajustes.</p>`}
    </section>`;
}

function renderUpload() {
  return html`
    ${pageHead("Importar extrato")}
    ${bankNotes.available() ? html`<section class="panel">
      <h2>Notificações do banco</h2>
      <p class="muted">Cada compra ou Pix que o app do banco avisar vira um lançamento para você conferir aqui. As notificações são lidas neste celular e não saem dele.</p>
      <div class="row-actions">
        <button type="button" data-action="bank-notes">Revisar notificações</button>
        <button type="button" class="secondary" data-action="bank-notes-allow">Permitir ler as notificações</button>
      </div>
    </section>` : ""}
    <section class="panel">
      <h2>Direto do banco</h2>
      <p class="muted">Com o Open Finance configurado em Ajustes (Meu Pluggy, grátis), os lançamentos vêm do banco sem arquivo.</p>
      <form data-form="openfinance" class="extra-form">
        <label>Desde <input type="date" name="from" value="${shiftMonth(today().slice(0, 7), -1)}-01" required></label>
        <button type="submit">Buscar do banco</button>
      </form>
    </section>
    <section class="panel">
      <h2>Traga os lançamentos do banco</h2>
      <p class="muted">Exporte o extrato da conta ou a fatura do cartão no app ou site do banco e escolha o arquivo aqui. O finan_rich sugere a categoria de cada lançamento, aprende com as suas correções e avisa o que já foi importado. O arquivo é lido neste aparelho e não vai para a internet.</p>
      <label class="button file-button primary-file">Escolher arquivo (OFX, CSV ou fatura em PDF)… <input type="file" name="statement" accept=".ofx,.qfx,.csv,.txt,.pdf,text/csv,application/x-ofx,application/pdf" class="visually-hidden"></label>
      <details class="banks">
        <summary>Onde encontro o extrato no meu banco?</summary>
        <ul class="small">
          <li><strong>Nubank</strong>: no app, Conta → Extrato → ícone de compartilhar → OFX ou CSV. Fatura do cartão: no site (app.nubank.com.br) → Cartão → Exportar fatura.</li>
          <li><strong>Itaú, Bradesco, Banco do Brasil, Santander, Caixa</strong>: no internet banking, Extrato → Exportar / Salvar como → OFX (Money) ou CSV/Excel.</li>
          <li><strong>Inter, C6, PicPay, Mercado Pago</strong>: no app, Extrato → Exportar → CSV, OFX ou Excel (salve como CSV).</li>
          <li><strong>Fatura do cartão em PDF</strong> (quando o banco só oferece PDF): escolha o PDF da fatura. As compras são lidas do texto; confira as datas e os valores antes de importar. PDF com senha também funciona (a senha não é guardada).</li>
          <li>Planilha própria: salve como CSV com colunas de <em>data</em>, <em>descrição</em> e <em>valor</em> (gastos negativos ou uma coluna <em>tipo</em>).</li>
        </ul>
      </details>
    </section>`;
}

export function bind(root, ctx) {
  const rerender = () => ctx.rerender();
  root.querySelector('input[name="statement"]')?.addEventListener("change", async (e) => {
    const file = e.target.files[0];
    e.target.value = "";
    if (!file) return;
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      const next = { fileName: file.name, aiMessage: "" };
      if (isPDF(bytes)) {
        ctx.flash("Lendo o PDF…");
        next.text = await pdfText(bytes, (msg) => prompt(msg));
      } else {
        next.base64 = toBase64(bytes);
      }
      next.preview = buildPreview(ctx.ledger, next, {});
      next.accountId = await guessAccount(ctx.ledger, next.preview);
      state = next;
      rerender();
    } catch (err) {
      state = null;
      ctx.error(err);
    }
  });
  root.querySelector('[data-form="openfinance"]')?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const button = e.target.querySelector("button");
    button.disabled = true;
    button.textContent = "Buscando no banco…";
    try {
      state = { bank: await openFinance.fetchAccounts(e.target.elements.from.value) };
      rerender();
    } catch (err) {
      button.disabled = false;
      button.textContent = "Buscar do banco";
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="bank-notes-allow"]')?.addEventListener("click", () => bankNotes.openSettings());
  root.querySelector('[data-action="bank-notes"]')?.addEventListener("click", async () => {
    try {
      if (!(await bankNotes.enabled())) {
        ctx.flash("Primeiro permita: Ajustes do Android → Acesso a notificações → finan_rich.");
        await bankNotes.openSettings();
        return;
      }
      const statement = await bankNotes.pending();
      if (!statement.Rows.length) {
        ctx.flash("Nenhuma compra ou Pix novo nas notificações do banco.");
        return;
      }
      const next = { fileName: "Notificações do banco", statement, aiMessage: "" };
      next.preview = buildPreview(ctx.ledger, next, {});
      state = next;
      rerender();
    } catch (err) {
      ctx.error(err);
    }
  });
  if (!state) return;
  if (state.bank) {
    root.querySelector('[data-action="cancel"]').addEventListener("click", () => { state = null; rerender(); });
    root.querySelectorAll("[data-bank]").forEach((b) => b.addEventListener("click", async () => {
      const { name, statement } = state.bank[Number(b.dataset.bank)];
      try {
        const next = { fileName: name, statement, aiMessage: "" };
        next.preview = buildPreview(ctx.ledger, next, {});
        next.accountId = await guessAccount(ctx.ledger, next.preview);
        state = next;
        rerender();
      } catch (err) {
        ctx.error(err);
      }
    }));
    return;
  }
  const items = state.preview.items;

  root.querySelector('[data-action="cancel"]').addEventListener("click", () => { state = null; rerender(); });
  root.querySelector('[data-action="invert"]').addEventListener("change", (e) => {
    try {
      state.preview = buildPreview(ctx.ledger, state, { invert: e.target.checked });
      rerender();
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="account"]')?.addEventListener("change", (e) => { state.accountId = e.target.value; });
  root.querySelector('[data-action="all"]').addEventListener("change", (e) => {
    for (const it of items) if (it.status !== "importado") it.selected = e.target.checked;
    rerender();
  });
  root.querySelectorAll("[data-row]").forEach((el) => el.addEventListener("change", () => {
    const it = items[Number(el.dataset.row)];
    const field = el.dataset.field;
    if (field === "selected") it.selected = el.checked;
    else if (field === "categoryId") { it.categoryId = el.value; it.reason = el.value ? "" : it.reason; }
    else it[field] = el.value.trim() || it[field];
    if (field !== "description") rerender();
  }));

  root.querySelector('[data-action="ai"]')?.addEventListener("click", async (e) => {
    const todo = items.filter((it) => it.selected && !it.categoryId);
    e.target.disabled = true;
    e.target.textContent = "Perguntando à IA… (no Ollama pode levar um minuto)";
    try {
      const asked = todo.map((it) => ({ description: it.description, kind: it.kind }));
      const api = await endpoint("/api/categorizar");
      let body;
      if (api) {
        const res = await fetch(api.url, {
          ...api.init, method: "POST",
          headers: { ...api.init.headers, "Content-Type": "application/json" },
          body: JSON.stringify({ items: asked }),
        });
        body = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(body.error || `o computador respondeu ${res.status}`);
      } else {
        // Sem computador: a IA na nuvem configurada neste celular (Ajustes).
        const cloud = await import("../cloud-ai.js");
        body = { categories: await cloud.categorize(ctx.ledger.categories, asked) };
      }
      let filled = 0;
      todo.forEach((it, i) => {
        if (body.categories?.[i]) { it.categoryId = body.categories[i]; it.reason = "ia"; filled++; }
      });
      state.aiMessage = `A IA sugeriu ${filled} de ${todo.length}. Confira antes de importar.`;
    } catch (err) {
      state.aiMessage = "❌ " + (err instanceof TypeError ? "sem conexão com o computador; a IA roda lá" : err.message);
    }
    rerender();
  });

  root.querySelector('[data-action="import"]').addEventListener("click", async () => {
    const chosen = items.filter((it) => it.selected);
    if (chosen.some((it) => !it.categoryId)) {
      ctx.error(new Error("escolha a categoria dos lançamentos marcados (ou desmarque-os)"));
      return;
    }
    try {
      const now = Date.now();
      const records = chosen.map((it) => ({
        type: "transaction",
        record: finance.validate("transaction", {
          id: it.id, updatedAt: now, kind: it.kind, description: it.description,
          amountCents: it.amountCents, categoryId: it.categoryId, date: it.date,
          ...(state.accountId ? { accountId: state.accountId } : {}),
        }),
      }));
      await db.saveManyLocal(records);
      if (state.statement?.Format === "notificacao") await bankNotes.reviewed();
      if (state.preview.account && state.accountId) { // da próxima vez este arquivo já vem com a conta certa
        await db.setMeta("import-accounts", { ...(await db.getMeta("import-accounts", {})), [state.preview.account]: state.accountId });
      }
      sync.soon();
      const latest = chosen.map((it) => it.date).sort().at(-1).slice(0, 7);
      state = null;
      await ctx.refresh();
      ctx.go(`/lancamentos?mes=${latest}`);
      ctx.flash(`${chosen.length} lançamento${chosen.length === 1 ? " importado" : "s importados"}.`);
    } catch (err) {
      ctx.error(err);
    }
  });
}

function toBase64(bytes) {
  let s = "";
  for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(s);
}
