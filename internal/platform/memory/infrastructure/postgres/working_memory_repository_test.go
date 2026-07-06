package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"
)

type WorkingMemoryRepositorySuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestWorkingMemoryRepositorySuite(t *testing.T) {
	suite.Run(t, new(WorkingMemoryRepositorySuite))
}

func (s *WorkingMemoryRepositorySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *WorkingMemoryRepositorySuite) TestUpsertMetadata_MergesJsonbWithGoal() {
	db := dbmocks.NewMockDBTX(s.T())
	result := dbmocks.NewMockResult(s.T())

	db.EXPECT().ExecContext(
		mock.Anything,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "ON CONFLICT") && strings.Contains(q, "|| EXCLUDED.metadata")
		}),
		"res-1",
		mock.MatchedBy(func(raw []byte) bool {
			return strings.Contains(string(raw), "objetivo_financeiro") && strings.Contains(string(raw), "Comprar uma casa nova")
		}),
	).Return(result, nil).Once()

	repo := NewWorkingMemoryRepository(db, s.obs)
	err := repo.UpsertMetadata(s.ctx, "res-1", map[string]any{"objetivo_financeiro": "Comprar uma casa nova"})

	s.NoError(err)
}
