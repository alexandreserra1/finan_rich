// Ajustes do app do celular: pareamento com o computador.
import * as db from "../db.js";
import * as device from "../device.js";
import * as finance from "../finance.js";
import * as cloud from "../cloud-ai.js";
import { resetChat } from "./advisor.js";
import * as sync from "../sync.js";
import { canScan, hasReminders, remindersEnabled, scanPairingQR, setRemindersEnabled } from "../native.js";
import { html, toString } from "../ui.js";

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
let ai = null; // { presets, cfg } da IA na nuvem (carregado depois, do advisor.wasm)

function cloudSection(server) {
  if (!ai || ai.loading) return html`<section class="panel" id="ia-nuvem"><h2>Consultor sem computador</h2><p class="muted small loading">Carregando…</p></section>`;
  if (ai.error) return html`<section class="panel" id="ia-nuvem"><h2>Consultor sem computador</h2><p class="notice warn">${ai.error}</p></section>`;
  const cfg = ai.cfg || {};
  const preset = ai.presets.find((p) => p.id === (ai.chosen || cfg.preset)) || ai.presets[0];
  const model = cfg.preset === preset.id && cfg.model ? cfg.model : preset.model;
  const hasKey = cfg.key && cfg.preset === preset.id;
  return html`<section class="panel" id="ia-nuvem">
    <h2>Consultor sem computador</h2>
    <p class="small muted">O consultor pode usar uma IA na nuvem, chamada direto deste celular. Antes de sair, os nomes de lojas,
      dívidas, credores e metas viram apelidos ("Loja 3", "Dívida 1") e as notas ficam de fora: a IA vê só valores, datas e
      categorias. A conexão é cifrada (HTTPS) e a chave fica guardada cifrada neste aparelho.</p>
    <form data-form="cloud-ai" class="grid-form">
      <label>IA na nuvem
        <select name="preset">${ai.presets.map((p) => html`<option value="${p.id}" ${p.id === preset.id ? "selected" : ""}>${p.name}</option>`)}</select>
      </label>
      <p class="small muted">${preset.description}</p>
      <details class="key-guide" ${hasKey ? "" : "open"}>
        <summary>${hasKey ? "Como pegar outra chave" : "Como pegar a chave (leva 1 minuto)"}</summary>
        <ol class="small">
          ${(preset.keySteps || []).map((s) => html`<li>${s}</li>`)}
          <li>Volte aqui, cole no campo "Chave da IA" e toque em Salvar. O app testa a chave na hora.</li>
        </ol>
        <a class="button" href="${preset.keyUrl}" target="_blank" rel="noopener">Abrir o site para pegar a chave</a>
      </details>
      <label>Modelo
        <input name="model" value="${model}" list="cloud-models" autocomplete="off">
        <datalist id="cloud-models">${preset.models.map((m) => html`<option value="${m}">`)}</datalist>
      </label>
      <label>Chave da IA
        <input name="apiKey" type="password" autocomplete="off" placeholder="${hasKey ? "guardada cifrada ✓ (cole outra para trocar)" : "cole a chave aqui"}">
      </label>
      ${server ? html`<label class="check"><input type="checkbox" name="enabled" ${cfg.enabled ? "checked" : ""}> Usar a nuvem mesmo com o computador conectado</label>` : ""}
      <div class="row-actions tight">
        <button type="submit">Salvar</button>
        ${cfg.key ? html`<button type="button" class="secondary" data-action="cloud-forget">Apagar a chave e a conversa</button>` : ""}
      </div>
    </form>
  </section>`;
}

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

    ${cloudSection(server)}

    ${!server ? html`<section class="panel" id="sozinho">
      <h2>Sem computador?</h2>
      ${ledger.categories.length
        ? html`<p class="small muted">Você está usando o finan_rich só neste celular. Faça um backup de vez em quando em <a href="/exportar">Exportar</a>. Se um dia conectar a um computador, tudo vai para lá, sem duplicar categorias.</p>`
        : html`<p class="small muted">Dá para usar só neste celular: lançamentos, dívidas, metas, importação de extratos, notas e exportação funcionam sem computador. O consultor com IA funciona com uma IA na nuvem (veja acima). Se depois conectar a um, seus dados vão junto.</p>
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
  bindCloud(root, ctx);
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
      resetChat(); // o consultor passa a usar o computador
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
    resetChat();
    await ctx.refresh("Desconectado do computador.");
  });
  on("wipe", async () => {
    if (!confirm("Apagar todos os dados deste celular? Os que já estão no computador continuam lá.")) return;
    await db.wipe();
    await ctx.refresh("Dados deste celular apagados.");
  });
}

// A seção da IA na nuvem se redesenha sozinha (carrega depois, do advisor.wasm):
// redesenhar a tela toda fecharia o que a pessoa abriu (ex.: o pareamento).
function bindCloud(root, ctx) {
  const redraw = () => {
    const section = root.querySelector("#ia-nuvem");
    if (!section) return;
    section.outerHTML = toString(cloudSection(ctx.server));
    bindCloud(root, ctx);
  };
  if (!ai) {
    ai = { loading: true };
    Promise.all([cloud.presets(), cloud.config()])
      .then(([presets, cfg]) => { ai = { presets, cfg }; })
      .catch((err) => { ai = { error: "O consultor sem computador só funciona no app do celular (" + err.message + ")." }; })
      .then(redraw);
  }
  const form = root.querySelector('[data-form="cloud-ai"]');
  form?.elements.preset.addEventListener("change", (e) => { ai.chosen = e.target.value; redraw(); });
  form?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    try {
      if (!f.apiKey.value && !(ai.cfg?.key && ai.cfg.preset === f.preset.value)) throw new Error("cole a chave da IA");
      ai.cfg = await cloud.saveConfig({ preset: f.preset.value, model: f.model.value.trim(), enabled: !!f.enabled?.checked, apiKey: f.apiKey.value });
      ai.chosen = null;
      resetChat();
      redraw();
      ctx.flash("Testando a chave…");
      try {
        await cloud.check();
        ctx.flash("Chave funcionando ✓ Ela ficou guardada cifrada neste celular. Já pode usar o Consultor.");
      } catch (err) {
        ctx.error(new Error("a chave foi guardada, mas o teste falhou: " + err.message));
      }
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="cloud-forget"]')?.addEventListener("click", async () => {
    if (!confirm("Apagar a chave da IA e a conversa com ela deste celular?")) return;
    await cloud.forget();
    ai.cfg = null;
    resetChat();
    redraw();
  });
}
