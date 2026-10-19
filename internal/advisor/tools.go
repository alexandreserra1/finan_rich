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
- propor_lancamento, alterar_lancamento, apagar_lancamento, criar_meta, criar_divida: use quando o usuário pedir para registrar, corrigir ou apagar algo. Para alterar ou apagar, busque antes para achar o id. Cada pedido vira um cartão com um botão para ele confirmar; nada é gravado sozinho. Nunca diga que já fez: diga que deixou pronto para ele confirmar.`

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
}

// Tools executa as ferramentas sobre os dados atuais.
type Tools struct {
	ledger    domain.Ledger
	today     time.Time
	onPropose func(Proposal)
}

func NewTools(l domain.Ledger, today time.Time, onPropose func(Proposal)) *Tools {
	return &Tools{ledger: l, today: today, onPropose: onPropose}
}

// maxListed limita quantos lançamentos a busca devolve (os totais contam todos).
const maxListed = 30

func tool(name, desc, params string) llm.Tool {
	return llm.Tool{Name: name, Description: desc, Parameters: json.RawMessage(params)}
}

func (t *Tools) Defs() []llm.Tool {
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
	}
}

// Run executa um pedido e devolve o texto que volta para a IA. Erros também
// voltam como texto, para ela corrigir o pedido.
func (t *Tools) Run(c llm.ToolCall) string {
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
		p   Proposal
		err error
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
	default:
		return "ferramenta desconhecida: " + c.Name
	}
	if err != nil {
		return "não deu para preparar: " + err.Error()
	}
	if t.onPropose != nil {
		t.onPropose(p)
	}
	return "Pronto na tela para o usuário confirmar (" + p.Label + "). Ainda NÃO foi feito."
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

// periodStart/periodEnd aceitam AAAA-MM-DD ou AAAA-MM (o mês inteiro).
func periodStart(s string) string {
	if len(s) == 7 {
		return s + "-01"
	}
	return s
}

func periodEnd(s string) string {
	if len(s) == 7 {
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
