package importer

import (
	"regexp"
	"strings"
)

// Notificações dos apps de banco no Android (lidas pelo app, no aparelho):
// "Compra de R$ 45,90 APROVADA em IFOOD", "Você recebeu um Pix de R$ 150,00 de
// MARIA". Viram lançamentos para revisar, nunca entram direto.

var (
	notifMoney = regexp.MustCompile(`R\$\s?(\d{1,3}(?:\.\d{3})*,\d{2})`)
	// Não é lançamento: compra negada, propaganda, saldo, fatura, código.
	notifSkip   = regexp.MustCompile(`negad|recusad|nao aprovad|cashback|aproveite|oferta|saldo e |saldo disponivel|fatura|codigo|limite`)
	notifIn     = regexp.MustCompile(`recebeu|recebid|deposito|estorno|reembolso`)
	notifOut    = regexp.MustCompile(`compra|enviou|enviad|pagamento|pago |debito|saque`)
	notifCutEnd = regexp.MustCompile(`(?i)\s+(em\s+\d|às\s|para o cart|no cart|com o cart|com final)`)
)

// ParseNotification lê o texto de uma notificação de banco. Devolve a
// descrição e o valor com sinal (a data é a da notificação, quem chama põe).
func ParseNotification(text string) (Row, bool) {
	norm := normalize(text) + " "
	if notifSkip.MatchString(norm) {
		return Row{}, false
	}
	in, out := notifIn.MatchString(norm), notifOut.MatchString(norm)
	m := notifMoney.FindStringSubmatchIndex(text)
	if m == nil || in == out {
		return Row{}, false
	}
	cents, ok := parseAmount(text[m[2]:m[3]])
	if !ok || cents <= 0 {
		return Row{}, false
	}
	if out {
		cents = -cents
	}
	// Quem recebeu ou de onde veio: o que vem depois do valor.
	tail, who, sep := text[m[1]:], "", ""
	for _, s := range []string{" em ", ": ", " para ", " de "} {
		if i := strings.Index(tail, s); i >= 0 {
			who, sep = tail[i+len(s):], strings.TrimSpace(s)
			break
		}
	}
	if i := notifCutEnd.FindStringIndex(who); i != nil {
		who = who[:i[0]]
	}
	who = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(who), "."))
	label := ""
	switch {
	case strings.Contains(norm, "pix"):
		label = "Pix"
	case strings.Contains(norm, "transferencia"):
		label = "Transferência"
	}
	switch {
	case who == "" && label == "":
		who = "Compra"
	case who == "":
		who = label
	case label != "" && (sep == "para" || sep == "de"):
		who = label + " " + sep + " " + who
	}
	return Row{Description: who, AmountCents: cents}, true
}
