package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

type router struct {
	proxies map[string]*httputil.ReverseProxy
	root    *httputil.ReverseProxy
}

func newRouter(cfg *Config) (*router, error) {
	if cfg.Root == "" {
		return nil, errors.New("config: root is required")
	}
	rootTarget, err := parseTarget(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("config: root: %w", err)
	}

	proxies := make(map[string]*httputil.ReverseProxy, len(cfg.Routes))
	for name, raw := range cfg.Routes {
		if !validRouteName(name) {
			return nil, fmt.Errorf("config: invalid route name %q", name)
		}
		t, err := parseTarget(raw)
		if err != nil {
			return nil, fmt.Errorf("config: route %q: %w", name, err)
		}
		proxies[name] = buildProxy(t)
	}

	return &router{proxies: proxies, root: buildProxy(rootTarget)}, nil
}

func validRouteName(name string) bool {
	if name == "" || len(name) > 63 || name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func (rt *router) route(host string) (*httputil.ReverseProxy, string) {
	hostname := host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		hostname = parsed
	}
	hostname = strings.TrimSuffix(strings.ToLower(hostname), ".")
	if hostname == "" || hostname == "localhost" {
		return rt.root, "root"
	}
	name, suffix, hasSubdomain := strings.Cut(hostname, ".")
	if !hasSubdomain || suffix != "localhost" {
		return nil, name
	}
	if p, ok := rt.proxies[name]; ok {
		return p, name
	}
	return nil, name
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	proxy, route := rt.route(r.Host)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	if proxy == nil {
		http.Error(rec, "no route for host", http.StatusNotFound)
	} else {
		proxy.ServeHTTP(rec, r)
	}

	slog.Info("request",
		"method", r.Method,
		"host", r.Host,
		"path", r.URL.Path,
		"route", route,
		"status", rec.status,
		"remote", r.RemoteAddr,
		"duration", time.Since(start),
	)
}
