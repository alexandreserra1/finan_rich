// Landing: a página pública do site, antes do login (design Rich, docs/rich-mais-recente).
// Aparece só no site, para quem chega sem conta Google ligada; quem já usa vai para o
// Painel, e o app do computador e o do Android nunca a mostram. "Entrar" leva ao login
// (/?entrar=1, com o Google). No site não há uso sem conta. Os números daqui são exemplos.
// O conteúdo aparece logo de cara: o movimento ao rolar é só enfeite (sem buracos se a
// página carregar devagar ou com "menos movimento" ligado).
import * as device from "../device.js";
import { html, raw, tilt, toString } from "../ui.js";

export const needsData = false;

// show: site, sem conta Google e sem ter pedido o login (?entrar=1).
export const show = (ctx, signedIn) => device.isSite && !signedIn && !ctx.params.get("entrar");

const LOGIN = "/?entrar=1";
const IMG = "/static/img/";
const REPO = "https://github.com/alexandreserra1/finan_rich";
const ARROW = raw('<span class="lp-arrow" aria-hidden="true">↗</span>');
const MOON = "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z";

// O time: os mesmos de finance.Team (aqui com o texto do site e a frase de cada um).
const TEAM = [
  { id: "otavio", name: "Otávio", label: "ORGANIZAÇÃO & DÍVIDAS", say: "Vamos colocar as contas na mesa. E um plano no lugar da preocupação.", cta: "Organizar minhas contas" },
  { id: "lia", name: "Lia", label: "HÁBITOS & ORÇAMENTO", say: "Seu cafezinho não é o vilão. Vamos entender o que realmente pesa?", cta: "Entender meus gastos" },
  { id: "tito", name: "Tito", label: "METAS & RESERVA", say: "Um dia é uma ideia. Com um plano, pode virar uma data.", cta: "Planejar minha próxima meta" },
];

// Exemplos da seção "Na prática" (dados fictícios, marcados como exemplo na tela).
const DEMO_BARS = [["MAI", 78], ["JUN", 68], ["JUL", 92], ["AGO", 73], ["SET", 71], ["OUT", 57]];
const demoHead = html`<div class="lp-dash-head"><b>Oi, vamos olhar sua grana? ✳</b><span>EXEMPLO</span></div>`;
const DEMOS = {
  gastos: html`${demoHead}<p class="lp-dash-label">Gastos em outubro</p><p class="lp-balance">R$ 2.480<small>,00</small></p>
    <div class="lp-chart" role="img" aria-label="Gastos de maio a outubro, em exemplo: o último mês é o menor.">${DEMO_BARS.map(([m, h]) => html`<span class="lp-bar" style="height: ${h}%"><i>${m}</i></span>`)}</div>
    <p class="lp-insight">✳ <strong>Opa, sobrou mais.</strong> Seus gastos caíram R$ 370 em relação a setembro. Que tal dar um destino pra essa diferença?</p>`,
  metas: html`${demoHead}<div class="lp-goaldemo"><h3>☀ Próxima parada: férias</h3><p class="lp-faint">Você já tem R$ 3.600 dos R$ 5.000.</p>
    <span class="lp-progress"><i style="width: 72%"></i></span>
    <label for="lp-monthly">Quanto guardar por mês? <b data-amount>R$ 350</b></label>
    <input id="lp-monthly" type="range" min="50" max="1000" step="50" value="350" data-monthly>
    <p class="lp-dash-label">Sua meta fica pronta em</p><p class="lp-goalresult" aria-live="polite" data-months>4 meses ✨</p>
    <p class="lp-insight">Um plano que cabe no seu ritmo. Simulação sem rendimentos.</p></div>`,
  resumo: html`${demoHead}<p class="lp-dash-label">Saldo do mês · outubro</p><p class="lp-balance">R$ 1.520<small>,00</small></p>
    <div class="lp-receipt flat"><div><span>↙ Entrou na conta</span><b>R$ 4.000</b></div><div><span>↗ Gastos do mês</span><b>R$ 2.480</b></div><div><span>✳ Disponível no mês</span><b>R$ 1.520</b></div></div>
    <p class="lp-insight">Seu dinheiro conta uma história. Você gastou <strong>62% da renda</strong> neste exemplo. O restante pode ganhar um plano.</p>`,
};

// "Grátis de verdade": o que a Rich faz e o que é comum em apps de finanças.
const COMPARE = [
  ["Onde ficam seus dados", "No seu aparelho e no seu Google Drive", "Em geral, no servidor da empresa"],
  ["Senha do banco", "Nunca. O Open Finance vai pelo Meu Pluggy, com a sua conta", "Alguns pedem acesso à conta"],
  ["Preço", "Grátis, sem plano pago", "Muitos cobram plano para o completo"],
  ["Anúncios", "Nenhum", "Alguns mostram"],
  ["Código", "Aberto: qualquer pessoa confere", "Fechado"],
  ["Sem internet", "Abre e lança mesmo assim", "Normalmente precisa de conexão"],
];

const FAQ = [
  ["O que é a Rich?", "Um app de finanças pessoais com um time que conversa sobre os seus números: Otávio organiza o mês e as dívidas, Lia olha os hábitos e o orçamento, Tito cuida das metas. As contas são feitas pelo app; a IA explica e aconselha."],
  ["É grátis mesmo?", "Sim. A Rich não cobra nada: sem plano e sem anúncio. O código é aberto, e a IA pode rodar de graça no seu computador ou com uma chave grátis sua."],
  ["Onde ficam os meus dados?", "No seu aparelho e, se você entrar com Google, numa pasta escondida do seu próprio Drive, que só a Rich usa. Nenhum servidor da Rich guarda seus lançamentos."],
  ["Preciso criar conta?", "No site, você entra com a sua conta Google (sem senha nova). No app do celular e no programa do computador, não precisa de conta: o aparelho é seu."],
  ["Conecta com o meu banco?", "Sim, pelo Open Finance do Meu Pluggy, de graça e com a sua conta: os dados vão do banco direto para o seu aparelho. Você também pode importar o extrato (OFX, CSV ou a fatura em PDF)."],
  ["A IA mexe no meu dinheiro?", "Não. A Rich não faz pagamentos nem movimenta dinheiro. Quando a IA sugere um lançamento, aparece um cartão e só vale se você confirmar."],
];

export function render() {
  return html`<div class="landing" data-landing>
    <nav class="lp-nav" aria-label="Navegação principal">
      <a class="lp-brand" href="/" aria-label="Rich, início">rich<span>✳</span></a>
      <div class="lp-links"><a href="#recursos">Por que Rich?</a><a href="#demo">Na prática</a><a href="#time">Nosso time</a><a href="#duvidas">Dúvidas</a></div>
      <div class="lp-nav-actions">
        <button type="button" class="lp-theme" data-theme-toggle aria-label="Trocar o tema"><svg viewBox="0 0 24 24" aria-hidden="true"><path data-theme-icon d="${MOON}"/></svg></button>
        <a class="lp-pill" href="${LOGIN}">Entrar na Rich ${ARROW}</a>
      </div>
    </nav>

    <div class="lp-main">
      <section class="lp-hero">
        <div class="lp-herocopy">
          <p class="lp-eyebrow"><span class="lp-dot"></span>A vida é mais que pagar boletos</p>
          <h1>Seu dinheiro<br>merece uma<br><em>boa <span class="lp-underline">conversa.</span></em></h1>
          <p>Menos “pra onde foi meu salário?”.<br>Mais controle, planos e dinheiro pra viver.<br>Vem mudar essa relação com a Rich.</p>
          <a class="lp-pill blue lp-cta" href="${LOGIN}">Bora cuidar da minha grana ${ARROW}</a>
          <div class="lp-micro"><span class="lp-faces" aria-hidden="true">${TEAM.map((t) => html`<img src="${IMG}time/${t.id}-mini.webp" alt="" width="32" height="32">`)}</span>
            <span>Sem julgamento. Sem economês. Do seu jeito.</span></div>
        </div>
        <div class="lp-scene">
          <div class="lp-photo"><img src="${IMG}rich-hero.webp" width="1122" height="1402" alt="Personagem da campanha Rich sorrindo, sentada em uma poltrona amarela" fetchpriority="high"><span>MAIS VIDA FORA DA PLANILHA.</span></div>
          <span class="lp-sticker" aria-hidden="true">a vida<br><em>cabe aqui.</em> ✳</span>
          <div class="lp-convo"><img src="${IMG}time/lia-mini.webp" alt="" width="46" height="46"><div><small>LIA · HÁBITOS & ORÇAMENTO</small><p>Sua grana precisa de um plano.<br><b>Você, de um respiro.</b></p></div><span aria-hidden="true">✳</span></div>
          <div class="lp-goalfloat"><span>UM PLANO DE CADA VEZ ↗</span><b>Próxima parada: férias.</b><span class="lp-progress"><i></i></span><small>R$ 3.600 de R$ 5.000 · exemplo</small></div>
        </div>
      </section>

      <div class="lp-ribbon" aria-hidden="true"><div><span>Seu dinheiro. Suas regras.</span><span>✳</span><span>Menos perrengue, mais planos.</span><span>✳</span><span>A vida acontece fora da planilha.</span><span>✳</span></div></div>

      <section class="lp-band lp-paper" id="recursos"><div class="lp-section">
        <p class="lp-label">Paz financeira tem outro jeito</p>
        <div class="lp-top"><h2 class="lp-title">Você vive a vida.<br>A Rich ajuda com <em>as contas.</em></h2>
          <p>Uma ideia simples: transformar números confusos em próximos passos que fazem sentido pra você.</p></div>
        <div class="lp-features">
          <article class="lp-feature"><span class="lp-num">01 / ENTENDA SUA GRANA</span><h3>O sumiço do salário?<br>Caso resolvido.</h3>
            <p>Veja o que entra, o que sai e aqueles pequenos gastos que adoram passar despercebidos.</p>
            <div class="lp-art lp-receipt"><div><span>☕ Um cafezinho aqui</span><b>R$ 18</b></div><div><span>🍔 Um delivery ali</span><b>R$ 45</b></div><div><span>▶ Aquela assinatura</span><b>R$ 29</b></div><div><span>Agora fez sentido.</span><b>R$ 92</b></div></div></article>
          <article class="lp-feature"><span class="lp-num">02 / TIRE OS PLANOS DO PAPEL</span><h3>Seu “um dia”<br>pode ter uma data.</h3>
            <p>A viagem, a reserva, o próximo capítulo. Divida uma grande meta em pequenos passos.</p>
            <div class="lp-art lp-target"><div class="lp-coin">✳</div><small>um pouquinho hoje. um mundão amanhã.</small></div></article>
          <article class="lp-feature"><span class="lp-num">03 / RESPIRE MAIS TRANQUILO</span><h3>Mais clareza.<br>Menos susto.</h3>
            <p>Encontre espaço no orçamento sem precisar abrir mão de tudo que faz a vida ser boa.</p>
            <div class="lp-art lp-note"><b>✳ Ei, olha essa possibilidade.</b><span>Revisar duas assinaturas pode abrir espaço pra sua próxima meta. Vamos dar uma olhada?</span></div></article>
        </div>
      </div></section>

      <section class="lp-band lp-people" id="time"><div class="lp-section">
        <div class="lp-top"><div><p class="lp-label">Um time. Vários jeitos de ajudar.</p><h2 class="lp-title">Números com contexto.<br>Conversas com <em>personalidade.</em></h2></div>
          <p>Cada momento pede uma conversa. Escolha por onde começar a organizar sua vida financeira.</p></div>
        <div class="lp-people-grid">${TEAM.map((t) => html`<article class="lp-agent">
          <div class="lp-agent-photo"><img src="${IMG}time/${t.id}.webp" alt="Retrato ilustrativo de ${t.name}, personagem virtual da Rich" width="640" height="640" loading="lazy"><span>${t.label}</span><i aria-hidden="true">✳</i></div>
          <div class="lp-agent-copy"><h3>${t.name}<span aria-hidden="true">↗</span></h3><p>“${t.say}”</p><a href="${LOGIN}">${t.cta} ${ARROW}</a></div>
        </article>`)}</div>
        <p class="lp-artnote">Otávio, Lia e Tito são personagens virtuais com retratos gerados por IA. As contas são feitas pelo app; a IA explica e aconselha.</p>
      </div></section>

      <section class="lp-band lp-demo" id="demo"><div class="lp-section lp-demogrid">
        <div class="lp-democopy"><p class="lp-label">Chega de só imaginar</p><h2 class="lp-title">Sua grana,<br>vista com<br><em>outros olhos.</em></h2>
          <p>Toque, explore e faça planos. Um gostinho de como cuidar do dinheiro pode ser mais leve.</p>
          <div class="lp-tabs" role="tablist" aria-label="Demonstrações">
            <button type="button" class="lp-tab" role="tab" aria-selected="true" aria-controls="lp-panel" id="lp-tab-gastos" data-mode="gastos">Meus gastos</button>
            <button type="button" class="lp-tab" role="tab" aria-selected="false" aria-controls="lp-panel" id="lp-tab-metas" data-mode="metas" tabindex="-1">Minhas metas</button>
            <button type="button" class="lp-tab" role="tab" aria-selected="false" aria-controls="lp-panel" id="lp-tab-resumo" data-mode="resumo" tabindex="-1">Meu resumo</button>
          </div>
          <p class="lp-fine">Demonstração com dados fictícios. Nenhuma conta conectada.</p>
          <a class="lp-pill light" href="${LOGIN}">Explorar o painel completo ${ARROW}</a>
        </div>
        <div class="lp-dashboard" id="lp-panel" role="tabpanel" aria-labelledby="lp-tab-gastos" tabindex="0" data-panel>${DEMOS.gastos}</div>
      </div></section>

      <section class="lp-band lp-wallet"><div class="lp-section lp-walletgrid">
        <div class="lp-walletstage">
          <span class="lp-orbit" aria-hidden="true"></span>
          <div class="rich-card" data-tilt role="img" aria-label="Cartão ilustrativo Rich azul, com chip e numeração mascarada. Não é um cartão emitido.">
            <div class="card-orbits" aria-hidden="true"></div>
            <div class="rc-top"><span class="rc-logo">rich<span>✳</span></span><span class="rc-tag">SEU JEITO.</span></div>
            <div class="rc-chiprow">${raw('<svg class="rc-chip" viewBox="0 0 52 40" aria-hidden="true"><rect x="1" y="1" width="50" height="38" rx="8" fill="#dac994" stroke="#b4a16a"/><path d="M18 1v11H1m50 0H34V1M18 39V28H1m50 0H34v11M18 12h16v16H18zM1 20h17m16 0h17" fill="none" stroke="#998752"/></svg><svg class="rc-contactless" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 9q3 3 0 6m4-9q6 6 0 12m4-15q9 9 0 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>')}</div>
            <div class="rc-number"><span>••••</span><span>••••</span><span>••••</span><span>••••</span></div>
            <div class="rc-bottom"><span>MAIS VIDA. MAIS PLANOS.</span><span class="rc-mark" aria-hidden="true">✳</span></div>
          </div>
          <span class="lp-walletnote">bonito de ver.<br><em>fácil de entender.</em> ↗</span>
        </div>
        <div class="lp-walletcopy"><p class="lp-label">O cartão entra. A surpresa sai.</p><h2 class="lp-title">Seu cartão.<br>Uma visão <em>mais clara.</em></h2>
          <p>Compras registradas, fechamento e vencimento no mesmo lugar. Para você saber o que vem pela frente, antes da fatura chegar.</p>
          <div class="lp-walletfeat"><span>01 <b>Acompanhe o valor em aberto</b></span><span>02 <b>Veja a fatura atual e a próxima</b></span><span>03 <b>Receba o aviso antes do vencimento</b></span></div>
          <a class="lp-pill blue" href="${LOGIN}">Conhecer minha carteira ${ARROW}</a>
          <p class="lp-artnote">Visual ilustrativo. A Rich não emite cartões nem realiza pagamentos.</p></div>
      </div></section>

      <section class="lp-band lp-paper" id="gratis"><div class="lp-section">
        <div class="lp-top"><div><p class="lp-label">Sem letra miúda</p><h2 class="lp-title">Grátis de verdade.<br>E o motivo é <em>simples.</em></h2></div>
          <p>A Rich roda no seu aparelho e guarda os dados no seu Drive. Sem servidor guardando seus dados, não tem conta para pagar, nem dado para vender.</p></div>
        <div class="lp-compare-wrap"><table class="lp-compare">
          <thead><tr><th scope="col"><span class="sr-only">O quê</span></th><th scope="col">Rich</th><th scope="col">Comum em apps de finanças</th></tr></thead>
          <tbody>${COMPARE.map(([what, rich, other]) => html`<tr><th scope="row">${what}</th><td><span class="lp-yes" aria-hidden="true">✳</span>${rich}</td><td>${other}</td></tr>`)}</tbody>
        </table></div>
      </div></section>

      <section class="lp-band lp-secure" id="seguranca"><div class="lp-section">
        <div class="lp-top"><div><p class="lp-label">Seus dados, do seu lado</p><h2 class="lp-title">Segurança que dá<br>para <em>conferir.</em></h2></div>
          <p>Nada de “confie em nós”: o que protege seus dados está no código, aberto para qualquer pessoa ver.</p></div>
        <div class="lp-secgrid">
          <article><b>Código aberto</b><p>Todo o código da Rich está no GitHub. Qualquer pessoa pode ler, conferir e apontar um erro.</p><a href="${REPO}" rel="noopener">Ver o código ${ARROW}</a></article>
          <article><b>Dados no seu aparelho e no seu Drive</b><p>Os lançamentos ficam no celular, no computador ou numa pasta escondida do seu Google Drive. Nenhum servidor da Rich guarda.</p></article>
          <article><b>Banco sem passar a senha pela Rich</b><p>O Open Finance vai pelo Meu Pluggy, regulado pelo Banco Central, com a sua conta. Os dados vão do banco direto para você.</p></article>
          <article><b>Bloqueio e sessão curta</b><p>No celular, a Rich pede a digital ou o rosto ao abrir. No site, entra com Google e sai sozinha depois de 30 minutos parada.</p></article>
        </div>
      </div></section>

      <section class="lp-band lp-how" id="como"><div class="lp-section lp-howgrid">
        <div class="lp-quoteart" aria-hidden="true"><span class="lp-bigstar">✳</span><div class="lp-sticky">Dinheiro é<br>um assunto.<br>Não um tabu.</div></div>
        <div><p class="lp-label">Um passo de cada vez</p><h2 class="lp-title">Descomplicar<br>é o <em>primeiro passo.</em></h2>
          <div class="lp-steps">
            <div class="lp-step"><span>01</span><div><h3>Entenda o seu momento</h3><p>Comece pelo que você já sabe: quanto recebe e para onde sua grana está indo.</p></div></div>
            <div class="lp-step"><span>02</span><div><h3>Escolha o que importa pra você</h3><p>Organizar o mês ou realizar um sonho? O plano começa com a sua prioridade.</p></div></div>
            <div class="lp-step"><span>03</span><div><h3>Faça caber na vida real</h3><p>Pequenos ajustes, no seu ritmo. Sem receita mágica e sem precisar virar outra pessoa.</p></div></div>
          </div></div>
      </div></section>

      <section class="lp-band lp-download" id="baixe"><div class="lp-section">
        <div class="lp-top"><div><p class="lp-label">Onde você estiver</p><h2 class="lp-title">No navegador,<br>no celular e <em>no computador.</em></h2></div>
          <p>As mesmas contas em todo lugar. O app e o programa funcionam até sem internet.</p></div>
        <div class="lp-dlgrid">
          <a class="lp-dl" href="${LOGIN}"><b>Navegador</b><span>Entre com Google e use agora, sem instalar nada.</span>${ARROW}</a>
          <a class="lp-dl" href="${REPO}/releases/latest" rel="noopener"><b>Android</b><span>Baixe o app (arquivo .apk) na página de versões do GitHub.</span>${ARROW}</a>
          <a class="lp-dl" href="${REPO}#instalar-no-computador" rel="noopener"><b>Mac, Windows e Linux</b><span>Instale o programa com um comando no terminal. A IA pode rodar no próprio computador.</span>${ARROW}</a>
        </div>
      </div></section>

      <section class="lp-band lp-paper lp-faq" id="duvidas"><div class="lp-section lp-wrap lp-faqgrid">
        <div><p class="lp-label">Vamos abrir o jogo</p><h2 class="lp-title">Perguntar<br><em>não custa.</em></h2></div>
        <div>${FAQ.map(([q, a], i) => html`<details ${i === 0 ? "open" : ""}><summary>${q}</summary><p>${a}</p></details>`)}</div>
      </div></section>

      <section class="lp-closing"><p class="lp-label">Um novo papo com o seu dinheiro</p><h2>Menos aperto.<br>Mais <em>possibilidades.</em></h2>
        <p>O próximo capítulo da sua grana começa com você.</p><a class="lp-pill blue" href="${LOGIN}">Quero experimentar ${ARROW}</a></section>
    </div>

    <footer class="lp-foot">
      <a class="lp-brand" href="/" aria-label="Rich, início">rich<span>✳</span></a>
      <nav class="lp-foot-links" aria-label="Rodapé"><a href="#recursos">Por que Rich?</a><a href="#time">Nosso time</a><a href="#seguranca">Segurança</a><a href="#duvidas">Dúvidas</a>
        <a href="/privacidade.html">Privacidade</a><a href="${REPO}" rel="noopener">Código no GitHub</a><a href="${REPO}/issues" rel="noopener">Fale com a gente</a><a href="${LOGIN}">Entrar</a></nav>
      <p>© 2026 Rich · Feito no Brasil · código aberto (MIT).<br>A Rich não é banco, não faz pagamentos e não movimenta dinheiro.</p>
    </footer>
  </div>`;
}

export function bind(root) {
  document.body.classList.add("landing-mode");

  // "Na prática": abas com setas do teclado (padrão de abas acessíveis).
  const panel = root.querySelector("[data-panel]");
  const tabs = [...root.querySelectorAll(".lp-tab")];
  const show = (mode) => {
    tabs.forEach((t) => {
      const on = t.dataset.mode === mode;
      t.setAttribute("aria-selected", String(on));
      t.tabIndex = on ? 0 : -1;
    });
    panel.setAttribute("aria-labelledby", `lp-tab-${mode}`);
    panel.innerHTML = toString(DEMOS[mode]);
    const range = panel.querySelector("[data-monthly]");
    range?.addEventListener("input", () => {
      const v = Number(range.value);
      panel.querySelector("[data-amount]").textContent = v.toLocaleString("pt-BR", { style: "currency", currency: "BRL", maximumFractionDigits: 0 });
      const n = Math.ceil(1400 / v); // faltam R$ 1.400 (de R$ 3.600 para R$ 5.000), sem rendimentos
      panel.querySelector("[data-months]").textContent = `${n} ${n === 1 ? "mês" : "meses"} ✨`;
    });
  };
  tabs.forEach((t, i) => {
    t.addEventListener("click", () => show(t.dataset.mode));
    t.addEventListener("keydown", (e) => {
      const n = { ArrowRight: (i + 1) % tabs.length, ArrowLeft: (i + tabs.length - 1) % tabs.length, Home: 0, End: tabs.length - 1 }[e.key];
      if (n === undefined) return;
      e.preventDefault();
      tabs[n].focus();
      show(tabs[n].dataset.mode);
    });
  });

  tilt(root.querySelector("[data-tilt]")); // o cartão acompanha o mouse
}
