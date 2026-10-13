package report

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Formatos de exportação.
const (
	FormatXLSX = "xlsx"
	FormatCSV  = "csv"
	FormatPDF  = "pdf"
	FormatTXT  = "txt" // só para notas
	FormatJSON = "json"
)

// Conjuntos de dados que podem sair num CSV (um arquivo por conjunto).
const (
	DatasetTransactions = "transactions"
	DatasetMonths       = "months"
	DatasetDebts        = "debts"
	DatasetNotes        = "notes"
	DatasetCategories   = "categories"
)

// Options diz o que exportar. Vem da tela (em JSON) e tem valores padrão
// razoáveis para tudo que faltar.
type Options struct {
	Format  string `json:"format"`
	Scope   string `json:"scope"`   // month, year ou all
	Month   string `json:"month"`   // mês de referência: "2026-10"
	Dataset string `json:"dataset"` // para CSV
	NoteID  string `json:"noteId"`  // exporta só esta nota (pdf ou txt)
}

// File é o arquivo pronto para baixar.
type File struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data []byte `json:"data"`
}

const (
	mimeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	mimeCSV  = "text/csv;charset=utf-8"
	mimePDF  = "application/pdf"
	mimeTXT  = "text/plain;charset=utf-8"
	mimeJSON = "application/json"
)

var ErrUnknownFormat = errors.New("formato de exportação desconhecido")

// Export gera o arquivo pedido. now é a hora da geração (aparece no PDF e no backup).
func Export(l domain.Ledger, o Options, now time.Time) (File, error) {
	if o.NoteID != "" {
		return exportNote(l, o.NoteID, o.Format, now)
	}
	p := NewPeriod(o.Scope, domain.ParseMonth(o.Month), l)
	switch o.Format {
	case FormatXLSX:
		data, err := writeXLSX(workbook(l, p))
		return File{Name: "finan_rich_" + p.Slug + ".xlsx", MIME: mimeXLSX, Data: data}, err
	case FormatCSV:
		t, slug := csvDataset(l, p, o.Dataset)
		return File{Name: "finan_rich_" + slug + ".csv", MIME: mimeCSV, Data: writeCSV(t)}, nil
	case FormatPDF:
		return File{Name: "finan_rich_relatorio_" + p.Slug + ".pdf", MIME: mimePDF, Data: reportPDF(l, p, now)}, nil
	case FormatJSON:
		data, err := Backup(l, now)
		return File{Name: "finan_rich_backup_" + now.In(brazil).Format("2006-01-02") + ".json", MIME: mimeJSON, Data: data}, err
	}
	return File{}, fmt.Errorf("%w: %q", ErrUnknownFormat, o.Format)
}

// workbook são as abas da planilha, no estilo de uma planilha de orçamento:
// resumo por mês, lançamentos, gastos por categoria, divisão do salário,
// dívidas e notas.
func workbook(l domain.Ledger, p Period) []Table {
	s := finance.SummarizePeriod(p.From, p.To, l)
	return []Table{
		monthsTable(l, p),
		transactionsTable(l, p),
		categoriesTable(s),
		splitTable(s),
		debtsTable(l),
		notesTable(l),
		categoryListTable(l),
	}
}

func csvDataset(l domain.Ledger, p Period, dataset string) (Table, string) {
	switch dataset {
	case DatasetMonths:
		return monthsTable(l, p), "resumo_" + p.Slug
	case DatasetDebts:
		return debtsTable(l), "dividas"
	case DatasetNotes:
		return notesTable(l), "notas"
	case DatasetCategories:
		return categoryListTable(l), "categorias"
	}
	return transactionsTable(l, p), "lancamentos_" + p.Slug
}

// reportPDF é o relatório para ler, imprimir ou mandar para alguém.
func reportPDF(l domain.Ledger, p Period, now time.Time) []byte {
	title := "Relatório de " + p.Label
	d := newPDF(title)
	d.text(marginX, d.y+18, 20, true, colorText, "Relatório financeiro")
	d.y += 34
	d.text(marginX, d.y, 11, false, colorMuted, strings.ToUpper(p.Label[:1])+p.Label[1:]+" · gerado em "+now.In(brazil).Format("02/01/2006 às 15:04"))
	d.y += 22

	s := finance.SummarizePeriod(p.From, p.To, l)
	balance := s.BalanceCents()
	balanceColor := colorAccent
	if balance < 0 {
		balanceColor = colorNeg
	}
	d.cards([][2]string{
		{"Entradas", brl(s.InCents)}, {"Saídas", brl(s.OutCents)}, {"Saldo", brl(balance)},
		{"Guardado (saldo + investido)", fmt.Sprintf("%d%% da renda", s.SavingsRatePct())},
	}, []rgb{colorAccent, colorText, balanceColor, colorText})

	if !p.single() {
		d.heading("Resumo por mês")
		d.table(monthsTable(l, p))
	}
	d.heading("Divisão do salário")
	split := splitTable(s)
	d.bars(split)
	d.table(split)

	d.heading("Gastos por categoria")
	d.table(categoriesTable(s))

	d.heading("Lançamentos")
	d.table(transactionsTable(l, p))

	if debts := debtsTable(l); len(debts.Rows) > 0 {
		d.heading("Dívidas (situação atual)")
		d.table(debts)
	}
	if notes := l.ActiveNotes(); len(notes) > 0 {
		d.heading("Notas")
		for _, n := range notes {
			writeNote(d, n)
		}
	}
	return d.bytes()
}

func writeNote(d *pdfDoc, n domain.Note) {
	d.ensure(40)
	title := n.Title
	if title == "" {
		title = "Sem título"
	}
	for _, line := range wrap(title, 11.5, true, usableW) {
		d.ensure(16)
		d.text(marginX, d.y+12, 11.5, true, colorText, line)
		d.y += 16
	}
	d.text(marginX, d.y+8, 8, false, colorMuted, "atualizada em "+msTime(n.UpdatedAt))
	d.y += 14
	d.paragraph(n.Body, 10, colorText)
	d.y += 10
}

func exportNote(l domain.Ledger, id, format string, now time.Time) (File, error) {
	var note *domain.Note
	for i := range l.Notes {
		if l.Notes[i].ID == id && !l.Notes[i].Deleted {
			note = &l.Notes[i]
		}
	}
	if note == nil {
		return File{}, errors.New("nota não encontrada")
	}
	name := "nota_" + slug(note.Title)
	switch format {
	case FormatPDF:
		d := newPDF(note.Title)
		writeNote(d, *note)
		return File{Name: name + ".pdf", MIME: mimePDF, Data: d.bytes()}, nil
	case FormatTXT:
		text := note.Body
		if note.Title != "" {
			text = "# " + note.Title + "\n\n" + text
		}
		return File{Name: name + ".txt", MIME: mimeTXT, Data: []byte(strings.ReplaceAll(text, "\n", "\r\n") + "\r\n")}, nil
	}
	return File{}, fmt.Errorf("%w: %q", ErrUnknownFormat, format)
}

// slug faz um nome de arquivo seguro: "Meta de 2027!" → "meta_de_2027".
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune("áàâãä", r):
			b.WriteByte('a')
		case strings.ContainsRune("éèêë", r):
			b.WriteByte('e')
		case strings.ContainsRune("íìîï", r):
			b.WriteByte('i')
		case strings.ContainsRune("óòôõö", r):
			b.WriteByte('o')
		case strings.ContainsRune("úùûü", r):
			b.WriteByte('u')
		case r == 'ç':
			b.WriteByte('c')
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "sem_titulo"
	}
	return out
}

var brl = money.Format
