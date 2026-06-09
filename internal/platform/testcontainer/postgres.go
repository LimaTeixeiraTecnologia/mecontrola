package testcontainer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const (
	pgImage           = "postgres:16-alpine"
	containerStartTTL = 90 * time.Second
	dbOpTTL           = 30 * time.Second
	cleanupTTL        = 15 * time.Second
	migrationTTL      = 120 * time.Second
)

var (
	containerOnce    sync.Once
	containerHost    string
	containerPort    int
	containerInitErr error

	adminOnce sync.Once
	adminMgr  manager.Manager
	adminErr  error

	dbCounter atomic.Int64
)

func Postgres(t *testing.T) (manager.Manager, string) {
	t.Helper()

	containerOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), containerStartTTL)
		defer cancel()
		containerHost, containerPort, containerInitErr = startContainer(ctx)
	})
	if containerInitErr != nil {
		t.Fatalf("start postgres container: %v", containerInitErr)
	}

	adminOnce.Do(func() {
		adminDSN := fmt.Sprintf("postgres://test:test@%s:%d/postgres?sslmode=disable", containerHost, containerPort)
		adminMgr, adminErr = manager.New(dbpostgres.PostgresConfig{DSN: adminDSN})
	})
	if adminErr != nil {
		t.Fatalf("create admin manager: %v", adminErr)
	}

	dbName := fmt.Sprintf("testdb_%d", dbCounter.Add(1))

	createCtx, cancelCreate := context.WithTimeout(context.Background(), dbOpTTL)
	defer cancelCreate()
	if _, err := adminMgr.DBTX(createCtx).ExecContext(createCtx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create database %s: %v", dbName, err)
	}

	dropDB := func() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTTL)
		defer cancel()
		_, _ = adminMgr.DBTX(ctx).ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	}

	dsn := fmt.Sprintf("postgres://test:test@%s:%d/%s?sslmode=disable&search_path=mecontrola,public", containerHost, containerPort, dbName)
	pgxDSN := fmt.Sprintf("pgx5://test:test@%s:%d/%s?sslmode=disable", containerHost, containerPort, dbName)

	mgr, err := manager.New(dbpostgres.PostgresConfig{DSN: dsn})
	if err != nil {
		dropDB()
		t.Fatalf("create manager: %v", err)
	}

	migrCtx, cancelMigr := context.WithTimeout(context.Background(), migrationTTL)
	defer cancelMigr()

	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(pgxDSN))
	if err != nil {
		shutdownCtx, cancelShut := context.WithTimeout(context.Background(), cleanupTTL)
		_ = mgr.Shutdown(shutdownCtx)
		cancelShut()
		dropDB()
		t.Fatalf("create migrator: %v", err)
	}

	if err := migrator.Up(migrCtx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		shutdownCtx, cancelShut := context.WithTimeout(context.Background(), cleanupTTL)
		_ = mgr.Shutdown(shutdownCtx)
		cancelShut()
		dropDB()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, cancelShut := context.WithTimeout(context.Background(), cleanupTTL)
		_ = mgr.Shutdown(shutdownCtx)
		cancelShut()
		dropDB()
	})

	return mgr, pgxDSN
}

func startContainer(ctx context.Context) (string, int, error) {
	req := tc.ContainerRequest{
		Image:        pgImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(containerStartTTL),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            false,
	})
	if err != nil {
		return "", 0, fmt.Errorf("generic container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", 0, fmt.Errorf("container host: %w", err)
	}

	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", 0, fmt.Errorf("container mapped port: %w", err)
	}

	portNum, err := strconv.Atoi(mapped.Port())
	if err != nil {
		_ = container.Terminate(ctx)
		return "", 0, fmt.Errorf("parse port: %w", err)
	}

	return host, portNum, nil
}
