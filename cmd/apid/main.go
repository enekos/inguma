// Command apid is inguma's HTTP API server.
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

	"strings"

	"github.com/enekos/inguma/internal/adapters/all"
	"github.com/enekos/inguma/internal/advisories"
	"github.com/enekos/inguma/internal/api"
	"github.com/enekos/inguma/internal/artifacts"
	"github.com/enekos/inguma/internal/auth"
	"github.com/enekos/inguma/internal/db"
	"github.com/enekos/inguma/internal/marrow"
	"github.com/enekos/inguma/internal/pkgstate"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	corpus := flag.String("corpus", "corpus", "path to corpus directory")
	marrowURL := flag.String("marrow", "http://localhost:8080", "Marrow service base URL")
	sqlite := flag.String("sqlite", "./inguma.sqlite", "path to SQLite database file")
	artifactsDir := flag.String("artifacts", "./artifacts", "path to artifacts directory")
	admins := flag.String("admins", os.Getenv("INGUMA_ADMINS"), "comma-separated admin GitHub logins")
	ghClientID := flag.String("gh-client-id", os.Getenv("INGUMA_GH_CLIENT_ID"), "GitHub OAuth client id (empty = auth disabled)")
	ghClientSecret := flag.String("gh-client-secret", os.Getenv("INGUMA_GH_CLIENT_SECRET"), "GitHub OAuth client secret")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	database, err := db.Open(*sqlite)
	if err != nil {
		log.Error("apid: open sqlite", "path", *sqlite, "err", err)
		os.Exit(1)
	}

	store := artifacts.NewFSStore(*artifactsDir)

	s := &api.Server{
		CorpusDir:  *corpus,
		Marrow:     marrow.New(*marrowURL),
		Adapters:   all.Default(),
		Store:      store,
		DB:         database,
		PkgState:   pkgstate.NewStore(database.SQL()),
		Advisories: advisories.NewStore(database.SQL()),
	}
	if *ghClientID != "" {
		adminList := splitCSV(*admins)
		s.AttachAuth(api.NewAuthDeps(
			auth.NewStore(database.SQL(), adminList),
			auth.NewGitHub(*ghClientID, *ghClientSecret),
			adminList,
		))
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

// splitCSV splits a comma-separated string, trims whitespace, and
// drops empty entries.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
