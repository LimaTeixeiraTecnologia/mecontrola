//go:build integration

package main_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type CmdIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	binaryPath  string
	pgContainer *tcpostgres.PostgresContainer
	pgConnStr   string
}

func TestCmdIntegration(t *testing.T) {
	suite.Run(t, new(CmdIntegrationSuite))
}

func (s *CmdIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	binPath := "/tmp/mecontrola-test"
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd")
	buildCmd.Dir = projectRoot()
	out, err := buildCmd.CombinedOutput()
	s.Require().NoError(err, "go build failed: %s", string(out))
	s.binaryPath = binPath

	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mecontrola_db"),
		tcpostgres.WithUsername("mecontrola"),
		tcpostgres.WithPassword("mecontrola_password"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err, "failed to start postgres container")
	s.pgContainer = container

	connStr, err := container.ConnectionString(s.ctx, "sslmode=disable")
	s.Require().NoError(err)
	s.pgConnStr = connStr
}

func (s *CmdIntegrationSuite) TearDownSuite() {
	if s.binaryPath != "" {
		_ = os.Remove(s.binaryPath)
	}
	if s.pgContainer != nil {
		_ = s.pgContainer.Terminate(s.ctx)
	}
}

func (s *CmdIntegrationSuite) TestHelpSubcommands() {
	scenarios := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  string
	}{
		{
			name:     "root help",
			args:     []string{"--help"},
			wantExit: 0,
			wantOut:  "mecontrola",
		},
		{
			name:     "server help",
			args:     []string{"server", "--help"},
			wantExit: 0,
			wantOut:  "server",
		},
		{
			name:     "worker help",
			args:     []string{"worker", "--help"},
			wantExit: 0,
			wantOut:  "worker",
		},
		{
			name:     "migrate help",
			args:     []string{"migrate", "--help"},
			wantExit: 0,
			wantOut:  "migrate",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			cmd := exec.Command(s.binaryPath, sc.args...)
			out, err := cmd.CombinedOutput()

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				}
			}

			s.Equal(sc.wantExit, exitCode, "unexpected exit code for %v: output=%s", sc.args, string(out))
			s.Contains(strings.ToLower(string(out)), strings.ToLower(sc.wantOut),
				"expected output to contain %q, got: %s", sc.wantOut, string(out))
		})
	}
}

func (s *CmdIntegrationSuite) TestMigrateApplied() {
	host, err := s.pgContainer.Host(s.ctx)
	s.Require().NoError(err)

	port, err := s.pgContainer.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	cmd := exec.CommandContext(s.ctx, s.binaryPath, "migrate")
	cmd.Dir = projectRoot()
	cmd.Env = s.buildEnv(host, int(port.Num()), 18080)

	out, err := cmd.CombinedOutput()
	s.Require().NoError(err, "migrate command failed: %s", string(out))

	db, err := sql.Open("pgx", s.pgConnStr)
	s.Require().NoError(err)
	defer func() { _ = db.Close() }()

	var count int
	err = db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM health_probe").Scan(&count)
	s.Require().NoError(err, "health_probe table should exist after migration")
	s.Greater(count, 0, "health_probe should have at least one row")
}

func (s *CmdIntegrationSuite) TestServerHealthEndpoints() {
	host, err := s.pgContainer.Host(s.ctx)
	s.Require().NoError(err)

	port, err := s.pgContainer.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	migrateCmd := exec.CommandContext(s.ctx, s.binaryPath, "migrate")
	migrateCmd.Dir = projectRoot()
	migrateCmd.Env = s.buildEnv(host, int(port.Num()), 18081)
	_, _ = migrateCmd.CombinedOutput()

	serverCmd := exec.CommandContext(s.ctx, s.binaryPath, "server")
	serverCmd.Dir = projectRoot()
	serverCmd.Env = s.buildEnv(host, int(port.Num()), 18081)
	s.Require().NoError(serverCmd.Start())

	defer func() {
		if serverCmd.Process != nil {
			_ = serverCmd.Process.Kill()
			_ = serverCmd.Wait()
		}
	}()

	s.Require().Eventually(func() bool {
		resp, err := http.Get("http://localhost:18081/health") //nolint:noctx
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 15*time.Second, 500*time.Millisecond, "server did not start within timeout")

	for _, path := range []string{"/health", "/ready"} {
		s.Run("GET "+path, func() {
			resp, err := http.Get("http://localhost:18081" + path) //nolint:noctx
			s.Require().NoError(err)
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)
			s.Equal(http.StatusOK, resp.StatusCode,
				"expected 200 from %s, got %d: %s", path, resp.StatusCode, string(body))
		})
	}
}

func (s *CmdIntegrationSuite) TestWorkerStartsIdle() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	host, err := s.pgContainer.Host(s.ctx)
	s.Require().NoError(err)

	port, err := s.pgContainer.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	workerCmd := exec.CommandContext(ctx, s.binaryPath, "worker")
	workerCmd.Dir = projectRoot()
	workerCmd.Env = s.buildEnv(host, int(port.Num()), 18082)

	var sb strings.Builder
	workerCmd.Stdout = &sb
	workerCmd.Stderr = &sb

	s.Require().NoError(workerCmd.Start())

	time.Sleep(2 * time.Second)

	if workerCmd.Process != nil {
		_ = workerCmd.Process.Kill()
	}
	_ = workerCmd.Wait()

	output := sb.String()
	s.T().Logf("worker output: %s", output)
	s.Contains(strings.ToLower(output), "worker", "worker process should emit at least one log line containing 'worker'")
}

func (s *CmdIntegrationSuite) buildEnv(dbHost string, dbPort int, httpPort int) []string {
	return append(os.Environ(),
		"ENVIRONMENT=local",
		fmt.Sprintf("DB_HOST=%s", dbHost),
		fmt.Sprintf("DB_PORT=%d", dbPort),
		"DB_USER=mecontrola",
		"DB_PASSWORD=mecontrola_password",
		"DB_NAME=mecontrola_db",
		"DB_SSL_MODE=disable",
		fmt.Sprintf("PORT=%d", httpPort),
		"SERVICE_NAME_API=mecontrola-test",
		"CORS_ALLOWED_ORIGINS=http://localhost:3000",
		"LOG_LEVEL=info",
		"LOG_FORMAT=json",
		"OTEL_TRACE_SAMPLE_RATE=0",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317",
	)
}

func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir != "/" {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		dir = dir[:strings.LastIndex(dir, "/")]
	}
	return "."
}
