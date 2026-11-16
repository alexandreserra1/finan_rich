// Sincronização com o computador onde o finan_rich roda e, se a pessoa entrou
// com Google, com o arquivo no Drive dela (gdrive.js). As duas podem conviver:
// a regra é a mesma (vence a alteração mais recente).
//
// Envia a fila de alterações feitas neste aparelho e recebe o que mudou lá
// (inclusive o que outros aparelhos enviaram). Roda ao abrir, quando a conexão
// volta, a cada 30 segundos e logo depois de cada alteração local. Sem
// conexão, nada se perde: a fila fica guardada até dar certo.

import { applyRemote, checkEpoch, clearSent, databaseEpoch, getMeta, pending, setMeta } from "./db.js";
import { endpoint } from "./device.js";
import { syncDrive } from "./gdrive.js";
import * as google from "./google.js";

const PERIOD_MS = 30_000;
let running = null;
let again = false;
let timer = null;

// status: "ok" | "syncing" | "offline" | "login" | "unpaired" | "google" | "error"
// notice: aviso do computador nesta sincronização (ex.: categoria em uso não apagada).
export const state = { status: "syncing", pending: 0, lastSync: null, message: "", notice: "", recovery: "" };

function emit(dataChanged = false) {
  window.dispatchEvent(new CustomEvent("finan-sync", { detail: { ...state, dataChanged } }));
}

// Tipos que o computador ainda não conhece (finan_rich desatualizado lá, ex.:
// notas antes da v1.3). Ficam na fila, sem travar o resto da sincronização.
const unsupported = new Set();

// Envia a fila em lotes: um celular muito tempo sem sincronizar (ou uma
// importação de anos de extrato) passaria do limite de um pedido e ficaria
// preso para sempre no mesmo erro.
const BATCH = 2000;

let localChanged = false; // houve alteração aqui desde a última rodada do Drive

async function round(epoch) {
  const out = await pending(epoch);
  checkEpoch(epoch);
  state.pending = out.length;
  let cursor = await getMeta("cursor", 0, epoch);
  checkEpoch(epoch);
  let changed = false;
  const notices = [];
  let toSend = out.filter((c) => !unsupported.has(c.type));
  const linked = await google.account();
  checkEpoch(epoch);
  if (linked) {
    const touched = localChanged;
    localChanged = false;
    try {
      changed = await syncDrive(touched, (now, sentAt) => learnClock(now, sentAt, epoch), epoch);
      checkEpoch(epoch);
    } catch (err) {
      localChanged ||= touched; // tenta de novo na próxima rodada
      throw err;
    }
  }
  const api = await endpoint("/api/sync");
  checkEpoch(epoch);
  if (!api && linked) return changed;
  if (!api) throw Object.assign(new Error("unpaired"), { kind: "unpaired" });
  for (;;) {
    const sentAt = Date.now();
    const res = await fetch(api.url, {
      ...api.init,
      method: "POST",
      headers: { ...api.init.headers, "Content-Type": "application/json" },
      body: JSON.stringify({ cursor, changes: toSend.slice(0, BATCH).map(({ type, record }) => ({ type, record })) }),
    });
    checkEpoch(epoch);
    if (res.status === 401) throw Object.assign(new Error("login"), { kind: "login" });
    const body = await res.json().catch(() => ({}));
    checkEpoch(epoch);
    const unknownType = res.status === 422 && /tipo "(\w+)"/.exec(body.error || "")?.[1];
    if (unknownType && !unsupported.has(unknownType) && toSend.some((c) => c.type === unknownType)) {
      unsupported.add(unknownType);
      toSend = toSend.filter((c) => c.type !== unknownType);
      continue;
    }
    if (!res.ok) throw Object.assign(new Error(body.error || res.statusText), { kind: "error" });
    if (body.now) await learnClock(body.now, sentAt, epoch);
    checkEpoch(epoch);
    notices.push(...(body.notices || []));
    if (toSend.length) {
      await clearSent(toSend.slice(0, BATCH), epoch);
      checkEpoch(epoch);
      toSend = toSend.slice(BATCH);
    }
    if (await applyRemote(body.changes || [], epoch)) changed = true;
    checkEpoch(epoch);
    cursor = body.cursor;
    await setMeta("cursor", cursor, epoch);
    checkEpoch(epoch);
    if (!body.more && !toSend.length) break;
  }
  const remaining = await pending(epoch);
  checkEpoch(epoch);
  state.pending = remaining.length;
  state.notice = notices.join(" ");
  state.message = unsupported.size ? `Atualize a Rich do computador para sincronizar ${[...unsupported].map((t) => ({ note: "as notas", transfer: "as transferências" })[t] || "tudo").join(" e ")}.` : "";
  return changed;
}

// sync roda uma sincronização (ou agenda outra, se já houver uma rodando).
// ---- relógio ----
// Diferença entre o relógio do computador e o deste aparelho (ms), aprendida a
// cada sincronização (com o computador ou pelo Drive). Entra no carimbo das edições: com o relógio do aparelho
// errado, uma edição feita depois, sem conexão, perderia para uma mais antiga
// de outro aparelho (docs/passo-a-passo.md, 1.3).
let clockOffset = 0;
const initialEpoch = databaseEpoch();
getMeta("clock-offset", 0, initialEpoch).then((v) => { checkEpoch(initialEpoch); clockOffset = v || 0; }).catch(() => {});

async function learnClock(serverNow, sentAt, epoch) {
  checkEpoch(epoch);
  const off = serverNow - (sentAt + Date.now()) / 2; // metade da ida e volta
  const nextOffset = Math.abs(off) > 2000 ? Math.round(off) : 0;
  await setMeta("clock-offset", nextOffset, epoch);
  checkEpoch(epoch);
  clockOffset = nextOffset; // até 2 s é ruído da rede
}

// stamp é o updatedAt de uma edição: a hora certa (pelo computador) e sempre
// depois da versão anterior.
export const stamp = (previous = 0) => Math.max(Date.now() + clockOffset, previous + 1);

export async function sync() {
  if (running) {
    again = true;
    return running;
  }
  const epoch = databaseEpoch();
  running = (async () => {
    state.status = "syncing";
    state.recovery = "";
    emit();
    try {
      do {
        again = false;
        const changed = await round(epoch);
        checkEpoch(epoch);
        state.status = "ok";
        state.lastSync = new Date();
        emit(changed);
        state.notice = "";
      } while (again);
    } catch (err) {
      const currentEpoch = databaseEpoch();
      let remaining;
      try {
        remaining = await pending(currentEpoch);
        checkEpoch(currentEpoch);
      } catch (error) {
        state.status = "error";
        state.message = error.message;
        state.recovery = "";
        emit();
        return;
      }
      state.pending = remaining.length;
      if (epoch !== currentEpoch) {
        clockOffset = 0;
        state.status = "error";
        state.message = "Os dados deste aparelho foram apagados. Sincronize de novo.";
        state.recovery = "";
        state.notice = "";
        emit();
        return;
      }
      if (err.kind === "login" || err.kind === "unpaired" || err.kind === "google") state.status = err.kind;
      else if (err.kind === "error") { state.status = "error"; state.message = err.message; state.recovery = err.recovery || ""; }
      else state.status = "offline"; // fetch falhou: sem conexão com o computador
      emit();
    } finally {
      running = null;
    }
  })();
  return running;
}

// soon agenda uma sincronização logo depois de uma alteração local.
export function soon() {
  localChanged = true;
  clearTimeout(timer);
  timer = setTimeout(sync, 400);
  pending().then((p) => { state.pending = p.length; emit(); });
}

export function start() {
  sync();
  window.addEventListener("online", sync);
  document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") sync(); });
  setInterval(sync, PERIOD_MS);
}

// flushBeforeLeaving sincroniza e devolve quantas alterações deste aparelho
// ainda não chegaram ao computador. Sair da conta só apaga com zero (senão
// elas se perderiam).
export async function flushBeforeLeaving() {
  await sync();
  return (await pending()).length;
}
