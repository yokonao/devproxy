package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Run("requires root", func(t *testing.T) {
		_, err := newRouter(&Config{Routes: map[string]string{"a": "http://localhost:1"}})
		assert.Error(t, err)
	})

	t.Run("rejects bad route", func(t *testing.T) {
		_, err := newRouter(&Config{
			Root:   "unix:///tmp/f.sock",
			Routes: map[string]string{"a": "ftp://nope"},
		})
		assert.Error(t, err)
	})

	t.Run("rejects invalid route name", func(t *testing.T) {
		for _, name := range []string{"", "API", "api.example", "-api", "api-", "api_1"} {
			_, err := newRouter(&Config{
				Root:   "http://localhost:8080",
				Routes: map[string]string{name: "http://localhost:8081"},
			})
			assert.Error(t, err, name)
		}
	})
}

func TestRoute(t *testing.T) {
	rt, err := newRouter(&Config{
		Port: 3000,
		Root: "unix:///tmp/root.sock",
		Routes: map[string]string{
			"api": "http://localhost:8081",
			"web": "http://localhost:8082",
		},
	})
	require.NoError(t, err)

	t.Run("known subdomain", func(t *testing.T) {
		proxy, route := rt.route("api.localhost")
		assert.Equal(t, "api", route)
		assert.Same(t, rt.proxies["api"], proxy)
	})

	t.Run("known subdomain with port", func(t *testing.T) {
		proxy, route := rt.route("web.localhost:3000")
		assert.Equal(t, "web", route)
		assert.Same(t, rt.proxies["web"], proxy)
	})

	t.Run("host is case insensitive", func(t *testing.T) {
		proxy, route := rt.route("API.LOCALHOST")
		assert.Equal(t, "api", route)
		assert.Same(t, rt.proxies["api"], proxy)
	})

	t.Run("no subdomain goes to root", func(t *testing.T) {
		for _, host := range []string{"localhost", "localhost:3000", ""} {
			proxy, route := rt.route(host)
			assert.Equal(t, "root", route, host)
			assert.Same(t, rt.root, proxy, host)
		}
	})

	t.Run("unknown subdomain is rejected", func(t *testing.T) {
		proxy, route := rt.route("unknown.localhost")
		assert.Nil(t, proxy)
		assert.Equal(t, "unknown", route)
	})

	t.Run("non-localhost suffix is rejected", func(t *testing.T) {
		proxy, route := rt.route("api.example.test")
		assert.Nil(t, proxy)
		assert.Equal(t, "api", route)
	})
}
