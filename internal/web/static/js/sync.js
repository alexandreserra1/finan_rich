// Sincronização com o computador onde o finan_rich roda e, se a pessoa entrou
// com Google, com o arquivo no Drive dela (gdrive.js). As duas podem conviver:
// a regra é a mesma (vence a alteração mais recente).
//
// Envia a fila de alterações feitas neste aparelho e recebe o que mudou lá
// (inclusive o que outros aparelhos enviaram). Roda ao abrir, quando a conexão
// volta, a cada 30 segundos e logo depois de cada alteração local. Sem
// conexão, nada se perde: a fila fica guardada até dar certo.

import { applyRemote, clearSent, getMeta, pending, setMeta } from "./db.js";
import { endpoint } from "./device.js";
import { syncDrive } from "./gdrive.js";
import * as google from "./google.js";

const PERIOD_MS = 30_000;
let running = null;
let again = false;
let timer = null;

// status: "ok" | "syncing" | "offline" | "login" | "unpaired" | "google" | "error"
// notice: aviso do computador nesta sincronização (ex.: categoria em uso não apagada).
export const state = { status: "syncing", pending: 0, lastSync: null, message: "", notice: "" };

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

async function round() {
  const out = await pending();
  state.pending = out.length;
  let cursor = await getMeta("cursor", 0);
  let changed = false;
  const notices = [];
  let toSend = out.filter((c) => !unsupported.has(c.type));
  const linked = await google.account();
  if (linked) {
    const touched = localChanged;
    localChanged = false;
    try {
      changed = await syncDrive(touched, learnClock);
    } catch (err) {
      localChanged ||= touched; // tenta de novo na próxima rodada
      throw err;
    }
  }
  const api = await endpoint("/api/sync");
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
    if (res.status === 401) throw Object.assign(new Error("login"), { kind: "login" });
    const body = await res.json().catch(() => ({}));
    const unknownType = res.status === 422 && /tipo "(\w+)"/.exec(body.error || "")?.[1];
    if (unknownType && !unsupported.has(unknownType) && toSend.some((c) => c.type === unknownType)) {
      unsupported.add(unknownType);
      toSend = toSend.filter((c) => c.type !== unknownType);
      continue;
    }
    if (!res.ok) throw Object.assign(new Error(body.error || res.statusText), { kind: "error" });
    if (body.now) learnClock(body.now, sentAt);
    notices.push(...(body.notices || []));
    if (toSend.length) {
      await clearSent(toSend.slice(0, BATCH));
      toSend = toSend.slice(BATCH);
    }
    if (await applyRemote(body.changes || [])) changed = true;
    cursor = body.cursor;
    await setMeta("cursor", cursor);
    if (!body.more && !toSend.length) break;
  }
  state.pending = (await pending()).length;
  state.notice = notices.join(" ");
  state.message = unsupported.size ? "Atualize o finan_rich do computador para sincronizar as notas." : "";
  return changed;
}

// sync roda uma sincronização (ou agenda outra, se já houver uma rodando).
// ---- relógio ----
// Diferença entre o relógio do computador e o deste aparelho (ms), aprendida a
// cada sincronização (com o computador ou pelo Drive). Entra no carimbo das edições: com o relógio do aparelho
// errado, uma edição feita depois, sem conexão, perderia para uma mais antiga
// de outro aparelho (docs/passo-a-passo.md, 1.3).
let clockOffset = 0;
getMeta("clock-offset", 0).then((v) => { clockOffset = v || 0; }).catch(() => {});

function learnClock(serverNow, sentAt) {
  const off = serverNow - (sentAt + Date.now()) / 2; // metade da ida e volta
  clockOffset = Math.abs(off) > 2000 ? Math.round(off) : 0; // até 2 s é ruído da rede
  setMeta("clock-offset", clockOffset).catch(() => {});
}

// stamp é o updatedAt de uma edição: a hora certa (pelo computador) e sempre
// depois da versão anterior.
export const stamp = (previous = 0) => Math.max(Date.now() + clockOffset, previous + 1);

export async function sync() {
  if (running) {
    again = true;
    return running;
  }
  running = (async () => {
    state.status = "syncing";
    emit();
    try {
      do {
        again = false;
        const changed = await round();
        state.status = "ok";
        state.lastSync = new Date();
        emit(changed);
        state.notice = "";
      } while (again);
    } catch (err) {
      state.pending = (await pending()).length;
      if (err.kind === "login" || err.kind === "unpaired" || err.kind === "google") state.status = err.kind;
      else if (err.kind === "error") { state.status = "error"; state.message = err.message; }
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
