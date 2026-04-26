package main

import (
	"bufio"
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/api"
	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func main() {
	loadDotEnv(".env")

	defaultAddr := defaultListenAddr()
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	seed := flag.Int64("seed", 42, "World generation seed")
	tickInterval := flag.Duration("tick", ticker.DefaultInterval, "Game tick interval")
	webDir := flag.String("web-dir", "./web", "Directory of static frontend files")
	faction1 := flag.String("faction1", "linux", "Faction for team 1 (e.g. linux, microsoft)")
	variant1 := flag.String("variant1", "blue", "Colour variant for team 1")
	faction2 := flag.String("faction2", "microsoft", "Faction for team 2 (e.g. linux, microsoft)")
	variant2 := flag.String("variant2", "red", "Colour variant for team 2")
	flag.Parse()

	slog.Info("initializing world", "seed", *seed)
	w := world.NewWorld(*seed)
	w.SetTeamAppearance(entity.Team1, world.TeamAppearance{Faction: *faction1, Variant: *variant1})
	w.SetTeamAppearance(entity.Team2, world.TeamAppearance{Faction: *faction2, Variant: *variant2})
	slog.Info("team appearances set",
		"team1", *faction1+"/"+*variant1,
		"team2", *faction2+"/"+*variant2,
	)

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

func defaultListenAddr() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENV"))) {
	case "prod", "production":
		return ":8080"
	default:
		return "127.0.0.1:8080"
	}
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		// Respect variables already set by the shell or process manager.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
