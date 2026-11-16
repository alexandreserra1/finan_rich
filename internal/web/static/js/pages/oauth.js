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
    return html`<section class="oauth-state"><h1 data-oauth-title>Login cancelado</h1>
      <p class="muted" aria-live="polite">O Google não completou o login (${params.get("error")}). Seus dados continuam como estavam.</p>
      <p><a href="/">Voltar</a></p></section>`;
  }
  return html`<section class="oauth-state"><h1 data-oauth-title>Entrando com Google…</h1><p class="muted" data-oauth-status aria-live="polite">Só um instante.</p></section>`;
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
let completing = null;

export async function bind(root, ctx) {
  const status = root.querySelector("[data-oauth-status]");
  const state = ctx.params.get("state") || "";
  const resuming = ctx.params.get("retomar") === "1";
  if (!status || (!resuming && handled.has(state))) return;
  if (!resuming) handled.add(state);
  try {
    const target = relay(ctx.params, !!window.Capacitor?.isNativePlatform?.());
    if (target) {
      status.textContent = "Voltando para a Rich… Se não abrir sozinho, volte ao app.";
      location.replace(target);
      return;
    }
    if (resuming) {
      if (!(await google.session()).signedIn) throw new Error("Sua sessão venceu. Entre de novo com Google.");
    } else {
      await google.finish(ctx.params.get("code"), state);
    }
    history.replaceState(null, "", "/oauth?retomar=1"); // o "voltar" não repete o login
    const retry = document.createElement("button");
    retry.type = "button";
    retry.textContent = "Tentar sincronizar de novo";
    status.after(retry);
    let reconnect = null;
    const complete = async () => {
      reconnect?.remove();
      reconnect = null;
      retry.hidden = true;
      const title = root.querySelector("[data-oauth-title]");
      if (title) title.textContent = "Trazendo seus dados…";
      status.classList.remove("neg");
      status.textContent = "Trazendo seus dados do Google Drive…";
      try {
        if (!completing) {
          completing = (async () => {
            await sync.sync();
            if (sync.state.status !== "ok") throw new Error(sync.state.message || "Não foi possível trazer seus dados do Google Drive. Tente de novo.");
            history.replaceState(null, "", "/");
            if (!(await loadLedger()).categories.length) await startSolo(ctx, "/"); // primeira vez: conta nova, sem dados no Drive
            else await ctx.refresh();
            sync.soon(); // grava no Drive o que este aparelho já tinha
            ctx.flash("Conectado ao Google. Seus dados ficam no seu Drive.");
          })().finally(() => { completing = null; });
        }
        await completing;
        retry.remove();
      } catch (err) {
        if (title) title.textContent = "Não foi possível sincronizar";
        status.textContent = err.message;
        status.classList.add("neg");
        retry.hidden = sync.state.recovery === "google";
        if (retry.hidden) {
          const link = document.createElement("a");
          reconnect = link;
          link.href = "/?entrar=1";
          link.dataset.oauthReconnect = "";
          link.className = "btn";
          link.textContent = "Entrar de novo e permitir acesso";
          link.addEventListener("click", async (event) => {
            event.preventDefault();
            if (link.getAttribute("aria-disabled") === "true") return;
            link.setAttribute("aria-disabled", "true");
            try {
              await google.signIn("web");
            } catch (error) {
              status.textContent = error.message;
              link.removeAttribute("aria-disabled");
            }
          });
          retry.after(reconnect);
        }
      }
    };
    retry.addEventListener("click", complete);
    await complete();
  } catch (err) {
    const title = root.querySelector("[data-oauth-title]");
    if (title) title.textContent = "Não foi possível entrar";
    const configurationError = /invalid_client|client secret/i.test(err.message);
    status.textContent = configurationError
      ? "O login com Google está temporariamente indisponível por um problema de configuração. Tente novamente mais tarde."
      : err.message;
    status.classList.add("neg");
    if (configurationError) {
      const details = document.createElement("details");
      const summary = document.createElement("summary");
      summary.textContent = "Detalhes técnicos";
      const message = document.createElement("p");
      message.textContent = err.message;
      details.append(summary, message);
      status.after(details);
    }
    status.parentElement.insertAdjacentHTML("beforeend", '<p><a href="/?entrar=1">Tentar de novo</a> · <a href="/">Voltar</a></p>');
  }
}
