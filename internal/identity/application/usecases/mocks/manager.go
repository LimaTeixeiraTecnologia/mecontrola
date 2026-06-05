package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type FakeManager struct {
	DBTXFunc func(ctx context.Context) database.DBTX
}

func NewFakeManager() *FakeManager {
	return &FakeManager{DBTXFunc: func(context.Context) database.DBTX { return nil }}
}

func (f *FakeManager) Driver() database.Driver { return "" }

func (f *FakeManager) DBTX(ctx context.Context) database.DBTX {
	if f.DBTXFunc == nil {
		return nil
	}
	return f.DBTXFunc(ctx)
}

func (f *FakeManager) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	return nil, nil
}

func (f *FakeManager) Ping(ctx context.Context) error { return nil }

func (f *FakeManager) Shutdown(ctx context.Context) error { return nil }
