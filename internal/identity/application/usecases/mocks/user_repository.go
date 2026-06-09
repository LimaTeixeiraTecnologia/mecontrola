package mocks

import (
	"context"
	"time"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserRepository struct {
	mock.Mock
}

type UserRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *UserRepository) EXPECT() *UserRepository_Expecter {
	return &UserRepository_Expecter{mock: &_m.Mock}
}

func (_m *UserRepository) UpsertByWhatsAppNumber(ctx context.Context, u entities.User, now time.Time) (entities.User, error) {
	ret := _m.Called(ctx, u, now)

	if len(ret) == 0 {
		panic("no return value specified for UpsertByWhatsAppNumber")
	}

	var r0 entities.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, entities.User, time.Time) (entities.User, error)); ok {
		return rf(ctx, u, now)
	}
	if rf, ok := ret.Get(0).(func(context.Context, entities.User, time.Time) entities.User); ok {
		r0 = rf(ctx, u, now)
	} else {
		r0 = ret.Get(0).(entities.User)
	}

	if rf, ok := ret.Get(1).(func(context.Context, entities.User, time.Time) error); ok {
		r1 = rf(ctx, u, now)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type UserRepository_UpsertByWhatsAppNumber_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) UpsertByWhatsAppNumber(ctx, u, now any) *UserRepository_UpsertByWhatsAppNumber_Call {
	return &UserRepository_UpsertByWhatsAppNumber_Call{Call: _e.mock.On("UpsertByWhatsAppNumber", ctx, u, now)}
}

func (_c *UserRepository_UpsertByWhatsAppNumber_Call) Run(run func(ctx context.Context, u entities.User, now time.Time)) *UserRepository_UpsertByWhatsAppNumber_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(entities.User), args[2].(time.Time))
	})
	return _c
}

func (_c *UserRepository_UpsertByWhatsAppNumber_Call) Return(_a0 entities.User, _a1 error) *UserRepository_UpsertByWhatsAppNumber_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *UserRepository_UpsertByWhatsAppNumber_Call) RunAndReturn(run func(context.Context, entities.User, time.Time) (entities.User, error)) *UserRepository_UpsertByWhatsAppNumber_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) FindByID(ctx context.Context, id string) (entities.User, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for FindByID")
	}

	var r0 entities.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (entities.User, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) entities.User); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(entities.User)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type UserRepository_FindByID_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) FindByID(ctx, id any) *UserRepository_FindByID_Call {
	return &UserRepository_FindByID_Call{Call: _e.mock.On("FindByID", ctx, id)}
}

func (_c *UserRepository_FindByID_Call) Run(run func(ctx context.Context, id string)) *UserRepository_FindByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *UserRepository_FindByID_Call) Return(_a0 entities.User, _a1 error) *UserRepository_FindByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *UserRepository_FindByID_Call) RunAndReturn(run func(context.Context, string) (entities.User, error)) *UserRepository_FindByID_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error) {
	ret := _m.Called(ctx, number)

	if len(ret) == 0 {
		panic("no return value specified for FindByWhatsAppNumber")
	}

	var r0 entities.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) (entities.User, error)); ok {
		return rf(ctx, number)
	}
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) entities.User); ok {
		r0 = rf(ctx, number)
	} else {
		r0 = ret.Get(0).(entities.User)
	}

	if rf, ok := ret.Get(1).(func(context.Context, valueobjects.WhatsAppNumber) error); ok {
		r1 = rf(ctx, number)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type UserRepository_FindByWhatsAppNumber_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) FindByWhatsAppNumber(ctx, number any) *UserRepository_FindByWhatsAppNumber_Call {
	return &UserRepository_FindByWhatsAppNumber_Call{Call: _e.mock.On("FindByWhatsAppNumber", ctx, number)}
}

func (_c *UserRepository_FindByWhatsAppNumber_Call) Run(run func(ctx context.Context, number valueobjects.WhatsAppNumber)) *UserRepository_FindByWhatsAppNumber_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(valueobjects.WhatsAppNumber))
	})
	return _c
}

func (_c *UserRepository_FindByWhatsAppNumber_Call) Return(_a0 entities.User, _a1 error) *UserRepository_FindByWhatsAppNumber_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *UserRepository_FindByWhatsAppNumber_Call) RunAndReturn(run func(context.Context, valueobjects.WhatsAppNumber) (entities.User, error)) *UserRepository_FindByWhatsAppNumber_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) FindByWhatsAppNumberIncludingDeleted(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error) {
	ret := _m.Called(ctx, number)

	if len(ret) == 0 {
		panic("no return value specified for FindByWhatsAppNumberIncludingDeleted")
	}

	var r0 entities.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) (entities.User, error)); ok {
		return rf(ctx, number)
	}
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) entities.User); ok {
		r0 = rf(ctx, number)
	} else {
		r0 = ret.Get(0).(entities.User)
	}

	if rf, ok := ret.Get(1).(func(context.Context, valueobjects.WhatsAppNumber) error); ok {
		r1 = rf(ctx, number)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type UserRepository_FindByWhatsAppNumberIncludingDeleted_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) FindByWhatsAppNumberIncludingDeleted(ctx, number any) *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call {
	return &UserRepository_FindByWhatsAppNumberIncludingDeleted_Call{Call: _e.mock.On("FindByWhatsAppNumberIncludingDeleted", ctx, number)}
}

func (_c *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call) Run(run func(ctx context.Context, number valueobjects.WhatsAppNumber)) *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(valueobjects.WhatsAppNumber))
	})
	return _c
}

func (_c *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call) Return(_a0 entities.User, _a1 error) *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call) RunAndReturn(run func(context.Context, valueobjects.WhatsAppNumber) (entities.User, error)) *UserRepository_FindByWhatsAppNumberIncludingDeleted_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) Reanimate(ctx context.Context, u entities.User, now time.Time) (entities.User, error) {
	ret := _m.Called(ctx, u, now)

	if len(ret) == 0 {
		panic("no return value specified for Reanimate")
	}

	var r0 entities.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, entities.User, time.Time) (entities.User, error)); ok {
		return rf(ctx, u, now)
	}
	if rf, ok := ret.Get(0).(func(context.Context, entities.User, time.Time) entities.User); ok {
		r0 = rf(ctx, u, now)
	} else {
		r0 = ret.Get(0).(entities.User)
	}

	if rf, ok := ret.Get(1).(func(context.Context, entities.User, time.Time) error); ok {
		r1 = rf(ctx, u, now)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type UserRepository_Reanimate_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) Reanimate(ctx, u, now any) *UserRepository_Reanimate_Call {
	return &UserRepository_Reanimate_Call{Call: _e.mock.On("Reanimate", ctx, u, now)}
}

func (_c *UserRepository_Reanimate_Call) Run(run func(ctx context.Context, u entities.User, now time.Time)) *UserRepository_Reanimate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(entities.User), args[2].(time.Time))
	})
	return _c
}

func (_c *UserRepository_Reanimate_Call) Return(_a0 entities.User, _a1 error) *UserRepository_Reanimate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *UserRepository_Reanimate_Call) RunAndReturn(run func(context.Context, entities.User, time.Time) (entities.User, error)) *UserRepository_Reanimate_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) MarkDeleted(ctx context.Context, id string, now time.Time) error {
	ret := _m.Called(ctx, id, now)

	if len(ret) == 0 {
		panic("no return value specified for MarkDeleted")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time) error); ok {
		r0 = rf(ctx, id, now)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type UserRepository_MarkDeleted_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) MarkDeleted(ctx, id, now any) *UserRepository_MarkDeleted_Call {
	return &UserRepository_MarkDeleted_Call{Call: _e.mock.On("MarkDeleted", ctx, id, now)}
}

func (_c *UserRepository_MarkDeleted_Call) Run(run func(ctx context.Context, id string, now time.Time)) *UserRepository_MarkDeleted_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(time.Time))
	})
	return _c
}

func (_c *UserRepository_MarkDeleted_Call) Return(_a0 error) *UserRepository_MarkDeleted_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *UserRepository_MarkDeleted_Call) RunAndReturn(run func(context.Context, string, time.Time) error) *UserRepository_MarkDeleted_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) TryFindActiveByWhatsApp(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, bool, error) {
	ret := _m.Called(ctx, number)

	if len(ret) == 0 {
		panic("no return value specified for TryFindActiveByWhatsApp")
	}

	var r0 entities.User
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) (entities.User, bool, error)); ok {
		return rf(ctx, number)
	}
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.WhatsAppNumber) entities.User); ok {
		r0 = rf(ctx, number)
	} else {
		r0 = ret.Get(0).(entities.User)
	}
	if rf, ok := ret.Get(1).(func(context.Context, valueobjects.WhatsAppNumber) bool); ok {
		r1 = rf(ctx, number)
	} else {
		r1 = ret.Bool(1)
	}
	if rf, ok := ret.Get(2).(func(context.Context, valueobjects.WhatsAppNumber) error); ok {
		r2 = rf(ctx, number)
	} else {
		r2 = ret.Error(2)
	}
	return r0, r1, r2
}

type UserRepository_TryFindActiveByWhatsApp_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) TryFindActiveByWhatsApp(ctx, number any) *UserRepository_TryFindActiveByWhatsApp_Call {
	return &UserRepository_TryFindActiveByWhatsApp_Call{Call: _e.mock.On("TryFindActiveByWhatsApp", ctx, number)}
}

func (_c *UserRepository_TryFindActiveByWhatsApp_Call) Run(run func(ctx context.Context, number valueobjects.WhatsAppNumber)) *UserRepository_TryFindActiveByWhatsApp_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(valueobjects.WhatsAppNumber))
	})
	return _c
}

func (_c *UserRepository_TryFindActiveByWhatsApp_Call) Return(_a0 entities.User, _a1 bool, _a2 error) *UserRepository_TryFindActiveByWhatsApp_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *UserRepository_TryFindActiveByWhatsApp_Call) RunAndReturn(run func(context.Context, valueobjects.WhatsAppNumber) (entities.User, bool, error)) *UserRepository_TryFindActiveByWhatsApp_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *UserRepository) AppendWhatsAppHistory(ctx context.Context, userID string, entry interfaces.WhatsAppHistoryEntry) error {
	ret := _m.Called(ctx, userID, entry)

	if len(ret) == 0 {
		panic("no return value specified for AppendWhatsAppHistory")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, interfaces.WhatsAppHistoryEntry) error); ok {
		r0 = rf(ctx, userID, entry)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type UserRepository_AppendWhatsAppHistory_Call struct {
	*mock.Call
}

func (_e *UserRepository_Expecter) AppendWhatsAppHistory(ctx, userID, entry any) *UserRepository_AppendWhatsAppHistory_Call {
	return &UserRepository_AppendWhatsAppHistory_Call{Call: _e.mock.On("AppendWhatsAppHistory", ctx, userID, entry)}
}

func (_c *UserRepository_AppendWhatsAppHistory_Call) Run(run func(ctx context.Context, userID string, entry interfaces.WhatsAppHistoryEntry)) *UserRepository_AppendWhatsAppHistory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(interfaces.WhatsAppHistoryEntry))
	})
	return _c
}

func (_c *UserRepository_AppendWhatsAppHistory_Call) Return(_a0 error) *UserRepository_AppendWhatsAppHistory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *UserRepository_AppendWhatsAppHistory_Call) RunAndReturn(run func(context.Context, string, interfaces.WhatsAppHistoryEntry) error) *UserRepository_AppendWhatsAppHistory_Call {
	_c.Call.Return(run)
	return _c
}

func NewUserRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *UserRepository {
	m := &UserRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
