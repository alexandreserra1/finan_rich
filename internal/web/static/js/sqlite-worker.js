// Banco SQLite da web (site e navegador do computador), num Worker. Grava no
// OPFS (os arquivos privados do site no navegador) pelo modo opfs-sahpool: não
// exige cabeçalhos especiais e funciona no Safari. Só uma aba por vez consegue
// abrir o banco. Recebe { id, ops: [{ sql, bind, rows, guard }] } e roda tudo
// numa transação só (ou tudo ou nada); responde { id, out } com as linhas
// (objetos) de cada operação com rows ou quantas linhas ela mudou, ou
// { id, error, code }. Uma operação guard sem linhas desfaz tudo (code "guard").
// { id, close: true } fecha o banco e solta os arquivos para outra aba. O SQL
// fica em db-sqlite.js.
import sqlite3InitModule from "../vendor/sqlite/sqlite3.js";

// Arquivo do banco ainda preso pelo Worker da aba anterior (recarregada ou fechada há
// pouco): é o erro do Chromium e o do WebKit.
const BUSY = ["NoModificationAllowedError", "InvalidStateError"];

// openPool abre o banco no OPFS. preserveOnInitFailure: sem ela, uma falha ao abrir
// apaga a pasta do banco (o padrão do pacote). Arquivo preso: tenta de novo, sem prazo
// (a tela de espera aparece depois de 3 s, em db.js). Esperar é seguro: a aba já tem a
// trava, então quem segura o arquivo é o Worker de uma página que já saiu, e ele vai
// soltar. No WebKit do CI, isso passou de 30 s; desistir só faria a pessoa recarregar e
// esperar de novo.
async function openPool(sqlite3) {
  for (let i = 1; ; i++) {
    try {
      return await sqlite3.installOpfsSAHPoolVfs({ name: "finan-rich", preserveOnInitFailure: true, forceReinitIfPreviouslyFailed: true });
    } catch (e) {
      if (!BUSY.includes(e?.name)) throw e;
      await new Promise((resolve) => setTimeout(resolve, Math.min(100 * i, 1000)));
    }
  }
}

const ready = sqlite3InitModule().then(async (sqlite3) => {
  const pool = await openPool(sqlite3);
  return { pool, db: new pool.OpfsSAHPoolDb("/finan_rich.sqlite3") };
});

onmessage = async ({ data: { id, ops, close } }) => {
  try {
    const { pool, db } = await ready;
    if (close) {
      db.close();
      pool.pauseVfs();
      return postMessage({ id, out: [] });
    }
    const out = [];
    db.transaction(() => {
      for (const op of ops) {
        if (!op.rows) {
          db.exec({ sql: op.sql, bind: op.bind });
          out.push(db.changes());
          continue;
        }
        const rows = db.selectObjects(op.sql, op.bind);
        if (op.guard && !rows.length) throw Object.assign(new Error("guard"), { code: "guard" });
        out.push(rows);
      }
    });
    postMessage({ id, out });
  } catch (e) {
    postMessage({ id, error: e?.name && e.name !== "Error" ? `${e.name}: ${e.message}` : String(e?.message || e), code: e?.code === "guard" ? "guard" : undefined });
  }
};
