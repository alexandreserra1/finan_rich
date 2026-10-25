// Landing: a página pública do site, antes do login (visual Esmeralda; ver
// docs/design_handoff_finan_rich_esmeralda 3/Landing.dc.html). Aparece só no
// site, para quem chega sem dados e sem conta Google ligada; quem já usa vai
// para o Painel, e o app do computador e o do Android nunca a mostram.
// "Começar agora" leva ao login (/?entrar=1, as boas-vindas com o Google) e
// "Usar sem conta" segue o fluxo de sempre. Os números daqui são ilustrativos.
import * as device from "../device.js";
import * as finance from "../finance.js";
import { html, raw, shiftMonth, today } from "../ui.js";
import { startSolo } from "./device.js";

export const needsData = false;

// show: site, sem conta Google e sem ter pedido o login (?entrar=1).
export const show = (ctx, signedIn) => device.isSite && !signedIn && !ctx.params.get("entrar");

const LOGIN = "/?entrar=1";
const IMG = "/static/img/";
const svg = (d, size = 18) => raw(`<svg class="lp-ico" width="${size}" height="${size}" viewBox="0 0 24 24" aria-hidden="true"><path d="${d}"/></svg>`);
const ICON = {
  arrow: "M5 12h14M13 6l6 6-6 6", check: "M20 6L9 17l-5-5", left: "M15 18l-6-6 6-6", right: "M9 18l6-6-6-6", up: "M12 19V5M5 12l7-7 7 7",
  food: "M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2M7 2v20M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3zm0 0v7",
  car: "M19 17h2v-4l-3-5H6l-3 5v4h2M7 17a2 2 0 1 0 4 0 2 2 0 1 0-4 0M13 17a2 2 0 1 0 4 0 2 2 0 1 0-4 0",
  cart: "M8 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM19 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM2 2h3l2.7 12.4a2 2 0 0 0 2 1.6h9.7a2 2 0 0 0 2-1.6L23 6H6",
  home: "M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z", list: "M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01",
  card: "M2 7a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2zM2 10h20", chart: "M3 3v18h18M7 15l4-4 3 3 5-6",
  chat: "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z", plus: "M12 5v14M5 12h14", flag: "M4 22V4M4 4h12l-2 4 2 4H4",
  bank: "M3 10l9-6 9 6M5 10v8M9.5 10v8M14.5 10v8M19 10v8M3 21h18", shield: "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z",
  wifi: "M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0M12 19.5h.01",
  gift: "M20 12v10H4V12M2 7h20v5H2zM12 22V7M12 7H7.5a2.5 2.5 0 0 1 0-5C11 2 12 7 12 7zM12 7h4.5a2.5 2.5 0 0 0 0-5C13 2 12 7 12 7z",
};

// O time: uma persona por função que o app já tem (não são agentes separados).
const TEAM = [
  { name: "Jade", role: "Consultora", img: "team-pantera.png", c: "#a184f0", say: "Restaurantes subiram 41% este mês.",
    what: "A pantera do time. Conversa sobre os seus números e propõe a próxima jogada; nada muda sem você confirmar." },
  { name: "Bento", role: "Dívidas", img: "team-urso.png", c: "#8fbf4a", say: "Pague o rotativo antes: 13,5% a.m.",
    what: "O urso de armadura. Diz qual dívida atacar primeiro e em que mês você fica livre de todas." },
  { name: "Luna", role: "Alertas", img: "team-raposa.png", c: "#f08a3c", say: "Fatura do cartão vence em 3 dias.",
    what: "A raposa esperta. Avisa quando a fatura está perto de vencer ou uma categoria passou do teto." },
  { name: "Tito", role: "Metas", img: "team-texugo.png", c: "#d0a46a", say: "Guarde R$ 550/mês para a reserva.",
    what: "O texugo com escudo. Protege a sua reserva e calcula quanto guardar por mês para cada meta." },
  { name: "Otávio", role: "Análise", img: "team-coruja.png", c: "#3fb6d8", say: "Academia: R$ 1.318,80 por ano.",
    what: "A coruja que enxerga tudo. Mostra para onde foi cada real e acha assinaturas esquecidas." },
];

// Exemplos do consultor (a seção "Você fala. A Jade faz as contas.").
const DEMOS = [
  { title: "Lançar um gasto", desc: "Fale do seu jeito: ela entende valor, categoria e data.", icon: ICON.plus,
    q: "gastei 58,90 no iFood agora", thinking: "Entendendo o lançamento…", a: "Lancei como saída em Restaurantes e delivery, hoje, no cartão.",
    kind: "Novo lançamento", prop: "iFood · R$ 58,90 · hoje", btn: ["Adicionar", "Adicionado"] },
  { title: "Sair das dívidas", desc: "Qual pagar primeiro, quanto de juros e quando acaba.", icon: ICON.card,
    q: "qual dívida eu pago primeiro?", thinking: "Comparando os juros…", a: "Comece pelo rotativo do cartão: é a mais cara e come R$ 396,90 de juros por mês.",
    rows: [["Rotativo do cartão", "13,50% a.m.", "neg"], ["Empréstimo pessoal", "3,20% a.m.", ""], ["Financiamento moto", "1,90% a.m.", ""]],
    kind: "Plano", prop: "R$ 300 a mais por mês no rotativo", btn: ["Aplicar", "Aplicado"] },
  { title: "Guardar na meta", desc: "Ela soma na meta e refaz quanto falta por mês.", icon: ICON.flag,
    q: "guardei 500 pra viagem", thinking: "Atualizando a meta…", a: "Boa! A viagem chega a 35%. Faltam R$ 5.150, ou R$ 572 por mês até julho.",
    ring: true, kind: "Guardar na meta", prop: "R$ 500,00 em Viagem para o Chile", btn: ["Guardar", "Guardado"] },
  { title: "Onde dá para cortar", desc: "Compara com a sua média e mostra quanto sobraria.", icon: ICON.chart,
    q: "onde estou gastando demais?", thinking: "Comparando com os últimos 3 meses…", a: "Três categorias passaram da sua média. Cortando 30% dos restaurantes, sobram uns R$ 260 por mês.",
    rows: [["Restaurantes e delivery", "+41%", "neg"], ["Compras", "+34%", "neg"], ["Lazer", "+18%", "warn"]],
    kind: "Nota", prop: "Salvar esta análise em Notas", btn: ["Salvar", "Salvo"] },
];

const FAQ = [
  ["É grátis mesmo?", "Sim. O finan_rich não cobra nada: sem plano e sem anúncio. O consultor também pode rodar de graça no seu computador."],
  ["Preciso criar conta?", "Não. Dá para usar sem conta, direto no navegador. Entrando com Google, seus dados ficam também numa pasta do seu Drive e aparecem iguais no celular e no computador."],
  ["Onde ficam os meus dados?", "No seu aparelho e, se você entrar com Google, no seu Drive. Não existe um servidor nosso guardando as suas finanças."],
  ["Funciona sem internet?", "Funciona. O app do computador e o do Android abrem e salvam offline. A sincronização acontece quando a conexão volta."],
  ["Conecta com o meu banco?", "Se você quiser. Pelo Open Finance, os lançamentos chegam direto do banco. Também dá para importar o extrato ou lançar à mão."],
  ["A IA mexe no meu dinheiro?", "Não. Ela lê os seus números e sugere. Quando propõe um lançamento ou guardar na meta, aparece um cartão: nada muda sem você confirmar."],
];

const FACTS = [
  ["Open Finance", "lançamentos direto do banco", ICON.bank], ["Seus dados", "no seu aparelho e no seu Drive", ICON.shield],
  ["Offline", "abre e salva sem internet", ICON.wifi], ["Grátis", "sem plano, sem anúncio", ICON.gift],
];

const portrait = (m, cls = "") => html`<img class="lp-pix ${cls}" src="${IMG}${m.img}" alt="" loading="lazy" width="54" height="54">`;
const monthName = (offset) => finance.monthLabel(shiftMonth(today().slice(0, 7), offset));

// Patrimônio ilustrativo subindo nos 12 meses (o cartão com o pontinho pulsando).
function sparkline() {
  const nw = [-8620, -7800, -6100, -5400, -4900, -3600, -4400, -2900, -1300, 300, 1900, 3400];
  const pts = nw.map((v, i) => `${(i / 11) * 1000},${(72 - ((v + 9000) / 13000) * 70).toFixed(1)}`);
  return raw(`<svg class="lp-spark" viewBox="0 0 1000 76" preserveAspectRatio="none" aria-hidden="true"><defs><linearGradient id="lp-g" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#34c08f" stop-opacity=".45"/><stop offset="1" stop-color="#34c08f" stop-opacity="0"/></linearGradient></defs><path d="M0,76 L${pts.join(" L")} L1000,76 Z" fill="url(#lp-g)"/><path d="M${pts.join(" L")}" fill="none" stroke="#34c08f" stroke-width="2.5" vector-effect="non-scaling-stroke"/></svg>`);
}

function phone() {
  const bars = [[82, 76], [82, 70], [84, 73], [84, 62], [84, 89], [84, 65], [92, 70], [84, 65], [84, 69], [84, 61]];
  const tx = [["iFood", "Restaurantes · hoje", "−R$ 58,90", "#f0a845", ICON.food, false], ["Freela", "Renda extra · ontem", "+R$ 1.200,00", "#34c08f", ICON.up, true],
    ["Mercado", "Mercado · 2 dias", "−R$ 312,45", "#5a93f0", ICON.cart, false]];
  return html`<div class="lp-phone-wrap"><div class="lp-phone">
    <span class="lp-phone-glow" aria-hidden="true"></span>
    <div class="lp-phone-body"><div class="lp-screen">
      <span class="lp-notch" aria-hidden="true"></span>
      <div class="lp-status"><span>9:41</span><span>5G ▮▮▮</span></div>
      <div class="lp-screen-in">
        <div class="lp-hello"><span><small>Boa noite,</small><strong>seu mês</strong></span><img class="lp-pix" src="${IMG}brand-mark.png" alt="" width="32" height="32"></div>
        <div class="lp-mini-hero"><small>Sobrou em ${monthName(0).split(" de ")[0]}</small><strong>R$ 2.282,60</strong>
          <span class="lp-split"><i></i><i></i></span><span class="lp-mini-legend"><span>↑ R$ 8.420,00</span><span>↓ R$ 6.137,40</span></span></div>
        <div class="lp-mini-card"><span class="lp-mini-title"><b>Fluxo de caixa</b><span>${today().slice(0, 4)}</span></span>
          <span class="lp-mini-bars">${bars.map(([a, b], k) => html`<span><i style="--h: ${a}%; --d: ${0.6 + k * 0.05}s" class="${k === 9 ? "now" : ""}"></i><i style="--h: ${b}%; --d: ${0.62 + k * 0.05}s"></i></span>`)}</span></div>
        <div class="lp-mini-card lp-mini-list">${tx.map(([d, c, v, col, icon, pos]) => html`<span class="lp-mini-row">
          <span class="lp-tile" style="--c: ${col}">${svg(icon, 13)}</span><span class="lp-mini-text"><b>${d}</b><small>${c}</small></span><b class="${pos ? "pos" : ""}">${v}</b></span>`)}</div>
      </div>
      <div class="lp-phone-tabs">${[ICON.home, ICON.list, ICON.card, ICON.chart, ICON.chat].map((d, i) => html`<span class="${i ? "" : "on"}">${svg(d, 17)}</span>`)}</div>
      <span class="lp-shine-glass" aria-hidden="true"></span>
    </div></div>
  </div></div>`;
}

function floats() {
  return html`
    <div class="lp-float lp-f1"><div class="lp-glass lp-bob" style="--t: 7s">
      <span class="lp-glass-title">Últimos lançamentos</span>
      ${[["Delivery", "Restaurantes · ontem", "R$ 42,00", "#f0a845", ICON.food], ["Uber", "Transporte · 2 dias", "R$ 24,80", "#5a93f0", ICON.car]].map(([d, c, v, col, icon]) =>
        html`<span class="lp-mini-row"><span class="lp-tile" style="--c: ${col}">${svg(icon, 14)}</span><span class="lp-mini-text"><b>${d}</b><small>${c}</small></span><b>${v}</b></span>`)}
    </div></div>
    <div class="lp-float lp-f2 lp-wide"><div class="lp-glass lp-alert lp-bob" style="--t: 8s; --dl: 1s">
      ${portrait(TEAM[2])}<span><b>Luna · agora</b><span>Fatura do cartão vence em 3 dias</span></span>
    </div></div>
    <div class="lp-float lp-f3 lp-wide"><div class="lp-glass lp-bob" style="--t: 9s; --dl: .5s">
      <span class="lp-glass-title">Seu time</span>
      <span class="lp-team-mini">${TEAM.slice(0, 3).map((m) => html`<span style="--c: ${m.c}">${portrait(m)}${m.name}</span>`)}</span>
    </div></div>
    <div class="lp-float lp-f4 lp-wide"><div class="lp-glass lp-nw lp-bob" style="--t: 7.5s; --dl: 1.6s">
      <span class="lp-nw-head"><b>Patrimônio</b><b class="pos">+R$ 3.400</b></span>
      <span class="lp-nw-chart">${sparkline()}<span class="lp-pulse"></span></span>
      <span class="lp-nw-foot"><span>${monthName(-11).slice(0, 3)}.</span><span>${monthName(0).slice(0, 3)}.</span></span>
    </div></div>
    <div class="lp-float lp-f5 lp-wide"><span class="lp-free lp-bob" style="--t: 6s; --dl: .8s"><i></i>Livre das dívidas em ${monthName(9)}</span></div>`;
}

function demo(d, i) {
  return html`<div class="lp-demo" data-demo="${i}" ${i ? "hidden" : ""}>
    <span class="lp-q">${d.q}</span>
    <span class="lp-typing" hidden><img class="lp-pix" src="${IMG}team-pantera.png" alt="" width="28" height="28"><span class="lp-dots"><i></i><i></i><i></i></span>${d.thinking}</span>
    <div class="lp-answer">
      <img class="lp-pix" src="${IMG}team-pantera.png" alt="" width="28" height="28">
      <div class="lp-answer-body">
        <span>${d.a}</span>
        ${d.rows ? html`<div class="lp-rows">${d.rows.map(([a, b, tone]) => html`<div><span>${a}</span><b class="${tone}">${b}</b></div>`)}</div>` : ""}
        ${d.ring ? html`<div class="lp-ring-row"><span class="lp-ring" style="--p: 29"><svg width="56" height="56" viewBox="0 0 48 48" aria-hidden="true"><circle cx="24" cy="24" r="20"/><circle cx="24" cy="24" r="20" class="fill"/></svg><b>29%</b></span>
          <span><strong>Viagem para o Chile</strong><small data-ring-text>R$ 2.350 de R$ 8.000</small></span></div>` : ""}
        <div class="lp-confirm"><span><small>${d.kind}</small><span>${d.prop}</span></span><button type="button" data-confirm="${i}" data-done="${d.btn[1]}">${d.btn[0]}</button></div>
      </div>
    </div>
  </div>`;
}

function splits() {
  const bar = (w, c) => html`<i style="--w: ${w}%; --c: ${c}"></i>`;
  return [
    { k: "Painel", h: "Entender o mês num olhar.", p: "Entradas, saídas e quanto sobrou, com o salário dividido em necessidades, desejos e investimentos.", tone: "green",
      title: "Divisão do salário", meta: monthName(0).split(" de ")[0], rows: [["Necessidades", "87%", bar(87, "#5a93f0")], ["Desejos", "73%", bar(73, "#f0a845")], ["Investimentos", "36%", bar(36, "#34c08f")]],
      who: TEAM[0], note: "Sobraram R$ 2.282,60" },
    { k: "Dívidas", h: "Sair do vermelho com data marcada.", p: "Veja quanto de juros cada dívida come por mês, qual pagar primeiro e em que mês você fica livre.", tone: "red",
      title: "Saldo das dívidas", meta: "R$ 12.480,00", rows: [["Rotativo do cartão", "R$ 2.940", bar(43, "#f07163")], ["Empréstimo pessoal", "R$ 6.820", bar(100, "#f0a845")], ["Financiamento moto", "R$ 2.720", bar(40, "#8d9a95")]],
      who: TEAM[1], note: `Livre em ${monthName(9)}` },
    { k: "Metas", h: "Juntar até lá, sem adivinhar.", p: "Diga quanto quer e até quando. O app mostra quanto guardar por mês e avisa se o prazo apertar.", tone: "blue",
      title: "Suas metas", meta: "4 metas", rows: [["Reserva de emergência", "42%", bar(42, "#5a93f0")], ["Viagem para o Chile", "29%", bar(29, "#5a93f0")], ["Trocar o notebook", "100%", bar(100, "#34c08f")]],
      who: TEAM[3], note: "R$ 550/mês até a reserva" },
  ].map((s, i) => html`<article class="lp-split-row ${i % 2 ? "rev" : ""}" data-reveal>
    <div class="lp-split-copy"><span class="lp-tag">${s.k}</span><h3>${s.h}</h3><p>${s.p}</p>
      <a class="lp-outline" href="${LOGIN}">Começar grátis <span>→</span></a></div>
    <div class="lp-scene ${s.tone}">
      <img class="lp-pix lp-scene-pic" src="${IMG}${s.who.img}" alt="" loading="lazy">
      <div class="lp-scene-card"><span class="lp-scene-head"><b>${s.title}</b><span>${s.meta}</span></span>
        ${s.rows.map(([n, v, b]) => html`<span class="lp-scene-row"><span>${n}</span><b>${v}</b><span class="lp-track">${b}</span></span>`)}</div>
      <div class="lp-scene-note lp-bob" style="--t: 7s">${portrait(s.who)}<span><b>${s.who.name} · agora</b><span>${s.note}</span></span></div>
    </div>
  </article>`);
}

export function render() {
  let seed = 5;
  const rnd = () => (seed = (seed * 9301 + 49297) % 233280) / 233280;
  const motes = Array.from({ length: 20 }, () => html`<span class="lp-mote" style="--x: ${(rnd() * 100).toFixed(1)}%; --y: ${(20 + rnd() * 75).toFixed(1)}%; --s: ${rnd() > 0.7 ? 6 : 4}px; --t: ${(3 + rnd() * 4).toFixed(1)}s; --dl: ${(rnd() * 4).toFixed(1)}s" aria-hidden="true"></span>`);
  return html`<div class="landing" data-landing>
    <header class="lp-bar">
      <a class="lp-brand" href="#topo"><img class="lp-pix" src="${IMG}brand-mark.png" alt="" width="30" height="30"><span>finan<span>_rich</span></span></a>
      <nav aria-label="Seções">
        <a class="lp-link lp-section-link" href="#recursos">Recursos</a>
        <a class="lp-link lp-section-link" href="#time">O time</a>
        <a class="lp-link lp-section-link" href="#duvidas">Dúvidas</a>
        <a class="lp-link" href="${LOGIN}">Entrar</a>
        <a class="lp-cta-sm" href="${LOGIN}">Criar conta</a>
      </nav>
    </header>

    <section id="topo" class="lp-hero" data-hero>
      <div class="lp-glow" aria-hidden="true"></div><div class="lp-grid" aria-hidden="true"></div>
      ${motes}
      <div class="lp-hero-copy">
        <span class="lp-pill"><span class="lp-avatars">${["brand-mark.png", ...TEAM.filter((_, i) => i !== 3).map((m) => m.img)].map((src) => html`<img class="lp-pix" src="${IMG}${src}" alt="" width="22" height="22">`)}</span>Um time cuidando do seu dinheiro</span>
        <h1>Seu dinheiro, <span>no seu controle.</span></h1>
        <p>Tudo o que entra e sai, as dívidas com data para acabar e as metas andando. Com um consultor de IA que conversa sobre os seus números.</p>
        <div class="lp-ctas">
          <a class="lp-cta" href="${LOGIN}"><span class="lp-shine" aria-hidden="true"></span><span>Começar agora</span>${svg(ICON.arrow)}</a>
          <button type="button" class="lp-ghost" data-action="solo">Usar sem conta</button>
        </div>
      </div>
      <div class="lp-stage">${phone()}${floats()}<div class="lp-fade" aria-hidden="true"></div></div>
    </section>

    <section class="lp-facts" aria-label="Destaques"><div data-reveal>
      ${FACTS.map(([a, b, icon]) => html`<div class="lp-fact"><span class="lp-fact-ico">${svg(icon, 20)}</span><span><strong>${a}</strong><span>${b}</span></span></div>`)}
    </div></section>

    <section id="recursos" class="lp-light lp-jade"><div class="lp-wrap lp-two">
      <div class="lp-col" data-reveal>
        <div class="lp-head"><span class="lp-tag light">Consultor IA</span><h2>Você fala.<br>A Jade faz as contas.</h2>
          <p>Peça do seu jeito. Ela lança o gasto, monta o plano das dívidas e guarda na meta, sempre com um cartão para você confirmar.</p></div>
        <div class="lp-tabs" role="tablist" aria-label="Exemplos">
          ${DEMOS.map((d, i) => html`<button type="button" role="tab" class="lp-tab" data-tab="${i}" aria-selected="${i === 0}">
            <span class="lp-tab-ico">${svg(d.icon)}</span><span class="lp-tab-text"><strong>${d.title}</strong><span>${d.desc}</span></span>
            <span class="lp-tab-bar"><i></i></span></button>`)}
        </div>
      </div>
      <div class="lp-chat-frame" data-reveal><div class="lp-chat" aria-live="polite">
        <div class="lp-chat-head"><img class="lp-pix" src="${IMG}team-pantera.png" alt="" width="34" height="34"><span><strong>Jade · Consultora</strong><small><i></i>online</small></span></div>
        ${DEMOS.map(demo)}
        <span class="lp-composer">Pergunte sobre o seu dinheiro…<span>${svg(ICON.up, 14)}</span></span>
      </div></div>
    </div></section>

    <section id="time" class="lp-team">
      <div class="lp-wrap lp-team-head" data-reveal>
        <div class="lp-head"><span class="lp-tag">O time</span><h2>Um time de olho no seu dinheiro.</h2><p>Cada uma cuida de uma parte e avisa quando algo pede a sua atenção.</p></div>
        <div class="lp-arrows"><button type="button" class="lp-arrow" data-scroll="-1" aria-label="Anterior">${svg(ICON.left)}</button><button type="button" class="lp-arrow on" data-scroll="1" aria-label="Próxima">${svg(ICON.right)}</button></div>
      </div>
      <div class="lp-track-x" data-track>
        ${TEAM.map((m) => html`<article class="lp-agent" style="--c: ${m.c}">
          <div class="lp-agent-stage"><img class="lp-pix" src="${IMG}${m.img}" alt="${m.name}, ${m.role}" loading="lazy"><span class="lp-bubble">${m.say}</span></div>
          <div class="lp-agent-text"><span class="lp-agent-name"><strong>${m.name}</strong><span>${m.role}</span></span><span>${m.what}</span></div>
        </article>`)}
      </div>
    </section>

    <section class="lp-cards" aria-label="Cartões" data-tilt><div class="lp-wrap lp-two">
      <div class="lp-col" data-reveal>
        <span class="lp-tag">Contas e cartões</span><h2>A fatura, antes da surpresa.</h2>
        <p>Cadastre o cartão com o dia de fechamento e o app separa a fatura atual da próxima. Compra feita depois do fechamento já cai no mês certo.</p>
        <ul class="lp-points">${["Fatura atual e próxima, separadas sozinhas", "Aviso 3 dias antes do vencimento", "Patrimônio mês a mês: contas menos cartões e dívidas"].map((p) => html`<li><span>${svg(ICON.check, 14)}</span>${p}</li>`)}</ul>
      </div>
      <div class="lp-card-stage" data-reveal>
        <span class="lp-orbit" aria-hidden="true"></span>
        <div class="lp-credit">
          <div class="lp-credit-art"><img class="lp-pix" src="${IMG}brand-mark.png" alt=""><span class="lp-holo"></span><span class="lp-shine wide"></span></div>
          <div class="lp-credit-face"><span class="lp-credit-top"><span>finan<span>_rich</span></span><small>crédito</small></span><span class="lp-chip"></span>
            <span class="lp-credit-bottom"><span>Cartão de crédito</span><span>fecha 20 · vence 27</span></span></div>
        </div>
        <div class="lp-glass lp-invoice lp-bob" style="--t: 7s"><span><span>Fatura de ${monthName(0).split(" de ")[0]}</span><b>Fechada</b></span><strong>R$ 1.842,60</strong><small>vence em 3 dias</small></div>
        <div class="lp-glass lp-next lp-bob" style="--t: 8s; --dl: 1.2s"><small>Próxima fatura</small><strong>R$ 467,55</strong></div>
      </div>
    </div></section>

    <section class="lp-splits"><div class="lp-wrap">${splits()}</div></section>

    <section id="duvidas" class="lp-light lp-faq"><div class="lp-wrap lp-two lp-top">
      <div class="lp-head" data-reveal><h2>Dúvidas? A gente explica.</h2><p>O que mais perguntam sobre o finan_rich.</p></div>
      <div class="lp-faq-list">${FAQ.map(([q, a], i) => html`<details name="lp-faq" ${i ? "" : "open"}><summary>${q}<span aria-hidden="true">+</span></summary><p>${a}</p></details>`)}</div>
    </div></section>

    <section class="lp-final-wrap"><div class="lp-final">
      <img class="lp-pix lp-final-art" src="${IMG}brand-mark.png" alt="" aria-hidden="true"><span class="lp-final-fade" aria-hidden="true"></span><span class="lp-final-eyes" aria-hidden="true"></span>
      <div class="lp-final-copy" data-reveal><h2>Menos susto no fim do mês. Mais calma para decidir.</h2>
        <div class="lp-ctas left"><a class="lp-cta" href="${LOGIN}"><span>Começar agora</span></a><button type="button" class="lp-ghost" data-action="solo">Usar sem conta</button></div></div>
    </div></section>

    <footer class="lp-foot"><div class="lp-wrap">
      <span class="lp-foot-brand"><img class="lp-pix" src="${IMG}brand-mark.png" alt="" width="24" height="24">finan_rich</span>
      <span class="lp-foot-links"><a href="#recursos">Recursos</a><a href="#time">O time</a><a href="#duvidas">Dúvidas</a><a href="${LOGIN}">Entrar</a><a href="/privacidade.html">Privacidade</a></span>
    </div></footer>
  </div>`;
}

export function bind(root, ctx) {
  const page = root.querySelector("[data-landing]");
  document.body.classList.add("landing-mode");
  root.querySelectorAll('[data-action="solo"]').forEach((b) => b.addEventListener("click", () => startSolo(ctx, "/")));
  const still = () => page.isConnected; // saiu da landing: os observadores se desligam sozinhos
  const calm = matchMedia("(prefers-reduced-motion: reduce)").matches;

  // Celular em 3D: relaxa com a rolagem (--sp) e gira com o mouse (--mx, --my).
  const hero = root.querySelector("[data-hero]");
  if (!calm) {
    const onScroll = () => { if (!still()) return removeEventListener("scroll", onScroll); page.style.setProperty("--sp", String(Math.max(0, Math.min(1, 1 - scrollY / 520)))); };
    addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    const track = (el, x, y) => {
      el.addEventListener("pointermove", (e) => {
        const r = el.getBoundingClientRect();
        el.style.setProperty(x, (((e.clientX - r.left) / r.width - 0.5) * 2).toFixed(3));
        el.style.setProperty(y, (((e.clientY - r.top) / r.height - 0.5) * 2).toFixed(3));
      });
      el.addEventListener("pointerleave", () => { el.style.setProperty(x, 0); el.style.setProperty(y, 0); });
    };
    track(hero, "--mx", "--my");
    track(root.querySelector("[data-tilt]"), "--cx", "--cy");
  } else page.style.setProperty("--sp", "0");

  // Blocos que sobem ao entrar na tela.
  const reveals = root.querySelectorAll("[data-reveal]");
  if (!calm && "IntersectionObserver" in window) {
    const io = new IntersectionObserver((entries) => entries.forEach((e) => {
      if (e.isIntersecting) { e.target.classList.add("in"); io.unobserve(e.target); }
    }), { threshold: 0.15 });
    reveals.forEach((el) => { el.classList.add("lp-reveal"); io.observe(el); });
  }

  // Exemplos do consultor: troca a cada 8 s (12 s depois de um clique).
  const tabs = [...root.querySelectorAll("[data-tab]")];
  let current = 0, timer, typing;
  const pick = (i, auto) => {
    current = i;
    tabs.forEach((t, k) => t.setAttribute("aria-selected", String(k === i)));
    root.querySelectorAll("[data-demo]").forEach((d) => { d.hidden = Number(d.dataset.demo) !== i; });
    const shown = root.querySelector(`[data-demo="${i}"]`);
    shown.classList.add("typing");
    shown.querySelector(".lp-typing").hidden = false;
    clearTimeout(typing);
    typing = setTimeout(() => { shown.classList.remove("typing"); shown.querySelector(".lp-typing").hidden = true; }, calm ? 0 : 1100);
    if (!auto) restart(12000);
  };
  const restart = (ms) => {
    clearInterval(timer);
    if (calm) return;
    timer = setInterval(() => (still() ? pick((current + 1) % tabs.length, true) : clearInterval(timer)), ms);
  };
  tabs.forEach((t) => t.addEventListener("click", () => pick(Number(t.dataset.tab))));
  restart(8000);
  root.querySelectorAll("[data-confirm]").forEach((b) => b.addEventListener("click", () => {
    b.textContent = b.dataset.done;
    b.disabled = true;
    const ring = b.closest(".lp-answer-body").querySelector(".lp-ring");
    if (ring) {
      ring.style.setProperty("--p", 35);
      ring.querySelector("b").textContent = "35%";
      b.closest(".lp-answer-body").querySelector("[data-ring-text]").textContent = "R$ 2.850 de R$ 8.000 · R$ 572/mês";
    }
  }));

  // Carrossel do time.
  const strip = root.querySelector("[data-track]");
  root.querySelectorAll("[data-scroll]").forEach((b) => b.addEventListener("click", () =>
    strip.scrollBy({ left: Number(b.dataset.scroll) * 336, behavior: calm ? "auto" : "smooth" })));
}
