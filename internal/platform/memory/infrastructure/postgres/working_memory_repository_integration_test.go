//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	memorypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type WorkingMemoryRepositoryIntegrationSuite struct {
	suite.Suite
}

func TestWorkingMemoryRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(WorkingMemoryRepositoryIntegrationSuite))
}

func (s *WorkingMemoryRepositoryIntegrationSuite) TestUpsertMetadataMergesAndPreservesWorkingMemory() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := memorypostgres.NewWorkingMemoryRepository(db, noop.NewProvider())
	const resourceID = "11111111-1111-1111-1111-111111111111"

	s.Require().NoError(repo.Upsert(ctx, resourceID, "## Objetivo Financeiro\n\nComprar uma casa nova"))
	s.Require().NoError(repo.UpsertMetadata(ctx, resourceID, map[string]any{"objetivo_financeiro": "Comprar uma casa nova"}))

	var wm, objetivo string
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT working_memory, metadata->>'objetivo_financeiro' FROM mecontrola.platform_resources WHERE resource_id = $1`,
		resourceID,
	).Scan(&wm, &objetivo))
	s.Contains(wm, "Comprar uma casa nova")
	s.Equal("Comprar uma casa nova", objetivo)

	s.Require().NoError(repo.UpsertMetadata(ctx, resourceID, map[string]any{"renda_mensal_cents": 1000000}))

	var objetivoAfter, wmAfter string
	var renda int64
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT working_memory, metadata->>'objetivo_financeiro', (metadata->>'renda_mensal_cents')::bigint FROM mecontrola.platform_resources WHERE resource_id = $1`,
		resourceID,
	).Scan(&wmAfter, &objetivoAfter, &renda))
	s.Equal("Comprar uma casa nova", objetivoAfter)
	s.Equal(int64(1000000), renda)
	s.Contains(wmAfter, "Comprar uma casa nova")
}
