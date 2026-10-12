package web

import (
	"net/http"
	"net/url"
	"strings"
)

// redirect volta para a página com uma mensagem de sucesso ou erro (padrão POST/redirect/GET).
func redirect(w http.ResponseWriter, r *http.Request, path, msg string, err error) {
	q := url.Values{}
	if err != nil {
		q.Set("erro", err.Error())
	} else if msg != "" {
		q.Set("msg", msg)
	}
	path, fragment, _ := strings.Cut(path, "#") // a query vem antes do #ancora
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	if fragment != "" {
		path += "#" + fragment
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
