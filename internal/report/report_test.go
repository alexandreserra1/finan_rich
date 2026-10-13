package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

var now = time.Date(2026, 10, 13, 15, 30, 0, 0, time.UTC)

func sampleLedger() domain.Ledger {
	meta := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1791900000000} }
	tx := func(id, kind, cat, desc string, cents int64, date string) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date}
	}
	return domain.Ledger{
		Categories: []domain.Category{
			{Meta: meta("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: meta("mor"), Name: "Moradia", Bucket: domain.BucketNecessidades},
			{Meta: meta("laz"), Name: "Lazer & cia <bar>", Bucket: domain.BucketDesejos},
		},
		Transactions: []domain.Transaction{
			tx("t1", domain.KindEntrada, "sal", "Salário", 500000, "2026-10-05"),
			tx("t2", domain.KindSaida, "mor", "Aluguel", 140000, "2026-10-06"),
			tx("t3", domain.KindSaida, "laz", "Cinema \"estreia\"; pipoca", 8750, "2026-10-07"),
			tx("t4", domain.KindSaida, "mor", "Aluguel", 140000, "2026-09-06"),
			{Meta: domain.Meta{ID: "apagado", UpdatedAt: 5, Deleted: true}, Kind: domain.KindSaida, Description: "não exportar", AmountCents: 1, CategoryID: "laz", Date: "2026-10-01"},
		},
		Debts: []domain.Debt{{Meta: meta("d1"), Name: "Cartão", Creditor: "Banco", BalanceCents: 400000, MonthlyRateBP: 1250, MinPaymentCents: 60000, DueDay: 10}},
		Notes: []domain.Note{{Meta: meta("n1"), Title: "Meta de 2027!", Body: "Juntar R$ 5.000 para a reserva.\nCortar delivery (ação já começou) 🙂", Pinned: true}},
	}
}

func export(t *testing.T, o Options) File {
	t.Helper()
	f, err := Export(sampleLedger(), o, now)
	if err != nil {
		t.Fatalf("Export(%+v): %v", o, err)
	}
	return f
}

func TestCSV(t *testing.T) {
	f := export(t, Options{Format: FormatCSV, Scope: ScopeMonth, Month: "2026-10"})
	got := string(f.Data)
	if f.Name != "finan_rich_lancamentos_2026-10.csv" || !strings.HasPrefix(got, "\xef\xbb\xbf") {
		t.Fatalf("nome %q ou sem BOM", f.Name)
	}
	for _, want := range []string{
		"Data;Tipo;Descrição;Categoria;Grupo;Valor (R$)\r\n",
		"2026-10-05;Entrada;Salário;Salário;Renda;5000,00\r\n",
		`2026-10-07;Saída;"Cinema ""estreia""; pipoca";Lazer & cia <bar>;Desejos;-87,50`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CSV sem %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "2026-09-06") || strings.Contains(got, "não exportar") {
		t.Error("CSV do mês trouxe lançamento de fora do mês ou apagado")
	}
	if f := export(t, Options{Format: FormatCSV, Dataset: DatasetDebts}); !strings.Contains(string(f.Data), "Cartão;Banco;4000,00;12,50%;500,00;600,00;10;16 meses") {
		t.Errorf("CSV de dívidas:\n%s", f.Data)
	}
}

func TestXLSX(t *testing.T) {
	f := export(t, Options{Format: FormatXLSX, Scope: ScopeYear, Month: "2026-10"})
	if f.Name != "finan_rich_2026.xlsx" {
		t.Errorf("nome = %q", f.Name)
	}
	z, err := zip.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, zf := range z.File {
		r, _ := zf.Open()
		b, _ := io.ReadAll(r)
		files[zf.Name] = string(b)
		// Todo XML precisa ser bem formado, senão o Excel diz que o arquivo está corrompido.
		dec := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s: XML inválido: %v", zf.Name, err)
			}
		}
	}
	for _, want := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/styles.xml", "xl/worksheets/sheet7.xml"} {
		if _, ok := files[want]; !ok {
			t.Errorf("xlsx sem %s", want)
		}
	}
	for _, sheet := range []string{"Resumo por mês", "Lançamentos", "Por categoria", "Divisão do salário", "Dívidas", "Notas", "Categorias"} {
		if !strings.Contains(files["xl/workbook.xml"], `name="`+sheet+`"`) {
			t.Errorf("sem a aba %q", sheet)
		}
	}
	tx := files["xl/worksheets/sheet2.xml"]
	// Valores como número (somáveis), datas como data do Excel, texto escapado.
	if !strings.Contains(tx, `<v>-1400.00</v>`) || !strings.Contains(tx, `<v>46301</v>`) || !strings.Contains(tx, "Lazer &amp; cia &lt;bar&gt;") {
		t.Errorf("aba de lançamentos:\n%s", tx)
	}
	if !strings.Contains(tx, `<autoFilter ref="A1:F5"/>`) || !strings.Contains(tx, `state="frozen"`) {
		t.Error("aba de lançamentos sem filtro ou sem cabeçalho fixo")
	}
	if months := files["xl/worksheets/sheet1.xml"]; strings.Count(months, "<row ") != 14 { // cabeçalho + 12 meses + total
		t.Errorf("resumo por mês deveria ter 14 linhas: %d", strings.Count(months, "<row "))
	}
}

func TestPDF(t *testing.T) {
	f := export(t, Options{Format: FormatPDF, Scope: ScopeMonth, Month: "2026-10"})
	checkPDF(t, f.Data)
	if f.Name != "finan_rich_relatorio_2026-10.pdf" || f.MIME != "application/pdf" {
		t.Errorf("arquivo = %q %q", f.Name, f.MIME)
	}
}

func TestPDFManyPages(t *testing.T) {
	l := sampleLedger()
	for i := range 300 {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: domain.Meta{ID: "x" + strconv.Itoa(i), UpdatedAt: 1},
			Kind: domain.KindSaida, Description: "Mercado " + strconv.Itoa(i), AmountCents: 1234, CategoryID: "mor", Date: "2026-10-10"})
	}
	f, err := Export(l, Options{Format: FormatPDF, Scope: ScopeAll}, now)
	if err != nil {
		t.Fatal(err)
	}
	if pages := checkPDF(t, f.Data); pages < 5 {
		t.Errorf("300 lançamentos deveriam ocupar várias páginas: %d", pages)
	}
}

// checkPDF confere a estrutura do arquivo (tabela xref com as posições certas)
// e devolve o número de páginas.
func checkPDF(t *testing.T, data []byte) int {
	t.Helper()
	s := string(data)
	if !strings.HasPrefix(s, "%PDF-1.4") || !strings.HasSuffix(s, "%%EOF\n") {
		t.Fatal("cabeçalho ou fim do PDF inválido")
	}
	m := regexp.MustCompile(`startxref\n(\d+)\n`).FindStringSubmatch(s)
	xref, _ := strconv.Atoi(m[1])
	if !strings.HasPrefix(s[xref:], "xref\n") {
		t.Fatal("startxref não aponta para a tabela xref")
	}
	lines := strings.Split(s[xref:], "\n")
	for i, line := range lines[3:] {
		if !strings.HasSuffix(line, " n ") {
			break
		}
		off, _ := strconv.Atoi(line[:10])
		if want := strconv.Itoa(i+1) + " 0 obj"; !strings.HasPrefix(s[off:], want) {
			t.Fatalf("xref do objeto %d aponta para %q", i+1, s[off:off+12])
		}
	}
	return strings.Count(s, "/Type /Page ")
}

func TestNoteExport(t *testing.T) {
	f := export(t, Options{NoteID: "n1", Format: FormatTXT})
	if f.Name != "nota_meta_de_2027.txt" || !strings.HasPrefix(string(f.Data), "# Meta de 2027!\r\n\r\nJuntar") {
		t.Errorf("nota em texto: %q %q", f.Name, f.Data)
	}
	checkPDF(t, export(t, Options{NoteID: "n1", Format: FormatPDF}).Data)
	if _, err := Export(sampleLedger(), Options{NoteID: "nao-existe", Format: FormatPDF}, now); err == nil {
		t.Error("nota inexistente deveria dar erro")
	}
}

func TestBackupRoundTrip(t *testing.T) {
	f := export(t, Options{Format: FormatJSON})
	if f.Name != "finan_rich_backup_2026-10-13.json" {
		t.Errorf("nome = %q", f.Name)
	}
	recs, err := ReadBackup(f.Data)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, r := range recs {
		count[r.Type]++
	}
	// 3 categorias, 4 lançamentos (o apagado fica de fora), 1 dívida, 1 nota.
	if count["category"] != 3 || count["transaction"] != 4 || count["debt"] != 1 || count["note"] != 1 {
		t.Errorf("registros no backup = %v", count)
	}
	for _, bad := range []string{`{}`, `nada`, `{"app":"finan_rich","version":99}`,
		`{"app":"finan_rich","version":1,"records":[{"type":"transaction","record":{"id":"x","updatedAt":1}}]}`} {
		if _, err := ReadBackup([]byte(bad)); err == nil {
			t.Errorf("backup inválido aceito: %s", bad)
		}
	}
}

func TestWinAnsiAndWrap(t *testing.T) {
	if got := string(winAnsi("Ação – R$ 5€ 🙂")); got != "A\xe7\xe3o \x96 R$ 5\x80 " {
		t.Errorf("winAnsi = %q", got)
	}
	lines := wrap(strings.Repeat("palavra ", 60)+strings.Repeat("x", 300), 10, false, 200)
	for _, l := range lines {
		if w := textWidth(l, 10, false); w > 200.01 {
			t.Errorf("linha mais larga que o limite (%.1f): %q", w, l)
		}
	}
	if slug("  Ação: plano #1 ") != "acao_plano_1" {
		t.Errorf("slug = %q", slug("  Ação: plano #1 "))
	}
	if colName(0) != "A" || colName(25) != "Z" || colName(26) != "AA" || colName(701) != "ZZ" {
		t.Error("colName errado")
	}
}
