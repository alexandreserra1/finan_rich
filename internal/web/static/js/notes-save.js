// Salva uma resposta do consultor como nota (o usuário escolhe o que guardar).
import * as db from "./db.js";
import * as finance from "./finance.js";
import * as sync from "./sync.js";
import { newId } from "./ui.js";

export async function saveNote(question, answer) {
  await finance.load();
  const title = "Consultor: " + (question || "resposta").replace(/\s+/g, " ").trim();
  const record = finance.validate("note", {
    id: newId(), updatedAt: Date.now(),
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
  const record = finance.validate(p.type, { ...p.record, updatedAt: Math.max(Date.now(), p.record.updatedAt + 1) });
  await db.saveLocal(p.type, record);
  sync.soon();
}

// Texto do botão: o que ele faz e como fica depois de feito.
export const proposalButton = (p) => ({ criar: ["Adicionar", "Adicionado ✓"], alterar: ["Alterar", "Alterado ✓"], apagar: ["Apagar", "Apagado ✓"] })[p.action] || ["Confirmar", "Feito ✓"];
