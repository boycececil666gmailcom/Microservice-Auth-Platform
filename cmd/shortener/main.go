package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/shortener"
)

// main validates configuration, starts the shortener HTTP server, and coordinates graceful shutdown.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := shortener.ConfigFromEnv()
	if err != nil {
		slog.Error("invalid shortener configuration", "error", err)
		os.Exit(1)
	}
	service, err := shortener.NewServer(ctx, config)
	if err != nil {
		slog.Error("shortener startup failed", "error", err)
		os.Exit(1)
	}
	defer service.Close()
	server := &http.Server{
		Addr:              ":8001",
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("shortener service listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("shortener server failed", "error", err)
		os.Exit(1)
	}
}
