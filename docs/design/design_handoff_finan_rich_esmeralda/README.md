# Handoff: finan_rich — redesign "Esmeralda"

## Overview
Full visual redesign of the finan_rich personal-finance app (Go backend, vanilla-JS device app in `internal/web/static/js`). It covers 9 screens (Boas-vindas, Painel, Lançamentos, Contas e cartões, Dívidas, Metas, Análise, Consultor IA, Ajustes), a mobile adaptation, dark mode (primary) + light mode, and a new brand mark (black panther with emerald eyes).

## About the design files
The `.dc.html` files in this folder are **design references built in HTML**: prototypes showing the intended look and behavior. They are **not production code to copy**. Recreate them inside the existing finan_rich stack:
- pages stay as `static/js/pages/*.js` modules that return `html\`…\`` strings (`ui.js`), rendered by `main.js`;
- styling goes in `static/app.css` (CSS variables + classes), **not** inline styles;
- charts keep using `static/js/charts.js` (Chart.js). Retune its dataset options to the specs below instead of hand-drawing SVG;
- all numbers come from the Go views (`finance.dashboard`, `finance.debts`, `finance.insights`, `finance.diagnose`, `finance.searchTransactions`) via WASM. **The sample values in the prototypes are fake and must never be hard-coded.**
- No new dependencies are needed (no font CDN in the offline app: self-host Geist under `static/fonts/` or fall back to `ui-sans-serif, system-ui`).

Open any file directly in a browser; `support.js` is the prototype runtime and `finan-proto.js` holds the shared tokens, nav and debt-plan math used by the mocks.

## Fidelity
**High fidelity.** Colors, type, spacing, radii and states are final. Copy is final Brazilian Portuguese and follows the current app's voice (short and concrete, "você").

---

## Design tokens
Put these in `app.css` as replacements for the current `:root` / `:root[data-theme="dark"]` blocks. The token **roles** match the existing ones (`--accent`, `--pos`, `--neg`, `--warn`, `--necessidades`…), so `charts.js` (`token(name)`) keeps working.

| Token | Dark (default) | Light |
|---|---|---|
| --bg | #0a0d0c | #f4f6f5 |
| --side (sidebar) | #0c100f | #fbfcfb |
| --surface (cards) | #101614 | #ffffff |
| --surface-2 | #141c1a | #f3f6f4 |
| --hover | #141c1a | #eef2f0 |
| --inset (inputs, chart tip strip) | #0c1110 | #f3f6f4 |
| --border | #1a2421 | #e2e7e4 |
| --border-2 (inputs, strong) | #24302c | #d5ddd9 |
| --grid (chart gridlines) | #18211e | #edf1ef |
| --track (progress bg) | #1a2421 | #e9eeeb |
| --text | #e8eeeb | #15201b |
| --text-2 | #b9c4bf | #3d4843 |
| --muted | #8d9a95 | #5f6b66 |
| --faint | #6c7a74 | #77837d |
| --accent | #34c08f | #0f7a5c |
| --accent-hi (hover) | #5fd6aa | #0b5e47 |
| --accent-ink (text on accent) | #05140f | #ffffff |
| --accent-soft | rgba(52,192,143,.13) | #e3f2eb |
| --accent-line | #1d3a30 | #bfe0d2 |
| --pos | #7fe0b8 | #0f7a5c |
| --neg | #f07163 | #c2412f |
| --neg-soft | rgba(240,113,99,.13) | #fbecea |
| --warn | #f0a845 | #a8650c |
| --warn-soft | rgba(240,168,69,.13) | #fdf3e2 |
| --necessidades | #5a93f0 | #2f6fd6 |
| --desejos | #f0a845 | #bc7718 |
| --investimentos | #34c08f | #0f9b6e |
| --renda | #a184f0 | #7a5ad1 |
| --hero (balance card bg) | linear-gradient(160deg,#0f2a22 0%,#0e1714 60%) | linear-gradient(160deg,#0f2e25 0%,#123a2e 100%) |
| --hero-border | #1d3a30 | #0f2e25 |
| --hero-muted | #a9cbbf | #b9d6cb |
| --shadow | 0 1px 0 rgba(255,255,255,.02) inset | 0 1px 2px rgba(16,24,20,.04), 0 6px 20px -12px rgba(16,24,20,.12) |
| --shadow-pop (modals, toasts) | 0 18px 40px -16px rgba(0,0,0,.7) | 0 18px 40px -16px rgba(16,24,20,.28) |
| --scrim | rgba(3,6,5,.62) | rgba(16,24,20,.38) |

The hero card is always dark in both themes (white text, `#7fe0b8` positives, `#f07163` negatives).
Remove the current body radial "glow" gradients and the gradient-clipped `h1`; they don't belong to this direction.

**Type:** Geist 400/500/600/700 (fallback `ui-sans-serif, system-ui`). Base 14px/1.5. All money uses `font-variant-numeric: tabular-nums`.
- Page title h1: 28px / 600 / -0.025em, with a 13px `--faint` kicker above it
- Hero balance: 46px / 600 / -0.035em / line-height 1
- KPI value: 28–30px / 600 / -0.03em
- Card title h2: 15px / 600
- Body: 13–14px; meta: 11–12px `--faint`
- Table header: 11px uppercase, letter-spacing .05em, `--faint`

**Spacing:** 4 · 6 · 8 · 10 · 12 · 14 · 16 · 18 · 20 · 22 · 24 · 28 · 36. Card padding 22px 24px. Grid gap 14px. Main padding 28px 36px 56px, max-width 1240px.

**Radii:** cards 16px · inputs/buttons 10px · chips/pills 99px · modal 18px · icon tiles 10px · bars 3px (top) / 99px (progress).

**Icons:** keep the current stroke set in `_layout.html` (`{{template "icon"}}`), 18px in nav, stroke 1.8, round caps. New glyphs (category icons, trash, search, sun/moon) are Lucide paths. Copy them from `finan-proto.js` / the page files.

## Layout shell (all desktop screens)
- `body.has-nav` grid: `240px | minmax(0,1fr)`. The top bar (`header.top`) is **removed** on desktop; the brand + sync pill move into the sidebar.
- Sidebar (sticky, 100vh, padding 20px 14px, border-right 1px --border, bg --side): brand (32px mark, radius 8px, `box-shadow: 0 0 0 1px var(--border-2)`, then "finan" + `<span>_rich</span>` in --accent, 16px/600). After it come the 7 links, then pinned to the bottom: Ajustes, a theme toggle ("Tema claro"/"Tema escuro" with sun/moon icon), and the sync pill (7px dot + "Sincronizado · HH:MM", 12px, 1px border, radius 10px).
- Nav link: 9px 10px, radius 9px, gap 12px, 500 weight, `--muted`. Hover: bg --hover, color --text. Current page (`aria-current`): bg --accent-soft, color --text, `inset 0 0 0 1px var(--accent-line)`. This replaces the 3px left bar.
- Keep **all 8 routes**. Contas and Metas have their own redesigned screens (sections 8 and 9).
- 721–1023px: the sidebar collapses to the current horizontal tab row (same tokens).
- ≤720px: see Mobile below.

## Screens

### 1. Boas-vindas (`start.js`, first visit on the site, plus `login.html`)
- Two columns `1fr | 1.1fr`, full height. Left (padding 40px 56px): brand row + theme icon button; a block max 440px with a pill ("Grátis e de código aberto"), the h1 "Suas finanças, no seu controle." (44px/600/-0.04em, balance), and a 16px lead.
- Buttons, 50px tall, radius 12px, full width: **Entrar com Google** (accent; white G disc) and **Usar sem conta** (1px --border-2 outline). Two notes with shield/monitor icons explain where the data lives (copy from `start.js`).
- Right column: pure #000 field with a 32px emerald grid masked radially and the panther art centered (70% width, max 520px, `image-rendering: pixelated`). It drifts ±8px over 8s, and an emerald glow over the eyes pulses 4s. At the bottom, a 3-column glass strip (Painel / Dívidas / Consultor one-liners).
- States: **loading** (button shows spinner + "Abrindo o Google…", disabled) and **error** (alert box with --neg-soft background: "Não deu para entrar com o Google." + reason). Show the Google button only when `google.available()` is true, exactly as today. Don't fake OAuth.
- Below 900px the art column hides and the left column centers.

### 2. Painel (`dashboard.js`)
Order, top to bottom:
1. Page head: kicker "Visão do mês", h1 "Painel"; on the right the month nav (segmented box: ‹ label ›, 32px buttons) and the primary "Novo lançamento" button (links to `/lancamentos`).
2. **Avisos** (`finance.alerts`): row of pill links (8px 14px, 1px border, radius 99px, 7px colored dot: danger → --neg, warn → --warn, ok → --accent). Title is 500 --text, detail --muted.
3. KPI row, grid `auto-fit minmax(260px,1fr)`: the **hero** spans 2 columns (label "Saldo do mês", a delta chip vs. the previous month, the balance, "Guardado (saldo + investido): X% da renda", and a 6px split bar of out vs. remaining). It's followed by **Entradas** and **Saídas** cards, each with a 26px tinted icon tile (↑ accent / ↓ neg).
4. Row: **Fluxo de caixa · {ano}** (spans 2) + **Divisão do salário**.
   - Fluxo: grouped bars per month (Entradas --accent, Saídas --neg, max 11px wide, radius 3px top, gap 3px). A y-axis is shown with 4 dashed gridlines. Above the plot sits a **live summary strip** (bg --inset, radius 10px) that shows the hovered month: label, Entradas, Saídas, Saldo (colored ±). The current month is selected by default. Bars of non-hovered months drop to 55% opacity. Months without data stay empty. Use Chart.js `bar` with `external` tooltip writing into that strip.
   - Divisão: a 118px doughnut (cutout ≈76%, 0.8% surface gaps; center "R$ X mil · gastos no mês"), the rule line ("Regra 50/30/20 sobre R$ renda"), then the 3 buckets: dot + name, "N% usado", a 6px bar, and "R$ gasto de R$ planejado · resta R$". Over 100% → bar --neg + "estourou R$".
5. Row of 3: **Para onde o dinheiro foi** (hbars from `v.categories`, 4px track, bucket colors, value + share %) · **Últimos lançamentos** (6 most recent: 34px icon tile tinted by bucket, description, "categoria · data", signed amount; income in --pos) · stacked **Próximos vencimentos** (debts by `dueDay` + recurring bills: 42px date tile with day + month, red tile when ≤3 days) and a **Dívidas** summary link card (total, juros/mês in --neg, "N dívidas · seguindo o plano, livre em {mês}" from `finance.debts(...).plan`).
- Keep the existing **Metas**, **Orçamentos do mês** and **Resumo do ano** table sections. They use the same card style and sit below row 5.
- **First use** (`welcome.show(ctx)`): replace with the two-column empty state. Left is a hero card ("Seu painel começa pelo salário.", button "Começar agora" → `?passo=1`). Right is a list of the 4 steps (36px numbered tiles). The existing `welcome.js` stepper keeps working and only gets restyled.
- New data needed: "últimos lançamentos" and "próximos vencimentos" can be derived client-side from `ledger.transactions`, `ledger.debts`, `ledger.recurring`. "Saldo vs mês anterior" = `dashboard(month-1).balanceCents`.

### 3. Lançamentos (`transactions.js`)
- Head: kicker "Receitas e despesas", month nav, "Importar extrato" (secondary, download icon), "Novo lançamento" (primary).
- Summary strip: a single bordered card split into 4 cells (Lançamentos no mês/Encontrados, Entradas --pos, Saídas, Saldo ±). It reflects `found` from `searchTransactions`.
- List card. The toolbar has: search field with icon (40px, inset bg; accent-insensitive like today), a **Todos / Entradas / Saídas** segmented control, a category select, the "Todos os meses" checkbox, and "Limpar" when filtering. Filters stay in the URL like today.
- Rows are **grouped by day** ("Hoje", "Ontem", "quinta, 22 de outubro"), with the day's net on the right. Row grid: `40px | 1.6fr | 1fr | .8fr | 140px | 72px`. That is the icon tile, description (+ "↻ mensal" / "3/6" chips), category pill (dot + name), account, signed amount (right aligned), and a trash icon button (hover --neg-soft). Clicking a row opens edit.
- **Modal** (560px, radius 18px, scrim + 2px blur, pop-in 220ms) for create/edit:
  - Saída/Entrada segmented control; this filters categories by `renda` bucket exactly like `filterCategories`.
  - Large amount field (60px, 28px/600 numerals, "R$" prefix).
  - Descrição, then a 2-column block: Categoria, Data, Conta ou cartão (only if accounts exist), Parcelas.
  - "Repetir todo mês" checkbox card.
  - Footer: Apagar (edit only, --neg text), Cancelar, Adicionar/Salvar alterações.
  - Validation (inline, red border + 12px message): amount required and > 0 ("Valor inválido. Use o formato 1.234,56."), description required, "Escolha repetir todo mês ou parcelar, não os dois." Use `finance.parseMoney` and `finance.validate` for the real rules. Esc closes.
- Delete: no `confirm()`. Delete immediately and show a **toast with "Desfazer"** (4.5s). Undo re-saves the record with `deleted:false`.
- **Recorrentes** card: rows with description, "dia N · categoria", amount, and Pausar/Retomar (paused rows go to 55% opacity).
- Empty and no-results states: a small stacked-card illustration, title and hint, plus "Limpar filtros" and "Novo lançamento" buttons.
- ≤720px: rows become `36px | 1fr | auto` (description over category, amount right), the toolbar turns into chips, a FAB "+" sits above the tab bar, and the form opens as a bottom sheet (see Mobile.dc.html).

### 4. Dívidas (`debts.js`)
- Head: kicker "Plano para sair delas", primary "Nova dívida".
- 3 KPI cards: Total devido · Parcelas mínimas/mês ("N% da sua renda") · Juros gerados/mês (value in --neg, card border tinted neg).
- Row: **Onde está o problema** (1 column) + **Seu plano** (spans 2, accent-line border).
  - Problema: 2 bullet facts from `diagnose` (costliest debt, top category). Then "Onde dá para guardar": lever checkboxes as selectable cards (checked → accent border + accent-soft). Each card has a title, R$/mês, a text, and a --pos effect line ("Sozinho: sai X antes e paga R$ a menos de juros"). Toggling one recalculates the plan live (keep the `?cortes=` URL).
  - Plano: the plan sentence at 20px/1.4 with bold figures and the duration in --accent. The comparison band (accent-soft) reads "Só no mínimo seriam … Você economiza R$". Below that, the "a mais por mês" input **plus a synced range slider (0–1500, step 50)** that recalculates on input with no submit button. The cuts note follows.
  - Chart: plan balance (2.5px accent line with a 28%→0 gradient fill, drawn in over 1s) vs. minimum-only (2px dashed --neg). A hover cursor line shows "{mês} · plano R$ · mínimo R$" in the header. Use Chart.js `line` (`fill`, `borderDash`) with the external tooltip.
  - "Ordem para quitar": numbered tiles with "zera em {mês}". Keep the "Calendário mês a mês" `<details>` table under it.
- **Suas dívidas** table: Dívida (name + "credor · vence dia N"), Saldo, Juros a.m. (in --neg/600 when >8%), Mínimo, "Só pagando o mínimo" ("X meses · R$ de juros", or "Não cobre os juros: a dívida cresce" in --neg). Clicking a row opens the edit modal (Nome*, Credor, Saldo devedor*, Juros ao mês %, Parcela mínima, Dia do vencimento; Apagar/Cancelar/Salvar).
- Do **not** add a "% quitado" bar: the domain model has no original amount. If you want it, add `originalCents` to `domain.Debt` first.
- Empty: check icon in an accent tile, "Nenhuma dívida cadastrada. Ótimo sinal!", and a CTA.

### 5. Análise (`insights.js`)
- Head: kicker "{mês} · dia D de N". On the right, a **Comparar com: Mês passado / Média 3 meses** segmented control that swaps the comparison line and the KPI note.
- 4 KPI cards: Pode gastar por dia (--pos, accent-line border) · Gastos variáveis até hoje (note in --neg if above) · Projeção do mês · **Taxa de economia** (new: `(in − out + invested) / in` for the month, plus the year average and the split target).
- **Ritmo do mês**: cumulative line this month (accent + gradient), comparison (dashed --muted), projection (dotted --warn from today to month end), hover strip "Dia N · Este mês · Setembro". Keep "Ver em tabela".
- Row: **Onde dá para economizar** (cards: title + amount chip, --neg for ritmo, --pos for savings) + **Categorias contra a média**. In the second card, each category has an 8px track, a filled bar for this month (--desejos when >5% above), a 2px --text tick at the 3-month average, and a diff % (--neg/--pos/--faint).
- Row of 3: **Gastei onde** (places, bars #3fb6d8 = the existing `--accent-2`) · **Taxa de economia por mês** (columns per month, ≥20% solid accent, below that 45% tint, negative --neg; dashed accent line at 20%) · **Patrimônio** (line from `netWorthPanel` data, dashed zero line, current value top-right).
- Row: **Assinaturas e contas fixas** (R$/mês + R$/ano; * marks found-but-not-recurring) · **Maiores gastos do mês**.

### 6. Consultor IA (`chat.html` + `chat.js`, and `advisor.js` in the app)
- Three columns: sidebar | chat | 300px context rail.
- Chat header: 36px panther avatar, "Consultor IA", and a status line with a dot (accent = ready "qwen3.5:9b · no seu computador", warn = no AI, neg = offline). "Nova conversa" sits on the right.
- Empty: h2 "Pergunte sobre o seu dinheiro." with an explainer, then a 2×2 grid of suggestion cards (icon + question; the same 4 `SUGGESTIONS`). On hover a card lifts 1px and gets an accent-line border.
- Messages, max-width 720px centered:
  - User: accent bubble, radius 16 16 4 16, right aligned, max 78%.
  - Assistant: 28px avatar + unboxed rich text at 14px/1.6. Markdown renders as paragraphs, ordered lists and **compact tables** (bordered, the first row tinted, numbers right-aligned, deltas colored). Below sits a "Salvar como nota" ghost action that changes to "Salvo em Notas".
  - **Proposal cards** (`ev.proposal`): dashed border, kind label (11px uppercase), description, and the button "Adicionar/Alterar/Apagar/Guardar". After it's applied the button turns accent-soft with a past-tense label and is disabled.
- Thinking: three bouncing accent dots + "Lendo seus lançamentos e fazendo as contas…", 2 shimmer lines, and the slow-first-response note. Use the partial streaming from `postSSE` to replace the shimmer as text arrives.
- Error: inline alert "Sem conexão com o computador." + "Tentar de novo". No AI configured: a warn card pointing to Ajustes → Consultor IA.
- Composer: auto-growing textarea inside a 16px-radius box (focus → accent border). The send button is a 38px square, accent when there's text and muted otherwise. Enter sends, Shift+Enter adds a newline. Footer hint: "Confira os números antes de seguir um plano."
- Context rail "O que o consultor vê": a key/value list (renda, saldo, dívidas, metas, notas), a Privacidade card (model + where it runs, anonymization note) and an "Ele também faz" card.

### 7. Ajustes (`settings.html` + app `device.js`)
- Two columns: 200px sticky section index (anchor links) | content cards.
- **Aparência**: 3 radio cards with mini theme previews (Claro / Escuro / Igual ao aparelho). They write `localStorage.tema` = `claro|escuro|auto` through the existing `theme.js`, replacing the `<select data-theme-pick>`. Animate the theme change with a 300ms `background-color, color` transition on `body` (skip it under reduced motion).
- **Conta e sincronização**: signed-in row (avatar initial, email, "Google Drive · pasta do finan_rich · sincronizado às …", Sair) or a signed-out row with "Entrar com Google". Only render this when Google sign-in is available.
- **Exibição financeira**: read-only "Real · R$ 1.234,56" (the only format supported). **Divisão do salário** has 3 % inputs, a live stacked bar, and a sum badge that reads green "Soma 100%" or red otherwise. It saves the `split` pref.
- **Consultor IA**: status band; provider radio cards from `.Presets` (name, description, "Recomendado" tag on Ollama); Modelo (monospace) and Chave da API (only for presets that need one); Salvar + "Testar conexão" (spinner → result text).
- **Open Finance**, **Celular** (paired devices with Desconectar; parear; acesso pelo navegador), **Seus dados** (3 link cards: Exportar, Backup .json, Banco .db), and **Sessão** ("Fechar o finan_rich" as an outlined destructive button). Keep every existing form action and field name from `settings.html`.

### 8. Contas e cartões (`accounts.js`)
- Head: kicker "Saldos, faturas e patrimônio", h1 "Contas e cartões", primary "Nova conta ou cartão".
- 3 KPI cards: **Nas contas** (`accountsCents`, "N contas, sem os cartões") · **Em aberto nos cartões** (`cardsOpenCents`, value in --neg) · **Patrimônio** (always-dark hero card, `netWorth` last point; "contas − cartões em aberto − dívidas").
- **Cartões** card: one row per card. On the left, a card tile (aspect 1.586, radius 16px, `linear-gradient(140deg,#14402f 0%,#0b1a15 70%)`, 1px --accent-line border, gold chip, "fecha dia N / vence dia N"); clicking it opens edit. On the right, two invoice panels from `current` and `next`. Each shows title · month, a status chip ("Fechada · pagar" in warn / "Aberta" neutral), the total (26px/600), the dates line, and a "N lançamentos ▾" disclosure that expands the item list (chevron rotates 180° over 200ms). A closed invoice gets a warn-tinted border. Below the cards, a warn band reads: "Pague a fatura inteira para não cair no rotativo…".
- **Contas** list (non-card accounts): 40px icon tile by kind (bank / piggy bank / wallet), name, kind label, balance (--neg if negative) with a 120px share bar relative to the largest balance, and a chevron. Clicking a row opens edit.
- **Patrimônio · últimos 12 meses**: subtitle with the 12-month change and the note "Dívidas entram pelo saldo de hoje em todos os meses." A hover strip shows Contas / Cartões / Dívidas / Patrimônio for the hovered month. The line chart has an accent stroke + gradient fill and a dashed zero line. A "Ver em tabela / Ver gráfico" toggle swaps the chart for the 5-column table. Reuse this same panel in Análise (`netWorthPanel`).
- Modal: Corrente / Poupança / Carteira / Cartão segmented control, Nome*, then the amount field. Its label is "Saldo inicial" (accepts "-350,00") or "Em aberto hoje" for cards. For cards it adds Dia do fechamento* and Dia do vencimento* (1–31). Footer: Apagar (toast: "Os lançamentos continuam, só sem conta.") / Cancelar / Salvar conta.
- Empty: a two-column state with copy plus a "Cadastrar a primeira conta" CTA and two overlapping card shapes.

### 9. Metas (`goals.js`)
- Head: kicker "Juntar um valor até um prazo", h1 "Metas", primary "Nova meta".
- Summary strip (4 cells): Guardado nas metas · Falta juntar · Guardar por mês, somando (sum of `perMonthCents`, in --accent) · "Cabe nos Investimentos (20%)?". The last one compares that sum with the Investimentos bucket's `plannedCents`: "Sim · sobra R$" in --pos or "Não · faltam R$" in --neg.
- Goal cards, grid `auto-fit minmax(340px,1fr)`. Each card has:
  - a 96px progress ring (8px stroke, round cap; accent, --pos when done, --neg when late; animates from 0 over 900ms) with the % in the center;
  - name (16px/600), status chip (Em dia / Atingida / Prazo passou), "R$ progresso de R$ alvo", and "até {prazo} · aportes de {categoria}";
  - an advice box: "Guarde R$ X por mês durante N meses para chegar lá." / late: "O prazo passou e faltam R$…" / done: "Meta atingida…";
  - **quick save**: an inline "R$ Quanto guardou? [Guardar]" field (Enter submits) that adds to `savedCents` and shows a toast. This is the same action the consultant does with "guardei 500 na viagem". Plus an "Editar" button.
  - Done cards get an accent-line border and no quick-save field.
- Replace the 🎉 emoji in `goalCards` with the "Atingida" chip.
- Modal: Nome*, Quanto quer juntar*, Já tem guardado, Prazo (`type=month`), Aportes vêm da categoria (non-renda categories + "Nenhuma (atualizo à mão)"). A live preview band reads "Para chegar até {mês}: guarde R$ X por mês durante N meses." It's computed with the same rule as `finance.Goals`: months = months to the deadline counting the current one, rounded up.
- Empty: "Comece pela reserva de emergência." The text cites 3–6× the average monthly spending. The CTA prefills the form with name "Reserva de emergência" and the 3× value.
- The Painel "Metas" section keeps using the compact variant: ring 40px, no actions.

### Mobile (≤720px), see Mobile.dc.html
- Keep the current floating tab bar: 6 items (Painel, Extrato, Dívidas, Análise, Consultor, Ajustes); Contas/Metas stay `.tab-extra`. Restyle it: radius 22px, surface at 92% + 14px blur, `--shadow-pop`. The active icon sits in a 52×28 accent-soft pill with an accent icon and 600 weight label.
- Top: brand + sync pill. The month nav goes full width (44px arrows).
- Hero card first, then Entradas/Saídas in 2 columns, an urgent alert, the buckets, and recent items.
- Lists become day-grouped bordered groups with rows of at least 44px. The FAB "+" (56px, radius 18px) sits above the tab bar. Forms open as a **bottom sheet** (radius 26px top, grab handle, 52px primary button, category chips instead of a select).
- Inputs use 16px font (no iOS zoom), as today.

## Interactions & motion
- Card entrance: `rise` (opacity 0→1, translateY 6px→0, 400ms ease, staggered 50ms) on the first paint of a screen only (like `charts.js` `animated` set).
- Bars grow height over 600ms with `cubic-bezier(.2,.8,.2,1)`. Lines draw in via stroke-dashoffset over 1–1.2s. Category bars scale from the left over 700ms.
- Hover: rows get bg --hover (120ms); cards lift on hover only when clickable; the primary button changes bg to --accent-hi and moves 1px down when pressed.
- Modals: scrim fade 180ms + panel pop 220ms; Esc closes; focus moves into the dialog and returns to the trigger afterwards.
- Toasts: bottom center, 12px radius, accent dot, auto-hide after 3–4.5s, `role="status"`.
- `prefers-reduced-motion: reduce` disables all of the above (already in app.css).
- Focus ring: `outline: 2px solid var(--accent); outline-offset: 2px`.

## State (per screen, on top of existing ctx)
- Painel: `hoverMonth` (default current month).
- Lançamentos: filters (URL), `form` {id?, kind, amount, desc, cat, date, account, parts, repeat}, `errors`, `toast` + `undoRecord`.
- Dívidas: `extra` (URL), `cortes` (URL), `hoverMonth`, `form`, `errors`.
- Análise: `compare` = `mes|media`, `hoverDay`.
- Consultor: existing `chat` object + `appliedProposals`, `savedNotes`.
- Ajustes: theme (localStorage), split draft, preset/model draft, test status.

## Assets
- `brand-mark.png`: the panther mark, cropped from the user's artwork (256×256, black background). Use it for the sidebar brand, the chat avatar and the welcome art. **Also regenerate** `favicon.svg` / `static/icons/*` and `packaging/icon.*` from the original artwork (the user has the full-resolution file). Update `<meta name="theme-color">` to `#0a0d0c` (dark) / `#f4f6f5` (light) in `theme.js`.
- Icons: the existing set plus Lucide paths (copied into the page files).
- Fonts: Geist (OFL). Self-host it for offline use.

## Files
- `Boas-vindas.dc.html`, `Painel.dc.html`, `Lancamentos.dc.html`, `Contas.dc.html`, `Dividas.dc.html`, `Metas.dc.html`, `Analise.dc.html`, `Consultor.dc.html`, `Ajustes.dc.html`, `Mobile.dc.html`: the screens. Contas `estado` (com contas / sem contas), Metas `estado` (com metas / sem metas). Tweak props: Painel `estado` (com dados / primeiro uso), Boas-vindas `estado` (inicial / carregando / erro), Dívidas `estado` (com dívidas / sem dívidas), Consultor `estado` (conversa / vazio / pensando / erro / sem IA).
- `finan-proto.js`: both theme token maps, nav, and the avalanche plan the mocks use (mirrors `finance.PayoffSchedule`; the real app must call the Go version).
- `brand-mark.png`, `support.js` (prototype runtime only).
