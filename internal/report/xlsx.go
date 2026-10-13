package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Planilha do Excel (.xlsx) escrita à mão: o formato é um zip com alguns
// arquivos XML. Sem bibliotecas, o arquivo do app continua pequeno.
// Cada Table vira uma aba, com cabeçalho fixo, filtro e números de verdade
// (valores somáveis, datas filtráveis).

// Estilos (índices de cellXfs em styles.xml).
const (
	styleDefault = iota
	styleHeader
	styleMoney
	styleDate
	styleNumber
	stylePercent
	styleWrap
	styleTotal
	styleTotalMoney
)

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<numFmts count="3">
<numFmt numFmtId="164" formatCode="&quot;R$&quot; #,##0.00;[Red]-&quot;R$&quot; #,##0.00"/>
<numFmt numFmtId="165" formatCode="dd/mm/yyyy"/>
<numFmt numFmtId="166" formatCode="0&quot;%&quot;"/>
</numFmts>
<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FFE7F3EE"/><bgColor indexed="64"/></patternFill></fill></fills>
<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border>
<border><left/><right/><top/><bottom style="thin"><color rgb="FF0F7A5C"/></bottom><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="9">
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
<xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/>
<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="1" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment wrapText="1" vertical="top"/></xf>
<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>
<xf numFmtId="164" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1" applyNumberFormat="1"/>
</cellXfs>
<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`

func writeXLSX(tables []Table) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name, content string) error {
		w, err := z.Create(name)
		if err == nil {
			_, err = w.Write([]byte(content))
		}
		return err
	}

	var types, sheets, rels, defined strings.Builder
	used := map[string]bool{}
	for i, t := range tables {
		n := i + 1
		name := sheetName(t.Name, used)
		fmt.Fprintf(&types, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		fmt.Fprintf(&sheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlEscape(name), n, n)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
		if len(t.Rows) > 0 {
			fmt.Fprintf(&defined, `<definedName name="_xlnm._FilterDatabase" localSheetId="%d" hidden="1">'%s'!$A$1:$%s$%d</definedName>`,
				i, xmlEscape(strings.ReplaceAll(name, "'", "''")), colName(len(t.Columns)-1), len(t.Rows)+1)
		}
		if err := add(fmt.Sprintf("xl/worksheets/sheet%d.xml", n), sheetXML(t)); err != nil {
			return nil, err
		}
	}
	stylesRel := len(tables) + 1
	files := []struct{ name, content string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
` + types.String() + `
</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>` + sheets.String() + `</sheets>` + wrapIf("definedNames", defined.String()) + `
</workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
` + rels.String() + fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, stylesRel) + `
</Relationships>`},
		{"xl/styles.xml", stylesXML},
	}
	for _, f := range files {
		if err := add(f.name, f.content); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sheetXML(t Table) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	// Cabeçalho sempre visível ao rolar.
	b.WriteString(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	for i, c := range t.Columns {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, max(c.Width, len([]rune(c.Title)))+2)
	}
	b.WriteString(`</cols><sheetData>`)

	b.WriteString(`<row r="1">`)
	for i, c := range t.Columns {
		writeStringCell(&b, cellRef(i, 1), c.Title, styleHeader)
	}
	b.WriteString(`</row>`)
	r := 2
	for _, row := range t.Rows {
		fmt.Fprintf(&b, `<row r="%d">`, r)
		for i, cell := range row {
			writeCell(&b, cellRef(i, r), t.Columns[i].Kind, cell, false)
		}
		b.WriteString(`</row>`)
		r++
	}
	if t.Total != nil && len(t.Rows) > 0 {
		fmt.Fprintf(&b, `<row r="%d">`, r)
		for i, cell := range t.Total {
			writeCell(&b, cellRef(i, r), t.Columns[i].Kind, cell, true)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData>`)
	if len(t.Rows) > 0 {
		fmt.Fprintf(&b, `<autoFilter ref="A1:%s%d"/>`, colName(len(t.Columns)-1), len(t.Rows)+1)
	}
	b.WriteString(`</worksheet>`)
	return b.String()
}

func writeCell(b *strings.Builder, ref string, k Kind, v Cell, total bool) {
	switch x := v.(type) {
	case int64:
		style := styleNumber
		if k == Money {
			style = styleMoney
			if total {
				style = styleTotalMoney
			}
			fmt.Fprintf(b, `<c r="%s" s="%d"><v>%s</v></c>`, ref, style, strconv.FormatFloat(float64(x)/100, 'f', 2, 64))
			return
		}
		fmt.Fprintf(b, `<c r="%s" s="%d"><v>%d</v></c>`, ref, style, x)
	case int:
		style := styleNumber
		if k == Percent {
			style = stylePercent
		}
		fmt.Fprintf(b, `<c r="%s" s="%d"><v>%d</v></c>`, ref, style, x)
	case string:
		if x == "" {
			return
		}
		if k == Date {
			if d, err := time.Parse("2006-01-02", x); err == nil {
				fmt.Fprintf(b, `<c r="%s" s="%d"><v>%d</v></c>`, ref, styleDate, excelDay(d))
				return
			}
		}
		style := styleDefault
		switch {
		case total:
			style = styleTotal
		case k == LongText:
			style = styleWrap
		}
		writeStringCell(b, ref, x, style)
	}
}

func writeStringCell(b *strings.Builder, ref, s string, style int) {
	fmt.Fprintf(b, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEscape(s))
}

// excelDay é o número de série da data no Excel (dias desde 30/12/1899).
func excelDay(d time.Time) int {
	return int(d.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// colName: 0 → A, 25 → Z, 26 → AA.
func colName(i int) string {
	name := ""
	for i++; i > 0; i = (i - 1) / 26 {
		name = string(rune('A'+(i-1)%26)) + name
	}
	return name
}

func cellRef(col, row int) string { return colName(col) + strconv.Itoa(row) }

// sheetName respeita as regras do Excel: até 31 caracteres, sem []:*?/\ e sem repetir.
func sheetName(name string, used map[string]bool) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '-'
		}
		return r
	}, name)
	if r := []rune(name); len(r) > 31 {
		name = string(r[:31])
	}
	if name == "" {
		name = "Planilha"
	}
	base := name
	for i := 2; used[strings.ToLower(name)]; i++ {
		suffix := " " + strconv.Itoa(i)
		r := []rune(base)
		if len(r)+len(suffix) > 31 {
			r = r[:31-len(suffix)]
		}
		name = string(r) + suffix
	}
	used[strings.ToLower(name)] = true
	return name
}

// xmlEscape escapa o texto e remove caracteres que o XML não aceita.
func xmlEscape(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r != 0xFFFE && r != 0xFFFF {
			return r
		}
		return -1
	}, s)
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func wrapIf(tag, inner string) string {
	if inner == "" {
		return ""
	}
	return "<" + tag + ">" + inner + "</" + tag + ">"
}
