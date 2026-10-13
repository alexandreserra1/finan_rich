package importer

import (
	"errors"
	"regexp"
	"strings"
)

// OFX (Open Financial Exchange): o formato de extrato que quase todo banco
// brasileiro exporta. A versão 1 é SGML (tags sem fechamento, uma por linha) e
// a 2 é XML; este leitor aceita as duas olhando só as tags que importam.

var (
	ofxField = regexp.MustCompile(`(?i)<(TRNTYPE|DTPOSTED|TRNAMT|FITID|NAME|MEMO|CHECKNUM|PAYEE)>([^<\r\n]*)`)
	ofxAcct  = regexp.MustCompile(`(?i)<ACCTID>([^<\r\n]*)`)
)

func parseOFX(text string) (Statement, error) {
	st := Statement{Format: "ofx", Card: strings.Contains(strings.ToUpper(text), "<CCSTMTRS>")}
	if m := ofxAcct.FindStringSubmatch(text); m != nil {
		st.Account = strings.TrimSpace(m[1])
	}
	// O lazy match do regexp consome o <STMTTRN> seguinte quando o OFX não
	// fecha as tags; por isso dividimos o texto pelas aberturas.
	parts := strings.Split(text, "<STMTTRN>")
	if len(parts) == 1 {
		parts = regexp.MustCompile(`(?i)<stmttrn>`).Split(text, -1)
	}
	for _, block := range parts[1:] {
		if i := strings.Index(strings.ToUpper(block), "</STMTTRN>"); i >= 0 {
			block = block[:i]
		}
		fields := map[string]string{}
		for _, m := range ofxField.FindAllStringSubmatch(block, -1) {
			fields[strings.ToUpper(m[1])] = strings.TrimSpace(unescapeSGML(m[2]))
		}
		date, ok := parseDate(fields["DTPOSTED"])
		if !ok {
			continue
		}
		amount, ok := parseAmount(fields["TRNAMT"])
		if !ok {
			continue
		}
		st.Rows = append(st.Rows, Row{Date: date, AmountCents: amount, FITID: fields["FITID"], Description: ofxDescription(fields)})
	}
	if len(st.Rows) == 0 {
		return st, errors.New("o arquivo OFX não tem lançamentos")
	}
	return st, nil
}

// ofxDescription junta NAME e MEMO sem repetir: cada banco usa um dos dois
// (ou os dois) para o estabelecimento e o detalhe.
func ofxDescription(f map[string]string) string {
	name, memo := f["NAME"], f["MEMO"]
	if name == "" {
		name = f["PAYEE"]
	}
	switch {
	case memo == "":
		return cleanDescription(name)
	case name == "", strings.Contains(normalize(memo), normalize(name)):
		return cleanDescription(memo)
	case strings.Contains(normalize(name), normalize(memo)):
		return cleanDescription(name)
	}
	return cleanDescription(name + " - " + memo)
}

func unescapeSGML(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'").Replace(s)
}
