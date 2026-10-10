package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// redirect volta para a página com uma mensagem de sucesso ou erro (padrão POST/redirect/GET).
func redirect(w http.ResponseWriter, r *http.Request, path, msg string, err error) {
	q := url.Values{}
	if err != nil {
		q.Set("erro", err.Error())
	} else if msg != "" {
		q.Set("msg", msg)
	}
	if mes := r.FormValue("mes"); mes != "" {
		q.Set("mes", mes)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// monthSummary carrega lançamentos + divisão e calcula o resumo do mês.
func (s *Server) monthSummary(ctx context.Context, m store.Month) (finance.Summary, []store.Transaction, error) {
	txs, err := s.store.ListTransactions(ctx, m)
	if err != nil {
		return finance.Summary{}, nil, err
	}
	split, err := s.store.Split(ctx)
	if err != nil {
		return finance.Summary{}, nil, err
	}
	return finance.Summarize(m, txs, split), txs, nil
}

// ---------- Painel ----------

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := store.ParseMonth(r.URL.Query().Get("mes"))
	sum, _, err := s.monthSummary(ctx, m)
	if err != nil {
		serverError(w, err)
		return
	}
	year, err := s.store.MonthlyTotals(ctx, m.Year())
	if err != nil {
		serverError(w, err)
		return
	}
	var yearIn, yearOut, maxMonth int64
	for _, t := range year {
		yearIn += t.InCents
		yearOut += t.OutCents
		maxMonth = max(maxMonth, t.InCents, t.OutCents)
	}
	var maxCat int64
	if len(sum.Categories) > 0 {
		maxCat = sum.Categories[0].SpentCents
	}
	s.render(w, r, "dashboard", map[string]any{
		"Month": m, "Summary": sum, "Year": year,
		"YearIn": yearIn, "YearOut": yearOut, "YearBalance": yearIn - yearOut, "MaxMonth": maxMonth, "MaxCat": maxCat,
	})
}

// ---------- Lançamentos ----------

func (s *Server) transactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := store.ParseMonth(r.URL.Query().Get("mes"))
	sum, txs, err := s.monthSummary(ctx, m)
	if err != nil {
		serverError(w, err)
		return
	}
	cats, err := s.store.ListCategories(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	// Data sugerida no formulário: hoje se estiver no mês, senão dia 1.
	defaultDate := m.Time
	if now := time.Now(); now.After(m.Time) && now.Before(m.End()) {
		defaultDate = now
	}
	s.render(w, r, "transactions", map[string]any{
		"Month": m, "Summary": sum, "Transactions": txs, "Categories": cats,
		"DefaultDate": defaultDate.Format("2006-01-02"),
	})
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	t, err := parseTransaction(r)
	if err == nil {
		err = s.store.CreateTransaction(r.Context(), t)
	}
	if err == nil {
		// Mantém o usuário no mês do lançamento.
		r.Form.Set("mes", t.OccurredOn.Format("2006-01"))
	}
	redirect(w, r, "/lancamentos", "Lançamento salvo.", err)
}

func parseTransaction(r *http.Request) (store.Transaction, error) {
	var t store.Transaction
	if err := r.ParseForm(); err != nil {
		return t, err
	}
	t.Kind = r.FormValue("kind")
	if t.Kind != store.KindEntrada && t.Kind != store.KindSaida {
		return t, errors.New("tipo inválido")
	}
	t.Description = strings.TrimSpace(r.FormValue("description"))
	if t.Description == "" {
		return t, errors.New("informe uma descrição")
	}
	var err error
	if t.AmountCents, err = money.Parse(r.FormValue("amount")); err != nil {
		return t, err
	}
	if t.CategoryID, err = strconv.ParseInt(r.FormValue("category_id"), 10, 64); err != nil {
		return t, errors.New("escolha uma categoria")
	}
	if t.OccurredOn, err = time.Parse("2006-01-02", r.FormValue("occurred_on")); err != nil {
		return t, errors.New("data inválida")
	}
	return t, nil
}

func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteTransaction(r.Context(), pathID(r))
	redirect(w, r, "/lancamentos", "Lançamento apagado.", err)
}

func (s *Server) copyPreviousMonth(w http.ResponseWriter, r *http.Request) {
	m := store.ParseMonth(r.FormValue("mes"))
	n, err := s.store.CopyFromPreviousMonth(r.Context(), m)
	redirect(w, r, "/lancamentos", strconv.FormatInt(n, 10)+" lançamento(s) copiado(s) do mês anterior.", err)
}

// ---------- Categorias e divisão do salário ----------

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cats, err := s.store.ListCategories(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	split, err := s.store.Split(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, r, "categories", map[string]any{
		"Categories": cats, "Split": split,
		"Buckets": append([]string{store.BucketRenda}, store.SpendingBuckets...),
	})
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	bucket := r.FormValue("bucket")
	var err error
	if name == "" {
		err = errors.New("informe o nome da categoria")
	} else if _, ok := bucketLabels[bucket]; !ok {
		err = errors.New("grupo inválido")
	} else {
		err = s.store.CreateCategory(r.Context(), name, bucket)
	}
	redirect(w, r, "/categorias", "Categoria criada.", err)
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteCategory(r.Context(), pathID(r))
	redirect(w, r, "/categorias", "Categoria apagada.", err)
}

func (s *Server) saveSplit(w http.ResponseWriter, r *http.Request) {
	split := map[string]int{}
	total := 0
	for _, b := range store.SpendingBuckets {
		n, err := strconv.Atoi(r.FormValue(b))
		if err != nil || n < 0 || n > 100 {
			redirect(w, r, "/categorias", "", errors.New("percentuais devem ser números de 0 a 100"))
			return
		}
		split[b] = n
		total += n
	}
	if total != 100 {
		redirect(w, r, "/categorias", "", errors.New("os percentuais precisam somar 100% (somaram "+strconv.Itoa(total)+"%)"))
		return
	}
	err := s.store.SaveSplit(r.Context(), split)
	redirect(w, r, "/categorias", "Divisão do salário atualizada.", err)
}

// ---------- Dívidas ----------

func (s *Server) debts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	debts, err := s.store.ListDebts(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	var editing store.Debt
	if id, _ := strconv.ParseInt(r.URL.Query().Get("editar"), 10, 64); id > 0 {
		if editing, err = s.store.GetDebt(ctx, id); err != nil {
			serverError(w, err)
			return
		}
	}
	s.render(w, r, "debts", map[string]any{"Plan": finance.PlanDebts(debts), "Editing": editing})
}

func (s *Server) saveDebt(w http.ResponseWriter, r *http.Request) {
	d, err := parseDebt(r)
	if err == nil {
		err = s.store.SaveDebt(r.Context(), d)
	}
	redirect(w, r, "/dividas", "Dívida salva.", err)
}

func parseDebt(r *http.Request) (store.Debt, error) {
	var d store.Debt
	d.ID, _ = strconv.ParseInt(r.FormValue("id"), 10, 64)
	d.Name = strings.TrimSpace(r.FormValue("name"))
	d.Creditor = strings.TrimSpace(r.FormValue("creditor"))
	if d.Name == "" {
		return d, errors.New("informe o nome da dívida")
	}
	var err error
	if d.BalanceCents, err = money.Parse(r.FormValue("balance")); err != nil {
		return d, errors.New("saldo devedor: " + err.Error())
	}
	if v := r.FormValue("min_payment"); v != "" {
		if d.MinPaymentCents, err = money.Parse(v); err != nil {
			return d, errors.New("parcela: " + err.Error())
		}
	}
	// Juros "1,99" (%) viram 199 pontos-base; reaproveita o parser de dinheiro.
	if v := r.FormValue("rate"); v != "" {
		bp, err := money.Parse(v)
		if err != nil {
			return d, errors.New("juros: " + err.Error())
		}
		d.MonthlyRateBP = int(bp)
	}
	if v := r.FormValue("due_day"); v != "" {
		day, err := strconv.Atoi(v)
		if err != nil || day < 1 || day > 31 {
			return d, errors.New("dia de vencimento deve ser de 1 a 31")
		}
		d.DueDay = &day
	}
	return d, nil
}

func (s *Server) deleteDebt(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteDebt(r.Context(), pathID(r))
	redirect(w, r, "/dividas", "Dívida apagada.", err)
}
