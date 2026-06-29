//go:build integration || e2e

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const (
	testImage           = "pgvector/pgvector:pg16"
	testUser            = "test"
	testPassword        = "test"
	testStartupTTL      = 90 * time.Second
	testOpTTL           = 30 * time.Second
	testCleanupTTL      = 15 * time.Second
	testMigrationTTL    = 120 * time.Second
	testSearchPath      = "mecontrola,public"
	testMigrationSchema = "mecontrola"
)

var (
	containerOnce    sync.Once
	containerHost    string
	containerPort    int
	containerInitErr error

	adminOnce sync.Once
	adminDB   *sql.DB
	adminErr  error

	dbCounter atomic.Int64
)

func NewTestDatabase(t *testing.T) (*sqlx.DB, string) {
	t.Helper()

	containerOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testStartupTTL)
		defer cancel()
		containerHost, containerPort, containerInitErr = startTestContainer(ctx)
	})
	if containerInitErr != nil {
		t.Fatalf("start postgres container: %v", containerInitErr)
	}

	adminOnce.Do(func() {
		adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", testUser, testPassword, containerHost, containerPort)
		adminDB, adminErr = sql.Open("pgx", adminDSN)
		if adminErr != nil {
			return
		}
		adminErr = adminDB.Ping()
	})
	if adminErr != nil {
		t.Fatalf("create admin connection: %v", adminErr)
	}

	dbName := fmt.Sprintf("testdb_%d", dbCounter.Add(1))

	createCtx, cancelCreate := context.WithTimeout(context.Background(), testOpTTL)
	defer cancelCreate()
	if _, err := adminDB.ExecContext(createCtx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create database %s: %v", dbName, err)
	}

	dropDB := func() {
		ctx, cancel := context.WithTimeout(context.Background(), testCleanupTTL)
		defer cancel()
		_, _ = adminDB.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&search_path=%s", testUser, testPassword, containerHost, containerPort, dbName, testSearchPath)

	if err := runTestMigrations(dsn); err != nil {
		dropDB()
		t.Fatalf("run migrations: %v", err)
	}

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		dropDB()
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		dropDB()
		t.Fatalf("ping test database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		dropDB()
	})

	return db, dsn
}

func runTestMigrations(dsn string) error {
	migrateDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer func() { _ = migrateDB.Close() }()

	if _, err := migrateDB.ExecContext(context.Background(), `CREATE SCHEMA IF NOT EXISTS mecontrola`); err != nil {
		return fmt.Errorf("ensure mecontrola schema: %w", err)
	}

	driver, err := migratepgx.WithInstance(migrateDB, &migratepgx.Config{
		MigrationsTable:       migratepgx.DefaultMigrationsTable,
		SchemaName:            testMigrationSchema,
		MigrationsTableQuoted: false,
	})
	if err != nil {
		return fmt.Errorf("build migration driver: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("build migration source: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("build migrator: %w", err)
	}

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func startTestContainer(ctx context.Context) (string, int, error) {
	req := tc.ContainerRequest{
		Image:        testImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     testUser,
			"POSTGRES_PASSWORD": testPassword,
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(testStartupTTL),
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
