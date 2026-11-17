package importer

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const documentTextLimit = 1 << 20
const documentRowLimit = 10000

var (
	documentDate         = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}|\d{1,2}/\d{1,2}/\d{4})(?:\s|$)`)
	documentReceiptDate  = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}|\d{1,2}/\d{1,2}/\d{4})(?:\s+(?:às\s+)?[0-2][0-9]:[0-5][0-9](?::[0-5][0-9])?)?$`)
	documentDateLike     = regexp.MustCompile(`^\d{1,4}[/.-]\d{1,2}`)
	documentAmount       = regexp.MustCompile(`^(?:\d{1,3}(?:\.\d{3})+|\d+),\d{2}$`)
	documentMoney        = regexp.MustCompile(`(?:\d{1,3}(?:\.\d{3})+|\d+),\d{2}`)
	documentTransaction  = regexp.MustCompile(`(?i)^(.+?)\s+([+-]?)\s*(?:R\$\s*)?([+-]?)\s*((?:\d{1,3}(?:\.\d{3})+|\d+),\d{2})\s*([+DC-]?)$`)
	documentInvoice      = regexp.MustCompile(`\b(fatura|cartao de credito)\b`)
	documentStatement    = regexp.MustCompile(`\b(extrato|conta corrente)\b`)
	documentReceipt      = regexp.MustCompile(`\b(comprovante|pix|transferencia|recibo)\b`)
	documentReceiptTitle = regexp.MustCompile(`\b(comprovante|recibo)\b`)
	documentOutgoing     = regexp.MustCompile(`\b(enviado|enviada|pagamento)\b`)
	documentIncoming     = regexp.MustCompile(`\b(recebido|recebida)\b`)
	documentRejected     = regexp.MustCompile(`\b(agendad[oa]|agendamento|pendente|cancelad[oa]|cancelamento|nao (efetivad[oa]|realizad[oa]|concluid[oa]|enviad[oa]|autorizad[oa]|efetuad[oa]|pag[oa])|em processamento|em analise|falhou|recusad[oa])\b`)
)

func ParseDocument(text string, today time.Time) (Statement, error) {
	return ParseDocumentKind(text, today, "auto")
}

func ParseDocumentKind(text string, today time.Time, kind string) (Statement, error) {
	if len(text) > documentTextLimit {
		return Statement{}, errors.New("o texto ultrapassa 1 MB; divida o documento e importe uma parte por vez")
	}
	if kind != "auto" && kind != "fatura" && kind != "extrato" && kind != "comprovante" {
		return Statement{}, errors.New("tipo de documento inválido; escolha auto, fatura, extrato ou comprovante")
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\u00a0", " ")
	if kind == "auto" {
		var context strings.Builder
		for _, line := range strings.Split(text, "\n") {
			if documentDateLike.MatchString(strings.TrimSpace(line)) {
				continue
			}
			label, _, _ := strings.Cut(line, ":")
			context.WriteString(normalize(label))
			context.WriteByte(' ')
		}
		normalized := context.String()
		if documentInvoice.MatchString(normalized) && documentStatement.MatchString(normalized) {
			return Statement{}, errors.New("o documento mistura indícios de fatura e extrato; confira o original e escolha o tipo manualmente")
		}
		switch {
		case documentReceiptTitle.MatchString(normalized):
			kind = "comprovante"
		case documentInvoice.MatchString(normalized):
			kind = "fatura"
		case documentStatement.MatchString(normalized):
			kind = "extrato"
		case documentReceipt.MatchString(normalized):
			kind = "comprovante"
		default:
			return Statement{}, errors.New("não reconheci o documento com segurança; escolha o tipo ou exporte OFX ou CSV no banco")
		}
	}
	var statement Statement
	var err error
	switch kind {
	case "fatura":
		statement, err = ParseText(text, today)
	case "extrato":
		statement, err = parseDocumentStatement(text)
	case "comprovante":
		statement, err = parseDocumentReceipt(text)
	}
	if err != nil {
		return Statement{}, err
	}
	if len(statement.Rows) > documentRowLimit {
		return Statement{}, errors.New("o documento ultrapassa 10 mil lançamentos; divida o período e importe novamente")
	}
	return statement, nil
}

func parseDocumentStatement(text string) (Statement, error) {
	statement := Statement{Format: "pdf"}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		dateMatch := documentDate.FindStringSubmatch(line)
		if dateMatch == nil {
			if documentDateLike.MatchString(line) {
				return Statement{}, errors.New("há uma data incompleta no extrato; use datas com ano ou exporte OFX ou CSV")
			}
			continue
		}
		rest := strings.TrimSpace(line[len(dateMatch[1]):])
		if len(documentMoney.FindAllStringIndex(rest, 2)) > 1 {
			return Statement{}, errors.New("há mais de um valor na linha do extrato e não identifiquei as colunas de movimento e saldo; exporte OFX ou CSV no banco")
		}
		date, validDate := parseDate(dateMatch[1])
		match := documentTransaction.FindStringSubmatch(rest)
		if !validDate || match == nil {
			return Statement{}, errors.New("há um lançamento ilegível no extrato; confira data, descrição e valor ou exporte OFX ou CSV")
		}
		description := cleanDescription(match[1])
		if documentSummary(description) {
			continue
		}
		sign := strings.ToUpper(match[2] + match[3] + match[5])
		amount, validAmount := documentCents(match[4])
		if !validAmount || len(sign) != 1 || strings.IndexFunc(description, unicode.IsLetter) < 0 {
			return Statement{}, errors.New("o extrato precisa de descrição e valor com sinal +, -, D ou C; confira o original ou exporte OFX ou CSV")
		}
		if sign == "-" || sign == "D" {
			amount = -amount
		}
		statement.Rows = append(statement.Rows, Row{Date: date, Description: description, AmountCents: amount})
		if len(statement.Rows) > documentRowLimit {
			return Statement{}, errors.New("o documento ultrapassa 10 mil lançamentos; divida o período e importe novamente")
		}
	}
	if len(statement.Rows) == 0 {
		return Statement{}, errors.New("não encontrei lançamentos seguros no extrato; use datas completas, descrições e valores com sinal ou exporte OFX ou CSV")
	}
	return statement, nil
}

func documentSummary(description string) bool {
	switch strings.TrimSpace(strings.TrimSuffix(normalize(description), ":")) {
	case "saldo", "saldo anterior", "saldo inicial", "saldo final", "saldo disponivel", "total", "subtotal", "total de entradas", "total de saidas", "valor total", "limite", "limite disponivel", "limite de credito":
		return true
	}
	return false
}

func documentCents(value string) (int64, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "R$"))
	if !documentAmount.MatchString(value) {
		return 0, false
	}
	cents, err := strconv.ParseInt(strings.NewReplacer(".", "", ",", "").Replace(value), 10, 64)
	return cents, err == nil && cents > 0
}

func parseDocumentReceipt(text string) (Statement, error) {
	normalized := normalize(text)
	if documentRejected.MatchString(normalized) {
		return Statement{}, errors.New("o comprovante indica operação não concluída; use o comprovante efetivado ou o extrato do banco")
	}
	var direction strings.Builder
	skipValue := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		field, value := documentReceiptLabel(line, true)
		if field == "" {
			field, value = documentReceiptLabel(line, false)
		}
		if field != "" {
			skipValue = value == ""
			continue
		}
		if skipValue {
			skipValue = false
			continue
		}
		direction.WriteString(normalize(line))
		direction.WriteByte(' ')
	}
	incoming, outgoing := documentIncoming.MatchString(direction.String()), documentOutgoing.MatchString(direction.String())
	if incoming == outgoing {
		return Statement{}, errors.New("não identifiquei se o dinheiro entrou ou saiu; use um comprovante com envio, recebimento ou pagamento explícito")
	}
	var date, description, pending string
	var amount int64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		field, value := documentReceiptLabel(line, incoming)
		if field != "" {
			if strings.TrimSpace(value) == "" {
				pending = field
				continue
			}
			pending = ""
		} else if pending != "" {
			field, value, pending = pending, line, ""
		} else {
			continue
		}
		value = strings.TrimSpace(value)
		switch field {
		case "date":
			match := documentReceiptDate.FindStringSubmatch(value)
			if match == nil {
				return Statement{}, errors.New("a data do comprovante está incompleta; use dia, mês e ano legíveis no original")
			}
			parsed, valid := parseDate(match[1])
			if !valid || date != "" && date != parsed {
				return Statement{}, errors.New("a data do comprovante é inválida ou ambígua; confira o original")
			}
			date = parsed
		case "amount":
			parsed, valid := documentCents(value)
			if !valid || amount != 0 && amount != parsed {
				return Statement{}, errors.New("o valor do comprovante é ilegível ou ambíguo; confira o total no original")
			}
			amount = parsed
		case "description":
			parsed := cleanDescription(value)
			if strings.Contains(value, ":") || strings.IndexFunc(value, unicode.IsLetter) < 0 || description != "" && description != parsed {
				return Statement{}, errors.New("há mais de uma pessoa ou estabelecimento no comprovante; use um comprovante por operação")
			}
			description = parsed
		}
	}
	if date == "" || amount == 0 || description == "" || pending != "" {
		return Statement{}, errors.New("faltam data completa, total ou pessoa/estabelecimento no comprovante; confira o original ou exporte OFX ou CSV")
	}
	if !incoming {
		amount = -amount
	}
	return Statement{Format: "pdf", Rows: []Row{{Date: date, Description: description, AmountCents: amount}}}, nil
}

func documentReceiptLabel(line string, incoming bool) (string, string) {
	label, value, separated := strings.Cut(line, ":")
	if field := documentReceiptField(normalize(label), incoming); field != "" {
		return field, strings.TrimSpace(value)
	}
	if !separated {
		words := strings.Fields(line)
		for split := min(4, len(words)-1); split > 0; split-- {
			if field := documentReceiptField(normalize(strings.Join(words[:split], " ")), incoming); field != "" {
				return field, strings.Join(words[split:], " ")
			}
		}
	}
	return "", ""
}

func documentReceiptField(label string, incoming bool) string {
	switch label {
	case "data", "data da transacao", "data da transferencia", "data do pagamento", "data e hora", "data hora":
		return "date"
	case "valor", "valor total", "total", "total pago", "valor pago", "valor da transacao", "valor da transferencia", "valor do pix", "valor do pagamento":
		return "amount"
	case "estabelecimento":
		return "description"
	case "destinatario", "favorecido", "beneficiario", "para", "recebedor":
		if !incoming {
			return "description"
		}
	case "remetente", "pagador", "de", "origem":
		if incoming {
			return "description"
		}
	}
	return ""
}
