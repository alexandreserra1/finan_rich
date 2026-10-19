// Ajustes do app do celular: pareamento com o computador.
import * as db from "../db.js";
import * as device from "../device.js";
import * as finance from "../finance.js";
import * as sync from "../sync.js";
import { canScan, hasReminders, remindersEnabled, scanPairingQR, setRemindersEnabled } from "../native.js";
import { html } from "../ui.js";

export const needsData = false; // funciona antes de baixar qualquer dado

function pairForm(params, paired) {
  return html`
    <form data-form="pair" class="grid-form">
      <label>Endereço do computador
        <input name="url" required inputmode="url" autocapitalize="off" autocomplete="off" placeholder="192.168.0.10:8080" value="${params.get("url") || ""}">
      </label>
      <label>Código de pareamento
        <input name="code" required autocapitalize="characters" autocomplete="off" placeholder="ABCD-EFGH" value="${params.get("codigo") || ""}">
      </label>
      <div class="row-actions tight">
        <button type="submit">${paired ? "Conectar a este computador" : "Conectar"}</button>
        ${canScan() ? html`<button type="button" class="secondary" data-action="scan">Ler QR code</button>` : ""}
      </div>
    </form>
    <p class="small muted">No computador, abra o finan_rich e vá em <strong>Ajustes → App do celular → Parear um celular</strong>. Aponte a câmera para o QR code ou digite aqui o endereço e o código que aparecem lá.</p>`;
}

let remindersOn = true;

export function render({ server, params, ledger }) {
  const fromLink = params.has("codigo");
  const status = sync.state;
  return html`
    <a class="panel link-card" href="/categorias"><span><strong>Categorias e divisão do salário</strong></span><span aria-hidden="true">›</span></a>

    <section class="panel" id="parear">
      <h2>Computador</h2>
      ${server
        ? html`
          <p class="notice ok">✅ Conectado a <code>${server.url}</code> como <strong>${server.device}</strong>.</p>
          <p class="small muted">${status.lastSync ? `Última sincronização: ${status.lastSync.toLocaleString("pt-BR")}.` : "Ainda não sincronizou nesta abertura do app."}
            ${status.pending ? ` ${status.pending} alteração(ões) esperando para enviar.` : ""}
            Tudo que você faz fica salvo neste celular e vai para o computador quando ele estiver ao alcance (mesmo Wi-Fi ligado e finan_rich aberto).</p>
          <div class="row-actions">
            <button type="button" data-action="sync">Sincronizar agora</button>
            <button type="button" class="secondary" data-action="unpair">Desconectar</button>
          </div>
          ${fromLink
            ? html`<h3>Conectar a outro computador?</h3>
              <p class="notice warn">Você abriu um link de pareamento. Só confirme se o endereço abaixo for mesmo o do seu computador.</p>
              ${pairForm(params, true)}`
            : html`<details><summary>Conectar a outro computador</summary>${pairForm(params, true)}</details>`}`
        : html`
          <p class="muted">O app guarda seus dados neste celular e funciona sem internet. Para trazer os dados que já estão no computador e manter os dois iguais, conecte-o uma vez ao finan_rich do computador.</p>
          ${fromLink ? html`<p class="notice warn">Confira se o endereço abaixo é mesmo o do seu computador antes de conectar.</p>` : ""}
          ${pairForm(params, false)}`}
    </section>

    ${!server ? html`<section class="panel" id="sozinho">
      <h2>Sem computador?</h2>
      ${ledger.categories.length
        ? html`<p class="small muted">Você está usando o finan_rich só neste celular. Faça um backup de vez em quando em <a href="/exportar">Exportar</a>. Se um dia conectar a um computador, tudo vai para lá, sem duplicar categorias.</p>`
        : html`<p class="small muted">Dá para usar só neste celular: lançamentos, dívidas, metas, importação de extratos, notas e exportação funcionam sem computador. O consultor com IA precisa do computador. Se depois conectar a um, seus dados vão junto.</p>
          <button type="button" class="secondary" data-action="solo">Usar só neste celular</button>`}
    </section>` : ""}

    ${hasReminders() ? html`<section class="panel">
      <h2>Lembretes</h2>
      <label class="check"><input type="checkbox" data-action="reminders" ${remindersOn ? "checked" : ""}> Avisar 2 dias antes do vencimento das dívidas (às 9h)</label>
      <p class="small muted">O aviso é do próprio celular: funciona sem internet. Cadastre o dia do vencimento em Dívidas.</p>
    </section>` : ""}

    <section class="panel">
      <h2>Dados deste celular</h2>
      <p class="muted small">Apaga os lançamentos, dívidas e categorias guardados neste celular e desconecta do computador. Os dados que já estão no computador continuam lá.</p>
      <button type="button" class="secondary" data-action="wipe">Apagar dados deste celular</button>
    </section>`;
}

export function bind(root, ctx) {
  if (hasReminders()) remindersEnabled().then((on) => { if (on !== remindersOn) { remindersOn = on; ctx.rerender(); } });
  root.querySelector('[data-action="scan"]')?.addEventListener("click", async () => {
    try {
      const found = await scanPairingQR();
      if (!found) return; // cancelou
      const form = root.querySelector('[data-form="pair"]');
      form.elements.url.value = found.url;
      form.elements.code.value = found.code;
      form.requestSubmit();
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="solo"]')?.addEventListener("click", async () => {
    try {
      await finance.load();
      const cats = finance.defaultCategories(Date.now());
      await db.saveManyLocal(cats.map((record) => ({ type: "category", record })));
      await ctx.refresh();
      ctx.go("/lancamentos");
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="reminders"]')?.addEventListener("change", async (e) => {
    remindersOn = e.target.checked;
    await setRemindersEnabled(remindersOn, ctx.ledger);
    ctx.flash(remindersOn ? "Lembretes ligados." : "Lembretes desligados.");
  });
  root.querySelector('[data-form="pair"]')?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target.elements;
    const button = e.target.querySelector("button");
    button.disabled = true;
    try {
      const url = await device.pair(f.url.value, f.code.value);
      history.replaceState(null, "", "/configuracoes");
      await ctx.refresh(`Conectado a ${url}. Baixando seus dados…`);
      sync.sync();
    } catch (err) {
      button.disabled = false;
      ctx.error(err);
    }
  });

  const on = (action, fn) => root.querySelector(`[data-action="${action}"]`)?.addEventListener("click", fn);
  on("sync", async () => {
    await sync.sync();
    await ctx.refresh(sync.state.status === "ok" ? "Sincronizado." : undefined);
    if (sync.state.status !== "ok") ctx.error(new Error("não consegui falar com o computador agora; tudo continua salvo aqui e vai depois"));
  });
  on("unpair", async () => {
    if (!confirm("Desconectar este celular do computador? Os dados continuam neste celular, mas param de sincronizar.")) return;
    await device.unpair();
    await ctx.refresh("Desconectado do computador.");
  });
  on("wipe", async () => {
    if (!confirm("Apagar todos os dados deste celular? Os que já estão no computador continuam lá.")) return;
    await db.wipe();
    await ctx.refresh("Dados deste celular apagados.");
  });
}
