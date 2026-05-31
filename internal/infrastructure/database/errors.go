// Package database fornece o Manager central, UnitOfWork genérico, migrations embarcadas
// e sentinels de erro para o subsistema Postgres do MeControla.
// Referências: ADR-002 (manager central + UoW), ADR-007 (migrations go:embed), ADR-004 (sentinels).
package database

import "errors"

// ErrConnection indica falha de conectividade ou indisponibilidade do pool Postgres.
// Mapeado para 503 database-unavailable pelo ToProblemDetails de internal/infrastructure/errors.
var ErrConnection = errors.New("database: connection failed")

// ErrMigration indica falha na aplicação ou reversão de uma migration via golang-migrate.
var ErrMigration = errors.New("database: migration apply failed")

// ErrDeadlineExceeded indica que o contexto de uma operação de banco expirou.
// Mapeado para 504 timeout pelo ToProblemDetails.
var ErrDeadlineExceeded = errors.New("database: deadline exceeded")
