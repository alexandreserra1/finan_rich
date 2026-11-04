package llm

import (
	"errors"
	"io"
	"net/http"
	"testing"
)

type browserBodyFixture struct {
	readErr  error
	closeErr error
	closed   bool
}

type browserTransportFixture struct {
	response *http.Response
	err      error
	request  *http.Request
}

func (transport *browserTransportFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.request = request
	return transport.response, transport.err
}

func TestBrowserStreamTransport(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		response *http.Response
		err      error
	}{
		{"corpo recebe a proteção", &http.Response{Body: &browserBodyFixture{}}, nil},
		{"resposta sem corpo continua igual", &http.Response{}, nil},
		{"erro do transporte continua igual", nil, errors.New("a rede falhou")},
	} {
		t.Run(test.name, func(t *testing.T) {
			underlying := &browserTransportFixture{response: test.response, err: test.err}
			transport := browserStreamTransport{RoundTripper: underlying}
			response, gotErr := transport.RoundTrip(request)
			if underlying.request != request || response != test.response || gotErr != test.err {
				t.Fatal("o transporte alterou a requisição, a resposta ou o erro")
			}
			if response != nil && response.Body != nil {
				if _, ok := response.Body.(*browserStreamBody); !ok {
					t.Fatal("o corpo da resposta ficou sem proteção")
				}
			}
		})
	}
}

func (body *browserBodyFixture) Read(buffer []byte) (int, error) {
	return copy(buffer, "ok"), body.readErr
}

func (body *browserBodyFixture) Close() error {
	body.closed = true
	return body.closeErr
}

func TestBrowserStreamBody(t *testing.T) {
	networkErr := errors.New("a conexão caiu")
	closeErr := errors.New("não foi possível fechar")
	for _, test := range []struct {
		name      string
		read      bool
		readErr   error
		wantClose bool
	}{
		{"leitura interrompida não cancela de novo", true, networkErr, false},
		{"fim normal fecha normalmente", true, io.EOF, true},
		{"leitura sem erro fecha normalmente", true, nil, true},
		{"saída antes da leitura fecha normalmente", false, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			underlying := &browserBodyFixture{readErr: test.readErr, closeErr: closeErr}
			body := &browserStreamBody{ReadCloser: underlying}
			if test.read {
				buffer := make([]byte, 2)
				count, err := body.Read(buffer)
				if count != 2 || string(buffer) != "ok" || err != test.readErr {
					t.Fatalf("leitura alterada: %d, %q, %v", count, buffer, err)
				}
			}
			err := body.Close()
			if underlying.closed != test.wantClose {
				t.Fatalf("fechou o corpo: %v; esperado: %v", underlying.closed, test.wantClose)
			}
			if test.wantClose && err != closeErr || !test.wantClose && err != nil {
				t.Fatalf("erro ao fechar: %v", err)
			}
		})
	}
}
