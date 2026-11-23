// Package gdrive fala com a API do Google Drive, só na pasta de dados do app
// (appDataFolder), onde fica o arquivo da sincronização (drivesync.FileName). Faz no
// programa do computador o que js/gdrive.js faz no site. Fica fora do drivesync
// porque aquele pacote também vai para o WebAssembly, que não precisa de net/http.
package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/drivesync"
)

// DefaultBase é o endereço da API do Google.
const DefaultBase = "https://www.googleapis.com"

// maxFile limita o arquivo baixado: anos de lançamentos dão alguns MB.
const maxFile = 64 << 20

// Client fala com o Drive da pessoa. Token devolve um token de acesso válido (quem
// chama renova quando vence).
type Client struct {
	HTTP  *http.Client
	Base  string
	Token func(context.Context) (string, error)
}

// File é o arquivo no Drive: o ID e a versão, que muda a cada gravação.
type File struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Error é uma recusa do Drive, com os motivos que o Google manda (ex.:
// "accessNotConfigured"). A mensagem é a mesma do site.
type Error struct {
	Status  int
	Reasons []string
}

var code = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,60}$`)

func (e *Error) has(reasons ...string) bool {
	for _, r := range e.Reasons {
		if slices.Contains(reasons, r) {
			return true
		}
	}
	return false
}

func (e *Error) Error() string {
	switch {
	case e.Status == http.StatusUnauthorized:
		return "o Google recusou o acesso; entre de novo com Google"
	case e.Status != http.StatusForbidden:
		return fmt.Sprintf("o Google Drive respondeu %d; tente de novo", e.Status)
	case e.has("accessNotConfigured", "SERVICE_DISABLED"):
		return "A API do Google Drive não está ativada no projeto do login da Rich. Quem administra a Rich precisa ativá-la no Google Cloud e tentar novamente."
	case e.has("insufficientPermissions", "ACCESS_TOKEN_SCOPE_INSUFFICIENT"):
		return "A Rich não recebeu permissão para usar a pasta de dados do app no seu Google Drive. Entre de novo com Google e permita esse acesso."
	case e.has("storageQuotaExceeded"):
		return "O armazenamento do seu Google Drive está cheio. Libere espaço e tente sincronizar novamente. Seus dados continuam neste computador."
	case e.has("rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded"):
		return "O Google Drive atingiu um limite de uso. Espere um pouco antes de tentar sincronizar novamente. Seus dados continuam neste computador."
	}
	msg := "O Google Drive recusou o acesso (403). Confira as permissões da conta e a configuração do projeto antes de tentar novamente."
	// O código do Google (ex.: "forbidden") é só um identificador, sem dado pessoal:
	// deixa a pessoa dizer a causa. Só entra o que tem cara de código.
	var codes []string
	for _, r := range e.Reasons {
		if code.MatchString(r) && !slices.Contains(codes, r) {
			codes = append(codes, r)
		}
	}
	if len(codes) > 0 {
		msg += " Código do Google: " + strings.Join(codes, ", ") + "."
	}
	return msg
}

// NeedsLogin diz se só entrar de novo com Google resolve: o token foi recusado ou a
// permissão da pasta de dados do app faltou.
func (e *Error) NeedsLogin() bool {
	return e.Status == http.StatusUnauthorized || e.has("insufficientPermissions", "ACCESS_TOKEN_SCOPE_INSUFFICIENT")
}

// Find devolve o arquivo da sincronização, ou nil se ainda não existe.
func (c *Client) Find(ctx context.Context) (*File, error) {
	q := url.Values{"spaces": {"appDataFolder"}, "q": {"name = '" + drivesync.FileName + "'"}, "fields": {"files(id,version)"}, "pageSize": {"10"}}
	var out struct {
		Files []File `json:"files"`
	}
	if err := c.call(ctx, http.MethodGet, c.Base+"/drive/v3/files?"+q.Encode(), "", nil, &out); err != nil {
		return nil, err
	}
	if len(out.Files) == 0 {
		return nil, nil
	}
	return &out.Files[0], nil // o mesmo que o site escolhe (files[0])
}

// Download baixa o conteúdo do arquivo.
func (c *Client) Download(ctx context.Context, id string) (drivesync.File, error) {
	var f drivesync.File
	err := c.call(ctx, http.MethodGet, c.Base+"/drive/v3/files/"+url.PathEscape(id)+"?alt=media", "", nil, &f)
	return f, err
}

// Write regrava o arquivo inteiro; com id vazio, cria na pasta de dados do app.
func (c *Client) Write(ctx context.Context, id string, content drivesync.File) (File, error) {
	body, err := json.Marshal(content)
	if err != nil {
		return File{}, err
	}
	var saved File
	if id != "" {
		err = c.call(ctx, http.MethodPatch, c.Base+"/upload/drive/v3/files/"+url.PathEscape(id)+"?uploadType=media&fields=id,version", "application/json", bytes.NewReader(body), &saved)
		return saved, err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	meta, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	json.NewEncoder(meta).Encode(map[string]any{"name": drivesync.FileName, "parents": []string{"appDataFolder"}})
	data, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	data.Write(body)
	mw.Close()
	err = c.call(ctx, http.MethodPost, c.Base+"/upload/drive/v3/files?uploadType=multipart&fields=id,version", "multipart/related; boundary="+mw.Boundary(), &buf, &saved)
	return saved, err
}

func (c *Client) call(ctx context.Context, method, u, contentType string, body io.Reader, out any) error {
	token, err := c.Token(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("o Google Drive não respondeu; confira a internet e tente de novo")
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return readError(resp)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxFile)).Decode(out); err != nil {
		return errors.New("o Google Drive mandou uma resposta que não deu para ler; tente de novo")
	}
	return nil
}

// readError lê os motivos de error.errors[].reason e error.details[].reason (ErrorInfo).
func readError(resp *http.Response) error {
	var body struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Details []struct {
				Type   string `json:"@type"`
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
	e := &Error{Status: resp.StatusCode}
	for _, it := range body.Error.Errors {
		e.Reasons = append(e.Reasons, it.Reason)
	}
	for _, it := range body.Error.Details {
		if it.Type == "type.googleapis.com/google.rpc.ErrorInfo" {
			e.Reasons = append(e.Reasons, it.Reason)
		}
	}
	return e
}
