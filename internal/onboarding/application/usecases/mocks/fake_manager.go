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

func (f *FakeManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}

func (f *FakeManager) Ping(_ context.Context) error { return nil }

func (f *FakeManager) Shutdown(_ context.Context) error { return nil }
