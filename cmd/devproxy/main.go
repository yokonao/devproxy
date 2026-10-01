package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/devproxy/internal/proxy"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func newServer(cfg *proxy.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func shutdownServer(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}

func newRootCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:           "devproxy",
		Short:         "Route local development traffic by hostname",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), configPath)
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.Flags().StringVar(&configPath, "config", proxy.DefaultConfigPath(), "path to the YAML config file")

	return cmd
}

func run(ctx context.Context, configPath string) error {
	cfg, err := proxy.LoadConfig(configPath)
	if err != nil {
		return err
	}
	rt, err := proxy.NewRouter(cfg)
	if err != nil {
		return err
	}

	srv := newServer(cfg, rt)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting devproxy", "addr", srv.Addr, "routes", rt.Len())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		return shutdownServer(srv, shutdownTimeout)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
