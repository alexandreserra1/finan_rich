// Ajustes do app do celular: pareamento com o computador.
import * as db from "../db.js";
import * as device from "../device.js";
import * as finance from "../finance.js";
import * as cloud from "../cloud-ai.js";
import * as local from "../local-ai.js";
import * as lock from "../lock.js";
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
  const free = !!preset.free; // grátis do finan_rich: sem chave nem modelo
  return html`<section class="panel" id="ia-nuvem">
    <h2>Consultor sem computador</h2>
    <p class="small muted">O consultor pode usar uma IA na nuvem, chamada direto deste celular. Antes de sair, os nomes de lojas e pessoas,
      dívidas, credores, metas e categorias criadas por você viram apelidos ("Loja AB", "Dívida 1") e as notas ficam de fora:
      a IA vê só valores, datas e categorias genéricas. A conexão é cifrada (HTTPS) e a chave fica ${cloud.inVault() ? "no cofre do sistema do celular (Android Keystore ou iOS Keychain)" : "guardada cifrada neste aparelho"}.</p>
    <form data-form="cloud-ai" class="grid-form">
      <label>IA na nuvem
        <select name="preset">${ai.presets.map((p) => html`<option value="${p.id}" ${p.id === preset.id ? "selected" : ""}>${p.name}</option>`)}</select>
      </label>
      <p class="small muted">${preset.description}</p>
      ${free ? "" : html`<details class="key-guide" ${hasKey ? "" : "open"}>
        <summary>${hasKey ? "Como pegar outra chave" : "Como pegar a chave (leva 1 minuto)"}</summary>
        <ol class="small">
          ${(preset.keySteps || []).map((s) => html`<li>${s}</li>`)}
          <li>Volte aqui, cole no campo "Chave da IA" e toque em Salvar. O app testa a chave na hora.</li>
        </ol>
        <a class="button" href="${preset.keyUrl}" target="_blank" rel="noopener">Abrir o site para pegar a chave</a>
      </details>
      <label>Modelo
        <input name="model" value="${model}" list="cloud-models" autocomplete="off">
        <datalist id="cloud-models">${(preset.models || []).map((m) => html`<option value="${m}">`)}</datalist>
      </label>
      <label>Chave da IA
        <input name="apiKey" type="password" autocomplete="off" placeholder="${hasKey ? "guardada ✓ (cole outra para trocar)" : "cole a chave aqui"}">
      </label>`}
      ${server ? html`<label class="check"><input type="checkbox" name="enabled" ${cfg.enabled ? "checked" : ""}> Usar a nuvem mesmo com o computador conectado</label>` : ""}
      <div class="row-actions tight">
        <button type="submit">Salvar</button>
        ${cloud.ready(cfg) ? html`<button type="button" class="secondary" data-action="cloud-forget">${cfg.key ? "Apagar a chave e a conversa" : "Desligar e apagar a conversa"}</button>` : ""}
      </div>
    </form>
  </section>`;
}

export function render({ server, params, ledger }) {
  const fromLink = params.has("codigo");
  const status = sync.state;
  return html`
    <a class="panel link-card" href="/contas"><span><strong>Contas e cartões</strong></span><span aria-hidden="true">›</span></a>
    <a class="panel link-card" href="/metas"><span><strong>Metas</strong></span><span aria-hidden="true">›</span></a>
    <a class="panel link-card" href="/notas"><span><strong>Notas</strong></span><span aria-hidden="true">›</span></a>
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

    ${localSection()}

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

    ${lockSection()}

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
  bindLocal(root, ctx);
  bindLock(root, ctx);
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
    await cloud.forget(); // a chave da IA no cofre do sistema também
    await db.wipe();
    lockState = null; // o bloqueio também sai
    localState = null; // e a escolha da IA dentro do celular (o modelo baixado continua)
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
      const free = ai.presets.find((p) => p.id === f.preset.value)?.free;
      if (!free && !f.apiKey.value && !(ai.cfg?.key && ai.cfg.preset === f.preset.value)) throw new Error("cole a chave da IA");
      ai.cfg = await cloud.saveConfig({ preset: f.preset.value, model: f.model?.value.trim() || "", enabled: !!f.enabled?.checked, apiKey: f.apiKey?.value || "" });
      ai.chosen = null;
      resetChat();
      redraw();
      ctx.flash("Testando a chave…");
      try {
        await cloud.check();
        ctx.flash(free ? "Consultor grátis ligado ✓ Já pode usar." : `Chave funcionando ✓ Ela ficou guardada ${cloud.inVault() ? "no cofre do sistema do celular" : "cifrada neste celular"}. Já pode usar o Consultor.`);
      } catch (err) {
        ctx.error(new Error((free ? "ficou salvo" : "a chave foi guardada") + ", mas o teste falhou: " + err.message));
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

// ---------- bloqueio do app ----------

let lockState = null; // { on, bio, editing } (carregado depois)

function lockSection() {
  const s = lockState || {};
  return html`<section class="panel" id="bloqueio">
    <h2>Bloqueio do app</h2>
    <label class="check"><input type="checkbox" data-action="lock" ${s.on || s.editing ? "checked" : ""} ${lockState ? "" : "disabled"}> Pedir digital/rosto ou senha para abrir</label>
    <p class="small muted">Pede ao abrir o app e ao voltar depois de 1 minuto fora dele.
      ${s.bio ? "Se a digital ou o rosto falhar, vale a senha do app." : "Sem digital ou rosto cadastrados no celular, vale a senha do app."}
      Protege de quem pegar o celular desbloqueado; os dados continuam no celular do mesmo jeito.</p>
    ${s.editing ? html`<form data-form="lock-pin" class="grid-form">
      <label>Senha do app (4 a 6 números)
        <input name="pin" type="password" inputmode="numeric" autocomplete="new-password" maxlength="6" required>
      </label>
      <label>Repita a senha
        <input name="confirm" type="password" inputmode="numeric" autocomplete="new-password" maxlength="6" required>
      </label>
      <p class="small muted">Se esquecer a senha e a digital/rosto não funcionar, o único jeito de abrir é apagar os dados deste celular (os do computador continuam lá).</p>
      <div class="row-actions tight">
        <button type="submit">Ligar o bloqueio</button>
        <button type="button" class="secondary" data-action="lock-cancel">Cancelar</button>
      </div>
    </form>` : ""}
  </section>`;
}

// Como a IA na nuvem, a seção se redesenha sozinha.
function bindLock(root, ctx) {
  const redraw = () => {
    const section = root.querySelector("#bloqueio");
    if (!section) return;
    section.outerHTML = toString(lockSection());
    bindLock(root, ctx);
  };
  if (!lockState) lock.status().then((s) => { lockState = s; redraw(); });
  root.querySelector('[data-action="lock"]')?.addEventListener("change", async (e) => {
    if (e.target.checked) {
      lockState.editing = true;
      redraw();
      root.querySelector('[data-form="lock-pin"] input[name="pin"]')?.focus();
      return;
    }
    if (lockState.editing) {
      lockState.editing = false;
    } else {
      if (!confirm("Desligar o bloqueio? O app vai abrir sem pedir digital, rosto ou senha.")) return redraw();
      await lock.disable();
      lockState.on = false;
      ctx.flash("Bloqueio desligado.");
    }
    redraw();
  });
  root.querySelector('[data-action="lock-cancel"]')?.addEventListener("click", () => { lockState.editing = false; redraw(); });
  root.querySelector('[data-form="lock-pin"]')?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target.elements;
    try {
      if (!lock.validPIN(f.pin.value)) throw new Error("a senha do app precisa ter de 4 a 6 números");
      if (f.pin.value !== f.confirm.value) throw new Error("as duas senhas não são iguais");
      await lock.enable(f.pin.value);
      lockState = { on: true, bio: lockState.bio };
      redraw();
      ctx.flash(lockState.bio ? "Bloqueio ligado: o app vai pedir a digital ou o rosto (ou a senha do app)." : "Bloqueio ligado: o app vai pedir a senha do app.");
    } catch (err) {
      ctx.error(err);
    }
  });
}

// ---------- IA dentro do celular ----------

let localState = null; // { downloaded, enabled, progress } (carregado depois; null = ainda não ou sem wllama)

function localSection() {
  if (!localState) return html`<section id="ia-local" hidden></section>`;
  const s = localState;
  const m = local.MODEL;
  return html`<section class="panel" id="ia-local">
    <h2>IA dentro do celular</h2>
    <p class="small muted">O consultor roda neste celular, sem internet: nem a pergunta nem os seus números saem do aparelho.
      Usa o modelo ${m.name} (código aberto, Apache-2.0), baixado uma vez (${m.mb} MB; prefira o Wi-Fi).
      É uma IA pequena: responde mais devagar que a do computador ou da nuvem, erra mais e não lança nada por você.</p>
    ${navigator.gpu ? "" : html`<p class="notice warn">Este celular não oferece aceleração gráfica (WebGPU) para o app: a IA dentro dele fica muito lenta, vários minutos por resposta.</p>`}
    ${s.progress != null
      ? html`<p><progress max="100" value="${Math.round(s.progress * 100)}" aria-label="Baixando o modelo"></progress> <span data-local="pct">${Math.round(s.progress * 100)}%</span></p>`
      : s.downloaded
        ? html`<label class="check"><input type="checkbox" data-action="local-ai" ${s.enabled ? "checked" : ""}> Usar a IA dentro do celular no Consultor</label>
          <div class="row-actions tight"><button type="button" class="secondary" data-action="local-remove">Apagar o modelo (libera ${m.mb} MB)</button></div>`
        : html`<div class="row-actions tight"><button type="button" data-action="local-download">Baixar o modelo (${m.mb} MB)</button></div>`}
  </section>`;
}

// Como a IA na nuvem, a seção se redesenha sozinha.
function bindLocal(root, ctx) {
  const redraw = () => {
    const section = root.querySelector("#ia-local");
    if (!section) return;
    section.outerHTML = toString(localSection());
    bindLocal(root, ctx);
  };
  if (!localState) {
    local.available().then(async (has) => {
      if (!has) return;
      localState = { downloaded: await local.downloaded(), enabled: !!(await local.config())?.enabled, progress: null };
      redraw();
    });
    return;
  }
  root.querySelector('[data-action="local-download"]')?.addEventListener("click", async () => {
    localState.progress = 0;
    redraw();
    try {
      await local.download((p) => {
        localState.progress = p;
        const bar = document.querySelector("#ia-local progress");
        if (bar) bar.value = Math.round(p * 100);
        const pct = document.querySelector('#ia-local [data-local="pct"]');
        if (pct) pct.textContent = `${Math.round(p * 100)}%`;
      });
      await local.setEnabled(true);
      localState = { downloaded: true, enabled: true, progress: null };
      resetChat();
      ctx.flash("Modelo baixado ✓ O Consultor já usa a IA dentro do celular.");
    } catch (err) {
      localState.progress = null;
      ctx.error(new Error("não deu para baixar o modelo: " + (err instanceof TypeError ? "sem internet" : err.message)));
    }
    redraw();
  });
  root.querySelector('[data-action="local-ai"]')?.addEventListener("change", async (e) => {
    await local.setEnabled(e.target.checked);
    localState.enabled = e.target.checked;
    resetChat();
    ctx.flash(e.target.checked ? "O Consultor vai usar a IA dentro do celular." : "O Consultor volta a usar o computador ou a nuvem.");
  });
  root.querySelector('[data-action="local-remove"]')?.addEventListener("click", async () => {
    if (!confirm(`Apagar o modelo da IA (${local.MODEL.mb} MB)? Dá para baixar de novo depois.`)) return;
    await local.remove();
    localState = { downloaded: false, enabled: false, progress: null };
    resetChat();
    ctx.flash("Modelo apagado.");
    redraw();
  });
}
