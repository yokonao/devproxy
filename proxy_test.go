package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		got, err := parseTarget("http://localhost:63015")
		require.NoError(t, err)
		assert.Equal(t, "http", got.url.Scheme)
		assert.Equal(t, "localhost:63015", got.url.Host)
		assert.Nil(t, got.transport)
	})

	t.Run("unix", func(t *testing.T) {
		socket := filepath.Join(t.TempDir(), "app.sock")
		got, err := parseTarget("unix://" + socket)
		require.NoError(t, err)
		assert.Equal(t, "http", got.url.Scheme)
		assert.NotNil(t, got.transport)
	})

	t.Run("errors", func(t *testing.T) {
		for _, raw := range []string{
			"ftp://example.com",
			"http://",
			"unix://",
			"unix:relative.sock",
			"unix://host/socket",
			"unix:///tmp/app.sock?option=value",
			"::://bad",
		} {
			_, err := parseTarget(raw)
			assert.Error(t, err, raw)
		}
	})
}

func TestHTTPProxy(t *testing.T) {
	type requestData struct {
		host             string
		path             string
		query            string
		forwardedFor     string
		forwardedHost    string
		forwardedProto   string
		internalHopValue string
	}
	requests := make(chan requestData, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- requestData{
			host:             r.Host,
			path:             r.URL.Path,
			query:            r.URL.RawQuery,
			forwardedFor:     r.Header.Get("X-Forwarded-For"),
			forwardedHost:    r.Header.Get("X-Forwarded-Host"),
			forwardedProto:   r.Header.Get("X-Forwarded-Proto"),
			internalHopValue: r.Header.Get("X-Internal-Hop"),
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	target, err := parseTarget(upstream.URL + "/base?fixed=1")
	require.NoError(t, err)
	proxy := httptest.NewServer(buildProxy(target))
	t.Cleanup(proxy.Close)

	req, err := http.NewRequest(http.MethodGet, proxy.URL+"/items?request=2", nil)
	require.NoError(t, err)
	req.Host = "api.localhost:3000"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("X-Forwarded-Host", "attacker.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Connection", "X-Internal-Hop")
	req.Header.Set("X-Internal-Hop", "remove-me")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	got := <-requests
	assert.Equal(t, strings.TrimPrefix(upstream.URL, "http://"), got.host)
	assert.Equal(t, "/base/items", got.path)
	assert.Equal(t, "fixed=1&request=2", got.query)
	assert.NotEmpty(t, got.forwardedFor)
	assert.NotContains(t, got.forwardedFor, "203.0.113.10")
	assert.Equal(t, "api.localhost:3000", got.forwardedHost)
	assert.Equal(t, "http", got.forwardedProto)
	assert.Empty(t, got.internalHopValue)
}

func TestUnixProxy(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "upstream.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)

	upstream := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "unix upstream")
	})}
	t.Cleanup(func() {
		_ = upstream.Close()
	})
	go func() {
		_ = upstream.Serve(listener)
	}()

	target, err := parseTarget("unix://" + socket)
	require.NoError(t, err)
	proxy := httptest.NewServer(buildProxy(target))
	t.Cleanup(proxy.Close)

	resp, err := http.Get(proxy.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "unix upstream", string(body))
}
