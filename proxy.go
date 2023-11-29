package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type (
	Mapping map[string][]string

	ProxyMap map[string]Backend

	Backend http.Handler

	Proxy struct {
		index   int
		url     *url.URL
		handler *httputil.ReverseProxy
	}

	ErrInvalidTarget struct{ target string }
	ErrNoHandler     struct{}
)

func (e *ErrInvalidTarget) Error() string {
	return "invalid target url: " + e.target + " - host or scheme missing"
}

func (e *ErrNoHandler) Error() string {
	return "handler not set. (use SetMap() to set a handler)"
}

func (m *Mapping) String() string {
	s := make([]string, 0, len(*m))
	for k, v := range *m {
		s = append(s, k+" -> "+strings.Join(v, ", "))
	}
	return strings.Join(s, "\n")
}

func NewProxy(target *url.URL, index int) *Proxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return &Proxy{
		index:   index,
		url:     target,
		handler: proxy,
	}
}

func (p *Proxy) Index() int {
	return p.index
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	p.handler.ServeHTTP(w, r)
}
