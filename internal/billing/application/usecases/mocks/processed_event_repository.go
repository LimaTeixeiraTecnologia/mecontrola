package mocks

import (
	"context"
	"time"

	mock "github.com/stretchr/testify/mock"
)

type ProcessedEventRepository struct {
	mock.Mock
}

type ProcessedEventRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *ProcessedEventRepository) EXPECT() *ProcessedEventRepository_Expecter {
	return &ProcessedEventRepository_Expecter{mock: &_m.Mock}
}

func (_m *ProcessedEventRepository) MarkApplied(ctx context.Context, eventKey string, trigger string, recursoID string, occurredAt time.Time) error {
	ret := _m.Called(ctx, eventKey, trigger, recursoID, occurredAt)
	if len(ret) == 0 {
		panic("no return value specified for MarkApplied")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, time.Time) error); ok {
		r0 = rf(ctx, eventKey, trigger, recursoID, occurredAt)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *ProcessedEventRepository) MarkSuperseded(ctx context.Context, eventKey string) error {
	ret := _m.Called(ctx, eventKey)
	if len(ret) == 0 {
		panic("no return value specified for MarkSuperseded")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, eventKey)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func NewProcessedEventRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessedEventRepository {
	m := &ProcessedEventRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
