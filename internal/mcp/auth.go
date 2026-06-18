package mcp

import (
	"net/http"
)

type apiKeyRoundTripper struct {
	base   http.RoundTripper
	header string
	value  string
}

func newAPIKeyClient(base *http.Client, auth AuthConfig) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	auth = auth.withDefaults()
	value := auth.Key
	if auth.Scheme != "" {
		value = auth.Scheme + " " + auth.Key
	}
	copy := *base
	copy.Transport = &apiKeyRoundTripper{base: rt, header: auth.Header, value: value}
	return &copy
}

func (t *apiKeyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set(t.header, t.value)
	return t.base.RoundTrip(clone)
}
