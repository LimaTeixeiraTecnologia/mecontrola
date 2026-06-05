package mocks

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	mock "github.com/stretchr/testify/mock"

	outbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type OutboxRepositoryFactory struct {
	mock.Mock
}

type OutboxRepositoryFactory_Expecter struct {
	mock *mock.Mock
}

func (_m *OutboxRepositoryFactory) EXPECT() *OutboxRepositoryFactory_Expecter {
	return &OutboxRepositoryFactory_Expecter{mock: &_m.Mock}
}

func (_m *OutboxRepositoryFactory) OutboxRepository(db database.DBTX) outbox.OutboxRepository {
	ret := _m.Called(db)

	if len(ret) == 0 {
		panic("no return value specified for OutboxRepository")
	}

	var r0 outbox.OutboxRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) outbox.OutboxRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(outbox.OutboxRepository)
		}
	}

	return r0
}

type OutboxRepositoryFactory_OutboxRepository_Call struct {
	*mock.Call
}

func (_e *OutboxRepositoryFactory_Expecter) OutboxRepository(db interface{}) *OutboxRepositoryFactory_OutboxRepository_Call {
	return &OutboxRepositoryFactory_OutboxRepository_Call{Call: _e.mock.On("OutboxRepository", db)}
}

func (_c *OutboxRepositoryFactory_OutboxRepository_Call) Run(run func(db database.DBTX)) *OutboxRepositoryFactory_OutboxRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(database.DBTX))
	})
	return _c
}

func (_c *OutboxRepositoryFactory_OutboxRepository_Call) Return(_a0 outbox.OutboxRepository) *OutboxRepositoryFactory_OutboxRepository_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *OutboxRepositoryFactory_OutboxRepository_Call) RunAndReturn(run func(database.DBTX) outbox.OutboxRepository) *OutboxRepositoryFactory_OutboxRepository_Call {
	_c.Call.Return(run)
	return _c
}

func NewOutboxRepositoryFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *OutboxRepositoryFactory {
	m := &OutboxRepositoryFactory{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
