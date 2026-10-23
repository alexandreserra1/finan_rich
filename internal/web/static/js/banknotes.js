// Notificações dos apps de banco (só no app Android, plugin BankNotifications
// em mobile/android): "Compra de R$ 45,90 aprovada em IFOOD" vira um
// lançamento para revisar na Importação. Tudo fica no aparelho.
import * as finance from "./finance.js";
import { getMeta, setMeta } from "./db.js";

const plugin = () => window.Capacitor?.Plugins?.BankNotifications;
const KEPT = "bank-notes"; // as que parecem lançamento e ainda não foram revisadas

export const available = () => !!plugin();
export const enabled = async () => !!(await plugin().status()).enabled;
export const openSettings = () => plugin().openSettings();

const localDate = (ms) => {
  const d = new Date(ms);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

// pending lê a fila do Android (ler apaga lá) e devolve o extrato com o que
// parece lançamento, guardando só esses até a revisão.
export async function pending() {
  const fresh = (await plugin().drain()).items || [];
  const all = [...(await getMeta(KEPT, [])), ...fresh.map((n) => ({ id: n.id, date: localDate(n.time), text: n.text }))];
  const st = finance.notificationStatement(all);
  const ids = new Set(st.Rows.map((r) => r.FITID));
  await setMeta(KEPT, all.filter((n) => ids.has(n.id)));
  return st;
}

// reviewed: depois de importar, as notificações revisadas saem do aparelho.
export const reviewed = () => setMeta(KEPT, []);
