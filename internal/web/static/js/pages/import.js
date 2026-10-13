// Importar extrato (OFX ou CSV do banco): o arquivo é lido neste aparelho
// pelas regras em Go, que sugerem a categoria de cada lançamento (pelo seu
// histórico e por palavras-chave) e marcam duplicados. A pessoa revisa e
// importa. "Sugerir com IA" é opcional e manda só as descrições ao computador.
import * as db from "../db.js";
import * as finance from "../finance.js";
import * as sync from "../sync.js";
import { endpoint } from "../device.js";
import { isPDF, pdfText } from "../pdftext.js";
import { BUCKETS, html, today } from "../ui.js";
import { pageHead } from "./dashboard.js";

export const needsData = false;

// O extrato em revisão fica aqui (e não na tela): a tela é redesenhada quando
// chegam dados da sincronização, e a revisão não pode se perder.
let state = null; // { fileName, base64 ou text (PDF), preview, aiMessage }

// buildPreview lê o arquivo guardado no estado (OFX/CSV em base64, ou o texto
// já extraído do PDF) com as opções da tela.
function buildPreview(ledger, s, options) {
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

function renderUpload() {
  return html`
    ${pageHead("Importar extrato")}
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
      state = next;
      rerender();
    } catch (err) {
      state = null;
      ctx.error(err);
    }
  });
  if (!state) return;
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
      const api = await endpoint("/api/categorizar");
      if (!api) throw new Error("conecte este aparelho ao computador para usar a IA");
      const res = await fetch(api.url, {
        ...api.init, method: "POST",
        headers: { ...api.init.headers, "Content-Type": "application/json" },
        body: JSON.stringify({ items: todo.map((it) => ({ description: it.description, kind: it.kind })) }),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error || `o computador respondeu ${res.status}`);
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
        }),
      }));
      await db.saveManyLocal(records);
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
