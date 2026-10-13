// Package domain define os dados do finan_rich: categorias, lançamentos,
// dívidas, preferências, notas, metas e lançamentos recorrentes. É Go puro (sem banco, sem HTTP), então roda tanto no
// servidor quanto no navegador (compilado para WebAssembly).
//
// Cada registro tem um ID universal (UUID) gerado por quem o cria, e a hora da
// última alteração. Assim, computador e celular podem criar e editar registros
// sem conexão, e a sincronização junta tudo depois: em conflito, vale a
// alteração mais recente. Apagar é marcar Deleted (para o apagamento também
// chegar aos outros aparelhos).
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Buckets da divisão do salário (estilo 50/30/20).
const (
	BucketNecessidades  = "necessidades"
	BucketDesejos       = "desejos"
	BucketInvestimentos = "investimentos"
	BucketRenda         = "renda"
)

// SpendingBuckets são os grupos que dividem a renda, na ordem de exibição.
var SpendingBuckets = []string{BucketNecessidades, BucketDesejos, BucketInvestimentos}

// DebtPaymentCategory é a categoria padrão para lançar pagamentos de dívidas.
const DebtPaymentCategory = "Pagamento de dívidas"

const (
	KindEntrada = "entrada"
	KindSaida   = "saida"
)

// DateLayout é o formato das datas nos registros: "2026-10-05".
const DateLayout = "2006-01-02"

// Meta são os campos de sincronização comuns a todos os registros.
type Meta struct {
	ID        string `json:"id"`        // UUID
	UpdatedAt int64  `json:"updatedAt"` // milissegundos desde 1970 (UTC)
	Deleted   bool   `json:"deleted,omitempty"`
}

type Category struct {
	Meta
	Name   string `json:"name"`
	Bucket string `json:"bucket"`
}

type Transaction struct {
	Meta
	Kind        string `json:"kind"`
	Description string `json:"description"`
	AmountCents int64  `json:"amountCents"`
	CategoryID  string `json:"categoryId"`
	Date        string `json:"date"` // AAAA-MM-DD
}

// Day devolve a data do lançamento (zero se inválida).
func (t Transaction) Day() time.Time {
	d, _ := time.Parse(DateLayout, t.Date)
	return d
}

type Debt struct {
	Meta
	Name            string `json:"name"`
	Creditor        string `json:"creditor"`
	BalanceCents    int64  `json:"balanceCents"`
	MonthlyRateBP   int    `json:"monthlyRateBp"` // juros ao mês em pontos-base: 199 = 1,99%
	MinPaymentCents int64  `json:"minPaymentCents"`
	DueDay          int    `json:"dueDay,omitempty"` // 0 = não informado
}

// Note é uma anotação livre do usuário (metas, combinados, lembretes...).
// Sincroniza como os outros registros e o consultor de IA também lê.
type Note struct {
	Meta
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned,omitempty"` // fixada no topo da lista
}

// Goal é uma meta de dinheiro: juntar TargetCents até Deadline (opcional).
// O progresso é SavedCents (o que já existia ao criar a meta, atualizável à
// mão) mais as saídas lançadas em CategoryID (ex.: "Reserva de emergência")
// a partir de StartDate.
type Goal struct {
	Meta
	Name        string `json:"name"`
	TargetCents int64  `json:"targetCents"`
	SavedCents  int64  `json:"savedCents"`
	Deadline    string `json:"deadline,omitempty"`   // "2026-12" (opcional)
	CategoryID  string `json:"categoryId,omitempty"` // aportes = saídas nesta categoria
	StartDate   string `json:"startDate"`            // AAAA-MM-DD: aportes contam a partir daqui
}

// Recurring é um lançamento que se repete todo mês (salário, aluguel,
// assinaturas). Cada aparelho gera os lançamentos do mês com o mesmo ID
// ("rec-<id>-<AAAA-MM>"), então não há duplicados entre aparelhos.
type Recurring struct {
	Meta
	Kind        string `json:"kind"`
	Description string `json:"description"`
	AmountCents int64  `json:"amountCents"`
	CategoryID  string `json:"categoryId"`
	Day         int    `json:"day"`           // dia do mês (31 vira o último dia em meses curtos)
	Start       string `json:"start"`         // primeiro mês: "2026-10"
	End         string `json:"end,omitempty"` // último mês (opcional)
	Paused      bool   `json:"paused,omitempty"`
}

// RecurringTxID é o ID do lançamento que a recorrência gera em um mês.
func RecurringTxID(recurringID string, m Month) string { return "rec-" + recurringID + "-" + m.Key() }

// Limites das notas (em caracteres).
const (
	NoteTitleMax = 120
	NoteBodyMax  = 20000
)

// Pref é uma preferência sincronizada entre aparelhos (ex.: "split").
type Pref struct {
	Meta
	Value json.RawMessage `json:"value"`
}

// PrefSplit é o ID da preferência com a divisão do salário.
const PrefSplit = "split"

// Split é o percentual planejado da renda para cada bucket de gasto.
type Split map[string]int

// DefaultSplit é a regra 50/30/20.
func DefaultSplit() Split {
	return Split{BucketNecessidades: 50, BucketDesejos: 30, BucketInvestimentos: 20}
}

// ---- Validação (a mesma no servidor e no navegador) ----

var (
	ErrNoID = errors.New("registro sem id")
)

func (m Meta) validate() error {
	if strings.TrimSpace(m.ID) == "" || len(m.ID) > 64 {
		return ErrNoID
	}
	if m.UpdatedAt <= 0 {
		return errors.New("registro sem data de atualização")
	}
	return nil
}

func validBucket(b string) bool {
	switch b {
	case BucketNecessidades, BucketDesejos, BucketInvestimentos, BucketRenda:
		return true
	}
	return false
}

func (c Category) Validate() error {
	if err := c.validate(); err != nil || c.Deleted {
		return err
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("informe o nome da categoria")
	}
	if !validBucket(c.Bucket) {
		return fmt.Errorf("grupo inválido: %q", c.Bucket)
	}
	return nil
}

func (t Transaction) Validate() error {
	if err := t.validate(); err != nil || t.Deleted {
		return err
	}
	if t.Kind != KindEntrada && t.Kind != KindSaida {
		return errors.New("tipo inválido")
	}
	if strings.TrimSpace(t.Description) == "" {
		return errors.New("informe uma descrição")
	}
	if t.AmountCents < 0 {
		return errors.New("valor não pode ser negativo")
	}
	if t.CategoryID == "" {
		return errors.New("escolha uma categoria")
	}
	if _, err := time.Parse(DateLayout, t.Date); err != nil {
		return errors.New("data inválida")
	}
	return nil
}

func (d Debt) Validate() error {
	if err := d.validate(); err != nil || d.Deleted {
		return err
	}
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("informe o nome da dívida")
	}
	if d.BalanceCents < 0 || d.MinPaymentCents < 0 || d.MonthlyRateBP < 0 {
		return errors.New("valores não podem ser negativos")
	}
	if d.DueDay < 0 || d.DueDay > 31 {
		return errors.New("dia de vencimento deve ser de 1 a 31")
	}
	return nil
}

func (p Pref) Validate() error {
	if err := p.validate(); err != nil || p.Deleted {
		return err
	}
	if p.ID == PrefSplit {
		var s Split
		if err := json.Unmarshal(p.Value, &s); err != nil {
			return errors.New("divisão do salário inválida")
		}
		return s.Validate()
	}
	return nil
}

func (n Note) Validate() error {
	if err := n.validate(); err != nil || n.Deleted {
		return err
	}
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Body) == "" {
		return errors.New("a nota está vazia")
	}
	if len([]rune(n.Title)) > NoteTitleMax {
		return fmt.Errorf("título muito longo (máximo %d caracteres)", NoteTitleMax)
	}
	if len([]rune(n.Body)) > NoteBodyMax {
		return fmt.Errorf("nota muito longa (máximo %d caracteres)", NoteBodyMax)
	}
	return nil
}

func validMonth(s string) bool {
	_, err := time.Parse("2006-01", s)
	return err == nil
}

func (g Goal) Validate() error {
	if err := g.validate(); err != nil || g.Deleted {
		return err
	}
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("dê um nome para a meta")
	}
	if g.TargetCents <= 0 {
		return errors.New("informe quanto quer juntar")
	}
	if g.SavedCents < 0 {
		return errors.New("o valor já guardado não pode ser negativo")
	}
	if g.Deadline != "" && !validMonth(g.Deadline) {
		return errors.New("prazo inválido")
	}
	if _, err := time.Parse(DateLayout, g.StartDate); err != nil {
		return errors.New("data de início inválida")
	}
	return nil
}

func (r Recurring) Validate() error {
	if err := r.validate(); err != nil || r.Deleted {
		return err
	}
	if r.Kind != KindEntrada && r.Kind != KindSaida {
		return errors.New("tipo inválido")
	}
	if strings.TrimSpace(r.Description) == "" {
		return errors.New("informe uma descrição")
	}
	if r.AmountCents <= 0 {
		return errors.New("informe o valor")
	}
	if r.CategoryID == "" {
		return errors.New("escolha uma categoria")
	}
	if r.Day < 1 || r.Day > 31 {
		return errors.New("dia deve ser de 1 a 31")
	}
	if !validMonth(r.Start) || r.End != "" && (!validMonth(r.End) || r.End < r.Start) {
		return errors.New("meses de início e fim inválidos")
	}
	if len(r.ID) > 40 {
		return errors.New("id da recorrência muito longo")
	}
	return nil
}

// Validate exige percentuais de 0 a 100 somando 100.
func (s Split) Validate() error {
	total := 0
	for _, b := range SpendingBuckets {
		n := s[b]
		if n < 0 || n > 100 {
			return errors.New("percentuais devem ser de 0 a 100")
		}
		total += n
	}
	if total != 100 {
		return fmt.Errorf("os percentuais precisam somar 100%% (somaram %d%%)", total)
	}
	return nil
}
