package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

const defaultHealthAddr = ":8081"

type healthServer struct {
	db      *sqlx.DB
	manager *worker.Manager
	server  *http.Server
	ln      net.Listener
}

func newHealthServer(db *sqlx.DB, manager *worker.Manager, addr string) *healthServer {
	if addr == "" {
		addr = defaultHealthAddr
	}

	h := &healthServer{
		db:      db,
		manager: manager,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", h.livez)
	mux.HandleFunc("/readyz", h.readyz)

	h.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return h
}

func (h *healthServer) start(ctx context.Context) error {
	ln, err := net.Listen("tcp", h.server.Addr)
	if err != nil {
		return fmt.Errorf("health server: listen %s: %w", h.server.Addr, err)
	}

	h.ln = ln

	go func() {
		if err := h.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "health server: serve error", "error", err)
		}
	}()

	return nil
}

func (h *healthServer) shutdown(ctx context.Context) error {
	if h.server == nil {
		return nil
	}

	return h.server.Shutdown(ctx)
}

func (h *healthServer) livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *healthServer) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
