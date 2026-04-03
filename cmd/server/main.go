package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jason9075/agents_of_empires/internal/api"
	"github.com/jason9075/agents_of_empires/internal/ticker"
	"github.com/jason9075/agents_of_empires/internal/world"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	seed := flag.Int64("seed", 42, "World generation seed")
	tickInterval := flag.Duration("tick", ticker.DefaultInterval, "Game tick interval")
	webDir := flag.String("web-dir", "./web", "Directory of static frontend files")
	flag.Parse()

	slog.Info("initializing world", "seed", *seed)
	w := world.NewWorld(*seed)

	queue := ticker.NewQueue()
	t := ticker.New(w, queue, *tickInterval)
	t.Start()
	slog.Info("game loop started", "interval", *tickInterval)

	srv := &http.Server{
		Addr:    *addr,
		Handler: api.NewServer(w, queue, *webDir),
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server listening", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	t.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("stopped")
}
