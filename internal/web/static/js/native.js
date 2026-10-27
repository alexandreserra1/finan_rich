// Recursos do celular no app (Capacitor): leitor de QR code e os lembretes do
// time (vencimentos, achados da semana, balanço das metas). No navegador, os lembretes não existem.
import { getMeta, setMeta } from "./db.js";
import { isApp, pairingLink } from "./device.js";
import { cameraAvailable, scanQR } from "./qrscan.js";

const plugins = () => window.Capacitor?.Plugins || {};

export const canScan = cameraAvailable;

// scanPairingQR abre a câmera e lê o QR code dos Ajustes do computador.
// Devolve { url, code }, null se a pessoa cancelar, ou lança erro se o QR
// não for o de pareamento.
export async function scanPairingQR() {
  const text = await scanQR("Aponte para o QR code em Ajustes → App do celular, no computador");
  if (text == null) return null;
  const found = parsePairingQR(text);
  if (!found) throw new Error("esse QR code não é o de pareamento do finan_rich");
  return found;
}

// O QR leva a http://computador:porta/parear?codigo=...; também aceitamos o
// link finanrich://parear?url=...&codigo=....
export function parsePairingQR(text) {
  if (!text) return null;
  const link = pairingLink(text);
  if (link) return link;
  try {
    const u = new URL(text);
    if (u.pathname === "/parear" && u.searchParams.get("codigo")) return { url: u.origin, code: u.searchParams.get("codigo") };
  } catch {}
  return null;
}

// ---- lembretes do time ----

const KINDS = ["debt-due", "team"]; // os nossos (os de vencimento antigos eram "debt-due")
const ours = async (notif) => (await notif.getPending()).notifications.filter((n) => KINDS.includes(n.extra?.kind));

// teamReminders monta os lembretes, todos às 9h e repetindo sozinhos (o
// próprio Android avisa, mesmo offline):
// - Luna: cada dívida com vencimento, dois dias antes, todo mês;
// - Otávio: toda segunda, os achados da semana (se há lançamentos);
// - Tito: no dia do salário, o balanço das metas (se há metas).
// O texto é fixo: a conta de verdade aparece ao abrir (cartão "O que o time viu").
function teamReminders(ledger) {
  const live = (list) => list.filter((r) => !r.deleted);
  const out = live(ledger.debts).filter((d) => d.dueDay > 0 && d.balanceCents > 0).map((d, i) => ({
    id: 7100 + i,
    title: `Luna: ${d.name} vence dia ${d.dueDay}`,
    body: "Abra o finan_rich para ver a parcela e o plano para quitar.",
    schedule: { on: { day: Math.max(1, d.dueDay - 2), hour: 9, minute: 0 }, allowWhileIdle: true },
    extra: { kind: "team", link: "/dividas" },
  }));
  if (live(ledger.transactions).length) {
    out.push({ id: 7300, title: "Otávio: os achados da semana", body: "Veja para onde foi o dinheiro e onde dá para guardar.",
      schedule: { on: { weekday: 2, hour: 9, minute: 0 }, allowWhileIdle: true }, extra: { kind: "team", link: "/analise" } }); // weekday 2 = segunda
  }
  if (live(ledger.goals).length) {
    const salary = live(ledger.recurring || []).find((r) => r.kind === "entrada" && r.day > 0);
    out.push({ id: 7301, title: "Tito: balanço das metas", body: "Veja quanto guardar este mês para chegar no prazo de cada meta.",
      schedule: { on: { day: Math.min(salary?.day || 1, 28), hour: 9, minute: 0 }, allowWhileIdle: true }, extra: { kind: "team", link: "/metas" } });
  }
  return out;
}

// scheduleReminders reagenda os lembretes do time quando o que eles dizem muda.
export async function scheduleReminders(ledger) {
  const notif = plugins().LocalNotifications;
  if (!isApp || !notif) return;
  if ((await getMeta("reminders", "on")) !== "on") return;
  const list = teamReminders(ledger);
  const signature = JSON.stringify(list.map((n) => [n.id, n.title, n.schedule.on]));
  if (signature === (await getMeta("reminders-sig", ""))) return;

  if (list.length) {
    let perm = await notif.checkPermissions();
    if (perm.display === "prompt" || perm.display === "prompt-with-rationale") perm = await notif.requestPermissions();
    if (perm.display !== "granted") return;
  }
  const old = await ours(notif);
  if (old.length) await notif.cancel({ notifications: old.map((n) => ({ id: n.id })) });
  if (list.length) await notif.schedule({ notifications: list });
  await setMeta("reminders-sig", signature);
}

// onReminderTap: tocar num lembrete abre a tela dele.
export function onReminderTap(open) {
  plugins().LocalNotifications?.addListener?.("localNotificationActionPerformed", (e) => {
    const link = e?.notification?.extra?.link;
    if (link) open(link);
  });
}

export async function setRemindersEnabled(on, ledger) {
  await setMeta("reminders", on ? "on" : "off");
  await setMeta("reminders-sig", "");
  if (on) return scheduleReminders(ledger);
  const notif = plugins().LocalNotifications;
  if (!notif) return;
  const old = await ours(notif);
  if (old.length) await notif.cancel({ notifications: old.map((n) => ({ id: n.id })) });
}

export const remindersEnabled = async () => isApp && !!plugins().LocalNotifications && (await getMeta("reminders", "on")) === "on";
export const hasReminders = () => isApp && !!plugins().LocalNotifications;
