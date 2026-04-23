// Command apid is agentpop's HTTP API server.
//
// Usage:
//
//	apid -addr :8090 -corpus corpus -marrow http://localhost:8080
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

	"github.com/enekos/agentpop/internal/adapters/all"
	"github.com/enekos/agentpop/internal/api"
	"github.com/enekos/agentpop/internal/artifacts"
	"github.com/enekos/agentpop/internal/db"
	"github.com/enekos/agentpop/internal/marrow"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	corpus := flag.String("corpus", "corpus", "path to corpus directory")
	marrowURL := flag.String("marrow", "http://localhost:8080", "Marrow service base URL")
	sqlite := flag.String("sqlite", "./agentpop.sqlite", "path to SQLite database file")
	artifactsDir := flag.String("artifacts", "./artifacts", "path to artifacts directory")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	database, err := db.Open(*sqlite)
	if err != nil {
		log.Error("apid: open sqlite", "path", *sqlite, "err", err)
		os.Exit(1)
	}

	store := artifacts.NewFSStore(*artifactsDir)

	s := &api.Server{
		CorpusDir: *corpus,
		Marrow:    marrow.New(*marrowURL),
		Adapters:  all.Default(),
		Store:     store,
		DB:        database,
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("apid listening", "addr", *addr, "corpus", *corpus, "marrow", *marrowURL,
		"sqlite", *sqlite, "artifacts", *artifactsDir)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("apid listen", "err", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = database.Close()
}
