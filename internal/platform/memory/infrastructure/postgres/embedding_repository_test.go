package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"
)

type EmbeddingRepositorySuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestEmbeddingRepositorySuite(t *testing.T) {
	suite.Run(t, new(EmbeddingRepositorySuite))
}

func (s *EmbeddingRepositorySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *EmbeddingRepositorySuite) TestIndex_UsesOnConflictForIdempotency() {
	db := dbmocks.NewMockDBTX(s.T())
	result := dbmocks.NewMockResult(s.T())

	db.EXPECT().ExecContext(
		mock.Anything,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "ON CONFLICT") && strings.Contains(q, "DO NOTHING")
		}),
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything,
	).Return(result, nil).Once()

	repo := NewEmbeddingRepository(db, s.obs)
	err := repo.Index(s.ctx, "res-1", "thr-1", uuid.New(), "hello world", "text-embedding-3-small", []float32{0.1, 0.2, 0.3})

	s.NoError(err)
}
