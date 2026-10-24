// /oauth: o Google volta aqui depois do login (é o único endereço de retorno
// cadastrado). O "state" diz de onde o login começou:
//   web.*        este site: termina o login aqui;
//   app.*        o app Android (o login abre no navegador do sistema): repassa
//                para finanrich://google, e o app termina o login;
//   pc.<porta>.* o programa do computador: repassa para 127.0.0.1 naquela porta.
// O código sozinho não serve para nada: falta o verificador PKCE, que ficou no
// aparelho que começou o login.
import { loadLedger } from "../db.js";
import * as google from "../google.js";
import * as sync from "../sync.js";
import { html } from "../ui.js";
import { startSolo } from "./device.js";

export const needsData = false;

export function render({ params }) {
  if (params.get("error")) {
    return html`<section class="start"><h1>Login cancelado</h1>
      <p class="muted">O Google não completou o login (${params.get("error")}). Seus dados continuam como estavam.</p>
      <p><a href="/">Voltar</a></p></section>`;
  }
  return html`<section class="start"><h1>Entrando com Google…</h1><p class="muted" data-oauth-status>Só um instante.</p></section>`;
}

// relay devolve o endereço para onde mandar a volta do login, ou "" se é aqui.
export function relay(params, native) {
  const state = params.get("state") || "";
  const [kind, port] = state.split(".");
  const query = new URLSearchParams({ code: params.get("code") || "", state }).toString();
  if (kind === "app" && !native) return "finanrich://google?" + query;
  if (kind === "pc") {
    if (!/^\d{2,5}$/.test(port || "")) throw new Error("endereço de volta inválido");
    return `http://127.0.0.1:${port}/google/retorno?` + query; // só esta máquina, nunca outro endereço
  }
  return "";
}

const handled = new Set(); // a tela se redesenha quando os dados chegam: o login roda uma vez só

export async function bind(root, ctx) {
  const status = root.querySelector("[data-oauth-status]");
  const state = ctx.params.get("state") || "";
  if (!status || handled.has(state)) return;
  handled.add(state);
  try {
    const target = relay(ctx.params, !!window.Capacitor?.isNativePlatform?.());
    if (target) {
      status.textContent = "Voltando para o finan_rich… Se não abrir sozinho, volte ao app.";
      location.replace(target);
      return;
    }
    await google.finish(ctx.params.get("code"), state);
    history.replaceState(null, "", "/"); // o "voltar" não repete o login
    status.textContent = "Trazendo seus dados do Google Drive…";
    await sync.sync();
    if (!(await loadLedger()).categories.length) await startSolo(ctx, "/"); // primeira vez: conta nova, sem dados no Drive
    else await ctx.refresh();
    sync.soon(); // grava no Drive o que este aparelho já tinha
    ctx.flash("Conectado ao Google. Seus dados ficam no seu Drive.");
  } catch (err) {
    status.textContent = err.message;
    status.classList.add("neg");
  }
}
