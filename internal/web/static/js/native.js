// Recursos do celular no app (Capacitor): leitor de QR code e lembretes de
// vencimento das dívidas. No navegador, nada disso existe e as funções avisam.
import { getMeta, setMeta } from "./db.js";
import { isApp, pairingLink } from "./device.js";

const plugins = () => window.Capacitor?.Plugins || {};

export const canScan = () => isApp && !!plugins().CapacitorBarcodeScanner;

// scanPairingQR abre a câmera e lê o QR code dos Ajustes do computador.
// Devolve { url, code } ou null se a pessoa cancelar.
export async function scanPairingQR() {
  const { ScanResult } = await plugins().CapacitorBarcodeScanner.scanBarcode({
    hint: 0, // QR code
    scanInstructions: "Aponte para o QR code em Ajustes → App do celular, no computador",
    scanButton: false, scanText: " ", cameraDirection: 1, scanOrientation: 3,
  });
  return parsePairingQR(ScanResult);
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

// ---- lembretes de vencimento ----

const REMINDER = "debt-due";

// scheduleDebtReminders agenda, para cada dívida com dia de vencimento, um
// aviso todo mês dois dias antes, às 9h. Funciona com o celular offline (é o
// próprio Android que avisa). Só reagenda quando as dívidas mudam.
export async function scheduleDebtReminders(ledger) {
  const notif = plugins().LocalNotifications;
  if (!isApp || !notif) return;
  if ((await getMeta("reminders", "on")) !== "on") return;
  const debts = ledger.debts.filter((d) => !d.deleted && d.dueDay > 0 && d.balanceCents > 0);
  const signature = JSON.stringify(debts.map((d) => [d.id, d.name, d.dueDay]));
  if (signature === (await getMeta("reminders-sig", ""))) return;

  if (debts.length) {
    let perm = await notif.checkPermissions();
    if (perm.display === "prompt" || perm.display === "prompt-with-rationale") perm = await notif.requestPermissions();
    if (perm.display !== "granted") return;
  }
  const pending = await notif.getPending();
  const ours = pending.notifications.filter((n) => n.extra?.kind === REMINDER);
  if (ours.length) await notif.cancel({ notifications: ours.map((n) => ({ id: n.id })) });
  if (debts.length) {
    await notif.schedule({
      notifications: debts.map((d, i) => ({
        id: 7100 + i,
        title: `${d.name} vence dia ${d.dueDay}`,
        body: "Abra o finan_rich para ver a parcela e o plano para quitar.",
        schedule: { on: { day: Math.max(1, d.dueDay - 2), hour: 9, minute: 0 }, allowWhileIdle: true },
        extra: { kind: REMINDER, debtId: d.id },
      })),
    });
  }
  await setMeta("reminders-sig", signature);
}

export async function setRemindersEnabled(on, ledger) {
  await setMeta("reminders", on ? "on" : "off");
  await setMeta("reminders-sig", "");
  if (on) return scheduleDebtReminders(ledger);
  const notif = plugins().LocalNotifications;
  if (!notif) return;
  const pending = await notif.getPending();
  const ours = pending.notifications.filter((n) => n.extra?.kind === REMINDER);
  if (ours.length) await notif.cancel({ notifications: ours.map((n) => ({ id: n.id })) });
}

export const remindersEnabled = async () => isApp && !!plugins().LocalNotifications && (await getMeta("reminders", "on")) === "on";
export const hasReminders = () => isApp && !!plugins().LocalNotifications;
