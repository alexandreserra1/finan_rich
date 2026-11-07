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
// apaga a pasta do banco (o padrão do pacote). Arquivo preso: tenta de novo por até 30 s.
// Esperar é seguro: a aba já tem a trava (db.js), então quem segura o arquivo é o Worker
// de uma página que já saiu, e ele vai soltar (no CI, às vezes passa de 5 s).
async function openPool(sqlite3) {
  for (let i = 0; ; i++) {
    try {
      return await sqlite3.installOpfsSAHPoolVfs({ name: "finan-rich", preserveOnInitFailure: true, forceReinitIfPreviouslyFailed: true });
    } catch (e) {
      if (!BUSY.includes(e?.name) || i === 300) throw e;
      await new Promise((resolve) => setTimeout(resolve, 100));
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
