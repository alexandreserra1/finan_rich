package advisor

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Ferramentas do consultor: a IA pode buscar lançamentos e propor mudanças
// (lançar, alterar, apagar, criar meta, criar dívida). Proposta nunca é
// gravada aqui: vira um cartão na tela com um botão, e só vale se a pessoa
// tocar nele (aí o aparelho grava com as mesmas regras de sempre).

// toolsPrompt complementa as instruções quando as ferramentas estão ligadas.
const toolsPrompt = `

Ferramentas:
- buscar_lancamentos: use quando a pergunta precisar de lançamentos que não estão no bloco de dados (outro mês, um período, um estabelecimento, uma categoria). Responda com os totais que ela devolver. Cada lançamento vem com o [id], que serve para alterar ou apagar.
- propor_lancamento, alterar_lancamento, apagar_lancamento, criar_meta, criar_divida, registrar_aporte (guardou dinheiro numa meta), registrar_pagamento (pagou uma dívida): use quando o usuário pedir para registrar, corrigir ou apagar algo. Para alterar ou apagar, busque antes para achar o id. Cada pedido vira um cartão com um botão para ele confirmar; nada é gravado sozinho. Nunca diga que já fez: diga que deixou pronto para ele confirmar.
- Você só tem as ferramentas da sua área. Se pedirem algo que nenhuma delas faz, diga quem do time cuida disso.`

// Ações de uma proposta.
const (
	ActionCreate = "criar"
	ActionEdit   = "alterar"
	ActionDelete = "apagar"
)

// Proposal é uma mudança sugerida pela IA, esperando a confirmação da pessoa.
// Record é o registro completo como vai ficar (validado pelas regras do
// domínio); o aparelho só grava. Applied é calculado ao mostrar a conversa.
type Proposal struct {
	Action  string          `json:"action"`
	Type    string          `json:"type"` // transaction, goal ou debt
	Record  json.RawMessage `json:"record"`
	Label   string          `json:"label"` // texto do cartão
	Applied bool            `json:"applied,omitempty"`
	// Before é o registro como estava quando a IA preparou a alteração: se ele
	// mudar depois, o aparelho recusa o cartão velho em vez de desfazer a mudança.
	Before json.RawMessage `json:"before,omitempty"`
}

// Tools executa as ferramentas sobre os dados atuais.
type Tools struct {
	ledger    domain.Ledger
	today     time.Time
	onPropose func(Proposal)
	only      []string // ferramentas do personagem que responde; nil = todas
}

func NewTools(l domain.Ledger, today time.Time, onPropose func(Proposal)) *Tools {
	// Cópias: as propostas vão sendo aplicadas aqui (ver Run) sem mexer nos dados de quem chamou.
	l.Transactions, l.Goals, l.Debts = slices.Clone(l.Transactions), slices.Clone(l.Goals), slices.Clone(l.Debts)
	return &Tools{ledger: l, today: today, onPropose: onPropose}
}

// maxListed limita quantos lançamentos a busca devolve (os totais contam todos).
const maxListed = 30

func tool(name, desc, params string) llm.Tool {
	return llm.Tool{Name: name, Description: desc, Parameters: json.RawMessage(params)}
}

// Defs são as ferramentas que a IA pode chamar: só as do personagem que responde.
func (t *Tools) Defs() []llm.Tool {
	if t.only == nil {
		return allTools()
	}
	return slices.DeleteFunc(allTools(), func(d llm.Tool) bool { return !slices.Contains(t.only, d.Name) })
}

func allTools() []llm.Tool {
	return []llm.Tool{
		tool("buscar_lancamentos", "Busca lançamentos do usuário e devolve os totais e os mais recentes, cada um com seu [id]. Todos os filtros são opcionais.",
			`{"type":"object","properties":{
				"texto":{"type":"string","description":"parte da descrição, ex.: ifood, uber, aluguel"},
				"categoria":{"type":"string","description":"nome da categoria, ex.: Mercado"},
				"tipo":{"type":"string","enum":["entrada","saida"]},
				"de":{"type":"string","description":"início: AAAA-MM-DD ou AAAA-MM"},
				"ate":{"type":"string","description":"fim: AAAA-MM-DD ou AAAA-MM"}}}`),
		tool("propor_lancamento", "Prepara um lançamento novo para o usuário confirmar.",
			`{"type":"object","properties":{
				"descricao":{"type":"string"},
				"valor":{"type":"string","description":"valor em reais, positivo, ex.: 52,30"},
				"tipo":{"type":"string","enum":["saida","entrada"],"description":"saida (gasto, padrão) ou entrada (renda)"},
				"categoria":{"type":"string","description":"nome de uma categoria existente"},
				"data":{"type":"string","description":"AAAA-MM-DD; padrão: hoje"}},
				"required":["descricao","valor","categoria"]}`),
		tool("alterar_lancamento", "Prepara a correção de um lançamento existente (pelo id da busca). Informe só o que muda.",
			`{"type":"object","properties":{
				"id":{"type":"string"},"descricao":{"type":"string"},"valor":{"type":"string"},
				"categoria":{"type":"string"},"data":{"type":"string","description":"AAAA-MM-DD"}},
				"required":["id"]}`),
		tool("apagar_lancamento", "Prepara a remoção de um lançamento existente (pelo id da busca).",
			`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		tool("criar_meta", "Prepara uma meta de economia para o usuário confirmar.",
			`{"type":"object","properties":{
				"nome":{"type":"string","description":"ex.: Viagem, Reserva de emergência"},
				"valor":{"type":"string","description":"quanto juntar, em reais"},
				"prazo":{"type":"string","description":"AAAA-MM (opcional)"},
				"ja_guardado":{"type":"string","description":"quanto já tem guardado, em reais (opcional)"}},
				"required":["nome","valor"]}`),
		tool("criar_divida", "Prepara o cadastro de uma dívida para o usuário confirmar.",
			`{"type":"object","properties":{
				"nome":{"type":"string","description":"ex.: Cartão Nubank"},
				"credor":{"type":"string"},
				"saldo":{"type":"string","description":"quanto deve hoje, em reais"},
				"juros_mes":{"type":"string","description":"juros ao mês em %, ex.: 12,9"},
				"parcela_minima":{"type":"string","description":"em reais"},
				"vencimento":{"type":"integer","description":"dia do vencimento (1 a 31, opcional)"}},
				"required":["nome","saldo","juros_mes","parcela_minima"]}`),
		tool("registrar_aporte", "Prepara um aporte (dinheiro guardado) numa meta existente, para o usuário confirmar.",
			`{"type":"object","properties":{
				"meta":{"type":"string","description":"nome da meta"},
				"valor":{"type":"string","description":"em reais"},
				"data":{"type":"string","description":"AAAA-MM-DD; padrão: hoje"}},
				"required":["meta","valor"]}`),
		tool("registrar_pagamento", "Prepara o pagamento de uma dívida existente (baixa o saldo e lança o gasto), para o usuário confirmar.",
			`{"type":"object","properties":{
				"divida":{"type":"string","description":"nome da dívida"},
				"valor":{"type":"string","description":"quanto pagou, em reais"},
				"data":{"type":"string","description":"AAAA-MM-DD; padrão: hoje"}},
				"required":["divida","valor"]}`),
	}
}

// Run executa um pedido e devolve o texto que volta para a IA. Erros também
// voltam como texto, para ela corrigir o pedido.
func (t *Tools) Run(c llm.ToolCall) string {
	if t.only != nil && !slices.Contains(t.only, c.Name) {
		return "essa ferramenta não é com você: diga ao usuário quem do time cuida disso."
	}
	var a map[string]json.RawMessage
	if err := json.Unmarshal(c.Args, &a); err != nil {
		return "argumentos inválidos: " + err.Error()
	}
	str := func(k string) string {
		var s string
		if json.Unmarshal(a[k], &s) != nil {
			s = string(a[k]) // número em vez de texto: aceita
		}
		return strings.TrimSpace(s)
	}
	var (
		p     Proposal
		extra []Proposal // um pedido pode virar mais de um cartão (pagamento = dívida + lançamento)
		err   error
	)
	switch c.Name {
	case "buscar_lancamentos":
		return t.search(str("texto"), str("categoria"), str("tipo"), str("de"), str("ate"))
	case "propor_lancamento":
		p, err = t.newTransaction(str("descricao"), str("valor"), str("tipo"), str("categoria"), str("data"))
	case "alterar_lancamento":
		p, err = t.editTransaction(str("id"), str("descricao"), str("valor"), str("categoria"), str("data"))
	case "apagar_lancamento":
		p, err = t.deleteTransaction(str("id"))
	case "criar_meta":
		p, err = t.newGoal(str("nome"), str("valor"), str("prazo"), str("ja_guardado"))
	case "criar_divida":
		p, err = t.newDebt(str("nome"), str("credor"), str("saldo"), str("juros_mes"), str("parcela_minima"), str("vencimento"))
	case "registrar_aporte":
		p, err = t.contribute(str("meta"), str("valor"), str("data"))
	case "registrar_pagamento":
		p, extra, err = t.payDebt(str("divida"), str("valor"), str("data"))
	default:
		return "ferramenta desconhecida: " + c.Name
	}
	if err != nil {
		return "não deu para preparar: " + err.Error()
	}
	labels := []string{}
	for _, q := range append([]Proposal{p}, extra...) {
		// A próxima proposta parte desta: "paguei 500 em jan e 500 em fev" dá
		// 3.200 → 2.700 → 2.200, e não dois cartões de 3.200 → 2.700.
		if q.Action == ActionEdit {
			q.Before = t.current(q)
		}
		t.apply(q)
		if t.onPropose != nil {
			t.onPropose(q)
		}
		labels = append(labels, q.Label)
	}
	return "Pronto na tela para o usuário confirmar (" + strings.Join(labels, "; ") + "). Ainda NÃO foi feito."
}

func (t *Tools) search(text, category, kind, from, to string) string {
	from, to = periodStart(from), periodEnd(to)
	cats := t.ledger.CategoryIndex()
	text, category = fold(text), fold(category)
	var found []domain.Transaction
	var out, in int64
	for _, tx := range t.ledger.ActiveTransactions() {
		if (from != "" && tx.Date < from) || (to != "" && tx.Date > to) ||
			(kind != "" && tx.Kind != kind) ||
			(text != "" && !strings.Contains(fold(tx.Description), text)) ||
			(category != "" && fold(cats[tx.CategoryID].Name) != category) {
			continue
		}
		found = append(found, tx)
		if tx.Kind == domain.KindEntrada {
			in += tx.AmountCents
		} else {
			out += tx.AmountCents
		}
	}
	if len(found) == 0 {
		return "Nenhum lançamento encontrado com esses filtros."
	}
	slices.SortFunc(found, func(x, y domain.Transaction) int { return strings.Compare(y.Date, x.Date) })
	var b strings.Builder
	fmt.Fprintf(&b, "%d lançamentos (de %s a %s). Saídas: %s. Entradas: %s.\n",
		len(found), found[len(found)-1].Date, found[0].Date, money.Format(out), money.Format(in))
	if len(found) > maxListed {
		fmt.Fprintf(&b, "Os %d mais recentes:\n", maxListed)
		found = found[:maxListed]
	}
	for _, tx := range found {
		fmt.Fprintf(&b, "- [%s] %s\n", tx.ID, t.txLabel(tx))
	}
	return b.String()
}

// periodStart/periodEnd aceitam AAAA-MM-DD, AAAA-MM (o mês inteiro) ou AAAA
// (o ano inteiro).
func periodStart(s string) string {
	switch len(s) {
	case 4:
		return s + "-01-01"
	case 7:
		return s + "-01"
	}
	return s
}

func periodEnd(s string) string {
	switch len(s) {
	case 4:
		return s + "-12-31"
	case 7:
		return s + "-31" // compara como texto: qualquer dia do mês é <= "-31"
	}
	return s
}

func (t *Tools) txLabel(tx domain.Transaction) string {
	sign := "-"
	if tx.Kind == domain.KindEntrada {
		sign = "+"
	}
	return fmt.Sprintf("%s · %s · %s · %s%s", dateBR(tx.Date), tx.Description, t.ledger.CategoryIndex()[tx.CategoryID].Name, sign, money.Format(tx.AmountCents))
}

func dateBR(iso string) string {
	if d, err := time.Parse(domain.DateLayout, iso); err == nil {
		return d.Format("02/01/2006")
	}
	return iso
}

// cents lê um valor em reais ("52,30", "52.3", "R$ 1.234,56").
func cents(s, field string) (int64, error) {
	v, err := money.Parse(s)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s inválido: informe um valor positivo em reais, ex.: 52,30", field)
	}
	return v, nil
}

// category acha a categoria pelo nome, do tipo certo (entrada só em renda).
func (t *Tools) category(name, kind string) (string, error) {
	var fits []domain.Category
	for _, c := range t.ledger.Categories {
		if !c.Deleted && (c.Bucket == domain.BucketRenda) == (kind == domain.KindEntrada) {
			fits = append(fits, c)
		}
	}
	if id := matchCategory(fits, name); id != "" {
		return id, nil
	}
	names := make([]string, len(fits))
	for i, c := range fits {
		names[i] = c.Name
	}
	return "", fmt.Errorf("categoria %q não existe para %s. Use uma destas: %s", name, kind, strings.Join(names, ", "))
}

// proposal valida o registro pelas regras do domínio e monta a proposta.
func proposal(action, typ string, record any, label string) (Proposal, error) {
	raw, err := json.Marshal(record)
	if err != nil {
		return Proposal{}, err
	}
	_, data, err := domain.DecodeRecord(typ, raw)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{Action: action, Type: typ, Record: data, Label: label}, nil
}

func (t *Tools) meta() domain.Meta {
	return domain.Meta{ID: newID(), UpdatedAt: t.today.UnixMilli()}
}

func (t *Tools) newTransaction(desc, value, kind, category, date string) (Proposal, error) {
	if kind != domain.KindEntrada {
		kind = domain.KindSaida
	}
	v, err := cents(value, "valor")
	if err != nil {
		return Proposal{}, err
	}
	if date == "" {
		date = t.today.Format(domain.DateLayout)
	}
	catID, err := t.category(category, kind)
	if err != nil {
		return Proposal{}, err
	}
	tx := domain.Transaction{Meta: t.meta(), Kind: kind, Description: desc, AmountCents: v, CategoryID: catID, Date: date}
	what := "Novo gasto"
	if kind == domain.KindEntrada {
		what = "Nova entrada"
	}
	return proposal(ActionCreate, domain.TypeTransaction, tx, what+": "+t.txLabel(tx))
}

func (t *Tools) findTransaction(id string) (domain.Transaction, error) {
	for _, tx := range t.ledger.ActiveTransactions() {
		if tx.ID == id {
			return tx, nil
		}
	}
	return domain.Transaction{}, fmt.Errorf("lançamento %q não encontrado; use buscar_lancamentos para achar o id", id)
}

func (t *Tools) editTransaction(id, desc, value, category, date string) (Proposal, error) {
	old, err := t.findTransaction(id)
	if err != nil {
		return Proposal{}, err
	}
	tx := old
	tx.UpdatedAt = max(t.today.UnixMilli(), old.UpdatedAt+1)
	var changes []string
	if desc != "" && desc != old.Description {
		tx.Description = desc
		changes = append(changes, fmt.Sprintf("descrição %q → %q", old.Description, desc))
	}
	if value != "" {
		if tx.AmountCents, err = cents(value, "valor"); err != nil {
			return Proposal{}, err
		}
		if tx.AmountCents != old.AmountCents {
			changes = append(changes, "valor "+money.Format(old.AmountCents)+" → "+money.Format(tx.AmountCents))
		}
	}
	if category != "" {
		if tx.CategoryID, err = t.category(category, tx.Kind); err != nil {
			return Proposal{}, err
		}
		if tx.CategoryID != old.CategoryID {
			cats := t.ledger.CategoryIndex()
			changes = append(changes, "categoria "+cats[old.CategoryID].Name+" → "+cats[tx.CategoryID].Name)
		}
	}
	if date != "" && date != old.Date {
		tx.Date = date
		changes = append(changes, "data "+dateBR(old.Date)+" → "+dateBR(date))
	}
	if len(changes) == 0 {
		return Proposal{}, fmt.Errorf("nada muda nesse lançamento")
	}
	return proposal(ActionEdit, domain.TypeTransaction, tx, "Alterar "+old.Description+" ("+dateBR(old.Date)+"): "+strings.Join(changes, "; "))
}

func (t *Tools) deleteTransaction(id string) (Proposal, error) {
	tx, err := t.findTransaction(id)
	if err != nil {
		return Proposal{}, err
	}
	label := "Apagar: " + t.txLabel(tx)
	tx.Deleted, tx.UpdatedAt = true, max(t.today.UnixMilli(), tx.UpdatedAt+1)
	return proposal(ActionDelete, domain.TypeTransaction, tx, label)
}

func (t *Tools) newGoal(name, value, deadline, saved string) (Proposal, error) {
	target, err := cents(value, "valor da meta")
	if err != nil {
		return Proposal{}, err
	}
	var savedCents int64
	if saved != "" && saved != "0" {
		if savedCents, err = cents(saved, "valor já guardado"); err != nil {
			return Proposal{}, err
		}
	}
	g := domain.Goal{Meta: t.meta(), Name: name, TargetCents: target, SavedCents: savedCents, Deadline: deadline, StartDate: t.today.Format(domain.DateLayout)}
	label := "Nova meta: " + name + " · " + money.Format(target)
	if deadline != "" {
		label += " até " + domain.ParseMonth(deadline).Label()
	}
	if savedCents > 0 {
		label += " · já guardado " + money.Format(savedCents)
	}
	return proposal(ActionCreate, domain.TypeGoal, g, label)
}

func (t *Tools) newDebt(name, creditor, balance, rate, minPay, due string) (Proposal, error) {
	bal, err := cents(balance, "saldo")
	if err != nil {
		return Proposal{}, err
	}
	// "12,9" (%) lido como reais dá 1290 = pontos-base.
	bp, err := money.Parse(strings.TrimSuffix(rate, "%"))
	if err != nil {
		return Proposal{}, fmt.Errorf("juros inválidos: informe o juros ao mês em %%, ex.: 12,9")
	}
	minCents, err := cents(minPay, "parcela mínima")
	if err != nil {
		return Proposal{}, err
	}
	day, _ := strconv.Atoi(due)
	d := domain.Debt{Meta: t.meta(), Name: name, Creditor: creditor, BalanceCents: bal, MonthlyRateBP: int(bp), MinPaymentCents: minCents, DueDay: day}
	label := fmt.Sprintf("Nova dívida: %s · deve %s · juros %s ao mês · mínimo %s", name, money.Format(bal), RatePct(int(bp)), money.Format(minCents))
	if day > 0 {
		label += fmt.Sprintf(" · vence dia %d", day)
	}
	return proposal(ActionCreate, domain.TypeDebt, d, label)
}

func (t *Tools) goal(name string) (domain.Goal, error) {
	goals := t.ledger.ActiveGoals()
	names := make([]string, len(goals))
	for i, g := range goals {
		if fold(g.Name) == fold(name) {
			return g, nil
		}
		names[i] = g.Name
	}
	if len(goals) == 0 {
		return domain.Goal{}, fmt.Errorf("o usuário não tem metas; crie uma com criar_meta")
	}
	return domain.Goal{}, fmt.Errorf("meta %q não existe. Metas: %s", name, strings.Join(names, ", "))
}

// contribute: com categoria de aportes, o aporte é um lançamento nela (é assim
// que a meta conta); sem categoria, soma no "já guardado" da meta.
func (t *Tools) contribute(goalName, value, date string) (Proposal, error) {
	g, err := t.goal(goalName)
	if err != nil {
		return Proposal{}, err
	}
	v, err := cents(value, "valor do aporte")
	if err != nil {
		return Proposal{}, err
	}
	if date == "" {
		date = t.today.Format(domain.DateLayout)
	}
	// A mesma regra do botão "Guardar" da tela Metas (finance.Contribution).
	typ, rec := finance.Contribution(g, v, date)
	if tx, ok := rec.(domain.Transaction); ok {
		tx.Meta = t.meta()
		return proposal(ActionCreate, typ, tx, "Aporte na meta "+g.Name+": "+t.txLabel(tx))
	}
	saved := rec.(domain.Goal)
	saved.UpdatedAt = max(t.today.UnixMilli(), g.UpdatedAt+1)
	return proposal(ActionEdit, domain.TypeGoal, saved, fmt.Sprintf("Aporte na meta %s: guardado %s → %s (de %s)",
		g.Name, money.Format(g.SavedCents), money.Format(saved.SavedCents), money.Format(g.TargetCents)))
}

// payDebt baixa o saldo da dívida e, se houver a categoria "Pagamento de
// dívidas", também lança o gasto (dois cartões; a pessoa confirma cada um).
func (t *Tools) payDebt(debtName, value, date string) (Proposal, []Proposal, error) {
	debts := t.ledger.ActiveDebts()
	var d domain.Debt
	names := make([]string, len(debts))
	for i, x := range debts {
		names[i] = x.Name
		if fold(x.Name) == fold(debtName) {
			d = x
		}
	}
	if d.ID == "" {
		if len(debts) == 0 {
			return Proposal{}, nil, fmt.Errorf("o usuário não tem dívidas cadastradas; cadastre com criar_divida")
		}
		return Proposal{}, nil, fmt.Errorf("dívida %q não existe. Dívidas: %s", debtName, strings.Join(names, ", "))
	}
	v, err := cents(value, "valor do pagamento")
	if err != nil {
		return Proposal{}, nil, err
	}
	paid := d
	paid.BalanceCents = max(0, d.BalanceCents-v)
	paid.UpdatedAt = max(t.today.UnixMilli(), d.UpdatedAt+1)
	label := fmt.Sprintf("Pagamento de %s em %s: saldo %s → %s", money.Format(v), d.Name, money.Format(d.BalanceCents), money.Format(paid.BalanceCents))
	debt, err := proposal(ActionEdit, domain.TypeDebt, paid, label)
	if err != nil {
		return Proposal{}, nil, err
	}
	catID, err := t.category("Pagamento de dívidas", domain.KindSaida)
	if err != nil {
		return debt, nil, nil // sem essa categoria, só baixa o saldo
	}
	if date == "" {
		date = t.today.Format(domain.DateLayout)
	}
	tx := domain.Transaction{Meta: t.meta(), Kind: domain.KindSaida, Description: "Pagamento: " + d.Name, AmountCents: v, CategoryID: catID, Date: date}
	spend, err := proposal(ActionCreate, domain.TypeTransaction, tx, "Novo gasto: "+t.txLabel(tx))
	if err != nil {
		return Proposal{}, nil, err
	}
	return debt, []Proposal{spend}, nil
}

// Applied diz se a proposta já está valendo nos dados (a pessoa confirmou):
// criado existe, apagado está apagado, alterado está como a proposta.
func Applied(l domain.Ledger, p Proposal) bool {
	var want map[string]any
	if json.Unmarshal(p.Record, &want) != nil {
		return false
	}
	id, _ := want["id"].(string)
	var cur any
	switch p.Type {
	case domain.TypeTransaction:
		cur = find(l.Transactions, id)
	case domain.TypeGoal:
		cur = find(l.Goals, id)
	case domain.TypeDebt:
		cur = find(l.Debts, id)
	}
	if cur == nil {
		return false
	}
	raw, _ := json.Marshal(cur)
	var have map[string]any
	json.Unmarshal(raw, &have)
	switch p.Action {
	case ActionCreate:
		return true
	case ActionDelete:
		return have["deleted"] == true
	}
	delete(want, "updatedAt")
	delete(have, "updatedAt")
	wantJSON, _ := json.Marshal(want)
	haveJSON, _ := json.Marshal(have)
	return string(wantJSON) == string(haveJSON)
}

// current devolve o registro atual que a proposta altera (JSON).
func (t *Tools) current(p Proposal) json.RawMessage {
	var m struct{ ID string }
	json.Unmarshal(p.Record, &m)
	var cur any
	switch p.Type {
	case domain.TypeTransaction:
		cur = find(t.ledger.Transactions, m.ID)
	case domain.TypeGoal:
		cur = find(t.ledger.Goals, m.ID)
	case domain.TypeDebt:
		cur = find(t.ledger.Debts, m.ID)
	}
	raw, _ := json.Marshal(cur)
	return raw
}

// apply põe a proposta nos dados das ferramentas (só nesta conversa).
func (t *Tools) apply(p Proposal) {
	switch p.Type {
	case domain.TypeTransaction:
		t.ledger.Transactions = upsert(t.ledger.Transactions, p.Record)
	case domain.TypeGoal:
		t.ledger.Goals = upsert(t.ledger.Goals, p.Record)
	case domain.TypeDebt:
		t.ledger.Debts = upsert(t.ledger.Debts, p.Record)
	}
}

func upsert[T any](list []T, raw json.RawMessage) []T {
	var rec T
	var m struct{ ID string }
	if json.Unmarshal(raw, &rec) != nil || json.Unmarshal(raw, &m) != nil {
		return list
	}
	for i, r := range list {
		if find([]T{r}, m.ID) != nil {
			list[i] = rec
			return list
		}
	}
	return append(list, rec)
}

// find acha o registro pelo ID (todo registro tem Meta, mas Go não deixa
// ler campo em genérico: compara pelo JSON).
func find[T any](list []T, id string) any {
	for _, r := range list {
		raw, _ := json.Marshal(r)
		var m struct{ ID string }
		if json.Unmarshal(raw, &m) == nil && m.ID == id {
			return r
		}
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ia-" + hex.EncodeToString(b)
}
