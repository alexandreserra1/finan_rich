package gdrive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/drivesync"
	"github.com/alexandreserra1/finan_rich/internal/gdrive/fakedrive"
)

func newClient(t *testing.T) (*Client, *fakedrive.Drive) {
	t.Helper()
	d := fakedrive.New()
	t.Cleanup(d.Close)
	d.RequireToken("acesso-1")
	return &Client{HTTP: d.Client(), Base: d.URL, Token: func(context.Context) (string, error) { return "acesso-1", nil }}, d
}

func items(ids ...string) []drivesync.Item {
	var out []drivesync.Item
	for _, id := range ids {
		out = append(out, drivesync.Item{Type: "note", Record: json.RawMessage(`{"id":"` + id + `","updatedAt":1}`)})
	}
	return out
}

// Arquivo novo é criado na pasta de dados do app; depois é regravado no lugar, e a
// versão muda a cada gravação (é por ela que os aparelhos sabem que algo mudou).
func TestRoundTrip(t *testing.T) {
	c, d := newClient(t)
	ctx := context.Background()
	if f, err := c.Find(ctx); err != nil || f != nil {
		t.Fatalf("Drive vazio: %v %v", f, err)
	}
	created, err := c.Write(ctx, "", drivesync.File{Version: 1, Items: items("a")})
	if err != nil || created.ID == "" || created.Version != "1" {
		t.Fatalf("criar: %+v %v", created, err)
	}
	found, err := c.Find(ctx)
	if err != nil || found == nil || *found != created {
		t.Fatalf("achar: %+v %v", found, err)
	}
	updated, err := c.Write(ctx, created.ID, drivesync.File{Version: 1, Items: items("a", "b")})
	if err != nil || updated.ID != created.ID || updated.Version != "2" {
		t.Fatalf("regravar: %+v %v", updated, err)
	}
	got, err := c.Download(ctx, created.ID)
	if err != nil || len(got.Items) != 2 || string(got.Items[1].Record) != `{"id":"b","updatedAt":1}` {
		t.Fatalf("baixar: %+v %v", got, err)
	}
	if d.Writes() != 2 || !strings.Contains(string(d.Content(drivesync.FileName)), `"id":"b"`) {
		t.Errorf("gravações %d, conteúdo %s", d.Writes(), d.Content(drivesync.FileName))
	}
}

// As recusas do Drive viram as mesmas mensagens do site (js/gdrive.js), sem texto do
// Google além do código; NeedsLogin diz quando só entrar de novo resolve.
func TestErrors(t *testing.T) {
	c, d := newClient(t)
	for _, tc := range []struct {
		status    int
		body      string
		want      string
		needLogin bool
	}{
		{403, `{"error":{"errors":[{"reason":"accessNotConfigured"}]}}`, "não está ativada", false},
		{403, `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED"}]}}`, "não está ativada", false},
		{403, `{"error":{"errors":[{"reason":"insufficientPermissions"}]}}`, "Entre de novo com Google", true},
		{403, `{"error":{"errors":[{"reason":"storageQuotaExceeded"}]}}`, "está cheio", false},
		{403, `{"error":{"errors":[{"reason":"userRateLimitExceeded"}]}}`, "limite de uso", false},
		{403, `{"error":{"errors":[{"reason":"forbidden"},{"reason":"forbidden"},{"reason":"<script>"}]}}`, "Código do Google: forbidden.", false},
		{401, `{"error":{"errors":[{"reason":"authError"}]}}`, "entre de novo com Google", true},
		{500, `não é JSON`, "respondeu 500", false},
	} {
		d.Fail(func(*http.Request) (int, string) { return tc.status, tc.body })
		_, err := c.Find(context.Background())
		var de *Error
		if !errors.As(err, &de) || de.Status != tc.status || !strings.Contains(err.Error(), tc.want) || de.NeedsLogin() != tc.needLogin {
			t.Errorf("%d %s: %v (precisa entrar: %v)", tc.status, tc.body, err, de != nil && de.NeedsLogin())
		}
		if strings.Contains(err.Error(), "<script>") {
			t.Errorf("motivo estranho foi para a mensagem: %v", err)
		}
	}
	// Sem token (a renovação falhou), nada vai para o Drive e o erro é o da renovação.
	d.Fail(nil)
	c.Token = func(context.Context) (string, error) { return "", errors.New("entre de novo com Google") }
	if _, err := c.Find(context.Background()); err == nil || err.Error() != "entre de novo com Google" {
		t.Errorf("sem token: %v", err)
	}
}
