package dedup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/mocks"
)

type CleanupProcessedMessagesSuite struct {
	suite.Suite
	repoMock *mocks.MessageRepository
}

func TestCleanupProcessedMessagesSuite(t *testing.T) {
	suite.Run(t, new(CleanupProcessedMessagesSuite))
}

func (s *CleanupProcessedMessagesSuite) SetupTest() {
	s.repoMock = mocks.NewMessageRepository(s.T())
}

func (s *CleanupProcessedMessagesSuite) newUseCase(cfg configs.WhatsAppConfig) *dedup.CleanupProcessedMessages {
	return dedup.NewCleanupProcessedMessages(s.repoMock, cfg, noop.NewProvider())
}

func (s *CleanupProcessedMessagesSuite) TestExecute() {
	scenarios := []struct {
		name      string
		cfg       configs.WhatsAppConfig
		setup     func(context.Context)
		expectErr string
	}{
		{
			name: "deve excluir linhas antigas em lotes ate esgotar",
			cfg:  configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 10000},
			setup: func(ctx context.Context) {
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 10000).Return(int64(10000), nil).Once()
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 10000).Return(int64(4000), nil).Once()
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 10000).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve usar defaults quando retencao e batch nao configurados",
			cfg:  configs.WhatsAppConfig{},
			setup: func(ctx context.Context) {
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 10000).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve concluir quando nao houver linhas para excluir",
			cfg:  configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 500},
			setup: func(ctx context.Context) {
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 500).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve propagar erro do repositorio",
			cfg:  configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 500},
			setup: func(ctx context.Context) {
				s.repoMock.EXPECT().DeleteProcessedBefore(ctx, mock.Anything, 500).Return(int64(0), errors.New("db error")).Once()
			},
			expectErr: "db error",
		},
		{
			name:      "deve parar quando o contexto for cancelado",
			cfg:       configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 500},
			expectErr: "context cancelled",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			if scenario.expectErr == "context cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				s.repoMock.EXPECT().
					DeleteProcessedBefore(mock.Anything, mock.Anything, 500).
					RunAndReturn(func(_ context.Context, _ time.Time, _ int) (int64, error) {
						cancel()
						return 500, nil
					}).
					Once()
			} else {
				scenario.setup(ctx)
			}

			uc := s.newUseCase(scenario.cfg)
			err := uc.Execute(ctx)

			if scenario.expectErr == "" {
				s.Require().NoError(err)
				return
			}

			s.Require().Error(err)
			s.Contains(err.Error(), scenario.expectErr)
		})
	}
}
