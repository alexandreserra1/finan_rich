// Banco SQLite nativo do app (Android e iPhone), pelo plugin
// @capacitor-community/sqlite. Devolve run(ops) com o mesmo contrato do Worker da
// web (sqlite-worker.js): roda tudo numa transação só (ou tudo ou nada) e
// devolve, para cada operação, as linhas (objetos) ou quantas linhas ela mudou;
// uma operação guard sem linhas desfaz tudo (erro com code "guard"). O SQL fica
// em db-sqlite.js. Uma transação aqui são várias chamadas ao plugin, então run
// entra numa fila: duas nunca se misturam.
const database = "finan";

export async function openNative(plugin) {
  // A conexão nativa sobrevive ao recarregar a tela: fecha as que sobraram antes de abrir.
  await plugin.checkConnectionsConsistency({ dbNames: [], openModes: [] });
  await plugin.createConnection({ database, version: 1, encrypted: false, mode: "no-encryption", readonly: false });
  await plugin.open({ database, readonly: false });

  // shortcut: cada operação é uma chamada à ponte com o nativo, lento em lotes enormes (a primeira migração); se pesar, usar executeSet nos lotes só de escrita.
  const exec = async (ops) => {
    await plugin.beginTransaction({ database });
    try {
      const out = [];
      for (const op of ops) {
        const values = op.bind ?? [];
        if (!op.rows) {
          out.push((await plugin.run({ database, statement: op.sql, values, transaction: false })).changes.changes);
          continue;
        }
        const res = (await plugin.query({ database, statement: op.sql, values, readonly: false })).values ?? [];
        const rows = res[0]?.ios_columns ? res.slice(1) : res; // no iPhone, a primeira linha traz só os nomes das colunas
        if (op.guard && !rows.length) throw Object.assign(new Error("guard"), { code: "guard" });
        out.push(rows);
      }
      await plugin.commitTransaction({ database });
      return out;
    } catch (error) {
      await plugin.rollbackTransaction({ database }).catch(() => {});
      throw error;
    }
  };

  let queue = Promise.resolve();
  return (ops) => {
    const result = queue.then(() => exec(ops));
    queue = result.catch(() => {});
    return result;
  };
}
