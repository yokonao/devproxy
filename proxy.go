package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
)

type target struct {
	url       *url.URL
	transport http.RoundTripper
}

func parseTarget(raw string) (target, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return target{}, fmt.Errorf("invalid target %q: %w", raw, err)
	}

	switch u.Scheme {
	case "http", "https":
		if u.Host == "" {
			return target{}, fmt.Errorf("invalid target %q: missing host", raw)
		}
		return target{url: u}, nil
	case "unix":
		if u.Host != "" || !filepath.IsAbs(u.Path) || u.RawQuery != "" || u.Fragment != "" {
			return target{}, fmt.Errorf("invalid target %q: unix target must contain only an absolute socket path", raw)
		}
		socket := u.Path
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
		}
		return target{url: &url.URL{Scheme: "http", Host: "localhost"}, transport: transport}, nil
	default:
		return target{}, fmt.Errorf("invalid target %q: unsupported scheme %q", raw, u.Scheme)
	}
}

func buildProxy(t target) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(t.url)
			pr.SetXForwarded()
		},
		Transport: t.transport,
	}
}
