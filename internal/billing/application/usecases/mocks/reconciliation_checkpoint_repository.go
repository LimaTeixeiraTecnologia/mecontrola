package mocks

import (
	"context"
	"time"

	mock "github.com/stretchr/testify/mock"
)

type ReconciliationCheckpointRepository struct {
	mock.Mock
}

type ReconciliationCheckpointRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *ReconciliationCheckpointRepository) EXPECT() *ReconciliationCheckpointRepository_Expecter {
	return &ReconciliationCheckpointRepository_Expecter{mock: &_m.Mock}
}

func (_m *ReconciliationCheckpointRepository) Get(ctx context.Context, name string) (time.Time, error) {
	ret := _m.Called(ctx, name)
	if len(ret) == 0 {
		panic("no return value specified for Get")
	}
	var r0 time.Time
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (time.Time, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) time.Time); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Get(0).(time.Time)
	}
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *ReconciliationCheckpointRepository) Set(ctx context.Context, name string, watermark time.Time) error {
	ret := _m.Called(ctx, name, watermark)
	if len(ret) == 0 {
		panic("no return value specified for Set")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time) error); ok {
		r0 = rf(ctx, name, watermark)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func NewReconciliationCheckpointRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *ReconciliationCheckpointRepository {
	m := &ReconciliationCheckpointRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
