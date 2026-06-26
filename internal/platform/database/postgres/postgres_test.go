package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSimpleProtocolConnConfigForcesSimpleProtocol(t *testing.T) {
	cfg, err := simpleProtocolConnConfig("postgres://mecontrola:secret@pgbouncer:6432/mecontrola_db?sslmode=disable&search_path=mecontrola,public")
	require.NoError(t, err)
	require.Equal(t, pgx.QueryExecModeSimpleProtocol, cfg.DefaultQueryExecMode)
	require.Equal(t, "mecontrola,public", cfg.RuntimeParams["search_path"])
}

func TestSimpleProtocolConnConfigRejectsInvalidDSN(t *testing.T) {
	_, err := simpleProtocolConnConfig("://not-a-valid-dsn")
	require.Error(t, err)
}

func TestRegisterSimpleProtocolConnReturnsResolvableDSN(t *testing.T) {
	dsn, err := registerSimpleProtocolConn("postgres://mecontrola:secret@pgbouncer:6432/mecontrola_db?sslmode=disable&search_path=mecontrola,public")
	require.NoError(t, err)
	require.NotEmpty(t, dsn)
	require.NotEqual(t, "pgx", dsn)
}
