package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(context.Background(), os.Getenv, logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("invalid redis url: %w", err)
	}

	client := redis.NewClient(opt)
	defer func() {
		if err := client.Close(); err != nil {
			logger.Error("failed to close redis client", "error", err)
		}
	}()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	if err := client.Ping(pingCtx).Err(); err != nil {
		cancelPing()
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	cancelPing()

	app := newApp(client, cfg, logger)
	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting http server", "addr", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
	case <-shutdownCtx.Done():
		logger.Info("shutting down http server")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("http server shutdown failed: %w", err)
		}

		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed during shutdown: %w", err)
		}
	}

	return nil
}
