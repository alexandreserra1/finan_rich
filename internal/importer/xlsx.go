package importer

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	xlsxMaxExpanded = 16 * 1024 * 1024
	xlsxMaxEntries  = 256
	xlsxMaxRows     = 10000
	xlsxMaxColumns  = 128
	xlsxMaxCells    = 200000
	xlsxMaxText     = 65536
)

type xlsxText struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (value xlsxText) text() (string, error) {
	var result strings.Builder
	result.WriteString(value.Text)
	for _, run := range value.Runs {
		result.WriteString(run.Text)
	}
	if result.Len() > xlsxMaxText {
		return "", errors.New("texto da célula XLSX excede o limite de 64 KB")
	}
	return result.String(), nil
}

type xlsxCell struct {
	Ref    string   `xml:"r,attr"`
	Type   string   `xml:"t,attr"`
	Style  int      `xml:"s,attr"`
	Value  string   `xml:"v"`
	Inline xlsxText `xml:"is"`
}

type xlsxRelationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
	Mode   string `xml:"TargetMode,attr"`
}

func parseXLSX(data []byte) (Statement, error) {
	fail := func(err error) (Statement, error) { return Statement{Format: "xlsx"}, err }
	files, err := readXLSXArchive(data)
	if err != nil {
		return fail(err)
	}
	var workbook struct {
		XMLName    xml.Name `xml:"workbook"`
		Properties struct {
			Date1904 bool `xml:"date1904,attr"`
		} `xml:"workbookPr"`
		Sheets []struct {
			ID string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(files["xl/workbook.xml"], &workbook); err != nil || len(workbook.Sheets) == 0 {
		return fail(errors.New("XLSX inválido: não encontrei as planilhas do arquivo"))
	}
	if len(workbook.Sheets) > 32 {
		return fail(errors.New("o XLSX excede o limite de 32 planilhas"))
	}
	var relationships struct {
		Items []xlsxRelationship `xml:"Relationship"`
	}
	if err := xml.Unmarshal(files["xl/_rels/workbook.xml.rels"], &relationships); err != nil {
		return fail(errors.New("XLSX inválido: não encontrei os caminhos das planilhas"))
	}
	targets := make(map[string]string)
	for _, relation := range relationships.Items {
		if !strings.HasSuffix(relation.Type, "/worksheet") {
			continue
		}
		target := path.Join("xl", relation.Target)
		if strings.HasPrefix(relation.Target, "/") {
			target = strings.TrimPrefix(relation.Target, "/")
		}
		if !strings.HasPrefix(target, "xl/worksheets/") || targets[relation.ID] != "" {
			return fail(errors.New("caminho de planilha XLSX inválido"))
		}
		targets[relation.ID] = target
	}
	var shared struct {
		XMLName xml.Name   `xml:"sst"`
		Items   []xlsxText `xml:"si"`
	}
	if content, ok := files["xl/sharedStrings.xml"]; ok {
		if err := xml.Unmarshal(content, &shared); err != nil {
			return fail(errors.New("textos compartilhados do XLSX inválidos"))
		}
	}
	texts := make([]string, len(shared.Items))
	for index, item := range shared.Items {
		texts[index], err = item.text()
		if err != nil {
			return fail(err)
		}
	}
	dateStyles, err := xlsxDateStyles(files["xl/styles.xml"])
	if err != nil {
		return fail(err)
	}
	for _, sheet := range workbook.Sheets {
		content, ok := files[targets[sheet.ID]]
		if !ok {
			return fail(errors.New("planilha XLSX ausente ou sem caminho válido"))
		}
		records, cells, err := xlsxRecords(content, texts, dateStyles)
		if err != nil {
			return fail(err)
		}
		header, columns, recognized := findHeader(records)
		if !recognized {
			continue
		}
		for rowIndex := header + 1; rowIndex < len(records); rowIndex++ {
			if strings.TrimSpace(strings.Join(records[rowIndex], "")) == "" {
				continue
			}
			for columnIndex, cell := range cells[rowIndex] {
				value := records[rowIndex][columnIndex]
				if value == "" {
					continue
				}
				if cell.Type == "b" || cell.Type == "e" {
					return fail(errors.New("células com erro ou valor lógico não são aceitas na planilha financeira; exporte como CSV"))
				}
				if columnIndex == columns.date && (cell.Type == "" || cell.Type == "n") {
					if dateStyles[cell.Style] {
						value, err = xlsxDate(value, workbook.Properties.Date1904)
						if err != nil {
							return fail(err)
						}
						records[rowIndex][columnIndex] = value
					}
					if _, valid := parseDate(value); !valid {
						return fail(errors.New("data numérica sem formato de data reconhecido no XLSX; exporte datas como texto AAAA-MM-DD"))
					}
				}
				if (columnIndex == columns.amount || columnIndex == columns.credit || columnIndex == columns.debit) && (cell.Type == "" || cell.Type == "n") {
					value, err = xlsxAmount(value)
					if err != nil {
						return fail(err)
					}
					records[rowIndex][columnIndex] = value
				}
			}
			get := func(column int) string {
				if column < 0 || column >= len(records[rowIndex]) {
					return ""
				}
				return strings.TrimSpace(records[rowIndex][column])
			}
			if _, valid := parseDate(get(columns.date)); !valid {
				return fail(fmt.Errorf("data inválida no registro %d do XLSX; confira a planilha, sem importar parcialmente", rowIndex+1))
			}
			if columns.amount < 0 && get(columns.credit) == "" && get(columns.debit) == "" {
				return fail(fmt.Errorf("valor ausente no registro %d do XLSX", rowIndex+1))
			}
			for _, column := range []int{columns.amount, columns.credit, columns.debit} {
				value := get(column)
				if column < 0 || value == "" && column != columns.amount {
					continue
				}
				if !xlsxValidTextAmount(value) {
					return fail(fmt.Errorf("valor inválido ou com fração de centavo no registro %d do XLSX", rowIndex+1))
				}
			}
			var description strings.Builder
			for _, column := range columns.desc {
				description.WriteString(get(column))
			}
			if description.Len() == 0 {
				return fail(fmt.Errorf("descrição ausente no registro %d do XLSX", rowIndex+1))
			}
		}
		statement, err := xlsxParseCSV(records)
		statement.Format = "xlsx"
		if err != nil {
			return fail(fmt.Errorf("importo somente a primeira planilha com cabeçalho financeiro, sem juntar contas: %w", err))
		}
		return statement, nil
	}
	return fail(errors.New("não reconheci colunas de data, descrição e valor no XLSX; importo somente a primeira planilha financeira, sem juntar contas; exporte essa planilha como CSV"))
}

var xlsxTextMoney = regexp.MustCompile(`^(?:\d{1,3}(?:\.\d{3})+(?:,\d{1,2}0*)?|\d+(?:,\d{1,2}0*)?|\d+\.\d{1,2}0*|\d{1,3}(?:,\d{3})+\.\d{1,2}0*)$`)

func xlsxValidTextAmount(value string) bool {
	clean := strings.NewReplacer("R$", "", "r$", "", " ", "", "\u00a0", "").Replace(value)
	clean = strings.Trim(clean, "()+-DCdc")
	_, valid := parseAmount(value)
	return valid && xlsxTextMoney.MatchString(clean)
}

func readXLSXArchive(data []byte) (map[string][]byte, error) {
	if len(data) > 8*1024*1024 {
		return nil, errors.New("o XLSX excede o limite de 8 MB")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("não consegui ler o ZIP do XLSX")
	}
	if len(reader.File) > xlsxMaxEntries {
		return nil, errors.New("o ZIP do XLSX excede o limite de 256 entradas")
	}
	files := make(map[string][]byte)
	seen := make(map[string]bool)
	total := 0
	for _, entry := range reader.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "." || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\:") || seen[name] {
			return nil, errors.New("entrada repetida ou caminho inválido no ZIP do XLSX")
		}
		seen[name] = true
		lower := strings.ToLower(name)
		if strings.Contains(lower, "vbaproject") || strings.Contains(lower, "macrosheet") {
			return nil, errors.New("planilhas com macros não são aceitas; exporte como XLSX sem macros ou CSV")
		}
		if strings.HasPrefix(lower, "xl/externallinks/") {
			return nil, errors.New("referências externas não são aceitas no XLSX; exporte os valores como CSV")
		}
		if entry.UncompressedSize64 > uint64(xlsxMaxExpanded-total) {
			return nil, errors.New("o XLSX excede o limite de 16 MB descompactados")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, errors.New("entrada ZIP do XLSX inválida")
		}
		content, readErr := io.ReadAll(io.LimitReader(stream, int64(xlsxMaxExpanded-total)+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.New("entrada ZIP do XLSX corrompida")
		}
		total += len(content)
		if total > xlsxMaxExpanded {
			return nil, errors.New("o XLSX excede o limite de 16 MB descompactados")
		}
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") {
			if err := validateXLSXXML(content); err != nil {
				return nil, err
			}
		}
		if strings.HasSuffix(lower, ".rels") {
			var relationships struct {
				Items []xlsxRelationship `xml:"Relationship"`
			}
			if err := xml.Unmarshal(content, &relationships); err != nil {
				return nil, errors.New("XML de referências do XLSX inválido")
			}
			for _, relation := range relationships.Items {
				if strings.EqualFold(relation.Mode, "External") || strings.ContainsAny(relation.Target, ":\\") || strings.HasPrefix(relation.Target, "//") {
					return nil, errors.New("referências externas não são aceitas no XLSX; exporte os valores como CSV")
				}
			}
		}
		files[name] = content
	}
	return files, nil
}

func validateXLSXXML(content []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	depth, roots, elements, rows, cells, rowCells, texts := 0, 0, 0, 0, 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("XML do XLSX inválido")
		}
		switch value := token.(type) {
		case xml.Directive:
			return errors.New("diretivas e entidades XML não são aceitas no XLSX")
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			elements++
			if depth > 32 || elements > 1000000 {
				return errors.New("XML do XLSX excede os limites de estrutura")
			}
			switch value.Name.Local {
			case "f":
				return errors.New("células com fórmula não são aceitas; exporte somente os valores como XLSX ou CSV")
			case "row":
				rows++
				rowCells = 0
				if rows > xlsxMaxRows {
					return errors.New("o XLSX excede o limite de 10000 linhas por planilha")
				}
			case "c":
				cells++
				rowCells++
				if rowCells > xlsxMaxColumns {
					return errors.New("o XLSX excede o limite de 128 colunas")
				}
				if cells > xlsxMaxCells {
					return errors.New("o XLSX excede o limite de 200000 células por planilha")
				}
			case "si":
				texts++
				if texts > 100000 {
					return errors.New("textos compartilhados do XLSX excedem o limite de 100000 itens")
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if len(value) > xlsxMaxText || depth == 0 && strings.TrimSpace(string(value)) != "" {
				return errors.New("texto XML do XLSX inválido ou acima de 64 KB")
			}
		}
	}
	if depth != 0 || roots != 1 {
		return errors.New("XML do XLSX incompleto ou com mais de uma raiz")
	}
	return nil
}

func xlsxDateStyles(content []byte) ([]bool, error) {
	if len(content) == 0 {
		return []bool{false}, nil
	}
	var styles struct {
		XMLName xml.Name `xml:"styleSheet"`
		Formats []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Cells []struct {
			ID int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := xml.Unmarshal(content, &styles); err != nil {
		return nil, errors.New("estilos de data do XLSX inválidos")
	}
	custom := make(map[int]bool)
	for _, format := range styles.Formats {
		var code strings.Builder
		quoted, bracket, escaped := false, false, false
		for _, letter := range strings.ToLower(format.Code) {
			if escaped {
				escaped = false
				continue
			}
			switch letter {
			case '\\', '_', '*':
				escaped = true
			case '"':
				quoted = !quoted
			case '[':
				if !quoted {
					bracket = true
				}
			case ']':
				if !quoted {
					bracket = false
				}
			default:
				if !quoted && !bracket {
					code.WriteRune(letter)
				}
			}
		}
		custom[format.ID] = strings.Contains(code.String(), "d") && (strings.Contains(code.String(), "y") || strings.Contains(code.String(), "m"))
	}
	result := make([]bool, max(1, len(styles.Cells)))
	for index, style := range styles.Cells {
		result[index] = style.ID >= 14 && style.ID <= 17 || style.ID == 22 || custom[style.ID]
	}
	return result, nil
}

func xlsxRecords(content []byte, shared []string, dateStyles []bool) ([][]string, [][]xlsxCell, error) {
	var sheet struct {
		XMLName xml.Name `xml:"worksheet"`
		Rows    []struct {
			Ref   int        `xml:"r,attr"`
			Cells []xlsxCell `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(content, &sheet); err != nil {
		return nil, nil, errors.New("XML da planilha XLSX inválido")
	}
	var records [][]string
	var cells [][]xlsxCell
	lastRow := 0
	for _, row := range sheet.Rows {
		rowNumber := row.Ref
		if rowNumber == 0 {
			rowNumber = lastRow + 1
		}
		if rowNumber <= lastRow || rowNumber > xlsxMaxRows {
			return nil, nil, errors.New("linhas XLSX repetidas, fora de ordem ou acima do limite de 10000")
		}
		lastRow = rowNumber
		var record []string
		var rowCells []xlsxCell
		for _, cell := range row.Cells {
			column := len(record)
			if cell.Ref != "" {
				column = 0
				position := 0
				for position < len(cell.Ref) && cell.Ref[position] >= 'A' && cell.Ref[position] <= 'Z' {
					column = column*26 + int(cell.Ref[position]-'A') + 1
					if column > xlsxMaxColumns {
						return nil, nil, errors.New("o XLSX excede o limite de 128 colunas")
					}
					position++
				}
				refRow, err := strconv.Atoi(cell.Ref[position:])
				if err != nil || column == 0 || refRow != rowNumber {
					return nil, nil, errors.New("referência de célula XLSX inválida")
				}
				column--
			}
			if column < len(record) || column >= xlsxMaxColumns {
				return nil, nil, errors.New("célula XLSX repetida, fora de ordem ou acima do limite de colunas")
			}
			if cell.Style < 0 || cell.Style >= len(dateStyles) {
				return nil, nil, errors.New("estilo de célula XLSX inválido")
			}
			value := strings.TrimSpace(cell.Value)
			var err error
			switch cell.Type {
			case "s":
				index, parseErr := strconv.Atoi(value)
				if parseErr != nil || index < 0 || index >= len(shared) {
					return nil, nil, errors.New("índice de texto compartilhado XLSX inválido")
				}
				value = shared[index]
			case "inlineStr":
				value, err = cell.Inline.text()
			case "", "n", "d", "str", "b", "e":
			default:
				return nil, nil, errors.New("tipo de célula XLSX não aceito; exporte somente texto, datas e valores como CSV")
			}
			if err != nil {
				return nil, nil, err
			}
			for len(record) <= column {
				record = append(record, "")
				rowCells = append(rowCells, xlsxCell{})
			}
			record[column], rowCells[column] = value, cell
		}
		records = append(records, record)
		cells = append(cells, rowCells)
	}
	return records, cells, nil
}

func xlsxDate(value string, date1904 bool) (string, error) {
	serial, ok := xlsxNumber(value)
	if !ok || serial.Sign() < 0 || serial.Cmp(big.NewRat(110000, 1)) > 0 {
		return "", errors.New("data serial do XLSX inválida")
	}
	days := new(big.Int).Quo(serial.Num(), serial.Denom()).Int64()
	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	if date1904 {
		base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		if days == 60 {
			return "", errors.New("data fictícia 29/02/1900 do Excel não é aceita")
		}
		if days > 60 {
			days--
		}
	}
	date := base.AddDate(0, 0, int(days))
	if date.Year() <= 1990 || date.Year() >= 2200 {
		return "", errors.New("data do XLSX fora do período aceito: 1991 a 2199")
	}
	return date.Format("2006-01-02"), nil
}

func xlsxNumber(value string) (*big.Rat, bool) {
	if len(value) == 0 || len(value) > 128 {
		return nil, false
	}
	mantissa := value
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		exponent, err := strconv.Atoi(value[index+1:])
		if err != nil || exponent < -128 || exponent > 128 {
			return nil, false
		}
		mantissa = value[:index]
	}
	mantissa = strings.TrimPrefix(strings.TrimPrefix(mantissa, "-"), "+")
	if strings.Count(mantissa, ".") > 1 || !isDigits(strings.ReplaceAll(mantissa, ".", "")) {
		return nil, false
	}
	return new(big.Rat).SetString(value)
}

func xlsxAmount(value string) (string, error) {
	amount, ok := xlsxNumber(value)
	if !ok {
		return "", errors.New("valor numérico do XLSX inválido")
	}
	amount.Mul(amount, big.NewRat(100, 1))
	if !amount.IsInt() || !amount.Num().IsInt64() || amount.Num().Int64() == -1<<63 {
		return "", errors.New("valor do XLSX fora do limite de centavos int64 ou com fração de centavo")
	}
	return strings.Replace(amount.Quo(amount, big.NewRat(100, 1)).FloatString(2), ".", ",", 1), nil
}

func xlsxParseCSV(records [][]string) (Statement, error) {
	separator := ','
	for attempt := 0; attempt < 3; attempt++ {
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		writer.Comma = separator
		if err := writer.WriteAll(records); err != nil {
			return Statement{}, errors.New("não consegui normalizar a planilha XLSX")
		}
		text := buffer.String()
		first := text
		if index := strings.IndexByte(text, '\n'); index >= 0 {
			first = text[:min(len(text), index+2000)]
		}
		chosen, best := ',', strings.Count(first, ",")
		for _, candidate := range []rune{';', '\t'} {
			if count := strings.Count(first, string(candidate)); count > best {
				chosen, best = candidate, count
			}
		}
		if chosen == separator {
			return parseCSV(text)
		}
		separator = chosen
	}
	return Statement{}, errors.New("não consegui reconhecer o separador da planilha XLSX; exporte como CSV")
}
