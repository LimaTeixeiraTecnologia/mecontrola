package card_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
)

type stubDBTX struct{}

func (s *stubDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (s *stubDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (s *stubDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}

type stubManager struct{}

func (s *stubManager) Driver() database.Driver              { return "" }
func (s *stubManager) DBTX(_ context.Context) database.DBTX { return &stubDBTX{} }
func (s *stubManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (s *stubManager) Ping(_ context.Context) error     { return nil }
func (s *stubManager) Shutdown(_ context.Context) error { return nil }

func TestNewCardModule_FieldsNotNil(t *testing.T) {
	o11y := noop.NewProvider()
	mgr := &stubManager{}
	cfg := &configs.Config{}

	passthrough := func(next http.Handler) http.Handler { return next }
	m, err := card.NewCardModule(cfg, o11y, manager.Manager(mgr), passthrough)

	assert.NoError(t, err, "NewCardModule nao deve retornar erro")
	assert.NotNil(t, m.RepositoryFactory, "RepositoryFactory deve ser nao-nil")
	assert.NotNil(t, m.CardRouter, "CardRouter deve ser nao-nil")
	assert.NotNil(t, m.CardLookup, "CardLookup deve ser nao-nil")
}
