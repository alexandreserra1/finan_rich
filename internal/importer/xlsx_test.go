package importer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

func xlsxFixture(t *testing.T, sheets []string, extra map[string]string) []byte {
	t.Helper()
	files := map[string]string{}
	var workbook, relationships strings.Builder
	workbook.WriteString(`<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><workbookPr date1904="false"/><sheets>`)
	relationships.WriteString(`<Relationships>`)
	for index, sheet := range sheets {
		fmt.Fprintf(&workbook, `<sheet name="Conta %d" sheetId="%d" r:id="rId%d"/>`, index+1, index+1, index+1)
		fmt.Fprintf(&relationships, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, index+1, index+1)
		files[fmt.Sprintf("xl/worksheets/sheet%d.xml", index+1)] = `<worksheet><sheetData>` + sheet + `</sheetData></worksheet>`
	}
	files["xl/workbook.xml"] = workbook.String() + `</sheets></workbook>`
	files["xl/_rels/workbook.xml.rels"] = relationships.String() + `</Relationships>`
	for name, content := range extra {
		files[name] = content
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func xlsxHeader() string {
	return `<row r="1"><c r="A1" t="inlineStr"><is><t>Data</t></is></c><c r="B1" t="inlineStr"><is><t>Descrição</t></is></c><c r="C1" t="inlineStr"><is><t>Valor</t></is></c></row>`
}

func xlsxData(date, amount string) string {
	return `<row r="2"><c r="A2" t="inlineStr"><is><t>` + date + `</t></is></c><c r="B2" t="inlineStr"><is><t>Mercado; queijo, pão</t></is></c><c r="C2"><v>` + amount + `</v></c></row>`
}

func TestXLSXImport(t *testing.T) {
	statement, err := Parse(xlsxFixture(t, []string{xlsxHeader() + xlsxData("16/11/2026", "-12.340")}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if statement.Format != "xlsx" || len(statement.Rows) != 1 || statement.Rows[0].AmountCents != -1234 || statement.Rows[0].Date != "2026-11-16" || statement.Rows[0].Description != "Mercado; queijo, pão" {
		t.Fatalf("extrato incorreto: %+v", statement)
	}
}

func TestXLSXFirstFinancialSheet(t *testing.T) {
	data := xlsxFixture(t, []string{`<row><c t="inlineStr"><is><t>Resumo</t></is></c><c t="b"><v>1</v></c></row>`, xlsxHeader() + xlsxData("16/11/2026", "-1e2"), xlsxHeader() + xlsxData("16/11/2026", "-999")}, nil)
	statement, err := Parse(data)
	if err != nil || len(statement.Rows) != 1 || statement.Rows[0].AmountCents != -10000 {
		t.Fatalf("extrato: %+v, erro: %v", statement, err)
	}
}

func TestXLSXSharedStringsDates(t *testing.T) {
	for _, date1904 := range []bool{false, true} {
		t.Run(fmt.Sprint(date1904), func(t *testing.T) {
			serial := "46342.5"
			if date1904 {
				serial = "44880.5"
			}
			rows := `<row><c t="s"><v>0</v></c><c t="s"><v>1</v></c><c t="s"><v>2</v></c></row><row><c s="1"><v>` + serial + `</v></c><c t="s"><v>3</v></c><c><v>-42.50</v></c></row>`
			extra := map[string]string{
				"xl/sharedStrings.xml": `<sst><si><t>Data</t></si><si><t>Descrição</t></si><si><t>Valor</t></si><si><r><t>Loja </t></r><r><t>local</t></r></si></sst>`,
				"xl/styles.xml":        `<styleSheet><cellXfs><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`,
			}
			if date1904 {
				extra["xl/workbook.xml"] = `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><workbookPr date1904="1"/><sheets><sheet r:id="rId1"/></sheets></workbook>`
			}
			statement, err := Parse(xlsxFixture(t, []string{rows}, extra))
			if err != nil || len(statement.Rows) != 1 || statement.Rows[0].Date != "2026-11-16" || statement.Rows[0].Description != "Loja local" || statement.Rows[0].AmountCents != -4250 {
				t.Fatalf("extrato: %+v, erro: %v", statement, err)
			}
		})
	}
}

func TestXLSXRejectUnsafeInput(t *testing.T) {
	valid := xlsxHeader() + xlsxData("16/11/2026", "-12.34")
	tests := []struct {
		name, sheet string
		extra       map[string]string
		message     string
	}{
		{"fórmula com cache", xlsxHeader() + strings.Replace(xlsxData("16/11/2026", "-12.34"), `<v>-12.34</v>`, `<f>1+1</f><v>-12.34</v>`, 1), nil, "fórmula"},
		{"índice compartilhado", xlsxHeader() + strings.Replace(xlsxData("16/11/2026", "-12.34"), `t="inlineStr"><is><t>16/11/2026</t></is>`, `t="s"><v>999</v>`, 1), nil, "texto"},
		{"XML inválido", valid, map[string]string{"xl/styles.xml": `<styleSheet>`}, "XML"},
		{"entidade", valid, map[string]string{"xl/sharedStrings.xml": `<!DOCTYPE sst [<!ENTITY a "b">]><sst/>`}, "XML"},
		{"macro", valid, map[string]string{"xl/vbaProject.bin": "macro"}, "macro"},
		{"referência externa", valid, map[string]string{"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="https://example.test/sheet.xml" TargetMode="External"/></Relationships>`}, "extern"},
		{"sem cabeçalho", `<row/>`, nil, "primeira"},
		{"coluna distante", strings.Replace(valid, `r="C2"`, `r="XFD2"`, 1), nil, "colunas"},
		{"linha distante", strings.Replace(valid, `r="2"`, `r="1048576"`, 1), nil, "linhas"},
		{"célula repetida", strings.Replace(valid, `r="C2"`, `r="B2"`, 1), nil, "célula"},
		{"dinheiro com fração de centavo", xlsxHeader() + xlsxData("16/11/2026", "-12.345"), nil, "valor"},
		{"dinheiro excedido", xlsxHeader() + xlsxData("16/11/2026", "92233720368547758.08"), nil, "valor"},
		{"serial sem estilo", strings.Replace(valid, `t="inlineStr"><is><t>16/11/2026</t></is>`, `t="n"><v>46342</v>`, 1), nil, "data"},
		{"expoente excessivo", xlsxHeader() + xlsxData("16/11/2026", "1e999999999"), nil, "valor"},
		{"fração não decimal", xlsxHeader() + xlsxData("16/11/2026", "1/2"), nil, "valor"},
		{"caminho relativo", valid, map[string]string{"../escape.xml": `<x/>`}, "ZIP"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(xlsxFixture(t, []string{test.sheet}, test.extra))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("erro: %v; esperado %q", err, test.message)
			}
		})
	}
}

func TestXLSXSizeLimits(t *testing.T) {
	for _, data := range [][]byte{
		append([]byte("PK\x03\x04"), make([]byte, 8*1024*1024)...),
		xlsxFixture(t, []string{xlsxHeader() + xlsxData("16/11/2026", "-1")}, map[string]string{"bomb.bin": strings.Repeat("x", 16*1024*1024)}),
	} {
		if _, err := Parse(data); err == nil {
			t.Fatal("arquivo excessivo aceito")
		}
	}
}

func TestXLSLegacy(t *testing.T) {
	_, err := Parse([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	if err == nil || !strings.Contains(err.Error(), "XLSX ou CSV") {
		t.Fatalf("erro: %v", err)
	}
}

func TestXLSXStructureLimits(t *testing.T) {
	valid := xlsxHeader() + xlsxData("16/11/2026", "-12.34")
	entries := make(map[string]string)
	for index := 0; index < 256; index++ {
		entries[fmt.Sprintf("extra%d.bin", index)] = "x"
	}
	for name, data := range map[string][]byte{
		"entradas":              xlsxFixture(t, []string{valid}, entries),
		"planilhas":             xlsxFixture(t, make([]string, 33), nil),
		"linhas":                xlsxFixture(t, []string{strings.Repeat(`<row/>`, 10001)}, nil),
		"colunas":               xlsxFixture(t, []string{`<row>` + strings.Repeat(`<c/>`, 129) + `</row>`}, nil),
		"profundidade":          xlsxFixture(t, []string{strings.Repeat(`<x>`, 33) + strings.Repeat(`</x>`, 33)}, nil),
		"texto":                 xlsxFixture(t, []string{valid}, map[string]string{"xl/sharedStrings.xml": `<sst><si><t>` + strings.Repeat("a", 65537) + `</t></si></sst>`}),
		"células":               xlsxFixture(t, []string{strings.Repeat(`<row>`+strings.Repeat(`<c/>`, 128)+`</row>`, 1563)}, nil),
		"textos compartilhados": xlsxFixture(t, []string{valid}, map[string]string{"xl/sharedStrings.xml": `<sst>` + strings.Repeat(`<si/>`, 100001) + `</sst>`}),
		"raiz dupla":            xlsxFixture(t, []string{valid}, map[string]string{"xl/styles.xml": `<styleSheet/><styleSheet/>`}),
		"ZIP truncado":          []byte("PK\x03\x04"),
	} {
		t.Run(name, func(t *testing.T) {
			if statement, err := Parse(data); err == nil || len(statement.Rows) != 0 {
				t.Fatalf("arquivo excessivo ou inválido aceito: %+v, %v", statement, err)
			}
		})
	}
}

func TestXLSXCustomDateStyle(t *testing.T) {
	for _, format := range []string{`dd/mm/yyyy`, `[$-416]dd\-mm\-yyyy`, `&quot;Data:&quot; dd/mm/yyyy`} {
		extra := map[string]string{"xl/styles.xml": `<styleSheet><numFmts><numFmt numFmtId="164" formatCode="` + format + `"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/></cellXfs></styleSheet>`}
		row := strings.Replace(xlsxData("16/11/2026", "-12.34"), `t="inlineStr"><is><t>16/11/2026</t></is>`, `s="1"><v>46342</v>`, 1)
		statement, err := Parse(xlsxFixture(t, []string{xlsxHeader() + row}, extra))
		if err != nil || statement.Rows[0].Date != "2026-11-16" {
			t.Fatalf("formato %q: %+v, %v", format, statement, err)
		}
	}
	for _, format := range []string{`0.00`, `&quot;dd/mm/yyyy&quot; 0.00`, `hh:mm:ss`, `[Red]0.00`} {
		styles, err := xlsxDateStyles([]byte(`<styleSheet><numFmts><numFmt numFmtId="164" formatCode="` + format + `"/></numFmts><cellXfs><xf numFmtId="164"/></cellXfs></styleSheet>`))
		if err != nil || styles[0] {
			t.Fatalf("formato não é data: %q: %v, %v", format, styles, err)
		}
	}
}

func TestXLSXCSVSemantics(t *testing.T) {
	header := `<row><c t="inlineStr"><is><t>Data</t></is></c><c t="inlineStr"><is><t>Descrição</t></is></c><c t="inlineStr"><is><t>Crédito</t></is></c><c t="inlineStr"><is><t>Débito</t></is></c><c t="inlineStr"><is><t>Categoria</t></is></c></row>`
	row := `<row><c t="d"><v>2026-11-16</v></c><c t="inlineStr"><is><t>Salário</t></is></c><c><v>1250.50</v></c><c/><c t="inlineStr"><is><t>Trabalho</t></is></c></row>`
	statement, err := Parse(xlsxFixture(t, []string{header + row}, nil))
	if err != nil || len(statement.Rows) != 1 || statement.Rows[0].AmountCents != 125050 || statement.Rows[0].Category != "Trabalho" || statement.Unsigned {
		t.Fatalf("extrato: %+v, %v", statement, err)
	}
	row = strings.Replace(row, `r="A2"`, `r="B2"`, 1)
	_, err = Parse(xlsxFixture(t, []string{xlsxHeader(), header + row}, nil))
	if err == nil || !strings.Contains(err.Error(), "primeira") {
		t.Fatalf("não deve pular a primeira aba financeira vazia: %v", err)
	}
}

func TestXLSXCSVDelimiter(t *testing.T) {
	for _, description := range []string{strings.Repeat(";", 100), strings.Repeat(",", 100), strings.Repeat("\t", 100)} {
		records := [][]string{{"Data", "Descrição", "Valor"}, {"2026-11-16", "Loja" + description + "local", "-12,34"}}
		statement, err := xlsxParseCSV(records)
		if err != nil || len(statement.Rows) != 1 || statement.Rows[0].AmountCents != -1234 {
			t.Fatalf("separador na descrição: %+v, %v", statement, err)
		}
	}
}

func TestXLSXDateAndNumberBounds(t *testing.T) {
	for _, serial := range []string{"60", "-1", "110001", "1e999999999", "NaN", "1/2"} {
		if date, err := xlsxDate(serial, false); err == nil {
			t.Errorf("serial inválido aceito: %s = %s", serial, date)
		}
	}
	for _, value := range []string{"1e999999999", "1e-999999999", "0x10", "NaN", "1/2", "1.001", strings.Repeat("1", 129)} {
		if amount, err := xlsxAmount(value); err == nil {
			t.Errorf("valor inválido aceito: %s = %s", value, amount)
		}
	}
	for value, expected := range map[string]string{"1.2300": "1,23", "1.2e2": "120,00", "-92233720368547758.07": "-92233720368547758,07"} {
		if amount, err := xlsxAmount(value); err != nil || amount != expected {
			t.Errorf("valor: %s = %s, %v", value, amount, err)
		}
	}
}

func TestXLSXRejectCorruptAndDuplicateZIP(t *testing.T) {
	data := xlsxFixture(t, []string{xlsxHeader() + xlsxData("16/11/2026", "-12.34")}, nil)
	central := bytes.Index(data, []byte("PK\x01\x02"))
	if central < 0 {
		t.Fatal("fixture ZIP sem diretório central")
	}
	data[central+16] ^= 1
	if _, err := Parse(data); err == nil {
		t.Fatal("ZIP com CRC incorreto aceito")
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for count := 0; count < 2; count++ {
		entry, err := writer.Create("xl/workbook.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(`<workbook/>`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(buffer.Bytes()); err == nil {
		t.Fatal("ZIP com entradas repetidas aceito")
	}
}

func TestParseAmountOverflow(t *testing.T) {
	for _, input := range []string{"92233720368547758.08", "-92233720368547758.08", "92233720368547759", "184467440737095516.16", "9223372036854775807"} {
		if value, ok := parseAmount(input); ok {
			t.Errorf("valor excedido aceito: %q = %d", input, value)
		}
	}
	for input, expected := range map[string]int64{"92233720368547758.07": math.MaxInt64, "-92233720368547758.07": -math.MaxInt64} {
		if value, ok := parseAmount(input); !ok || value != expected {
			t.Errorf("limite: %q = %d, %v", input, value, ok)
		}
	}
}

func TestXLSXExactMonetaryCells(t *testing.T) {
	for _, header := range []string{"Valor", "Crédito", "Débito"} {
		for _, cellType := range []string{"inlineStr", "s", "n"} {
			for _, input := range []struct {
				text    string
				numeric string
				valid   bool
			}{
				{"-45,901", "-45.901", false},
				{"-45,9099", "-45.9099", false},
				{"-45,90", "-45.90", true},
				{"-45,900", "-45.900", true},
				{"valor inválido", "inválido", false},
				{"92233720368547758,08", "92233720368547758.08", false},
			} {
				t.Run(header+"/"+cellType+"/"+input.text, func(t *testing.T) {
					columns := strings.Replace(xlsxHeader(), "Valor", header, 1)
					if header != "Valor" {
						other := "Débito"
						if header == "Débito" {
							other = "Crédito"
						}
						columns = strings.Replace(columns, `</row>`, `<c r="D1" t="inlineStr"><is><t>`+other+`</t></is></c></row>`, 1)
					}
					cell := `<c r="C2" t="n"><v>` + input.numeric + `</v></c>`
					extra := map[string]string{}
					switch cellType {
					case "inlineStr":
						cell = `<c r="C2" t="inlineStr"><is><t>` + input.text + `</t></is></c>`
					case "s":
						cell = `<c r="C2" t="s"><v>0</v></c>`
						extra["xl/sharedStrings.xml"] = `<sst><si><t>` + input.text + `</t></si></sst>`
					}
					row := strings.Replace(xlsxData("16/11/2026", "-1"), `<c r="C2"><v>-1</v></c>`, cell, 1)
					validRow := strings.ReplaceAll(xlsxData("16/11/2026", "-1"), `2"`, `3"`)
					statement, err := Parse(xlsxFixture(t, []string{columns + row + validRow}, extra))
					if input.valid {
						expected := int64(-4590)
						if header == "Crédito" {
							expected = 4590
						}
						if err != nil || len(statement.Rows) != 2 || statement.Rows[0].AmountCents != expected {
							t.Fatalf("centavos alterados: %+v, %v", statement, err)
						}
					} else if err == nil || len(statement.Rows) != 0 || !strings.Contains(err.Error(), "valor") {
						t.Fatalf("deve recusar o arquivo inteiro, sem omitir a linha: %+v, %v", statement, err)
					}
				})
			}
		}
	}
}

func TestXLSXDebitMarkersDoNotCancelEachOther(t *testing.T) {
	row := strings.Replace(xlsxData("16/11/2026", "-1"), `<c r="C2"><v>-1</v></c>`, `<c r="C2" t="inlineStr"><is><t>-45,90 D</t></is></c>`, 1)
	other := strings.ReplaceAll(xlsxData("16/11/2026", "-10.00"), `2"`, `3"`)
	statement, err := Parse(xlsxFixture(t, []string{xlsxHeader() + row + other}, nil))
	if err != nil || len(statement.Rows) != 2 || statement.Rows[0].AmountCents != -4590 || statement.Rows[1].AmountCents != -1000 {
		t.Fatalf("marcadores de débito inverteram o sentido: %+v, %v", statement, err)
	}
}
func TestXLSXRejectsInvalidTextFieldsWithoutPartialImport(t *testing.T) {
	for _, invalid := range []string{
		xlsxData("31/02/2026", "-20.00"),
		`<row r="2"><c r="A2" t="inlineStr"><is><t>16/11/2026</t></is></c><c r="B2" t="inlineStr"><is><t>Loja</t></is></c><c r="C2" t="inlineStr"><is><t>-45,901</t></is></c></row>`,
		`<row r="2"><c r="A2" t="inlineStr"><is><t>16/11/2026</t></is></c><c r="B2" t="inlineStr"><is><t>Loja</t></is></c><c r="C2" t="s"><v>0</v></c></row>`,
	} {
		invalid = strings.ReplaceAll(invalid, `r="2"`, `r="3"`)
		invalid = strings.ReplaceAll(invalid, `2"`, `3"`)
		data := xlsxFixture(t, []string{xlsxHeader() + xlsxData("16/11/2026", "-10.00") + invalid}, map[string]string{"xl/sharedStrings.xml": `<sst><si><t>-45,901</t></si></sst>`})
		statement, err := Parse(data)
		if err == nil || len(statement.Rows) != 0 {
			t.Fatalf("arquivo inválido aceito parcialmente: %+v, %v", statement, err)
		}
	}
}
