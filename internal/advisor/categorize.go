package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// CategorizeItem é um lançamento de extrato sem categoria.
type CategorizeItem struct {
	Description string `json:"description"`
	Kind        string `json:"kind"` // entrada ou saida
}

// categorizeBatch: quantos lançamentos por pergunta. Lotes pequenos deixam
// modelos pequenos (como os que rodam no PC) acertarem mais.
const categorizeBatch = 40

const categorizeSystem = `Você classifica lançamentos de extrato bancário brasileiro nas categorias de orçamento do usuário. Responda somente com JSON, sem comentários.`

// Categorize pede à IA a categoria de cada lançamento. Devolve o ID da
// categoria para cada item, na mesma ordem ("" quando a IA não sabe ou
// responde algo que não é uma das categorias).
func Categorize(ctx context.Context, p llm.Provider, cats []domain.Category, items []CategorizeItem) ([]string, error) {
	out := make([]string, len(items))
	var income, spending []domain.Category
	for _, c := range cats {
		if c.Deleted {
			continue
		}
		if c.Bucket == domain.BucketRenda {
			income = append(income, c)
		} else {
			spending = append(spending, c)
		}
	}
	for start := 0; start < len(items); start += categorizeBatch {
		batch := items[start:min(start+categorizeBatch, len(items))]
		reply, err := p.Chat(ctx, llm.Request{System: categorizeSystem, Messages: []llm.Message{
			{Role: llm.RoleUser, Content: categorizePrompt(income, spending, batch)},
		}}, func(string) {})
		if err != nil {
			return out, err
		}
		names := parseCategorized(reply.Text)
		for i, it := range batch {
			list := spending
			if it.Kind == domain.KindEntrada {
				list = income
			}
			out[start+i] = matchCategory(list, names[strconv.Itoa(i+1)])
		}
	}
	return out, nil
}

func categorizePrompt(income, spending []domain.Category, items []CategorizeItem) string {
	var b strings.Builder
	names := func(cs []domain.Category) string {
		out := make([]string, len(cs))
		for i, c := range cs {
			out[i] = c.Name
		}
		return strings.Join(out, "; ")
	}
	fmt.Fprintf(&b, "Categorias para SAÍDAS: %s\nCategorias para ENTRADAS: %s\n\nLançamentos:\n", names(spending), names(income))
	for i, it := range items {
		kind := "saída"
		if it.Kind == domain.KindEntrada {
			kind = "entrada"
		}
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, kind, it.Description)
	}
	b.WriteString("\nPara cada lançamento, escolha UMA categoria da lista do seu tipo, escrita exatamente como está acima. " +
		"Se não der para saber, use \"?\". Responda só o JSON, no formato {\"1\": \"Mercado\", \"2\": \"?\"}.")
	return b.String()
}

// parseCategorized lê o JSON da resposta, mesmo cercado de texto ou de ```.
func parseCategorized(answer string) map[string]string {
	out := map[string]string{}
	i, j := strings.Index(answer, "{"), strings.LastIndex(answer, "}")
	if i < 0 || j <= i {
		return out
	}
	var raw map[string]any
	if json.Unmarshal([]byte(answer[i:j+1]), &raw) != nil {
		return out
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[strings.TrimSpace(strings.TrimSuffix(k, "."))] = s
		}
	}
	return out
}

// matchCategory aceita o nome com diferenças de maiúsculas e acentos.
func matchCategory(list []domain.Category, name string) string {
	key := fold(name)
	if key == "" || key == "?" {
		return ""
	}
	for _, c := range list {
		if fold(c.Name) == key {
			return c.ID
		}
	}
	return ""
}

func fold(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ç", "c", "Ã", "a", "Õ", "o", "Ê", "e", "Â", "a", "Ô", "o",
	).Replace(s)), " "))
}
