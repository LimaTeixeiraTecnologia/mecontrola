package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/require"
)

type shutdownManager struct {
	manager.Manager
	err    error
	called bool
}

func (m *shutdownManager) Shutdown(context.Context) error {
	m.called = true
	return m.err
}

type shutdownObservability struct {
	observability.Observability
	err    error
	called bool
}

func (o *shutdownObservability) Shutdown(context.Context) error {
	o.called = true
	return o.err
}

func TestRuntimeShutdownDoesNotFailSuccessfulMigrationOnTelemetryFlushError(t *testing.T) {
	dbManager := &shutdownManager{}
	o11y := &shutdownObservability{err: errors.New("collector unavailable")}
	rt := &runtime{dbManager: dbManager, o11y: o11y}

	err := rt.shutdown(context.Background())

	require.NoError(t, err)
	require.True(t, dbManager.called)
	require.True(t, o11y.called)
}

func TestRuntimeShutdownPreservesDatabaseShutdownError(t *testing.T) {
	dbErr := errors.New("database shutdown failed")
	dbManager := &shutdownManager{err: dbErr}
	o11y := &shutdownObservability{}
	rt := &runtime{dbManager: dbManager, o11y: o11y}

	err := rt.shutdown(context.Background())

	require.ErrorIs(t, err, dbErr)
	require.True(t, dbManager.called)
	require.True(t, o11y.called)
}
