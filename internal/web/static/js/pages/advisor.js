// Consultor IA no app do celular: a IA roda no computador, então abrimos a
// tela do consultor de lá (precisa estar ao alcance do celular).
import { html } from "../ui.js";

export const needsData = false;

export function render({ server }) {
  return html`
    <section class="panel">
      <h2>Consultor IA</h2>
      <p class="muted">O consultor conversa sobre suas dívidas, gastos e divisão do salário. Ele roda no computador (é lá que fica a IA), então só funciona com o celular ao alcance dele.</p>
      ${server
        ? html`<p><a class="button" href="${server.url}/chat">Abrir o consultor</a></p>
          <p class="small muted">Abre no navegador do celular. Na primeira vez, ele pede a senha de acesso do computador.</p>`
        : html`<p class="notice warn">Conecte este celular ao computador primeiro, em <a href="/configuracoes">Ajustes</a>.</p>`}
    </section>`;
}
