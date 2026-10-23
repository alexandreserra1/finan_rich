// Gráficos com Chart.js (static/vendor/chartjs, MIT). A tela descreve o gráfico
// com chart() (rótulos, séries em centavos, cores pelos nomes dos tokens do
// app.css) e o main.js chama mount() depois de desenhar. As cores são lidas do
// CSS na hora, então seguem o tema; trocar o tema redesenha os gráficos.
// A dica (.chart-tip) é HTML: aparece ao passar o dedo ou o mouse e é lida
// pelo leitor de tela. Cada gráfico continua com a sua tabela "Ver em tabela".
import * as finance from "./finance.js";
import { html } from "./ui.js";

const pending = new Map(); // gráficos desenhados na tela e ainda não montados
let live = []; // { canvas, spec, chart }
const animated = new Set(); // anima só na primeira vez que o gráfico aparece
let seq = 0;

let lib;
function load() {
  lib ||= new Promise((resolve, reject) => {
    if (window.Chart) return resolve(window.Chart);
    const s = Object.assign(document.createElement("script"), { src: "/static/vendor/chartjs/chart.umd.min.js" });
    s.onload = () => resolve(window.Chart);
    s.onerror = () => { lib = null; reject(new Error("não foi possível carregar os gráficos")); };
    document.head.append(s);
  });
  return lib;
}

const token = (name) => getComputedStyle(document.documentElement).getPropertyValue(`--${name}`).trim();
// alpha("#0f7a5c", 0.3) => "#0f7a5c4d" (os tokens de cor são #rrggbb).
const alpha = (hex, a) => hex + Math.round(a * 255).toString(16).padStart(2, "0");
const compact = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL", notation: "compact", maximumFractionDigits: 1 });
const axisMoney = (cents) => compact.format(cents / 100);

// chart devolve o lugar do gráfico na tela. spec:
//   key     nome estável (não anima de novo ao redesenhar a mesma tela)
//   type    "line", "bar" ou "doughnut"
//   labels  rótulos do eixo (ou das fatias)
//   series  [{ name, data: [centavos|null], color: "accent", dash, dots, fill, line }]
//           no doughnut, uma série com colors: ["necessidades", ...]
//   tip(i)  título da dica para o ponto i (padrão: o rótulo)
//   horizontal, stacked, height (px), center: { value, text } no doughnut
export function chart(spec, label) {
  const id = `ch${++seq}`;
  pending.set(id, spec);
  const legend = spec.type === "doughnut" || spec.legend === false ? "" : html`<p class="legend">${spec.series.map((s) =>
    html`<span><span class="key ${s.dash ? "dash" : s.dots ? "dots" : ""}" style="--c: var(--${s.color})"></span>${s.name}</span>`)}</p>`;
  return html`<div class="chart-box chart-${spec.type}">
    ${legend}
    <div class="chart" data-chart="${id}" style="height: ${spec.height || 240}px">
      <canvas role="img" aria-label="${label}"></canvas>
      ${spec.center ? html`<div class="chart-center"><strong>${spec.center.value}</strong><span>${spec.center.text}</span></div>` : ""}
      <p class="chart-tip" aria-live="polite" hidden></p>
    </div>
  </div>`;
}

// gradient pinta a área embaixo da linha, forte em cima e sumindo embaixo.
const gradient = (color) => ({ chart }) => {
  const a = chart.chartArea;
  if (!a) return alpha(color, 0.15);
  const g = chart.ctx.createLinearGradient(0, a.top, 0, a.bottom);
  g.addColorStop(0, alpha(color, 0.35));
  g.addColorStop(1, alpha(color, 0));
  return g;
};

function datasets(spec) {
  if (spec.type === "doughnut") {
    const s = spec.series[0];
    return [{ label: s.name, data: s.data, backgroundColor: s.colors.map(token), borderColor: token("surface"), borderWidth: 3, hoverOffset: 8, borderRadius: 6 }];
  }
  return spec.series.map((s) => {
    const c = token(s.color);
    const line = spec.type === "line" || s.line;
    return line
      ? { type: "line", label: s.name, data: s.data, borderColor: c, backgroundColor: s.fill ? gradient(c) : c, fill: !!s.fill,
          borderWidth: s.dash || s.dots ? 2 : 3, borderDash: s.dash ? [6, 5] : s.dots ? [2, 4] : [], cubicInterpolationMode: "monotone",
          pointRadius: 0, pointHoverRadius: 5, pointBackgroundColor: c, pointBorderColor: token("surface"), pointBorderWidth: 2, spanGaps: false, order: 0 }
      : { type: "bar", label: s.name, data: s.data, backgroundColor: c, hoverBackgroundColor: alpha(c, 0.8), borderRadius: 6, borderSkipped: false,
          maxBarThickness: 22, categoryPercentage: 0.7, barPercentage: 0.9, order: 1 };
  });
}

// tooltip escreve a dica no .chart-tip e a põe perto do ponto.
function tooltip(spec, tip) {
  return ({ chart, tooltip: t }) => {
    if (!t.opacity || !t.dataPoints?.length) { tip.hidden = true; return; }
    const i = t.dataPoints[0].dataIndex;
    const rows = spec.type === "doughnut"
      ? [[spec.series[0].colors[i], spec.labels[i], spec.series[0].data[i]]]
      : spec.series.filter((s) => s.data[i] != null).map((s) => [s.color, s.name, s.data[i]]);
    const el = (tag, text, cls) => Object.assign(document.createElement(tag), { textContent: text, className: cls || "" });
    tip.replaceChildren(
      ...(spec.type === "doughnut" ? [] : [el("strong", spec.tip ? spec.tip(i) : spec.labels[i])]),
      ...rows.map(([color, name, cents]) => {
        const row = el("span", "", "tip-row");
        const key = el("i", "", "key");
        key.style.setProperty("--c", `var(--${color})`);
        row.append(key, `${name} `, el("b", finance.money(cents)));
        return row;
      }));
    tip.hidden = false;
    const box = chart.canvas.parentNode.getBoundingClientRect();
    const x = Math.min(Math.max(t.caretX, tip.offsetWidth / 2), box.width - tip.offsetWidth / 2);
    tip.style.left = `${x}px`;
    tip.style.top = `${Math.max(0, t.caretY - tip.offsetHeight - 12)}px`;
  };
}

function config(spec, tip, animate) {
  const muted = token("muted"), grid = token("border");
  const money = { color: muted, callback: (v) => axisMoney(v), maxTicksLimit: 5, maxRotation: 0 };
  // Nome longo de categoria é cortado no eixo; a dica mostra o nome inteiro.
  const short = (l) => (l.length > 18 ? `${l.slice(0, 17)}…` : l);
  const cat = { color: muted, autoSkip: true, maxRotation: 0, maxTicksLimit: spec.horizontal ? undefined : 7,
    callback(v) { return short(String(this.getLabelForValue(v))); } };
  const valueAxis = { beginAtZero: true, stacked: !!spec.stacked, grid: { color: grid, drawTicks: false }, border: { display: false }, ticks: money };
  const catAxis = { stacked: !!spec.stacked, grid: { display: false }, border: { color: grid }, ticks: cat };
  return {
    type: spec.type,
    data: { labels: spec.labels, datasets: datasets(spec) },
    options: {
      responsive: true, maintainAspectRatio: false,
      animation: animate && !matchMedia("(prefers-reduced-motion: reduce)").matches ? { duration: 700, easing: "easeOutQuart" } : false,
      interaction: spec.type === "doughnut" ? { mode: "nearest", intersect: true } : { mode: "index", intersect: false },
      indexAxis: spec.horizontal ? "y" : "x",
      cutout: "72%",
      layout: { padding: { top: 6 } },
      plugins: { legend: { display: false }, tooltip: { enabled: false, external: tooltip(spec, tip) } },
      scales: spec.type === "doughnut" ? {} : spec.horizontal ? { x: valueAxis, y: catAxis } : { x: catAxis, y: valueAxis },
    },
  };
}

function draw(Chart, item) {
  Chart.defaults.font.family = getComputedStyle(document.body).fontFamily;
  const key = location.pathname + (item.spec.key || "");
  const tip = item.canvas.parentNode.querySelector(".chart-tip");
  item.chart = new Chart(item.canvas, config(item.spec, tip, !animated.has(key)));
  animated.add(key);
  item.canvas.addEventListener("pointerleave", () => { tip.hidden = true; });
}

// mount liga os gráficos que a tela acabou de desenhar (e solta os antigos).
export async function mount(root) {
  live.forEach((c) => c.chart?.destroy());
  live = [];
  const boxes = [...root.querySelectorAll("[data-chart]")];
  const items = boxes.filter((b) => pending.has(b.dataset.chart))
    .map((b) => ({ canvas: b.querySelector("canvas"), spec: pending.get(b.dataset.chart) }));
  pending.clear();
  if (!items.length) return;
  live = items;
  let Chart;
  try {
    Chart = await load();
  } catch {
    boxes.forEach((b) => b.closest(".chart-box").classList.add("failed"));
    return;
  }
  // A tela pode ter sido redesenhada enquanto a biblioteca carregava.
  items.filter((it) => live === items && it.canvas.isConnected).forEach((it) => draw(Chart, it));
}

// Trocar o tema redesenha os gráficos com as cores novas, sem animar.
new MutationObserver(() => {
  if (!window.Chart) return;
  live.forEach((it) => {
    if (!it.chart || !it.canvas.isConnected) return;
    it.chart.destroy();
    draw(window.Chart, it);
  });
}).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
