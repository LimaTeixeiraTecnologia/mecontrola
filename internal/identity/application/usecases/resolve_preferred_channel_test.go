package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type ResolvePreferredChannelSuite struct {
	suite.Suite
	repo *mocks.UserIdentityRepository
}

func TestResolvePreferredChannel(t *testing.T) {
	suite.Run(t, new(ResolvePreferredChannelSuite))
}

func (s *ResolvePreferredChannelSuite) SetupTest() {
	s.repo = mocks.NewUserIdentityRepository(s.T())
}

func hydrate(s *ResolvePreferredChannelSuite, userID uuid.UUID, channel, externalID string, verifiedAt time.Time, unlinked time.Time) entities.UserIdentity {
	identity, err := entities.HydrateUserIdentity(uuid.New(), userID, channel, externalID, verifiedAt, verifiedAt, unlinked)
	s.Require().NoError(err)
	return identity
}

func (s *ResolvePreferredChannelSuite) TestExecute() {
	now := time.Now().UTC()
	userID := uuid.New()

	scenarios := []struct {
		name      string
		setup     func()
		expectCh  string
		expectExt string
		expectOK  bool
		expectErr bool
	}{
		{
			name: "prefere whatsapp quando ambos presentes",
			setup: func() {
				identities := []entities.UserIdentity{
					hydrate(s, userID, "telegram", "100", now.Add(-time.Hour), time.Time{}),
					hydrate(s, userID, "whatsapp", "+5511999990000", now.Add(-2*time.Hour), time.Time{}),
				}
				s.repo.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
			},
			expectCh:  "whatsapp",
			expectExt: "+5511999990000",
			expectOK:  true,
		},
		{
			name: "retorna telegram quando whatsapp ausente",
			setup: func() {
				identities := []entities.UserIdentity{
					hydrate(s, userID, "telegram", "100", now.Add(-time.Hour), time.Time{}),
				}
				s.repo.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
			},
			expectCh:  "telegram",
			expectExt: "100",
			expectOK:  true,
		},
		{
			name: "ignora identidade unlinked",
			setup: func() {
				identities := []entities.UserIdentity{
					hydrate(s, userID, "whatsapp", "+5511999990001", now.Add(-2*time.Hour), now.Add(-time.Hour)),
					hydrate(s, userID, "telegram", "200", now.Add(-time.Hour), time.Time{}),
				}
				s.repo.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
			},
			expectCh:  "telegram",
			expectExt: "200",
			expectOK:  true,
		},
		{
			name: "retorna ok=false quando sem identidades",
			setup: func() {
				s.repo.EXPECT().ListByUser(mock.Anything, userID).Return([]entities.UserIdentity{}, nil).Once()
			},
			expectOK: false,
		},
		{
			name: "propaga erro do repository",
			setup: func() {
				s.repo.EXPECT().ListByUser(mock.Anything, userID).Return(nil, errors.New("db error")).Once()
			},
			expectErr: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := usecases.NewResolvePreferredChannel(s.repo, noop.NewProvider())
			result, ok, err := uc.Execute(context.Background(), userID)
			if scenario.expectErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(scenario.expectOK, ok)
			if ok {
				s.Equal(scenario.expectCh, result.Channel)
				s.Equal(scenario.expectExt, result.ExternalID)
			}
		})
	}
}
