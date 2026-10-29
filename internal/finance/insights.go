package finance

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Análise de gastos (docs/ANALISE.md): o ritmo do mês, quanto ainda dá para
// gastar por dia, onde o dinheiro foi e onde dá para economizar. Investimentos
// não contam como gasto; lançamentos de recorrência (ID "rec-…") são fixos.

// InsightsView é a tela Análise do mês de hoje.
type InsightsView struct {
	Month       string `json:"month"`
	Label       string `json:"label"`
	Day         int    `json:"day"` // hoje
	DaysInMonth int    `json:"daysInMonth"`

	VariableCents         int64   `json:"variableCents"`         // gastos variáveis até hoje
	FixedCents            int64   `json:"fixedCents"`            // fixos do mês: lançados + que ainda vão cair
	ProjectedCents        int64   `json:"projectedCents"`        // como o mês fecha no ritmo atual
	LastMonthCents        int64   `json:"lastMonthCents"`        // gastos do mês passado
	LastMonthSameDayCents int64   `json:"lastMonthSameDayCents"` // variáveis do mês passado até o mesmo dia
	Avg3Cents             int64   `json:"avg3Cents"`             // média dos 3 meses anteriores
	Pace                  []int64 `json:"pace"`                  // variável acumulado, do dia 1 até hoje
	LastPace              []int64 `json:"lastPace"`              // variável acumulado do mês passado
	Avg3Pace              []int64 `json:"avg3Pace"`              // variável acumulado médio dos 3 meses anteriores (com lançamentos), nos dias deste mês

	IncomeCents     int64 `json:"incomeCents"`
	IncomeEstimated bool  `json:"incomeEstimated"` // renda ainda não entrou: média dos meses anteriores
	FreeCents       int64 `json:"freeCents"`       // o que ainda cabe no mês (pode ser negativo)
	PerDayCents     int64 `json:"perDayCents"`     // pode gastar por dia até o fim do mês

	Places        []PlaceRow           `json:"places"`
	Categories    []CategoryTrend      `json:"categories"`
	Savings       []Saving             `json:"savings"`
	Subscriptions []Subscription       `json:"subscriptions"`
	Biggest       []domain.Transaction `json:"biggest"`
	Budgets       []BudgetView         `json:"budgets"` // orçamentos por categoria no mês

	// Taxa de economia: (entradas − saídas + investido) ÷ entradas.
	SavingsPct       int         `json:"savingsPct"`       // deste mês
	SavingsAvgPct    int         `json:"savingsAvgPct"`    // média dos meses anteriores com renda (até 12)
	SavingsTargetPct int         `json:"savingsTargetPct"` // o que a divisão do salário reserva para Investimentos
	SavingsRates     []RatePoint `json:"savingsRates"`     // os últimos 6 meses, este incluído
}

// RatePoint é a taxa de economia de um mês.
type RatePoint struct {
	Month     string `json:"month"` // "2026-10"
	Label     string `json:"label"` // "outubro de 2026"
	Pct       int    `json:"pct"`
	HasIncome bool   `json:"hasIncome"` // sem renda no mês, a taxa não diz nada
}

// PlaceRow é quanto foi gasto num lugar (iFood, Uber...) no mês.
type PlaceRow struct {
	Name      string `json:"name"`
	Cents     int64  `json:"cents"`
	Count     int    `json:"count"`
	SharePct  int    `json:"sharePct"`
	Avg3Cents int64  `json:"avg3Cents"`
}

// CategoryTrend compara a categoria no mês com a média dos 3 anteriores.
type CategoryTrend struct {
	Name      string `json:"name"`
	Bucket    string `json:"bucket"`
	Cents     int64  `json:"cents"`
	SharePct  int    `json:"sharePct"`
	Avg3Cents int64  `json:"avg3Cents"`
	DiffPct   int    `json:"diffPct"` // contra a média (0 sem histórico)
}

// Saving é uma sugestão de onde economizar, com a economia estimada por mês.
type Saving struct {
	Kind         string `json:"kind"` // categoria, frequente, desejos, ritmo, assinaturas
	Title        string `json:"title"`
	Text         string `json:"text"`
	MonthlyCents int64  `json:"monthlyCents"`
	Category     string `json:"category,omitempty"` // a categoria da economia (para não somar duas vezes a mesma)
	Bucket       string `json:"bucket,omitempty"`
}

// Subscription é uma cobrança que se repete todo mês.
type Subscription struct {
	Name         string `json:"name"`
	MonthlyCents int64  `json:"monthlyCents"`
	YearlyCents  int64  `json:"yearlyCents"`
	Registered   bool   `json:"registered"` // cadastrada como recorrente
	Bucket       string `json:"bucket"`     // grupo da categoria (desejos, necessidades...)
}

// Limites das sugestões (ver docs/ANALISE.md).
const (
	overAvgPct      = 25   // categoria pelo menos 25% acima da média...
	overAvgMinCents = 5000 // ...e R$ 50 a mais
	frequentMin     = 8    // compras em 30 dias
	frequentTicket  = 6000 // tíquete médio até R$ 60
	paceOverPct     = 10   // ritmo 10% acima do mês passado
	subsTolerance   = 15   // assinatura: valor varia até 15% entre meses
)

// Insights monta a análise do mês de hoje.
func Insights(l domain.Ledger, today time.Time) InsightsView {
	m := domain.NewMonth(today.Year(), today.Month())
	prev := m.Prev()
	history := []string{prev.Key(), prev.Prev().Key(), prev.Prev().Prev().Key()}
	days, lastDays := daysIn(m), daysIn(prev)
	day := today.Day()
	todayKey := today.Format(domain.DateLayout)
	cats := l.CategoryIndex()
	placeCats := placeCategories(l)
	v := InsightsView{
		Month: m.Key(), Label: m.Label(), Day: day, DaysInMonth: days,
		Pace: make([]int64, day), LastPace: make([]int64, lastDays),
		Places: []PlaceRow{}, Categories: []CategoryTrend{}, Savings: []Saving{}, Subscriptions: []Subscription{}, Biggest: []domain.Transaction{},
		Budgets: Budgets(l, m),
	}

	txs := l.ActiveTransactions()
	spendByMonth := map[string]int64{}
	incomeByMonth := map[string]int64{}
	byCat := map[string]map[string]int64{}   // categoria → mês → centavos
	byPlace := map[string]map[string]int64{} // lugar → mês → centavos
	placeCount := map[string]map[string]int{}
	placeNames := map[string]map[string]int{}
	var investedNow, futureCents int64
	exists := map[string]bool{}
	var monthSpends []domain.Transaction
	byDay3 := map[string][]int64{} // mês anterior → gasto variável por dia (dias deste mês)
	for _, t := range txs {
		exists[t.ID] = true
		mk := t.Date[:7]
		if t.Kind == domain.KindEntrada {
			incomeByMonth[mk] += t.AmountCents
			continue
		}
		if cats[t.CategoryID].Bucket == domain.BucketInvestimentos {
			if mk == m.Key() {
				investedNow += t.AmountCents
			}
			continue
		}
		spendByMonth[mk] += t.AmountCents
		add(byCat, t.CategoryID, mk, t.AmountCents)
		key, name := importer.PlaceKey(t.Description)
		add(byPlace, key, mk, t.AmountCents)
		if placeCount[key] == nil {
			placeCount[key], placeNames[key] = map[string]int{}, map[string]int{}
		}
		placeCount[key][mk]++
		placeNames[key][name]++
		fixed := strings.HasPrefix(t.ID, "rec-")
		switch {
		case mk == m.Key():
			monthSpends = append(monthSpends, t)
			switch {
			case fixed:
				v.FixedCents += t.AmountCents
			case t.Date > todayKey:
				futureCents += t.AmountCents // agendado para depois de hoje
			default:
				v.Pace[dayOf(t.Date)-1] += t.AmountCents
			}
		case mk == prev.Key() && !fixed:
			v.LastPace[min(dayOf(t.Date), lastDays)-1] += t.AmountCents
		}
		if !fixed && (mk == history[0] || mk == history[1] || mk == history[2]) {
			if byDay3[mk] == nil {
				byDay3[mk] = make([]int64, days)
			}
			byDay3[mk][min(dayOf(t.Date), days)-1] += t.AmountCents // dia 31 de um mês cai no último dia deste
		}
	}
	cumulate(v.Pace)
	cumulate(v.LastPace)
	v.VariableCents = v.Pace[day-1]
	v.LastMonthSameDayCents = v.LastPace[min(day, lastDays)-1]
	v.LastMonthCents = spendByMonth[prev.Key()]

	// Recorrentes deste mês que ainda não viraram lançamento.
	var dueCents int64
	for _, r := range l.ActiveRecurring() {
		if r.Kind != domain.KindSaida || r.Paused || r.Start > m.Key() || r.End != "" && r.End < m.Key() ||
			exists[domain.RecurringTxID(r.ID, m)] || cats[r.CategoryID].Bucket == domain.BucketInvestimentos {
			continue
		}
		dueCents += r.AmountCents
	}
	v.FixedCents += dueCents
	v.ProjectedCents = v.VariableCents*int64(days)/int64(day) + v.FixedCents + futureCents

	// Médias dos 3 meses anteriores (só os meses com algum lançamento).
	months := 0
	for _, k := range history {
		if spendByMonth[k] > 0 || incomeByMonth[k] > 0 {
			months++
		}
	}
	avg := func(byMonth map[string]int64) int64 {
		if months == 0 {
			return 0
		}
		var sum int64
		for _, k := range history {
			sum += byMonth[k]
		}
		return sum / int64(months)
	}
	v.Avg3Cents = avg(spendByMonth)
	v.Avg3Pace = make([]int64, days)
	if months > 0 {
		for _, k := range history {
			cumulate(byDay3[k])
			for i, c := range byDay3[k] {
				v.Avg3Pace[i] += c
			}
		}
		for i := range v.Avg3Pace {
			v.Avg3Pace[i] /= int64(months)
		}
	}
	savingsRates(&v, l, m)

	// Quanto ainda cabe no mês (o "pode gastar" do PocketGuard).
	v.IncomeCents = incomeByMonth[m.Key()]
	if v.IncomeCents == 0 {
		v.IncomeCents, v.IncomeEstimated = avg(incomeByMonth), true
	}
	// Livre = renda − investimento planejado (ou o já investido, se for mais) −
	// tudo que já foi lançado no mês − recorrentes que ainda vão cair.
	reserve := max(v.IncomeCents*int64(l.Split()[domain.BucketInvestimentos])/100, investedNow)
	v.FreeCents = v.IncomeCents - reserve - spendByMonth[m.Key()] - dueCents
	if left := int64(days - day + 1); v.FreeCents > 0 {
		v.PerDayCents = v.FreeCents / left
	}

	// Onde foi: lugares e categorias do mês.
	total := spendByMonth[m.Key()]
	for key, byMonth := range byPlace {
		if byMonth[m.Key()] == 0 {
			continue
		}
		name := "Outros"
		if key != "" {
			name = mostUsed(placeNames[key])
		}
		v.Places = append(v.Places, PlaceRow{Name: name, Cents: byMonth[m.Key()], Count: placeCount[key][m.Key()],
			SharePct: pct(byMonth[m.Key()], total), Avg3Cents: avg(byMonth)})
	}
	sort.Slice(v.Places, func(i, j int) bool {
		return less(v.Places[i].Cents, v.Places[j].Cents, v.Places[i].Name, v.Places[j].Name)
	})
	for id, byMonth := range byCat {
		now, a := byMonth[m.Key()], avg(byMonth)
		if now == 0 {
			continue
		}
		c := CategoryTrend{Name: cats[id].Name, Bucket: cats[id].Bucket, Cents: now, SharePct: pct(now, total), Avg3Cents: a}
		if a > 0 {
			c.DiffPct = int((now - a) * 100 / a)
		}
		v.Categories = append(v.Categories, c)
		if a > 0 && now*100 >= a*(100+overAvgPct) && now-a >= overAvgMinCents {
			v.Savings = append(v.Savings, Saving{Kind: "categoria", Title: c.Name, MonthlyCents: now - a, Category: c.Name, Bucket: c.Bucket,
				Text: fmt.Sprintf("%s este mês; a média dos últimos meses é %s. Voltar à média economiza %s.", money.Format(now), money.Format(a), money.Format(now-a))})
		}
	}
	sort.Slice(v.Categories, func(i, j int) bool {
		return less(v.Categories[i].Cents, v.Categories[j].Cents, v.Categories[i].Name, v.Categories[j].Name)
	})

	// Compras pequenas e frequentes nos últimos 30 dias.
	from := today.AddDate(0, 0, -29).Format(domain.DateLayout)
	recent := map[string][2]int64{} // lugar → {compras, centavos}
	for _, t := range txs {
		if t.Kind != domain.KindSaida || t.Date < from || t.Date > todayKey || strings.HasPrefix(t.ID, "rec-") ||
			cats[t.CategoryID].Bucket == domain.BucketInvestimentos {
			continue
		}
		if key, _ := importer.PlaceKey(t.Description); key != "" {
			r := recent[key]
			recent[key] = [2]int64{r[0] + 1, r[1] + t.AmountCents}
		}
	}
	for key, r := range recent {
		if r[0] >= frequentMin && r[1]/r[0] <= frequentTicket {
			c := cats[placeCats[key]]
			v.Savings = append(v.Savings, Saving{Kind: "frequente", Title: mostUsed(placeNames[key]), MonthlyCents: r[1] / 2, Category: c.Name, Bucket: c.Bucket,
				Text: fmt.Sprintf("%d compras em 30 dias, %s no total (média de %s). Cortar pela metade economiza %s por mês.",
					r[0], money.Format(r[1]), money.Format(r[1]/r[0]), money.Format(r[1]/2))})
		}
	}

	// "Desejos" acima do planejado na divisão do salário.
	for _, b := range Summarize(m, l).Buckets {
		if b.Bucket == domain.BucketDesejos && b.PlannedCents > 0 && b.RemainingCents() < 0 {
			v.Savings = append(v.Savings, Saving{Kind: "desejos", Title: "Desejos acima do planejado", MonthlyCents: -b.RemainingCents(), Bucket: domain.BucketDesejos,
				Text: fmt.Sprintf("Já passou %s do que a divisão do salário reserva para desejos (%s).", money.Format(-b.RemainingCents()), money.Format(b.PlannedCents))})
		}
	}

	// Ritmo acima do mês passado.
	if v.LastMonthCents > 0 && v.ProjectedCents*100 > v.LastMonthCents*(100+paceOverPct) {
		d := v.ProjectedCents - v.LastMonthCents
		v.Savings = append(v.Savings, Saving{Kind: "ritmo", Title: "Gastando mais rápido que no mês passado", MonthlyCents: d,
			Text: fmt.Sprintf("No ritmo atual, o mês fecha em %s, %s a mais que o mês passado.", money.Format(v.ProjectedCents), money.Format(d))})
	}

	v.Subscriptions = subscriptions(l, m, cats, placeCats, byPlace, placeCount, placeNames)
	var subsMonth int64
	for _, s := range v.Subscriptions {
		subsMonth += s.MonthlyCents
	}
	slices.SortStableFunc(v.Savings, func(a, b Saving) int { return int(b.MonthlyCents - a.MonthlyCents) })
	if len(v.Subscriptions) > 0 {
		v.Savings = append(v.Savings, Saving{Kind: "assinaturas", Title: "Assinaturas e contas fixas",
			Text: fmt.Sprintf("%d cobranças todo mês somam %s (%s por ano). Vale revisar as que você não usa mais.",
				len(v.Subscriptions), money.Format(subsMonth), money.Format(subsMonth*12))})
	}

	// Maiores gastos do mês.
	slices.SortStableFunc(monthSpends, func(a, b domain.Transaction) int { return int(b.AmountCents - a.AmountCents) })
	v.Biggest = append(v.Biggest, monthSpends[:min(5, len(monthSpends))]...)
	return v
}

// subscriptions junta as recorrências cadastradas com as cobranças que se
// repetem sozinhas: o mesmo lugar em pelo menos 3 dos últimos 4 meses, no
// máximo 2 vezes por mês e com valor parecido (o Rocket Money faz assim).
func subscriptions(l domain.Ledger, m domain.Month, cats map[string]domain.Category, placeCats map[string]string,
	byPlace map[string]map[string]int64, count map[string]map[string]int, names map[string]map[string]int) []Subscription {
	out := []Subscription{}
	covered := map[string]bool{}
	for _, r := range l.ActiveRecurring() {
		if r.Kind != domain.KindSaida || r.Paused || r.End != "" && r.End < m.Key() || cats[r.CategoryID].Bucket == domain.BucketInvestimentos {
			continue
		}
		key, _ := importer.PlaceKey(r.Description)
		covered[key] = true
		out = append(out, Subscription{Name: r.Description, MonthlyCents: r.AmountCents, YearlyCents: 12 * r.AmountCents, Registered: true, Bucket: cats[r.CategoryID].Bucket})
	}
	window := []string{m.Key(), m.Prev().Key(), m.Prev().Prev().Key(), m.Prev().Prev().Prev().Key()}
	for key, byMonth := range byPlace {
		if key == "" || covered[key] {
			continue
		}
		var amounts []int64
		ok := true
		for _, k := range window {
			if c := count[key][k]; c > 2 {
				ok = false
			} else if c > 0 {
				amounts = append(amounts, byMonth[k])
			}
		}
		if !ok || len(amounts) < 3 {
			continue
		}
		slices.Sort(amounts)
		median := amounts[len(amounts)/2]
		if amounts[0]*100 < median*(100-subsTolerance) || amounts[len(amounts)-1]*100 > median*(100+subsTolerance) {
			continue
		}
		out = append(out, Subscription{Name: mostUsed(names[key]), MonthlyCents: median, YearlyCents: 12 * median, Bucket: cats[placeCats[key]].Bucket})
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i].MonthlyCents, out[j].MonthlyCents, out[i].Name, out[j].Name) })
	return out
}

func add(m map[string]map[string]int64, key, month string, cents int64) {
	if m[key] == nil {
		m[key] = map[string]int64{}
	}
	m[key][month] += cents
}

func cumulate(s []int64) {
	for i := 1; i < len(s); i++ {
		s[i] += s[i-1]
	}
}

func daysIn(m domain.Month) int { return m.End().AddDate(0, 0, -1).Day() }

func dayOf(date string) int {
	t, err := time.Parse(domain.DateLayout, date)
	if err != nil {
		return 1
	}
	return t.Day()
}

func pct(part, total int64) int {
	if total == 0 {
		return 0
	}
	return int(part * 100 / total)
}

// mostUsed devolve o nome mais usado (empate: ordem alfabética).
// placeCategories: a categoria mais usada nos gastos de cada lugar (iFood → Restaurantes).
func placeCategories(l domain.Ledger) map[string]string {
	counts := map[string]map[string]int{}
	for _, t := range l.ActiveTransactions() {
		if t.Kind != domain.KindSaida {
			continue
		}
		if key, _ := importer.PlaceKey(t.Description); key != "" {
			if counts[key] == nil {
				counts[key] = map[string]int{}
			}
			counts[key][t.CategoryID]++
		}
	}
	out := make(map[string]string, len(counts))
	for key, c := range counts {
		out[key] = mostUsed(c)
	}
	return out
}

func mostUsed(names map[string]int) string {
	best, n := "", 0
	for name, c := range names {
		if c > n || c == n && name < best {
			best, n = name, c
		}
	}
	return best
}

// less ordena do maior valor para o menor; empate pelo nome.
func less(a, b int64, an, bn string) bool {
	if a != b {
		return a > b
	}
	return an < bn
}

// savingsRates preenche a taxa de economia: a deste mês, a dos últimos 6, a
// média dos meses anteriores com renda e a meta da divisão do salário.
func savingsRates(v *InsightsView, l domain.Ledger, m domain.Month) {
	start := m
	for range 5 {
		start = start.Prev()
	}
	v.SavingsRates = []RatePoint{}
	for k := start; k.Key() <= m.Key(); k = k.Next() {
		s := Summarize(k, l)
		v.SavingsRates = append(v.SavingsRates, RatePoint{Month: k.Key(), Label: k.Label(), Pct: s.SavingsRatePct(), HasIncome: s.InCents > 0})
	}
	now := Summarize(m, l)
	v.SavingsPct = now.SavingsRatePct()
	v.SavingsTargetPct = domain.DefaultSplit()[domain.BucketInvestimentos]
	for _, b := range now.Buckets {
		if b.Bucket == domain.BucketInvestimentos && now.InCents > 0 {
			v.SavingsTargetPct = int(b.PlannedCents * 100 / now.InCents)
		}
	}
	sum, n := 0, 0
	k := m.Prev()
	for range 12 {
		if s := Summarize(k, l); s.InCents > 0 {
			sum += s.SavingsRatePct()
			n++
		}
		k = k.Prev()
	}
	if n > 0 {
		v.SavingsAvgPct = sum / n
	}
}
