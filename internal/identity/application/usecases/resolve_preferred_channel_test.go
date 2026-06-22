package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type ResolvePreferredChannelSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repoMock *mocks.UserIdentityRepository
}

func TestResolvePreferredChannel(t *testing.T) {
	suite.Run(t, new(ResolvePreferredChannelSuite))
}

func (s *ResolvePreferredChannelSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = mocks.NewUserIdentityRepository(s.T())
}

func hydrateIdentity(s *ResolvePreferredChannelSuite, userID uuid.UUID, channel, externalID string, verifiedAt time.Time, unlinked time.Time) entities.UserIdentity {
	identity, err := entities.HydrateUserIdentity(uuid.New(), userID, channel, externalID, verifiedAt, verifiedAt, unlinked)
	s.Require().NoError(err)
	return identity
}

func (s *ResolvePreferredChannelSuite) TestExecute() {
	now := time.Now().UTC()
	userID := uuid.New()

	type dependencies struct {
		repoMock *mocks.UserIdentityRepository
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expectCh     string
		expectExt    string
		expectOK     bool
		expectErr    bool
	}{
		{
			name: "prefere whatsapp quando ambos presentes",
			dependencies: dependencies{
				repoMock: func() *mocks.UserIdentityRepository {
					identities := []entities.UserIdentity{
						hydrateIdentity(s, userID, "telegram", "100", now.Add(-time.Hour), time.Time{}),
						hydrateIdentity(s, userID, "whatsapp", "+5511999990000", now.Add(-2*time.Hour), time.Time{}),
					}
					s.repoMock.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
					return s.repoMock
				}(),
			},
			expectCh:  "whatsapp",
			expectExt: "+5511999990000",
			expectOK:  true,
		},
		{
			name: "retorna telegram quando whatsapp ausente",
			dependencies: dependencies{
				repoMock: func() *mocks.UserIdentityRepository {
					identities := []entities.UserIdentity{
						hydrateIdentity(s, userID, "telegram", "100", now.Add(-time.Hour), time.Time{}),
					}
					s.repoMock.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
					return s.repoMock
				}(),
			},
			expectCh:  "telegram",
			expectExt: "100",
			expectOK:  true,
		},
		{
			name: "ignora identidade unlinked",
			dependencies: dependencies{
				repoMock: func() *mocks.UserIdentityRepository {
					identities := []entities.UserIdentity{
						hydrateIdentity(s, userID, "whatsapp", "+5511999990001", now.Add(-2*time.Hour), now.Add(-time.Hour)),
						hydrateIdentity(s, userID, "telegram", "200", now.Add(-time.Hour), time.Time{}),
					}
					s.repoMock.EXPECT().ListByUser(mock.Anything, userID).Return(identities, nil).Once()
					return s.repoMock
				}(),
			},
			expectCh:  "telegram",
			expectExt: "200",
			expectOK:  true,
		},
		{
			name: "retorna ok=false quando sem identidades",
			dependencies: dependencies{
				repoMock: func() *mocks.UserIdentityRepository {
					s.repoMock.EXPECT().ListByUser(mock.Anything, userID).Return([]entities.UserIdentity{}, nil).Once()
					return s.repoMock
				}(),
			},
			expectOK: false,
		},
		{
			name: "propaga erro do repository",
			dependencies: dependencies{
				repoMock: func() *mocks.UserIdentityRepository {
					s.repoMock.EXPECT().ListByUser(mock.Anything, userID).Return(nil, errors.New("db error")).Once()
					return s.repoMock
				}(),
			},
			expectErr: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewResolvePreferredChannel(scenario.dependencies.repoMock, s.obs)
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
