// Ajustes do app do celular e do site: índice das seções à esquerda (no
// computador) e um cartão por assunto, como no settings.html.
import * as db from "../db.js";
import * as device from "../device.js";
import * as finance from "../finance.js";
import * as google from "../google.js";
import * as cloud from "../cloud-ai.js";
import * as local from "../local-ai.js";
import * as openFinance from "../openfinance.js";
import * as lock from "../lock.js";
import { resetChat } from "./advisor.js";
import * as sync from "../sync.js";
import { canScan, hasReminders, remindersEnabled, scanPairingQR, setRemindersEnabled } from "../native.js";
import { html, toString } from "../ui.js";
import { pageHead } from "./dashboard.js";

export const needsData = false; // funciona antes de baixar qualquer dado

// head: título do cartão com a linha de explicação embaixo.
const head = (title, sub, extra = "") => html`<div class="panel-head"><div><h2>${title}</h2>${sub ? html`<span class="muted small">${sub}</span>` : ""}</div>${extra}</div>`;

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
let gState; // conta Google: undefined = carregando; { on, account }

// googleSection: entrar com Google para os dados ficarem no Drive da pessoa.
// Some se o login não estiver ligado no servidor.
function googleSection() {
  if (!gState?.on) return html`<section id="google" hidden></section>`;
  const st = sync.state;
  const email = gState.account?.email || "sua conta Google";
  return html`<section class="panel" id="google">
    ${head("Conta e sincronização", "Com Google, os dados ficam também no seu Drive e aparecem iguais no celular, no site e no computador.")}
    ${gState.account
      ? html`<div class="set-row">
          <span class="avatar" aria-hidden="true">${email[0].toUpperCase()}</span>
          <span class="set-row-text"><strong>${email}</strong>
            <span class="faint small">Google Drive · pasta do finan_rich${st.lastSync ? ` · sincronizado às ${st.lastSync.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}` : ""}</span></span>
          <span class="set-row-actions">
            <button type="button" class="small-btn" data-action="google-sync">Sincronizar agora</button>
            <button type="button" class="secondary small-btn" data-action="google-out">Sair</button>
          </span>
        </div>
        <p class="faint small set-note">${device.isSite
          ? "Ao sair, os dados saem deste navegador e continuam no seu Drive."
          : "Ao sair, os dados continuam neste celular e no seu Drive."} Nenhum servidor do finan_rich guarda nada.</p>`
      : html`<div class="set-row dashed">
          <span class="set-row-text"><strong>Só neste aparelho</strong>
            <span class="faint small">Entre com Google para não perder seus dados. Eles ficam numa pasta do seu Drive que só o finan_rich usa.</span></span>
          <button type="button" class="google" data-action="google-in"><span class="g" aria-hidden="true">G</span>Entrar com Google</button>
        </div>`}
  </section>`;
}

function bindGoogle(root, ctx) {
  const redraw = () => {
    const section = root.querySelector("#google");
    if (!section) return;
    section.outerHTML = toString(googleSection());
    const link = root.querySelector('.settings-index a[href="#google"]');
    if (link) link.hidden = !gState?.on;
    bindGoogle(root, ctx);
  };
  if (gState === undefined) {
    gState = null;
    Promise.all([google.available(), google.account()])
      .then(([on, account]) => { gState = { on, account }; })
      .catch(() => { gState = { on: false }; })
      .then(redraw);
  }
  const on = (action, fn) => root.querySelector(`[data-action="${action}"]`)?.addEventListener("click", async () => {
    try { await fn(); } catch (err) { ctx.error(err); }
  });
  on("google-in", () => google.signIn(device.isSite ? "web" : "app"));
  on("google-sync", async () => {
    await sync.sync();
    if (sync.state.status !== "ok") throw new Error(sync.state.message || "não deu para sincronizar agora; tente de novo");
    redraw();
    ctx.flash("Sincronizado com o Google Drive.");
  });
  on("google-out", async () => {
    if (device.isSite) {
      // O site pode estar num computador de outra pessoa: sair leva os dados
      // embora deste navegador (ficam no Drive). Antes, garante que o Drive
      // tem tudo; sem isso, não apaga nada.
      if (!confirm("Sair da conta Google? Seus dados saem deste navegador e continuam no seu Google Drive.")) return;
      await sync.sync();
      if (sync.state.status !== "ok") throw new Error("não consegui salvar no Google Drive agora. Confira a internet e tente sair de novo; nada foi apagado.");
      await forgetHere();
      location.assign("/"); // recarrega do zero: nada dos dados fica na memória da página
      return;
    }
    if (!confirm("Sair da conta Google? Seus dados continuam neste aparelho e no seu Drive.")) return;
    await google.signOut();
    gState = { on: true, account: null };
    await ctx.refresh("Você saiu da conta Google.");
  });
}
let ai = null; // { presets, cfg } da IA na nuvem (carregado depois, do advisor.wasm)

// aiPath: "finan" (a IA grátis do finan_rich) ou "propria" (a chave da pessoa).
function aiPath(free) {
  if (!free) return "propria";
  if (ai.path) return ai.path;
  return ai.cfg?.preset && ai.cfg.preset !== free.id ? "propria" : "finan";
}

function cloudSection(server) {
  const title = "Consultor IA";
  if (!ai || ai.loading) return html`<section class="panel" id="ia-nuvem">${head(title)}<p class="muted small loading">Carregando…</p></section>`;
  if (ai.error) return html`<section class="panel" id="ia-nuvem">${head(title)}<p class="notice warn">${ai.error}</p></section>`;
  const cfg = ai.cfg || {};
  const free = ai.presets.find((p) => p.free); // some se o grátis não estiver ligado
  const own = ai.presets.filter((p) => !p.free);
  const path = aiPath(free);
  const preset = own.find((p) => p.id === (ai.chosen || cfg.preset)) || own[0];
  const model = cfg.preset === preset.id && cfg.model ? cfg.model : preset.model;
  const hasKey = cfg.key && cfg.preset === preset.id;
  const where = cloud.inVault() ? "no cofre do sistema do celular (Android Keystore ou iOS Keychain)" : "guardada cifrada neste aparelho";
  const active = cloud.ready(cfg) && ai.presets.find((p) => p.id === cfg.preset);
  const card = (value, name, line, tag) => html`<label class="choice-card">
    <input type="radio" class="pick" name="caminho" value="${value}" ${path === value ? "checked" : ""}>
    <span><span class="choice-title"><strong>${name}</strong>${tag ? html`<span class="tag on">${tag}</span>` : ""}</span><span class="muted small">${line}</span></span>
  </label>`;
  return html`<section class="panel" id="ia-nuvem">
    ${head(title, server
      ? "Com o computador conectado, o Consultor usa a IA dele. Aqui fica a IA para quando você quiser usar sem ele."
      : "Escolha de onde vem a IA que conversa com você no Consultor.")}
    ${active
      ? html`<p class="ai-status on"><span class="dot" aria-hidden="true"></span><span>Ligado: <b>${active.free ? "IA do finan_rich" : active.name}</b>${active.free ? "" : html` com o modelo <code>${cfg.model || active.model}</code>`}</span></p>`
      : html`<p class="ai-status"><span class="dot" aria-hidden="true"></span><span>Ainda não ligado. Escolha um dos caminhos e salve.</span></p>`}
    <form data-form="cloud-ai">
      <div class="choice-cards" role="radiogroup" aria-label="De onde vem a IA">
        ${free ? card("finan", "Usar a IA do finan_rich", "Grátis, sem conta e sem chave. É só ligar.", "Grátis") : ""}
        ${card("propria", "Usar a minha própria chave", "Você escolhe o provedor e o modelo, sem o limite do finan_rich.")}
      </div>
      ${path === "finan"
        ? html`<ul class="ai-facts">
            <li>Não precisa criar conta nem pegar chave.</li>
            <li>Antes de sair do aparelho, nomes de lojas, dívidas e metas viram apelidos ("Loja AB", "Dívida 1"). Notas ficam de fora.</li>
            <li>Tem um limite justo de perguntas por dia, para dar para todo mundo.</li>
            <li>Se a IA não responder, o time responde do mesmo jeito, com as contas do próprio app.</li>
          </ul>`
        : html`<div class="ai-own">
          <p class="small muted">Os nomes também saem trocados por apelidos, direto deste aparelho para o provedor, com conexão cifrada (HTTPS). A chave fica ${where}.</p>
          <label>Provedor
            <select name="preset">${own.map((p) => html`<option value="${p.id}" ${p.id === preset.id ? "selected" : ""}>${p.name}</option>`)}</select>
          </label>
          <p class="small muted">${preset.description}</p>
          <details class="key-guide" ${hasKey ? "" : "open"}>
            <summary>${hasKey ? "Como pegar outra chave" : "Como pegar a chave (leva 1 minuto)"}</summary>
            <ol class="small">
              ${(preset.keySteps || []).map((st) => html`<li>${st}</li>`)}
              <li>Volte aqui, cole no campo "Chave da IA" e toque em Salvar e testar.</li>
            </ol>
            <a class="button" href="${preset.keyUrl}" target="_blank" rel="noopener">Abrir o site para pegar a chave</a>
          </details>
          <div class="grid-form">
            <label>Chave da IA
              <input name="apiKey" type="password" autocomplete="off" placeholder="${hasKey ? "guardada ✓ (cole outra para trocar)" : "cole a chave aqui"}">
            </label>
            <label>Modelo
              <input name="model" class="mono" value="${model}" list="cloud-models" autocomplete="off">
              <datalist id="cloud-models">${(preset.models || []).map((m) => html`<option value="${m}">`)}</datalist>
            </label>
          </div>
        </div>`}
      ${server ? html`<label class="check"><input type="checkbox" name="enabled" ${cfg.enabled ? "checked" : ""}> Usar esta IA mesmo com o computador conectado</label>` : ""}
      <div class="row-actions">
        <button type="submit">${path === "finan" ? "Usar a IA do finan_rich" : "Salvar e testar"}</button>
        ${cloud.ready(cfg) ? html`<button type="button" class="secondary" data-action="cloud-forget">${cfg.key ? "Apagar a chave e a conversa" : "Desligar e apagar a conversa"}</button>` : ""}
      </div>
    </form>
  </section>`;
}

const THEMES = [["claro", "Claro"], ["escuro", "Escuro"], ["auto", "Igual ao aparelho"]];

// themeSection: um cartão com a miniatura de cada tema (theme.js troca e guarda).
export function themeCards(current) {
  return html`<div class="theme-cards" role="radiogroup" aria-label="Tema">
    ${THEMES.map(([v, label]) => html`<label class="theme-card">
      <span class="theme-preview tp-${v}" aria-hidden="true"><i class="tp-side"></i><span class="tp-main"><i class="tp-bar"></i><i class="tp-bar tp-accent"></i><i class="tp-card"></i></span></span>
      <span class="theme-card-foot"><strong>${label}</strong><input type="radio" class="pick" name="tema" value="${v}" data-theme-pick ${current === v ? "checked" : ""}></span>
    </label>`)}
  </div>`;
}

const SHORTCUTS = [["/contas", "Contas e cartões", "Saldos, faturas e patrimônio"], ["/metas", "Metas", "Quanto falta e quando chega"],
  ["/notas", "Notas", "Anotações que o Consultor também lê"], ["/categorias", "Categorias e divisão do salário", "Grupos, nomes e os 50/30/20"]];

export function render({ server, params, ledger }) {
  const fromLink = params.has("codigo");
  const status = sync.state;
  const index = [["#atalhos", "Mais telas"], ["#tema", "Aparência"], ["#google", "Conta e sincronização", !gState?.on],
    !device.isSite && ["#parear", "Computador"], ["#ia-nuvem", "Consultor IA"], !device.isSite && ["#open-finance", "Open Finance"],
    hasReminders() && ["#lembretes", "Lembretes"], ["#bloqueio", "Bloqueio"], ["#dados", "Seus dados"]].filter(Boolean);
  return html`
    ${pageHead("Ajustes", null, { kicker: device.isSite ? "Este navegador" : "Este celular" })}
    <div class="settings">
      <nav class="settings-index" aria-label="Seções de ajustes">
        ${index.map(([href, label, hidden]) => html`<a href="${href}" ${hidden ? "hidden" : ""}>${label}</a>`)}
      </nav>
      <div class="settings-body">

    <section class="panel" id="atalhos">
      ${head("Mais telas")}
      <div class="link-grid">
        ${SHORTCUTS.map(([href, name, sub]) => html`<a class="link-card" href="${href}"><span><span class="lc-text"><strong>${name}</strong><span class="faint small">${sub}</span></span></span><span aria-hidden="true">›</span></a>`)}
      </div>
    </section>

    <section class="panel" id="tema">
      ${head("Aparência", "Vale só para este aparelho e não vai na sincronização.")}
      ${themeCards(window.finanTheme?.get() || "auto")}
    </section>

    ${googleSection()}

    ${device.isSite ? "" : html`<section class="panel" id="parear">
      ${head("Computador", "O celular guarda tudo e sincroniza com o finan_rich do computador pelo Wi-Fi.")}
      ${server
        ? html`
          <div class="set-row">
            <span class="avatar" aria-hidden="true"><svg class="icon" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></svg></span>
            <span class="set-row-text"><strong>Conectado a <code>${server.url}</code></strong>
              <span class="faint small">como ${server.device} · ${status.lastSync ? `última sincronização ${status.lastSync.toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })}` : "ainda não sincronizou nesta abertura"}${status.pending ? ` · ${status.pending} alteração(ões) esperando` : ""}</span></span>
            <span class="set-row-actions">
              <button type="button" class="small-btn" data-action="sync">Sincronizar agora</button>
              <button type="button" class="secondary small-btn danger" data-action="unpair">Desconectar</button>
            </span>
          </div>
          <p class="faint small set-note">Tudo que você faz fica salvo neste celular e vai para o computador quando ele estiver ao alcance (mesmo Wi-Fi e finan_rich aberto).</p>
          ${fromLink
            ? html`<h3>Conectar a outro computador?</h3>
              <p class="notice warn">Você abriu um link de pareamento. Só confirme se o endereço abaixo for mesmo o do seu computador.</p>
              ${pairForm(params, true)}`
            : html`<details><summary>Conectar a outro computador</summary>${pairForm(params, true)}</details>`}`
        : html`
          <p class="small muted">O app guarda seus dados neste celular e funciona sem internet. Para trazer os dados que já estão no computador e manter os dois iguais, conecte-o uma vez ao finan_rich do computador.</p>
          ${fromLink ? html`<p class="notice warn">Confira se o endereço abaixo é mesmo o do seu computador antes de conectar.</p>` : ""}
          ${pairForm(params, false)}`}
    </section>`}

    ${cloudSection(server)}

    ${localSection()}

    ${device.isSite ? "" : openFinanceSection()}

    ${hasReminders() ? html`<section class="panel" id="lembretes">
      ${head("Lembretes", "O aviso é do próprio celular: funciona sem internet.")}
      <label class="check"><input type="checkbox" data-action="reminders" ${remindersOn ? "checked" : ""}> Lembretes do time (às 9h)</label>
      <ul class="small muted team-reminders">
        <li><b>Luna</b>: 2 dias antes do vencimento de cada dívida (cadastre o dia em Dívidas).</li>
        <li><b>Otávio</b>: toda segunda, os achados da semana.</li>
        <li><b>Tito</b>: no dia do salário, o balanço das metas.</li>
      </ul>
    </section>` : ""}

    ${lockSection()}

    <section class="panel danger-zone" id="dados">
      ${head("Seus dados", device.isSite
        ? "Ficam neste navegador e no seu Google Drive (nada vai para servidor do finan_rich). Sem usar por 30 minutos, o site pede para entrar de novo."
        : server ? "Ficam neste celular e no computador conectado." : "Ficam só neste celular.")}
      ${!device.isSite && !server ? html`<div id="sozinho">
        ${ledger.categories.length
          ? html`<p class="small muted">Você está usando o finan_rich só neste celular. Faça um backup de vez em quando em <a href="/exportar">Exportar</a>. Se um dia conectar a um computador, tudo vai para lá, sem duplicar categorias.</p>`
          : html`<p class="small muted">Dá para usar só neste celular: lançamentos, dívidas, metas, importação de extratos, notas e exportação funcionam sem computador. O Consultor funciona com a IA do finan_rich ou com a sua chave (veja acima). Se depois conectar a um computador, seus dados vão junto.</p>
            <div class="row-actions"><button type="button" class="secondary" data-action="solo">Usar só neste celular</button></div>`}
      </div>` : ""}
      <div class="set-row">
        <span class="set-row-text"><strong>Apagar dados ${device.here}</strong>
          <span class="faint small">Apaga os lançamentos, dívidas e categorias guardados ${device.here}${device.isSite ? ". Os do seu Drive continuam lá." : " e desconecta do computador. Os dados que já estão no computador continuam lá."}</span></span>
        <button type="button" class="secondary danger" data-action="wipe">Apagar dados ${device.here}</button>
      </div>
    </section>
      </div>
    </div>`;
}

// startSolo começa a usar sem computador: cria as categorias padrão neste aparelho.
export async function startSolo(ctx, to) {
  try {
    await finance.load();
    const cats = finance.defaultCategories(Date.now());
    await db.saveManyLocal(cats.map((record) => ({ type: "category", record })));
    await ctx.refresh();
    ctx.go(to);
  } catch (err) {
    ctx.error(err);
  }
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
  bindGoogle(root, ctx);
  bindCloud(root, ctx);
  bindLocal(root, ctx);
  bindOpenFinance(root, ctx);
  bindLock(root, ctx);
  root.querySelector('[data-action="solo"]')?.addEventListener("click", () => startSolo(ctx, "/lancamentos"));
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
    if (!confirm(device.isSite ? "Apagar todos os dados deste navegador? Não dá para desfazer." : "Apagar todos os dados deste celular? Os que já estão no computador continuam lá.")) return;
    await forgetHere();

    await ctx.refresh(`Dados ${device.here} apagados.`);
  });
}

// forgetHere apaga tudo deste aparelho: conta Google, chaves, pareamento e o banco.
async function forgetHere() {
  await google.signOut(); // a conta Google sai antes do banco (o token está nele)
  gState = undefined;
  await cloud.forget(); // a chave da IA no cofre do sistema também
  await device.unpair(); // o token do computador sai do cofre também
  await db.wipe();
  lockState = null; // o bloqueio também sai
  localState = null; // e a escolha da IA dentro do celular (o modelo baixado continua)
  await openFinance.forget();
  ofState = undefined;
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
  form?.elements.preset?.addEventListener("change", (e) => { ai.chosen = e.target.value; redraw(); });
  form?.querySelectorAll('input[name="caminho"]').forEach((r) => r.addEventListener("change", () => {
    ai.path = r.value;
    redraw();
    root.querySelector(`#ia-nuvem input[name="caminho"][value="${r.value}"]`)?.focus(); // o foco não se perde ao redesenhar
  }));
  form?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    try {
      const free = f.caminho.value === "finan";
      const preset = free ? ai.presets.find((p) => p.free).id : f.preset.value;
      if (!free && !f.apiKey.value && !(ai.cfg?.key && ai.cfg.preset === preset)) throw new Error("cole a chave da IA");
      ai.cfg = await cloud.saveConfig({ preset, model: free ? "" : f.model.value.trim(), enabled: !!f.enabled?.checked, apiKey: free ? "" : f.apiKey.value });
      ai.chosen = null;
      ai.path = null;
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
    ${head("Bloqueio do app", "Protege de quem pegar o celular desbloqueado; os dados continuam no celular do mesmo jeito.")}
    <label class="check"><input type="checkbox" data-action="lock" ${s.on || s.editing ? "checked" : ""} ${lockState ? "" : "disabled"}> Pedir digital/rosto ou senha para abrir</label>
    <p class="small muted">Pede ao abrir o app e ao voltar depois de 1 minuto fora dele.
      ${s.bio ? "Se a digital ou o rosto falhar, vale a senha do app." : "Sem digital ou rosto cadastrados no celular, vale a senha do app."}</p>
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

// ---------- Open Finance (Meu Pluggy) ----------

let ofState; // undefined = ainda carregando; null = não configurado; { clientId, itemIds }

function openFinanceSection() {
  if (ofState === undefined) return html`<section id="open-finance" hidden></section>`;
  const c = ofState || {};
  return html`<section class="panel" id="open-finance">
    ${head("Open Finance", "Lançamentos direto do banco, de graça, pelo Meu Pluggy, com a sua própria conta. Sem passar por servidor do finan_rich.",
      html`<span class="tag ${ofState ? "on" : ""}">${ofState ? "Configurado" : "Não configurado"}</span>`)}
    <p class="small muted">Depois de configurar, use <a href="/importar">Importar → Buscar do banco</a>.</p>
    <details ${ofState ? "" : "open"}><summary>Como configurar (uma vez)</summary>
      <ol class="small">
        <li>Crie uma conta grátis em dashboard.pluggy.ai, crie uma <strong>Application</strong> e copie o <strong>Client ID</strong> e o <strong>Client Secret</strong>.</li>
        <li>Em meu.pluggy.ai, conecte seus bancos (Open Finance oficial, autorizado no app do banco).</li>
        <li>No painel da Pluggy: sua Application → <em>Ir para Demo</em> → ⋮ → <em>Copiar Item ID</em> (um por banco).</li>
        <li>Faça isso durante o teste da conta Pluggy (cerca de 15 dias): depois os bancos continuam, mas não dá para incluir outros.</li>
      </ol>
    </details>
    <form data-form="open-finance" class="grid-form">
      <label>Client ID <input name="client_id" value="${c.clientId || ""}" autocomplete="off" spellcheck="false" required></label>
      <label>Client Secret <input type="password" name="client_secret" autocomplete="off" placeholder="${ofState ? "salvo (deixe em branco para manter)" : ""}"></label>
      <label class="wide">Item IDs (um por linha) <textarea name="items" rows="2" spellcheck="false" required>${(c.itemIds || []).join("\n")}</textarea></label>
      <div class="form-actions">
        <button type="submit">Salvar</button>
        ${ofState ? html`<button type="button" class="secondary" data-action="of-forget">Apagar credenciais</button>` : ""}
      </div>
    </form>
    <p class="small muted">O Client Secret fica no cofre do sistema (o mesmo da digital).</p>
  </section>`;
}

function bindOpenFinance(root, ctx) {
  const redraw = () => {
    const section = root.querySelector("#open-finance");
    if (!section) return;
    section.outerHTML = toString(openFinanceSection());
    bindOpenFinance(root, ctx);
  };
  if (ofState === undefined) {
    openFinance.config().then((c) => { ofState = c; redraw(); });
    return;
  }
  root.querySelector('[data-form="open-finance"]')?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target.elements;
    const itemIds = f.items.value.split(/[\s,;]+/).filter(Boolean);
    try {
      if (itemIds.some((id) => !/^[0-9A-Za-z-]{1,64}$/.test(id))) throw new Error("Item ID inválido: copie do painel da Pluggy (Ir para Demo → Copiar Item ID)");
      await openFinance.save({ clientId: f.client_id.value, clientSecret: f.client_secret.value, itemIds });
      ofState = await openFinance.config();
      redraw();
      ctx.flash("Open Finance salvo. Use Importar → Buscar do banco.");
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelector('[data-action="of-forget"]')?.addEventListener("click", async () => {
    if (!confirm("Apagar as credenciais do Open Finance deste celular?")) return;
    await openFinance.forget();
    ofState = null;
    redraw();
  });
}

// ---------- IA dentro do celular ----------

let localState = null; // { downloaded, enabled, progress } (carregado depois; null = ainda não ou sem wllama)

function localSection() {
  if (!localState) return html`<section id="ia-local" hidden></section>`;
  const s = localState;
  const m = local.MODEL;
  return html`<section class="panel" id="ia-local">
    ${head("IA dentro do celular", "Um terceiro caminho, sem internet nenhuma.")}
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
