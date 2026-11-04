package llm

import (
	"io"
	"net/http"
	"sync/atomic"
)

type browserStreamTransport struct {
	http.RoundTripper
}

func (transport browserStreamTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.RoundTripper.RoundTrip(request)
	if err == nil && response != nil && response.Body != nil {
		response.Body = &browserStreamBody{ReadCloser: response.Body}
	}
	return response, err
}

type browserStreamBody struct {
	io.ReadCloser
	failed atomic.Bool
}

func (body *browserStreamBody) Read(buffer []byte) (int, error) {
	count, err := body.ReadCloser.Read(buffer)
	if err != nil && err != io.EOF {
		body.failed.Store(true)
	}
	return count, err
}

func (body *browserStreamBody) Close() error {
	if body.failed.Load() {
		return nil
	}
	return body.ReadCloser.Close()
}
