package mocks

import (
	"context"
	"database/sql"
)

type FakeManager struct{}

func NewFakeManager() *FakeManager { return &FakeManager{} }

func (f *FakeManager) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, nil
}

func (f *FakeManager) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (f *FakeManager) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (f *FakeManager) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}
