// Entrada do site: só com conta Google. Sem login, nenhuma tela do app aparece
// (main.js), e o "Usar sem conta" não existe aqui: o computador de quem abre o
// site pode ser de outra pessoa. Os dados ficam no Drive da própria pessoa.
import * as google from "../google.js";
import { html, raw } from "../ui.js";

export const needsData = false;

const icon = (d) => raw(`<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">${d}</svg>`);
const BENEFITS = [
  [icon('<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>'), "Seus dados ficam no seu Google Drive, numa pasta que só o finan_rich usa."],
  [icon('<rect x="2" y="4" width="14" height="11" rx="2"/><rect x="16" y="8" width="6" height="12" rx="1.5"/><path d="M6 19h6"/>'), "Celular, site e computador mostram as mesmas contas."],
  [icon('<path d="M3 3v18h18"/><path d="M7 15l4-4 3 3 5-6"/>'), "Mostra qual dívida pagar primeiro e quando cada meta chega."],
];

export function render(ctx) {
  return html`<section class="start">
    <div class="start-main">
      <a class="brand" href="/" aria-label="finan_rich, voltar ao início">finan<span>_rich</span></a>
      <div class="start-body">
        <div class="start-intro">
          <span class="start-pill"><span aria-hidden="true"></span>Grátis e de código aberto</span>
          <h1>Suas finanças, no seu controle.</h1>
          <p class="start-lead">Gastos, dívidas, metas e um time com IA que conversa sobre os seus números.</p>
        </div>
        ${ctx?.expired ? html`<p class="notice warn login-note">Você ficou um tempo sem usar, então saiu da conta para proteger seus dados. Entre de novo para continuar.</p>` : ""}
        <div class="start-google" hidden>
          <button type="button" class="google" data-action="google"><span class="g" aria-hidden="true">G</span><span data-label>Entrar com Google</span></button>
        </div>
        <p class="notice warn login-note" data-unavailable hidden>O login com Google abre em breve. Por segurança, o site só funciona com conta: enquanto isso, use o app do celular ou o programa do computador.</p>
        <ul class="start-benefits">
          ${BENEFITS.map(([svg, text]) => html`<li>${svg}<span>${text}</span></li>`)}
        </ul>
      </div>
      <p class="small faint">Prefere instalar? Há o <a href="https://github.com/alexandreserra1/finan_rich/releases/latest" rel="noopener">app Android e o programa do computador</a>.</p>
    </div>
    <div class="start-art" aria-hidden="true">
      <div class="start-art-mark"><img src="/static/img/brand-mark.png" alt="" width="256" height="256"><i></i></div>
    </div>
  </section>`;
}

export function bind(root, ctx) {
  const button = root.querySelector('[data-action="google"]');
  button.addEventListener("click", () => {
    // O Google abre em seguida; se falhar, o botão volta ao normal.
    button.disabled = true;
    button.querySelector("[data-label]").textContent = "Abrindo o Google…";
    google.signIn("web").catch((err) => {
      button.disabled = false;
      button.querySelector("[data-label]").textContent = "Entrar com Google";
      ctx.error(err);
    });
  });
  google.available().then((on) => {
    root.querySelector(".start-google").hidden = !on;
    root.querySelector("[data-unavailable]").hidden = on;
  });
}
