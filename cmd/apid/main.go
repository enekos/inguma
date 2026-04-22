// Command apid is agentpop's HTTP API server.
//
// Usage:
//
//	apid -addr :8090 -corpus corpus -marrow http://localhost:8080
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/enekos/agentpop/internal/adapters/all"
	"github.com/enekos/agentpop/internal/api"
	"github.com/enekos/agentpop/internal/marrow"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	corpus := flag.String("corpus", "corpus", "path to corpus directory")
	marrowURL := flag.String("marrow", "http://localhost:8080", "Marrow service base URL")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := &api.Server{
		CorpusDir: *corpus,
		Marrow:    marrow.New(*marrowURL),
		Adapters:  all.Default(),
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("apid listening", "addr", *addr, "corpus", *corpus, "marrow", *marrowURL)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("apid shutdown", "err", err)
		os.Exit(1)
	}
}
