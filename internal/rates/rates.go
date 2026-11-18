// Package rates busca as taxas do dia no Banco Central (API pública do SGS, sem
// chave e sem enviar nenhum dado da pessoa): Selic, CDI e IPCA em 12 meses. O
// consultor usa para não mandar "confira a taxa do dia" quando o app já sabe.
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// BaseURL é o endereço da API do Banco Central (aceita chamadas do navegador: CORS *).
const BaseURL = "https://api.bcb.gov.br"

// Séries do SGS: meta da Selic, CDI anualizado (base 252) e IPCA acumulado em 12 meses.
const (
	serieSelic = "432"
	serieCDI   = "4389"
	serieIPCA  = "13522"
)

// Rates são as taxas em pontos-base (1375 = 13,75%), sem float, como os juros das dívidas.
type Rates struct {
	SelicBP   int    `json:"selicBP"`
	CDIBP     int    `json:"cdiBP"`
	IPCABP    int    `json:"ipcaBP"`
	IPCAMonth string `json:"ipcaMonth"` // AAAA-MM: último mês do IPCA
	Date      string `json:"date"`      // AAAA-MM-DD: dia do CDI (o último dia útil)
}

// Fetch busca as três séries; qualquer falha devolve erro (o consultor segue sem taxas).
func Fetch(ctx context.Context, client *http.Client, base string) (Rates, error) {
	var r Rates
	var selicDate, ipcaDate string
	for _, s := range []struct {
		code string
		bp   *int
		date *string
	}{{serieSelic, &r.SelicBP, &selicDate}, {serieCDI, &r.CDIBP, &r.Date}, {serieIPCA, &r.IPCABP, &ipcaDate}} {
		bp, date, err := last(ctx, client, base, s.code)
		if err != nil {
			return Rates{}, fmt.Errorf("taxas do Banco Central (série %s): %w", s.code, err)
		}
		*s.bp, *s.date = bp, date
	}
	r.IPCAMonth = ipcaDate[:7]
	return r, nil
}

// last devolve o último valor da série e a data dele (AAAA-MM-DD).
func last(ctx context.Context, client *http.Client, base, code string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/dados/serie/bcdata.sgs."+code+"/dados/ultimos/1?formato=json", nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("resposta %d", resp.StatusCode)
	}
	var rows []struct{ Data, Valor string }
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return 0, "", err
	}
	if len(rows) == 0 {
		return 0, "", fmt.Errorf("série vazia")
	}
	day, err := time.Parse("02/01/2006", rows[len(rows)-1].Data)
	if err != nil {
		return 0, "", err
	}
	bp, err := parseBP(rows[len(rows)-1].Valor)
	return bp, day.Format("2006-01-02"), err
}

// parseBP lê "13.75" (ou "13,75") como 1375 pontos-base, arredondando a 3ª casa.
func parseBP(s string) (int, error) {
	neg := strings.HasPrefix(s, "-")
	whole, frac, _ := strings.Cut(strings.Replace(strings.TrimPrefix(s, "-"), ",", ".", 1), ".")
	digits := func(x string) bool { return strings.Trim(x, "0123456789") == "" }
	if whole == "" || len(whole) > 6 || !digits(whole) || !digits(frac) {
		return 0, fmt.Errorf("taxa inválida: %q", s)
	}
	frac += "000"
	w, _ := strconv.Atoi(whole)
	f, _ := strconv.Atoi(frac[:2])
	bp := w*100 + f
	if frac[2] >= '5' {
		bp++
	}
	if neg {
		bp = -bp
	}
	return bp, nil
}

// pct escreve pontos-base como porcentagem brasileira ("13,75%").
func pct(bp int) string {
	sign := ""
	if bp < 0 {
		sign, bp = "-", -bp
	}
	return fmt.Sprintf("%s%d,%02d%%", sign, bp/100, bp%100)
}

// Text é a linha que vai nos dados do consultor (vazia sem taxas). Leva a data: sem
// internet, valem as últimas guardadas, que podem ser de outro dia.
func (r Rates) Text() string {
	if r.Date == "" {
		return ""
	}
	day, _ := time.Parse("2006-01-02", r.Date)
	return fmt.Sprintf("Taxas do Banco Central (%s): Selic %s ao ano; CDI %s ao ano; IPCA %s em 12 meses (até %s).",
		day.Format("02/01/2006"), pct(r.SelicBP), pct(r.CDIBP), pct(r.IPCABP), domain.ParseMonth(r.IPCAMonth).Label())
}
