package advisor

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// Anonimização para IA na nuvem: antes de sair do aparelho, lojas,
// estabelecimentos, dívidas, credores e metas viram apelidos ("Loja 3",
// "Dívida 1") e as notas ficam de fora. A IA vê valores, datas e categorias,
// que é o que ela precisa para aconselhar, mas não sabe onde a pessoa compra
// nem com quem tem dívida. Na volta, o app troca os apelidos pelos nomes.

// Anonymizer guarda os apelidos de uma conversa (só na memória do aparelho).
type Anonymizer struct {
	phrases []phrase                     // nomes como a pessoa escreve → apelido (para a pergunta)
	display map[string]string            // apelido → nome para mostrar
	records map[string]map[string]string // ID do registro → campo → texto original
	alias   map[string]map[string]string // ID do registro → campo → apelido usado
}

type phrase struct {
	text  []rune // já sem acento e minúsculo
	alias string
}

// Anonymize devolve uma cópia dos dados com apelidos no lugar dos nomes.
func Anonymize(l domain.Ledger) (domain.Ledger, *Anonymizer) {
	a := &Anonymizer{display: map[string]string{}, records: map[string]map[string]string{}, alias: map[string]map[string]string{}}
	out := domain.Ledger{Categories: l.Categories, Prefs: l.Prefs} // notas ficam de fora

	byKey := map[string]string{} // chave da descrição → apelido
	shop := func(id, desc, kind string) string {
		key := importer.DescKey(desc)
		if key == "" {
			return "Lançamento"
		}
		// O estabelecimento é a primeira palavra ("ifood pizza" e "ifood lanche"
		// são o mesmo iFood); palavras muito curtas não bastam sozinhas.
		show := desc
		if w, _, _ := strings.Cut(key, " "); len([]rune(w)) >= 3 {
			key = w
			// Na resposta, o nome do grupo é a palavra como a pessoa escreveu
			// ("iFood", não "iFood pizza"), quando dá para achá-la.
			if f := strings.Fields(desc); len(f) > 0 && importer.DescKey(f[0]) == w {
				show = f[0]
			}
		}
		al, ok := byKey[key]
		if !ok {
			prefix := "Loja"
			if kind == domain.KindEntrada {
				prefix = "Fonte"
			}
			al = fmt.Sprintf("%s %d", prefix, len(byKey)+1)
			byKey[key] = al
			a.display[al] = show
			a.addPhrase(key, al)
		}
		a.keep(id, "description", desc, al)
		return al
	}
	for _, t := range l.Transactions {
		t.Description = shop(t.ID, t.Description, t.Kind)
		out.Transactions = append(out.Transactions, t)
	}
	for _, r := range l.Recurring {
		r.Description = shop(r.ID, r.Description, r.Kind)
		out.Recurring = append(out.Recurring, r)
	}
	creditors := map[string]string{}
	for i, d := range l.Debts {
		al := fmt.Sprintf("Dívida %d", i+1)
		a.named(d.ID, "name", d.Name, al)
		d.Name = al
		if d.Creditor != "" {
			c, ok := creditors[fold(d.Creditor)]
			if !ok {
				c = fmt.Sprintf("Credor %d", len(creditors)+1)
				creditors[fold(d.Creditor)] = c
				a.named("", "", d.Creditor, c)
			}
			a.keep(d.ID, "creditor", d.Creditor, c)
			d.Creditor = c
		}
		out.Debts = append(out.Debts, d)
	}
	for i, g := range l.Goals {
		al := fmt.Sprintf("Meta %d", i+1)
		a.named(g.ID, "name", g.Name, al)
		g.Name = al
		out.Goals = append(out.Goals, g)
	}
	// Nomes mais longos primeiro: "cartão nubank" antes de "cartão".
	slices.SortStableFunc(a.phrases, func(x, y phrase) int { return len(y.text) - len(x.text) })
	return out, a
}

func (a *Anonymizer) named(id, field, name, alias string) {
	a.display[alias] = name
	a.addPhrase(name, alias)
	if id != "" {
		a.keep(id, field, name, alias)
	}
}

func (a *Anonymizer) keep(id, field, original, alias string) {
	if a.records[id] == nil {
		a.records[id], a.alias[id] = map[string]string{}, map[string]string{}
	}
	a.records[id][field], a.alias[id][field] = original, alias
}

func (a *Anonymizer) addPhrase(text, alias string) {
	if r := foldRunes(text); len(r) > 0 {
		a.phrases = append(a.phrases, phrase{r, alias})
	}
}

// foldRunes: minúsculas e sem acento, uma runa por runa (mantém as posições).
func foldRunes(s string) []rune {
	out := []rune(s)
	for i, r := range out {
		out[i] = []rune(fold(string(unicode.ToLower(r))) + " ")[0]
	}
	return out
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// Text troca, num texto da pessoa (pergunta, histórico), os nomes conhecidos
// pelos apelidos. Só troca palavras inteiras.
func (a *Anonymizer) Text(s string) string {
	src, low := []rune(s), foldRunes(s)
	var out []rune
	for i := 0; i < len(src); {
		matched := false
		if i == 0 || !isWordRune(low[i-1]) {
			for _, p := range a.phrases {
				end := i + len(p.text)
				if end <= len(low) && slices.Equal(low[i:end], p.text) && (end == len(low) || !isWordRune(low[end])) {
					out = append(out, []rune(p.alias)...)
					i, matched = end, true
					break
				}
			}
		}
		if !matched {
			out = append(out, src[i])
			i++
		}
	}
	return string(out)
}

// Reveal troca os apelidos de volta pelos nomes (resposta da IA).
func (a *Anonymizer) Reveal(s string) string {
	aliases := make([]string, 0, len(a.display))
	for al := range a.display {
		aliases = append(aliases, al)
	}
	// "Loja 12" antes de "Loja 1".
	slices.SortFunc(aliases, func(x, y string) int { return len(y) - len(x) })
	for _, al := range aliases {
		s = strings.ReplaceAll(s, al, a.display[al])
	}
	return s
}

// RevealProposal devolve a proposta com os nomes verdadeiros. Campos que a IA
// não mudou voltam exatamente como estavam no registro (pelo ID); o resto
// passa por Reveal. Assim um apelido nunca é gravado nos dados.
func (a *Anonymizer) RevealProposal(p Proposal) Proposal {
	var rec map[string]any
	if json.Unmarshal(p.Record, &rec) != nil {
		return p
	}
	id, _ := rec["id"].(string)
	for _, field := range []string{"description", "name", "creditor"} {
		v, ok := rec[field].(string)
		if !ok {
			continue
		}
		if a.alias[id][field] == v {
			rec[field] = a.records[id][field]
		} else {
			rec[field] = a.Reveal(v)
		}
	}
	p.Record, _ = json.Marshal(rec)
	p.Label = a.Reveal(p.Label)
	return p
}
