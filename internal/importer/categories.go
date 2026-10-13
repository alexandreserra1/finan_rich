package importer

import (
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Sugestão de categoria, nesta ordem:
//  1. o próprio arquivo (CSV exportado pelo finan_rich traz a categoria);
//  2. o histórico: como a pessoa classificou lançamentos parecidos antes
//     (aprende sozinho: corrigiu uma vez, na próxima importação já acerta);
//  3. palavras-chave de lojas e serviços conhecidos (iFood, Uber, Netflix...).
// O que sobrar fica sem categoria, para a pessoa escolher (ou pedir à IA).

type categoryMatcher struct {
	byName  map[string]domain.Category // nome normalizado → categoria ativa
	history map[string]map[string]int  // chave da descrição → categoria → vezes
	first   map[string]map[string]int  // primeira palavra → categoria → vezes
}

func newCategoryMatcher(l domain.Ledger) *categoryMatcher {
	m := &categoryMatcher{byName: map[string]domain.Category{}, history: map[string]map[string]int{}, first: map[string]map[string]int{}}
	active := map[string]domain.Category{}
	for _, c := range l.Categories {
		if !c.Deleted {
			m.byName[normalize(c.Name)] = c
			active[c.ID] = c
		}
	}
	count := func(idx map[string]map[string]int, key, cat string) {
		if key == "" {
			return
		}
		if idx[key] == nil {
			idx[key] = map[string]int{}
		}
		idx[key][cat]++
	}
	for _, t := range l.ActiveTransactions() {
		if _, ok := active[t.CategoryID]; !ok {
			continue
		}
		key := DescKey(t.Description)
		count(m.history, key, t.CategoryID)
		count(m.first, firstWord(key), t.CategoryID)
	}
	return m
}

func (m *categoryMatcher) suggest(r Row, kind string) (string, string) {
	fits := func(c domain.Category) bool { // entrada só em categoria de renda, saída nunca
		return (c.Bucket == domain.BucketRenda) == (kind == domain.KindEntrada)
	}
	if c, ok := m.byName[normalize(r.Category)]; ok && r.Category != "" && fits(c) {
		return c.ID, ReasonFile
	}
	key := DescKey(r.Description)
	if id := best(m.history[key], 1); id != "" {
		return id, ReasonHistory
	}
	if id := best(m.first[firstWord(key)], 2); id != "" {
		return id, ReasonHistory
	}
	text := " " + normalize(r.Description) + " "
	for _, k := range keywords {
		if k.kind != kind {
			continue
		}
		for _, w := range k.words {
			if strings.Contains(text, " "+w+" ") || strings.HasSuffix(w, "*") && strings.Contains(text, " "+strings.TrimSuffix(w, "*")) {
				if c, ok := m.byName[normalize(k.category)]; ok && fits(c) {
					return c.ID, ReasonKeyword
				}
			}
		}
	}
	return "", ""
}

// best devolve a categoria mais usada, se ela tiver pelo menos min usos e
// 70% das vezes (evita chutar quando o histórico está dividido).
func best(counts map[string]int, min int) string {
	total, top, id := 0, 0, ""
	for cat, n := range counts {
		total += n
		if n > top || n == top && cat < id {
			top, id = n, cat
		}
	}
	if top < min || top*10 < total*7 {
		return ""
	}
	return id
}

// DescKey é a "assinatura" de uma descrição para comparar com o histórico:
// sem números (datas, parcelas, códigos) e sem palavras genéricas do banco.
// "Compra no débito - IFOOD *RESTAURANTE 1234" → "ifood restaurante".
func DescKey(desc string) string {
	var words []string
	for _, w := range strings.Fields(normalize(desc)) {
		if stopwords[w] || len(w) < 2 || strings.IndexFunc(w, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
			continue
		}
		words = append(words, w)
		if len(words) == 4 {
			break
		}
	}
	return strings.Join(words, " ")
}

func firstWord(key string) string {
	w, _, _ := strings.Cut(key, " ")
	if len(w) < 4 {
		return ""
	}
	return w
}

var stopwords = map[string]bool{
	"compra": true, "no": true, "na": true, "em": true, "de": true, "do": true, "da": true, "dos": true, "das": true,
	"debito": true, "credito": true, "cartao": true, "pix": true, "enviado": true, "enviada": true, "recebido": true,
	"recebida": true, "transferencia": true, "pagamento": true, "pagto": true, "pag": true, "ted": true, "doc": true,
	"com": true, "para": true, "parcela": true, "parc": true, "visa": true, "master": true, "mastercard": true,
	"elo": true, "elec": true, "eletron": true, "a": true, "o": true, "e": true, "sp": true, "rj": true, "mg": true,
	"br": true, "bra": true, "ltda": true, "me": true, "sa": true, "eireli": true, "epp": true,
}

// keywords: lojas e serviços comuns → nome da categoria padrão. A ordem
// importa (o mais específico primeiro). Termina em "*" = começo de palavra.
var keywords = []struct {
	kind     string
	category string
	words    []string
}{
	{domain.KindEntrada, "Salário", []string{"salario", "folha", "proventos", "remuneracao", "vencimentos"}},
	{domain.KindEntrada, "Renda extra", []string{"rendimento*", "reembolso", "cashback", "estorno"}},
	{domain.KindSaida, "Assinaturas", []string{"netflix", "spotify", "disney*", "hbo*", "max", "prime video", "amazon prime", "amazonprime", "youtube*", "apple com", "icloud", "google one", "deezer", "globoplay", "paramount*", "crunchyroll", "chatgpt", "openai", "microsoft", "adobe", "canva", "duolingo"}},
	{domain.KindSaida, "Restaurantes e delivery", []string{"ifood", "ifd*", "rappi", "aiqfome", "ze delivery", "restaurante*", "lanchonete", "lanches", "pizza*", "burger*", "mcdonald*", "mc donalds", "bk brasil", "subway", "starbucks", "cafeteria", "cafe", "padaria", "churrasc*", "sushi", "bar", "boteco", "outback", "habibs", "spoleto", "giraffas"}},
	{domain.KindSaida, "Mercado", []string{"supermerc*", "mercado", "mercadinho", "atacad*", "assai", "carrefour", "pao de acucar", "extra", "hortifruti", "sacolao", "acougue", "bompreco", "zaffari", "savegnago", "sonda", "tenda", "makro", "sams club", "oxxo"}},
	{domain.KindSaida, "Transporte", []string{"uber*", "99", "99app", "99pop", "99 taxi", "cabify", "taxi", "posto*", "combustivel", "shell", "ipiranga", "petrobras", "br mania", "estacionamento", "estapar", "sem parar", "conectcar", "veloe", "metro", "onibus", "bilhete unico", "cptm", "pedagio", "detran", "ipva", "localiza", "movida", "unidas"}},
	{domain.KindSaida, "Contas da casa (luz, água, internet)", []string{"enel", "light", "cemig", "copel", "celesc", "coelba", "neoenergia", "energisa", "equatorial", "cpfl", "sabesp", "cedae", "sanepar", "copasa", "embasa", "comgas", "naturgy", "vivo", "claro", "tim", "oi", "net", "internet", "energia", "conta de luz", "agua", "gas"}},
	{domain.KindSaida, "Saúde", []string{"farmacia", "drogaria", "droga raia", "drogasil", "raia", "pague menos", "pacheco", "panvel", "unimed", "amil", "hapvida", "sulamerica", "bradesco saude", "notredame", "hospital", "clinica", "laboratorio", "dentista", "odonto*", "consulta", "academia", "smart fit", "smartfit", "bluefit"}},
	{domain.KindSaida, "Educação", []string{"escola", "colegio", "faculdade", "universidade", "curso*", "udemy", "alura", "coursera", "livraria", "mensalidade escolar"}},
	{domain.KindSaida, "Moradia", []string{"aluguel", "condominio", "iptu", "imobiliaria", "quintoandar", "quinto andar"}},
	{domain.KindSaida, "Pagamento de dívidas", []string{"emprestimo", "financiamento", "consignado", "acordo", "renegociacao"}},
	{domain.KindSaida, "Investimentos", []string{"aplicacao", "investimento*", "cdb", "tesouro direto", "corretora", "nuinvest", "xp investimentos", "btg", "clear corretora", "inter invest"}},
	{domain.KindSaida, "Compras", []string{"amazon*", "mercado livre", "mercadolivre", "mercadopago", "shopee", "magalu", "magazine luiza", "americanas", "casas bahia", "aliexpress", "shein", "renner", "riachuelo", "cea", "zara", "centauro", "netshoes", "kabum", "leroy merlin", "decathlon", "havan"}},
	{domain.KindSaida, "Lazer", []string{"cinema*", "cinemark", "ingresso*", "sympla", "eventim", "ticketmaster", "steam", "playstation", "xbox", "nintendo", "teatro", "parque", "show"}},
}
