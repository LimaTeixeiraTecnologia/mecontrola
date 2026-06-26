package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
}

func New(uri string, opts ...Option) (*Database, error) {
	if uri == "" {
		return nil, fmt.Errorf("postgres: uri não pode estar vazia")
	}

	db, err := instrumentDriver("pgx", uri)
	if err != nil {
		return nil, fmt.Errorf("postgres: falha ao abrir conexão: %w", err)
	}

	d := &Database{db: db, closed: false}
	d.applyDefaultPoolConfig()

	for _, opt := range opts {
		opt(d)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: falha ao pingar banco: %w", err)
	}

	return d, nil
}

func (d *Database) applyDefaultPoolConfig() {
	d.db.SetMaxOpenConns(25)
	d.db.SetMaxIdleConns(6)
	d.db.SetConnMaxLifetime(5 * time.Minute)
	d.db.SetConnMaxIdleTime(2 * time.Minute)
}

func (d *Database) DB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return nil
	}

	return d.db
}

func (d *Database) Ping(ctx context.Context) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return fmt.Errorf("postgres: conexão já foi fechada")
	}

	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: falha no ping: %w", err)
	}

	return nil
}

func (d *Database) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	d.closed = true

	done := make(chan error, 1)

	go func() {
		done <- d.db.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("postgres: erro ao fechar conexão: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("postgres: shutdown cancelado: %w", ctx.Err())
	}
}
