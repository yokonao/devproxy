package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		path := writeConfig(t, "port: 3000\nroot: http://127.0.0.1:8080\nroutes:\n  api: http://127.0.0.1:8081\n")
		cfg, err := loadConfig(path)
		require.NoError(t, err)
		assert.Equal(t, 3000, cfg.Port)
		assert.Equal(t, "http://127.0.0.1:8080", cfg.Root)
		assert.Equal(t, "http://127.0.0.1:8081", cfg.Routes["api"])
	})

	for name, data := range map[string]string{
		"unknown field": "port: 3000\nroot: http://127.0.0.1:8080\nextra: true\n",
		"zero port":     "port: 0\nroot: http://127.0.0.1:8080\n",
		"large port":    "port: 65536\nroot: http://127.0.0.1:8080\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfig(writeConfig(t, data))
			assert.Error(t, err)
		})
	}
}

func writeConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	return path
}
