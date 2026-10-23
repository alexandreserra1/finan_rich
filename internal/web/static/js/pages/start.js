// Boas-vindas do site (primeira visita, sem dados): começar a usar com os
// dados só neste navegador. O "Entrar com Google" entra aqui também.
import { html } from "../ui.js";
import { startSolo } from "./device.js";

export const needsData = false;

export function render() {
  return html`<section class="start">
    <h1 class="brand big">finan<span>_rich</span></h1>
    <p class="start-lead">Suas finanças, no seu controle: gastos, dívidas, metas e um consultor com IA. Grátis e de código aberto.</p>
    <div class="start-actions">
      <button type="button" data-action="solo">Começar a usar</button>
    </div>
    <p class="small muted">Seus dados ficam só neste navegador: nada vai para servidor do finan_rich. Faça um backup de vez em quando em Exportar. Se limpar os dados do navegador, eles se perdem.</p>
    <p class="small muted">Prefere instalar? Há o <a href="https://github.com/alexandreserra1/finan_rich/releases/latest" rel="noopener">app Android e o programa do computador</a>.</p>
  </section>`;
}

export function bind(root, ctx) {
  root.querySelector('[data-action="solo"]').addEventListener("click", () => startSolo(ctx, "/"));
}
