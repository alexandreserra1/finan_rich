package advisor

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Ferramentas do consultor: a IA pode pedir ao app para buscar lançamentos
// (qualquer período, estabelecimento ou categoria) e para propor um
// lançamento. Proposta nunca é gravada aqui: ela vai para a tela com um botão
// "Adicionar", e só entra se a pessoa aprovar.

// toolsPrompt complementa as instruções quando as ferramentas estão ligadas.
const toolsPrompt = `

Ferramentas:
- buscar_lancamentos: use quando a pergunta precisar de lançamentos que não estão no bloco de dados (outro mês, um período, um estabelecimento, uma categoria). Responda com os totais que ela devolver.
- propor_lancamento: use quando o usuário pedir para lançar/registrar um gasto ou entrada. A proposta aparece para ele com um botão "Adicionar"; ela NÃO é gravada sozinha. Nunca diga que já lançou: diga que deixou pronto para ele confirmar.`

// Proposal é um lançamento sugerido pela IA, esperando a aprovação da pessoa.
type Proposal struct {
	domain.Transaction
	CategoryName string `json:"categoryName"`
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

func (t *Tools) Defs() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "buscar_lancamentos",
			Description: "Busca lançamentos do usuário e devolve os totais e os mais recentes. Todos os filtros são opcionais.",
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"texto":{"type":"string","description":"parte da descrição, ex.: ifood, uber, aluguel"},
				"categoria":{"type":"string","description":"nome da categoria, ex.: Mercado"},
				"tipo":{"type":"string","enum":["entrada","saida"]},
				"de":{"type":"string","description":"início: AAAA-MM-DD ou AAAA-MM"},
				"ate":{"type":"string","description":"fim: AAAA-MM-DD ou AAAA-MM"}}}`),
		},
		{
			Name:        "propor_lancamento",
			Description: "Prepara um lançamento para o usuário aprovar com um botão. Não grava nada sozinho.",
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"descricao":{"type":"string"},
				"valor":{"type":"string","description":"valor em reais, positivo, ex.: 52,30"},
				"tipo":{"type":"string","enum":["saida","entrada"],"description":"saida (gasto, padrão) ou entrada (renda)"},
				"categoria":{"type":"string","description":"nome de uma categoria existente"},
				"data":{"type":"string","description":"AAAA-MM-DD; padrão: hoje"}},
				"required":["descricao","valor","categoria"]}`),
		},
	}
}

// Run executa um pedido e devolve o texto que volta para a IA. Erros também
// voltam como texto, para ela corrigir o pedido.
func (t *Tools) Run(c llm.ToolCall) string {
	switch c.Name {
	case "buscar_lancamentos":
		return t.search(c.Args)
	case "propor_lancamento":
		return t.propose(c.Args)
	}
	return "ferramenta desconhecida: " + c.Name
}

type searchArgs struct {
	Texto, Categoria, Tipo, De, Ate string
}

func (t *Tools) search(raw json.RawMessage) string {
	var a searchArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return "argumentos inválidos: " + err.Error()
	}
	from, to := periodStart(a.De), periodEnd(a.Ate)
	cats := t.ledger.CategoryIndex()
	text, cat := fold(a.Texto), fold(a.Categoria)
	var found []domain.Transaction
	var out, in int64
	for _, tx := range t.ledger.ActiveTransactions() {
		if (from != "" && tx.Date < from) || (to != "" && tx.Date > to) ||
			(a.Tipo != "" && tx.Kind != a.Tipo) ||
			(text != "" && !strings.Contains(fold(tx.Description), text)) ||
			(cat != "" && fold(cats[tx.CategoryID].Name) != cat) {
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
		sign := "-"
		if tx.Kind == domain.KindEntrada {
			sign = "+"
		}
		fmt.Fprintf(&b, "- %s · %s · %s · %s%s\n", tx.Date, tx.Description, cats[tx.CategoryID].Name, sign, money.Format(tx.AmountCents))
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

type proposeArgs struct {
	Descricao, Tipo, Categoria, Data string
	Valor                            json.RawMessage // aceita "52,30" ou 52.3
}

func (t *Tools) propose(raw json.RawMessage) string {
	var a proposeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return "argumentos inválidos: " + err.Error()
	}
	kind := domain.KindSaida
	if a.Tipo == domain.KindEntrada {
		kind = domain.KindEntrada
	}
	cents, err := money.Parse(strings.Trim(string(a.Valor), `"`))
	if err != nil || cents <= 0 {
		return "valor inválido: informe um valor positivo em reais, ex.: 52,30"
	}
	date := a.Data
	if date == "" {
		date = t.today.Format(domain.DateLayout)
	}
	// Só categorias do tipo certo: entrada vai para renda, saída nunca.
	var fits []domain.Category
	for _, c := range t.ledger.Categories {
		if !c.Deleted && (c.Bucket == domain.BucketRenda) == (kind == domain.KindEntrada) {
			fits = append(fits, c)
		}
	}
	catID := matchCategory(fits, a.Categoria)
	if catID == "" {
		names := make([]string, len(fits))
		for i, c := range fits {
			names[i] = c.Name
		}
		return fmt.Sprintf("categoria %q não existe para %s. Use uma destas: %s", a.Categoria, kind, strings.Join(names, ", "))
	}
	p := Proposal{Transaction: domain.Transaction{
		Meta: domain.Meta{ID: newID(), UpdatedAt: t.today.UnixMilli()},
		Kind: kind, Description: strings.TrimSpace(a.Descricao), AmountCents: cents, CategoryID: catID, Date: date,
	}}
	if err := p.Validate(); err != nil {
		return "não deu para preparar o lançamento: " + err.Error()
	}
	p.CategoryName = t.ledger.CategoryIndex()[catID].Name
	t.onPropose(p)
	return fmt.Sprintf("Proposta pronta na tela para o usuário aprovar: %s, %s, %s, %s. Ainda NÃO foi lançada.",
		p.Description, money.Format(cents), p.CategoryName, date)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ia-" + hex.EncodeToString(b)
}
