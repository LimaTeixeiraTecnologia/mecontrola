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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

type SoftDeleteUserSuite struct {
	suite.Suite
}

func TestSoftDeleteUserSuite(t *testing.T) {
	suite.Run(t, new(SoftDeleteUserSuite))
}

func (s *SoftDeleteUserSuite) TestExecute() {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	id, _ := entities.NewUserID("550e8400-e29b-41d4-a716-446655440000")

	scenarios := []struct {
		name    string
		rawID   string
		setup   func(repo *mocks.UserRepository)
		wantErr bool
	}{
		{
			name:  "happy path deleta user",
			rawID: "550e8400-e29b-41d4-a716-446655440000",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().SoftDelete(mock.Anything, id, now).Return(nil)
			},
		},
		{
			name:    "erro de validação de VO — id inválido",
			rawID:   "not-a-uuid",
			setup:   func(_ *mocks.UserRepository) {},
			wantErr: true,
		},
		{
			name:  "erro de repositório",
			rawID: "550e8400-e29b-41d4-a716-446655440000",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().SoftDelete(mock.Anything, id, now).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			repo := mocks.NewUserRepository(s.T())
			uc := usecases.NewSoftDeleteUserUseCase(repo, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
			sc.setup(repo)

			err := uc.Execute(context.Background(), sc.rawID, now)
			if sc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}
