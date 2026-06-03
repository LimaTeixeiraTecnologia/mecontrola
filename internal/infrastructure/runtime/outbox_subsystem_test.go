package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

type OutboxSubsystemSuite struct {
	suite.Suite
	ctx context.Context
}

func TestOutboxSubsystem(t *testing.T) {
	suite.Run(t, new(OutboxSubsystemSuite))
}

func (s *OutboxSubsystemSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *OutboxSubsystemSuite) TestName() {
	sub := &lazyOutboxSubsystem{}
	s.Equal("outbox", sub.Name())
}

func (s *OutboxSubsystemSuite) TestStopWithNilSubsystem() {
	sub := &lazyOutboxSubsystem{}
	err := sub.Stop(s.ctx)
	s.NoError(err)
}

type mockOutboxStopper struct {
	err   error
	calls int
}

func (m *mockOutboxStopper) Stop(_ context.Context) error {
	m.calls++
	return m.err
}

func (s *OutboxSubsystemSuite) TestBuildOutboxStopFnClosersInReverseOrder() {
	scenarios := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "deve chamar closers em ordem inversa sem erro",
			wantErr: false,
		},
		{
			name:    "deve acumular erros dos closers",
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			order := make([]int, 0)
			var closerErr error
			if sc.wantErr {
				closerErr = errors.New("closer error")
			}

			closers := []func(context.Context) error{
				func(_ context.Context) error {
					order = append(order, 1)
					return nil
				},
				func(_ context.Context) error {
					order = append(order, 2)
					return closerErr
				},
				func(_ context.Context) error {
					order = append(order, 3)
					return nil
				},
			}

			mock := &mockOutboxStopper{}
			stopFn := buildOutboxStopFn(mock, closers)

			err := stopFn(s.ctx)
			if sc.wantErr {
				s.Error(err)
				s.ErrorContains(err, "closer error")
			} else {
				s.NoError(err)
			}
			// Ordem inversa: 3, 2, 1
			s.Equal([]int{3, 2, 1}, order)
			s.Equal(1, mock.calls)
		})
	}
}

func (s *OutboxSubsystemSuite) TestRegisterSubscriptions() {
	scenarios := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "deve registrar subscriptions sem erro",
			wantErr: false,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			registry := outbox.NewRegistry()
			err := registerSubscriptions(registry)
			if sc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
		})
	}
}

func (s *OutboxSubsystemSuite) TestRegisterSubscriptionsDuplicate() {
	registry := outbox.NewRegistry()
	s.Require().NoError(registerSubscriptions(registry))
	// Chamada duplicada deve retornar erro de duplicidade.
	err := registerSubscriptions(registry)
	s.Error(err)
	s.ErrorIs(err, outbox.ErrDuplicateSubscription)
}
