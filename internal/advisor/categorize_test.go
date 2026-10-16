package advisor

import (
	"context"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// fakeLLM devolve sempre a mesma resposta e guarda a pergunta.
type fakeLLM struct {
	answer string
	asked  []llm.Request
}

func (f *fakeLLM) Chat(_ context.Context, r llm.Request, _ func(string)) (llm.Reply, error) {
	f.asked = append(f.asked, r)
	return llm.Reply{Text: f.answer}, nil
}

func TestCategorize(t *testing.T) {
	cats := []domain.Category{
		{Meta: domain.Meta{ID: "sal"}, Name: "Salário", Bucket: domain.BucketRenda},
		{Meta: domain.Meta{ID: "mer"}, Name: "Mercado", Bucket: domain.BucketNecessidades},
		{Meta: domain.Meta{ID: "pet"}, Name: "Pet", Bucket: domain.BucketDesejos},
		{Meta: domain.Meta{ID: "old", Deleted: true}, Name: "Apagada", Bucket: domain.BucketDesejos},
	}
	// Resposta com texto em volta, acento diferente, categoria inventada e de outro tipo.
	f := &fakeLLM{answer: "Claro! ```json\n{\"1\": \"mercado\", \"2\": \"Petshop\", \"3\": \"SALARIO\", \"4\": \"Mercado\", \"5\": \"?\"}\n```"}
	items := []CategorizeItem{
		{"SUPERMERCADO BOA COMPRA", "saida"}, {"COBASI", "saida"}, {"EMPRESA X", "entrada"}, {"MERCADO?", "entrada"}, {"XYZ", "saida"},
	}
	got, err := Categorize(context.Background(), f, cats, items)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "mer,,sal,," {
		t.Errorf("categorias = %q", got)
	}
	prompt := f.asked[0].Messages[0].Content
	if !strings.Contains(prompt, "Categorias para SAÍDAS: Mercado; Pet") || strings.Contains(prompt, "Apagada") ||
		!strings.Contains(prompt, "3. [entrada] EMPRESA X") {
		t.Errorf("pergunta:\n%s", prompt)
	}

	many := make([]CategorizeItem, 95)
	for i := range many {
		many[i] = CategorizeItem{"x", "saida"}
	}
	f.asked = nil
	if got, _ := Categorize(context.Background(), f, cats, many); len(got) != 95 || len(f.asked) != 3 {
		t.Errorf("95 itens: %d respostas em %d perguntas", len(got), len(f.asked))
	}
}
