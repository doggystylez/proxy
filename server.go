package proxy

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/doggystylez/utils/log"
)

const (
	defaultTimeout = 10 * time.Second
	defaultAddress = ":8080"
	name           = "proxy"
)

type (
	Server struct {
		server  *http.Server
		handler atomic.Value
		logger  *log.Logger
		silent  bool
	}

	Opts struct {
		Handler map[string]Backend
		Timeout time.Duration
		Address string
		Silent  bool
		Logger  *log.Logger
	}
)

func NewServer(opts *Opts) *Server {
	s := &http.Server{
		ReadTimeout:  defaultTimeout,
		WriteTimeout: defaultTimeout,
		Addr:         defaultAddress,
	}
	if opts == nil {
		return &Server{server: s, logger: log.NewLogger(nil)}
	}
	if opts.Timeout != 0 {
		s.ReadTimeout = opts.Timeout
		s.WriteTimeout = opts.Timeout
	}
	if opts.Address != "" {
		s.Addr = opts.Address
	}
	var logger *log.Logger
	if opts.Logger == nil {
		logger = log.NewLogger(nil)
	} else {
		logger = opts.Logger
	}
	server := Server{server: s, logger: logger, silent: opts.Silent}
	if opts.Handler != nil {
		server.SetMap(opts.Handler)
	}
	return &server
}

func (s *Server) SetMap(p map[string]Backend) {
	mux := http.NewServeMux()
	for path, backend := range p {
		pathPrefix := "/" + path + "/"
		mux.Handle(pathPrefix, s.silenceMiddleware(pathPrefix, backend))
	}
	s.logger.Info("handler", "reloaded")
	s.handler.Store(mux)
}

func (s *Server) Run() error {
	s.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentHandler := s.handler.Load().(http.Handler)
		currentHandler.ServeHTTP(w, r)
	})
	s.logger.Info("server", "listening on", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) silenceMiddleware(pathPrefix string, backend Backend) http.Handler {
	return http.StripPrefix(pathPrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("handler", "received request from", r.RemoteAddr, "for path", pathPrefix)
		if s.silent {
			r.Header.Del("User-Agent")
			r.Header.Del("X-Forwarded-Host")
			r.Header.Del("X-Forwarded-For")
			r.Header.Del("X-Forwarded-Proto")
		} else {
			if prior, ok := r.Header["X-Forwarded-For"]; ok {
				r.Header.Set("X-Forwarded-For", prior[0]+", "+r.RemoteAddr)
			} else {
				r.Header.Set("X-Forwarded-For", r.RemoteAddr)
			}
			r.Header.Set("X-Forwarded-Host", r.Host)
			r.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
		}
		backend.ServeHTTP(w, r)
	}))
}
