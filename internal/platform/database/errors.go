package database

import "errors"

// ErrConnection indica falha de conectividade ou indisponibilidade do pool Postgres.
// Mapeado para 503 database-unavailable pelo ToProblemDetails de internal/platform/errors.
var ErrConnection = errors.New("database: connection failed")

var ErrMigration = errors.New("database: migration apply failed")

// ErrDeadlineExceeded indica que o contexto de uma operação de banco expirou.
// Mapeado para 504 timeout pelo ToProblemDetails.
var ErrDeadlineExceeded = errors.New("database: deadline exceeded")
