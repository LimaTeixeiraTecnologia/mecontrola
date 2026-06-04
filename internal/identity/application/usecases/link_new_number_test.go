package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

type LinkNewNumberSuite struct {
	suite.Suite
}

func TestLinkNewNumberSuite(t *testing.T) {
	suite.Run(t, new(LinkNewNumberSuite))
}

func (s *LinkNewNumberSuite) TestExecute() {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	id, _ := entities.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	number, _ := valueobjects.NewWhatsAppNumber("+5521987654321")

	scenarios := []struct {
		name      string
		rawID     string
		rawNumber string
		reason    string
		setup     func(repo *mocks.UserRepository)
		wantErr   bool
	}{
		{
			name:      "happy path vincula novo número",
			rawID:     "550e8400-e29b-41d4-a716-446655440000",
			rawNumber: "+5521987654321",
			reason:    "troca_de_chip",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().LinkNewNumber(mock.Anything, id, number, "troca_de_chip", now).Return(nil)
			},
		},
		{
			name:      "rejeita reason reservada user_soft_deleted",
			rawID:     "550e8400-e29b-41d4-a716-446655440000",
			rawNumber: "+5521987654321",
			reason:    "user_soft_deleted",
			setup:     func(_ *mocks.UserRepository) {},
			wantErr:   true,
		},
		{
			name:      "erro de validação de VO — id inválido",
			rawID:     "not-a-uuid",
			rawNumber: "+5521987654321",
			reason:    "motivo",
			setup:     func(_ *mocks.UserRepository) {},
			wantErr:   true,
		},
		{
			name:      "erro de validação de VO — número inválido",
			rawID:     "550e8400-e29b-41d4-a716-446655440000",
			rawNumber: "invalid",
			reason:    "motivo",
			setup:     func(_ *mocks.UserRepository) {},
			wantErr:   true,
		},
		{
			name:      "erro de repositório",
			rawID:     "550e8400-e29b-41d4-a716-446655440000",
			rawNumber: "+5521987654321",
			reason:    "troca_de_chip",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().LinkNewNumber(mock.Anything, id, number, "troca_de_chip", now).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			repo := mocks.NewUserRepository(s.T())
			uc := usecases.NewLinkNewNumberUseCase(repo, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
			sc.setup(repo)

			err := uc.Execute(context.Background(), sc.rawID, sc.rawNumber, sc.reason, now)
			if sc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *LinkNewNumberSuite) TestReservadoUserSoftDeleted_ErrorWrap() {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	s.Run("rejeita reason reservada user_soft_deleted com erro tipado", func() {
		repo := mocks.NewUserRepository(s.T())
		uc := usecases.NewLinkNewNumberUseCase(repo, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
		err := uc.Execute(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "+5521987654321", "user_soft_deleted", now)
		s.ErrorIs(err, usecases.ErrReservedReason)
	})
}
