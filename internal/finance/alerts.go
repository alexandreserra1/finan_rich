package finance

import (
	"fmt"
	"sort"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Alert é um aviso do painel (e, no celular, base das notificações).
type Alert struct {
	// ID: tipo, item, período e, no vencimento, a gravidade. Fica igual enquanto a
	// situação não muda; é por ele que o sininho lembra o que a pessoa limpou.
	ID    string `json:"id"`
	Kind  string `json:"kind"`  // debt-due, debt-growing, bucket-over, budget-near, budget-over, unusual, goal-late, goal-done
	Level string `json:"level"` // danger, warn, ok
	Title string `json:"title"`
	Text  string `json:"text"`
	Link  string `json:"link"`  // tela onde agir
	Agent Agent  `json:"agent"` // quem do time avisa
	// Ask: a pergunta do "Perguntar para ...", que abre a conversa com quem avisou.
	// Sempre que serve, é uma das que o Consultor responde sem IA (advisor.prepared).
	Ask string `json:"ask"`
}

// alert monta o aviso; key é o item e o período (o ID fica kind:key).
func alert(kind, key, level, title, text, link, ask string) Alert {
	return Alert{ID: kind + ":" + key, Kind: kind, Level: level, Title: title, Text: text, Link: link, Agent: agentFor(kind), Ask: ask}
}

const maxAlerts = 6

// Alerts levanta o que merece atenção hoje (o Consultor vê todos).
func Alerts(l domain.Ledger, today time.Time) []Alert {
	return top(allAlerts(l, today))
}

// PendingAlerts são os avisos do sininho: os de Alerts menos os que a pessoa
// limpou (domain.PrefAlertsCleared). Um aviso limpo volta quando a situação
// muda: piorou, venceu de novo ou virou o mês com o problema ainda lá.
func PendingAlerts(l domain.Ledger, today time.Time) []Alert {
	cleared := l.ClearedAlerts()
	var out []Alert
	for _, a := range allAlerts(l, today) {
		if !cleared[a.ID] {
			out = append(out, a)
		}
	}
	return top(out)
}

// top deixa os graves primeiro e no máximo maxAlerts (lista vazia, nunca nula).
func top(out []Alert) []Alert {
	rank := map[string]int{"danger": 0, "warn": 1, "ok": 2}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Level] < rank[out[j].Level] })
	if len(out) > maxAlerts {
		out = out[:maxAlerts]
	}
	if out == nil {
		out = []Alert{}
	}
	return out
}

func allAlerts(l domain.Ledger, today time.Time) []Alert {
	var out []Alert
	brl := money.Format
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	current := domain.NewMonth(today.Year(), today.Month())

	for _, d := range PlanDebts(l.ActiveDebts()).Debts {
		if d.BalanceCents <= 0 {
			continue
		}
		if d.DueDay > 0 {
			due := current.Day(d.DueDay)
			if due.Before(day) {
				due = current.Next().Day(d.DueDay)
			}
			switch days := int(due.Sub(day).Hours() / 24); {
			case days == 0:
				out = append(out, alert("debt-due", d.ID+":"+due.Format(time.DateOnly)+":danger", "danger", d.Name+" vence hoje", "Parcela mínima: "+brl(d.MinPaymentCents)+".", "/dividas", "O que vence esta semana?"))
			case days <= 3:
				out = append(out, alert("debt-due", d.ID+":"+due.Format(time.DateOnly)+":warn", "warn", fmt.Sprintf("%s vence em %d dia%s (dia %d)", d.Name, days, plural(days), d.DueDay), "Parcela mínima: "+brl(d.MinPaymentCents)+".", "/dividas", "O que vence esta semana?"))
			}
		}
		if d.MinPaymentCents > 0 && !d.Payable {
			out = append(out, alert("debt-growing", d.ID+":"+current.Key(), "danger", d.Name+" está crescendo",
				"A parcela mínima ("+brl(d.MinPaymentCents)+") não cobre os juros do mês ("+brl(d.MonthlyInterestCents)+").", "/dividas",
				"Qual dívida devo pagar primeiro?"))
		}
	}

	s := Summarize(current, l)
	if s.InCents > 0 {
		for _, b := range s.Buckets {
			if b.PlannedCents > 0 && b.SpentCents > b.PlannedCents {
				out = append(out, alert("bucket-over", b.Bucket+":"+current.Key(), "warn", fmt.Sprintf("%s passou da meta do mês", bucketName(b.Bucket)),
					fmt.Sprintf("Gasto %s de %s planejados (%d%%).", brl(b.SpentCents), brl(b.PlannedCents), b.UsedPct()), "/",
					fmt.Sprintf("Passei da meta de %s este mês. Onde dá para cortar?", bucketName(b.Bucket))))
			}
		}
	}
	for _, b := range Budgets(l, current) {
		link := "/lancamentos?categoria=" + b.CategoryID
		switch {
		case b.SpentCents >= b.BudgetCents:
			title := "Orçamento de " + b.Name + " estourou"
			if b.RemainingCents == 0 {
				title = "Orçamento de " + b.Name + " acabou"
			}
			out = append(out, alert("budget-over", b.CategoryID+":"+current.Key(), "danger", title,
				fmt.Sprintf("Gasto %s de %s no mês (%d%%).", brl(b.SpentCents), brl(b.BudgetCents), b.UsedPct), link,
				"O orçamento de "+b.Name+" estourou. Como seguro os gastos até o fim do mês?"))
		case b.UsedPct >= budgetWarnPct:
			out = append(out, alert("budget-near", b.CategoryID+":"+current.Key(), "warn", fmt.Sprintf("Orçamento de %s em %d%%", b.Name, b.UsedPct),
				fmt.Sprintf("Restam %s de %s no mês.", brl(b.RemainingCents), brl(b.BudgetCents)), link,
				"Como chego no fim do mês sem estourar o orçamento de "+b.Name+"?"))
		}
	}
	out = append(out, unusualSpending(l, current)...)

	for _, g := range Goals(l, today) {
		switch {
		case g.Late:
			out = append(out, alert("goal-late", g.ID+":"+current.Key(), "warn", "Prazo da meta \""+g.Name+"\" passou", "Faltam "+brl(g.RemainingCents)+". Que tal ajustar o prazo?", "/metas", "Minhas metas estão no ritmo?"))
		case g.Done && g.Deadline != "" && g.Deadline >= current.Key():
			out = append(out, alert("goal-done", g.ID, "ok", "Meta \""+g.Name+"\" atingida!", "Você juntou "+brl(g.ProgressCents)+".", "/metas", "Bati a meta \""+g.Name+"\". O que eu faço agora?"))
		}
	}
	return out
}

// unusualSpending: categorias em que o mês já passou bem da média dos três
// meses anteriores (pelo menos 50% e R$ 100 acima).
func unusualSpending(l domain.Ledger, m domain.Month) []Alert {
	now := Summarize(m, l)
	avg := map[string]int64{}
	months := int64(0) // só meses com lançamentos: quem começou há 2 meses não tem média de 3
	for _, prev := range []domain.Month{m.Prev(), m.Prev().Prev(), m.Prev().Prev().Prev()} {
		s := Summarize(prev, l)
		if s.InCents == 0 && s.OutCents == 0 {
			continue
		}
		months++
		for _, c := range s.Categories {
			avg[c.Name] += c.SpentCents
		}
	}
	var out []Alert
	for _, c := range now.Categories {
		a := avg[c.Name] / max(months, 1)
		if a > 0 && c.SpentCents*2 >= a*3 && c.SpentCents-a >= 10000 {
			out = append(out, alert("unusual", c.Name+":"+m.Key(), "warn", c.Name+" acima do normal",
				fmt.Sprintf("%s este mês, %d%% acima da média dos últimos 3 meses (%s).", money.Format(c.SpentCents), (c.SpentCents-a)*100/a, money.Format(a)), "/lancamentos",
				"Onde estou gastando demais este mês?"))
		}
	}
	return out
}

func bucketName(b string) string {
	switch b {
	case domain.BucketNecessidades:
		return "Necessidades"
	case domain.BucketDesejos:
		return "Desejos"
	case domain.BucketInvestimentos:
		return "Investimentos"
	}
	return b
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
