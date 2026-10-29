// Entrada do site: só com conta Google. Sem login, nenhuma tela do app aparece
// (main.js), e o "Usar sem conta" não existe aqui: o computador de quem abre o
// site pode ser de outra pessoa. Os dados ficam no Drive da própria pessoa.
import * as google from "../google.js";
import { html } from "../ui.js";

export const needsData = false;

export function render(ctx) {
  return html`<section class="start">
    <a class="brand big" href="/" aria-label="finan_rich, voltar ao início">finan<span>_rich</span></a>
    <p class="start-lead">Suas finanças, no seu controle: gastos, dívidas, metas e um time com IA. Grátis e de código aberto.</p>
    ${ctx?.expired ? html`<p class="notice warn login-note">Você ficou um tempo sem usar, então saiu da conta para proteger seus dados. Entre de novo para continuar.</p>` : ""}
    <div class="start-google" hidden>
      <div class="start-actions"><button type="button" class="google" data-action="google"><span class="g" aria-hidden="true">G</span>Entrar com Google</button></div>
      <p class="small muted">Seus dados ficam no seu Google Drive, numa pasta que só o finan_rich usa. Celular, site e computador ficam iguais.</p>
    </div>
    <p class="notice warn login-note" data-unavailable hidden>O login com Google abre em breve. Por segurança, o site só funciona com conta: enquanto isso, use o app do celular ou o programa do computador.</p>
    <p class="small muted">Prefere instalar? Há o <a href="https://github.com/alexandreserra1/finan_rich/releases/latest" rel="noopener">app Android e o programa do computador</a>.</p>
  </section>`;
}

export function bind(root, ctx) {
  root.querySelector('[data-action="google"]').addEventListener("click", () => google.signIn("web").catch(ctx.error));
  google.available().then((on) => {
    root.querySelector(".start-google").hidden = !on;
    root.querySelector("[data-unavailable]").hidden = on;
  });
}
