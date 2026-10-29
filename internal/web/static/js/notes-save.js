// Salva uma resposta do consultor como nota (o usuário escolhe o que guardar).
import * as db from "./db.js";
import * as finance from "./finance.js";
import * as sync from "./sync.js";
import { newId } from "./ui.js";

export async function saveNote(question, answer) {
  await finance.load();
  const title = "Consultor: " + (question || "resposta").replace(/\s+/g, " ").trim();
  const record = finance.validate("note", {
    id: newId(), updatedAt: sync.stamp(0),
    title: title.length > 110 ? title.slice(0, 109) + "…" : title,
    body: answer,
  });
  await db.saveLocal("note", record);
  sync.soon();
}

// Aplica uma mudança proposta pelo consultor (lançar, alterar, apagar, criar
// meta ou dívida), depois que a pessoa toca no botão. Passa pelas mesmas regras
// (WebAssembly) de uma mudança feita à mão.
export async function applyProposal(p) {
  await finance.load();
  if (p.before) {
    // O registro mudou depois que o consultor preparou o cartão: aplicar agora
    // desfaria a mudança mais nova.
    const list = (await db.loadLedger())[{ transaction: "transactions", goal: "goals", debt: "debts" }[p.type]] || [];
    const cur = list.find((r) => r.id === p.before.id) || {};
    const keys = new Set([...Object.keys(cur), ...Object.keys(p.before)]);
    keys.delete("updatedAt");
    if ([...keys].some((k) => JSON.stringify(cur[k]) !== JSON.stringify(p.before[k]))) {
      throw new Error("este cartão ficou velho: o registro mudou depois. Peça de novo ao consultor");
    }
  }
  const record = finance.validate(p.type, { ...p.record, updatedAt: sync.stamp(p.record.updatedAt) });
  await db.saveLocal(p.type, record);
  sync.soon();
}

// Texto do botão: o que ele faz e como fica depois de feito.
export const proposalButton = (p) => ({ criar: ["Adicionar", "Adicionado ✓"], alterar: ["Alterar", "Alterado ✓"], apagar: ["Apagar", "Apagado ✓"] })[p.action] || ["Confirmar", "Feito ✓"];

// Rótulo pequeno do cartão: o que muda ("Novo lançamento", "Apagar meta"…).
export const proposalKind = (p) => {
  const [what, novo] = { transaction: ["lançamento", "Novo"], goal: ["meta", "Nova"], debt: ["dívida", "Nova"] }[p.type] || ["registro", "Novo"];
  return { criar: novo + " " + what, alterar: "Alterar " + what, apagar: "Apagar " + what }[p.action] || "Mudança";
};
