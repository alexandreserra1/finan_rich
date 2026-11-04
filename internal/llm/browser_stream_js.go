package llm

func init() {
	httpClient.Transport = browserStreamTransport{RoundTripper: httpClient.Transport}
}
