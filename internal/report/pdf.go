package report

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// PDF escrito à mão, sem bibliotecas: páginas A4, fontes padrão do PDF
// (Helvetica, que todo leitor tem) na codificação WinAnsi, que cobre os
// acentos do português. Desenha texto, linhas e retângulos, o suficiente para
// relatórios com tabelas e barras.

const (
	pageW   = 595.28 // A4 em pontos
	pageH   = 841.89
	marginX = 40.0
	marginY = 48.0
	usableW = pageW - 2*marginX
)

type rgb struct{ r, g, b float64 }

var (
	colorText   = rgb{0.08, 0.13, 0.11}
	colorMuted  = rgb{0.37, 0.42, 0.40}
	colorAccent = rgb{0.06, 0.48, 0.36}
	colorSoft   = rgb{0.91, 0.95, 0.93}
	colorZebra  = rgb{0.97, 0.98, 0.975}
	colorLine   = rgb{0.85, 0.88, 0.87}
	colorNeg    = rgb{0.72, 0.22, 0.16}
	colorTrack  = rgb{0.91, 0.93, 0.92}
	bucketColor = map[string]rgb{
		"Necessidades": {0.18, 0.44, 0.84}, "Desejos": {0.74, 0.47, 0.09}, "Investimentos": {0.06, 0.61, 0.43},
	}
)

type pdfDoc struct {
	title string // aparece no topo de cada página
	pages []*bytes.Buffer
	cur   *bytes.Buffer
	y     float64 // distância do topo da página até a próxima linha
}

func newPDF(title string) *pdfDoc {
	d := &pdfDoc{title: title}
	d.newPage()
	return d
}

func (d *pdfDoc) newPage() {
	d.cur = &bytes.Buffer{}
	d.pages = append(d.pages, d.cur)
	d.y = marginY
	if len(d.pages) > 1 {
		d.text(marginX, d.y, 8, false, colorMuted, "finan_rich · "+d.title)
		d.y += 18
	}
}

// ensure abre uma página nova se não couber mais h pontos nesta.
func (d *pdfDoc) ensure(h float64) bool {
	if d.y+h > pageH-marginY {
		d.newPage()
		return true
	}
	return false
}

// text escreve na posição (x, y medido do topo, na linha de base).
func (d *pdfDoc) text(x, y, size float64, bold bool, c rgb, s string) {
	font := "F1"
	if bold {
		font = "F2"
	}
	fmt.Fprintf(d.cur, "BT %s rg /%s %s Tf %s %s Td (%s) Tj ET\n", c, font, num(size), num(x), num(pageH-y), pdfString(s))
}

func (d *pdfDoc) rect(x, y, w, h float64, c rgb) {
	fmt.Fprintf(d.cur, "%s rg %s %s %s %s re f\n", c, num(x), num(pageH-y-h), num(w), num(h))
}

func (d *pdfDoc) line(x1, y1, x2, y2, width float64, c rgb) {
	fmt.Fprintf(d.cur, "%s RG %s w %s %s m %s %s l S\n", c, num(width), num(x1), num(pageH-y1), num(x2), num(pageH-y2))
}

func (c rgb) String() string { return num(c.r) + " " + num(c.g) + " " + num(c.b) }

// ---- blocos de layout ----

func (d *pdfDoc) heading(s string) {
	d.ensure(90) // título não fica sozinho no fim da página
	d.y += 14
	d.text(marginX, d.y, 13, true, colorAccent, s)
	d.y += 6
	d.line(marginX, d.y, marginX+usableW, d.y, 0.8, colorAccent)
	d.y += 14
}

// paragraph quebra o texto em linhas que cabem na largura.
func (d *pdfDoc) paragraph(s string, size float64, c rgb) {
	lh := size * 1.4
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		lines := wrap(para, size, false, usableW)
		if len(lines) == 0 {
			d.y += lh * 0.6
			continue
		}
		for _, l := range lines {
			d.ensure(lh)
			d.text(marginX, d.y+size, size, false, c, l)
			d.y += lh
		}
	}
}

// cards desenha uma linha de quadros com rótulo e valor (ex.: Entradas).
func (d *pdfDoc) cards(items [][2]string, valueColors []rgb) {
	const h = 50.0
	d.ensure(h + 12)
	gap := 10.0
	w := (usableW - gap*float64(len(items)-1)) / float64(len(items))
	for i, it := range items {
		x := marginX + float64(i)*(w+gap)
		d.rect(x, d.y, w, h, colorSoft)
		d.text(x+10, d.y+17, 8.5, false, colorMuted, it[0])
		d.text(x+10, d.y+38, 14, true, valueColors[i], fit(it[1], 14, true, w-20))
	}
	d.y += h + 12
}

// bars desenha uma barra por grupo: quanto do planejado foi gasto.
func (d *pdfDoc) bars(t Table) {
	for _, row := range t.Rows {
		name := row[0].(string)
		planned, spent := row[2].(int64), row[3].(int64)
		used := row[5].(int)
		d.ensure(30)
		d.text(marginX, d.y+9, 9.5, true, colorText, name)
		label := fmt.Sprintf("%s de %s (%d%%)", brl(spent), brl(planned), used)
		d.text(marginX+usableW-textWidth(label, 9, false), d.y+9, 9, false, colorMuted, label)
		d.y += 14
		d.rect(marginX, d.y, usableW, 7, colorTrack)
		frac := 0.0
		if planned > 0 {
			frac = float64(spent) / float64(planned)
		} else if spent > 0 {
			frac = 1
		}
		c := bucketColor[name]
		if frac > 1 {
			frac, c = 1, colorNeg
		}
		if frac > 0 {
			d.rect(marginX, d.y, usableW*frac, 7, c)
		}
		d.y += 16
	}
}

// table desenha a tabela, repetindo o cabeçalho a cada página.
func (d *pdfDoc) table(t Table) {
	if len(t.Rows) == 0 {
		d.paragraph("Nada neste período.", 9.5, colorMuted)
		return
	}
	const size, rowH = 8.5, 16.0
	widths := columnWidths(t, size)
	right := func(k Kind) bool { return k == Money || k == Number || k == Percent }
	drawRow := func(cells []string, bold bool, fill *rgb, colors []rgb) {
		if fill != nil {
			d.rect(marginX, d.y, usableW, rowH, *fill)
		}
		x := marginX
		for i, s := range cells {
			w := widths[i]
			s = fit(s, size, bold, w-8)
			c := colorText
			if colors != nil {
				c = colors[i]
			}
			tx := x + 4
			if right(t.Columns[i].Kind) {
				tx = x + w - 4 - textWidth(s, size, bold)
			}
			d.text(tx, d.y+11, size, bold, c, s)
			x += w
		}
		d.y += rowH
	}
	header := func() {
		titles := make([]string, len(t.Columns))
		for i, c := range t.Columns {
			titles[i] = c.Title
		}
		drawRow(titles, true, &colorSoft, nil)
	}
	d.ensure(rowH * 3)
	header()
	for n, row := range t.Rows {
		if d.ensure(rowH) {
			header()
		}
		cells, colors := pdfCells(t, row)
		var fill *rgb
		if n%2 == 1 {
			fill = &colorZebra
		}
		drawRow(cells, false, fill, colors)
	}
	if t.Total != nil {
		d.ensure(rowH)
		d.line(marginX, d.y, marginX+usableW, d.y, 0.6, colorLine)
		cells, colors := pdfCells(t, t.Total)
		drawRow(cells, true, nil, colors)
	}
	d.y += 6
}

// columnWidths dá às colunas de números a largura que o maior valor precisa
// (valores nunca são cortados) e divide o resto entre as de texto, na
// proporção de Column.Width, sem dar a nenhuma mais do que ela precisa.
func columnWidths(t Table, size float64) []float64 {
	const pad = 8.0
	n := len(t.Columns)
	need := make([]float64, n)
	measure := func(row []Cell, bold bool) {
		cells, _ := pdfCells(t, row)
		for i, s := range cells {
			need[i] = max(need[i], textWidth(s, size, bold)+pad)
		}
	}
	for _, row := range t.Rows {
		measure(row, false)
	}
	if t.Total != nil {
		measure(t.Total, true)
	}
	widths := make([]float64, n)
	isNum := func(k Kind) bool { return k == Money || k == Number || k == Percent }
	free := usableW
	for i, c := range t.Columns {
		if isNum(c.Kind) {
			// o título pode quebrar em "…", mas não fica menor que o valor
			widths[i] = min(max(need[i], textWidth(c.Title, size, true)*0.6+pad), usableW/3)
			free -= widths[i]
		}
	}
	if free < usableW*0.25 { // números demais: volta à divisão proporcional
		total := 0
		for _, c := range t.Columns {
			total += c.Width
		}
		for i, c := range t.Columns {
			widths[i] = usableW * float64(c.Width) / float64(total)
		}
		return widths
	}
	open := map[int]bool{}
	for i, c := range t.Columns {
		if !isNum(c.Kind) {
			open[i] = true
		}
	}
	for len(open) > 0 {
		weight := 0
		for i := range open {
			weight += t.Columns[i].Width
		}
		capped := false
		for i := range open {
			share := free * float64(t.Columns[i].Width) / float64(weight)
			if need[i] > 0 && need[i] < share {
				widths[i] = need[i]
				free -= need[i]
				delete(open, i)
				capped = true
			}
		}
		if !capped {
			for i := range open {
				widths[i] = free * float64(t.Columns[i].Width) / float64(weight)
			}
			break
		}
	}
	// Sobrou espaço (textos curtos): distribui entre todas as colunas.
	sum := 0.0
	for _, w := range widths {
		sum += w
	}
	if sum < usableW {
		for i := range widths {
			widths[i] *= usableW / sum
		}
	}
	return widths
}

// pdfCells formata os valores de uma linha (e pinta de vermelho os negativos).
func pdfCells(t Table, row []Cell) ([]string, []rgb) {
	cells := make([]string, len(row))
	colors := make([]rgb, len(row))
	for i, v := range row {
		colors[i] = colorText
		switch x := v.(type) {
		case int64:
			if t.Columns[i].Kind == Money {
				cells[i] = brl(x)
				if x < 0 {
					colors[i] = colorNeg
				}
			} else {
				cells[i] = strconv.FormatInt(x, 10)
			}
		case int:
			cells[i] = strconv.Itoa(x)
			if t.Columns[i].Kind == Percent {
				cells[i] += "%"
			}
		case string:
			cells[i] = x
			if t.Columns[i].Kind == Date && len(x) == 10 {
				cells[i] = x[8:10] + "/" + x[5:7] + "/" + x[:4]
			}
			if t.Columns[i].Kind == LongText {
				cells[i] = strings.Join(strings.Fields(x), " ")
			}
		}
	}
	return cells, colors
}

// bytes monta o arquivo PDF, com o número da página no rodapé.
func (d *pdfDoc) bytes() []byte {
	for i, p := range d.pages {
		d.cur = p
		label := fmt.Sprintf("%d de %d", i+1, len(d.pages))
		d.text(pageW-marginX-textWidth(label, 8, false), pageH-24, 8, false, colorMuted, label)
	}

	var out bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	n := len(d.pages)
	// 1: catálogo, 2: páginas, 3 e 4: fontes, depois página e conteúdo de cada uma.
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	kids := make([]string, n)
	for i := range d.pages {
		kids[i] = fmt.Sprintf("%d 0 R", 5+2*i)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", n, strings.Join(kids, " ")))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, p := range d.pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			num(pageW), num(pageH), 6+2*i))
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(p.Bytes())
		zw.Close()
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n<< /Length %d /Filter /FlateDecode >>\nstream\n", len(offsets), z.Len())
		out.Write(z.Bytes())
		out.WriteString("\nendstream\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return out.Bytes()
}

// ---- texto ----

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// winAnsi converte para a codificação das fontes padrão do PDF (Windows-1252).
// Emojis e símbolos fora dela somem; o resto vira "?".
func winAnsi(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			out = append(out, ' ', ' ', ' ', ' ')
		case r >= 0x20 && r < 0x7f, r >= 0xa0 && r <= 0xff:
			out = append(out, byte(r))
		case cp1252[r] != 0:
			out = append(out, cp1252[r])
		case r > 0xffff, r < 0x20, r >= 0xfe00 && r <= 0xfe0f, r == 0x200d:
			// emoji, controle e modificadores: sem representação
		default:
			out = append(out, '?')
		}
	}
	return out
}

var cp1252 = map[rune]byte{
	'€': 0x80, '‚': 0x82, '„': 0x84, '…': 0x85, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94,
	'•': 0x95, '–': 0x96, '—': 0x97, '™': 0x99, 'Œ': 0x8c, 'œ': 0x9c, 'Š': 0x8a, 'š': 0x9a,
}

func pdfString(s string) string {
	var b strings.Builder
	for _, c := range winAnsi(s) {
		switch c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			if c >= 0x80 {
				fmt.Fprintf(&b, "\\%03o", c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// Larguras da Helvetica (em milésimos do tamanho da fonte), caracteres 32 a 126.
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

var helveticaBold = [95]int{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
}

// accentBase devolve a letra sem acento (mesma largura na Helvetica).
func accentBase(c byte) byte {
	const (
		upper = "AAAAAAACEEEEIIIIDNOOOOOxOUUUUYPs"
		lower = "aaaaaaaceeeeiiiidnooooo/ouuuuypy"
	)
	switch {
	case c >= 0xc0 && c <= 0xdf:
		return upper[c-0xc0]
	case c >= 0xe0:
		return lower[c-0xe0]
	}
	return 'o'
}

func textWidth(s string, size float64, bold bool) float64 {
	table := &helvetica
	if bold {
		table = &helveticaBold
	}
	total := 0
	for _, c := range winAnsi(s) {
		if c >= 0x80 {
			c = accentBase(c)
		}
		if c >= 32 && c <= 126 {
			total += table[c-32]
		} else {
			total += 556
		}
	}
	return float64(total) * size / 1000
}

// fit corta o texto com "…" para caber na largura.
func fit(s string, size float64, bold bool, width float64) string {
	if textWidth(s, size, bold) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && textWidth(string(r)+"…", size, bold) > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// wrap quebra um parágrafo em linhas de até width pontos.
func wrap(s string, size float64, bold bool, width float64) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		for textWidth(word, size, bold) > width { // palavra maior que a linha
			cut := len(word)
			for cut > 1 && textWidth(word[:cut], size, bold) > width {
				_, n := utf8.DecodeLastRuneInString(word[:cut])
				cut -= n
			}
			if line != "" {
				lines, line = append(lines, line), ""
			}
			lines, word = append(lines, word[:cut]), word[cut:]
		}
		switch {
		case line == "":
			line = word
		case textWidth(line+" "+word, size, bold) <= width:
			line += " " + word
		default:
			lines, line = append(lines, line), word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
