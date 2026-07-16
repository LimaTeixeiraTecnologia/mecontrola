//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

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

func (s *WorkingMemoryRepositoryIntegrationSuite) TestOnboardingConclusionWithTreatmentNamePersistsBothSectionsAndMetadataKeyWithoutTouchingIdentity() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := memorypostgres.NewWorkingMemoryRepository(db, noop.NewProvider())
	const resourceID = "22222222-2222-2222-2222-222222222222"

	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, display_name)
		 VALUES ($1, '5511999990000', 'Nome Cadastral Original')`,
		resourceID,
	)
	s.Require().NoError(err)

	var displayNameBefore string
	var updatedAtBefore time.Time
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT display_name, updated_at FROM mecontrola.users WHERE id = $1`, resourceID,
	).Scan(&displayNameBefore, &updatedAtBefore))

	conclusionContent := "## Nome de Tratamento\n\nStef\n\n## Objetivo Financeiro\n\nComprar uma casa nova"
	s.Require().NoError(repo.Upsert(ctx, resourceID, conclusionContent))
	s.Require().NoError(repo.UpsertMetadata(ctx, resourceID, map[string]any{
		"objetivo_financeiro": "Comprar uma casa nova",
		"nome_tratamento":     "Stef",
	}))

	var wm, nomeTratamento, objetivo string
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT working_memory, metadata->>'nome_tratamento', metadata->>'objetivo_financeiro'
		   FROM mecontrola.platform_resources WHERE resource_id = $1`,
		resourceID,
	).Scan(&wm, &nomeTratamento, &objetivo))
	s.Contains(wm, "## Objetivo Financeiro", "sentinel de onboarding concluído deve estar presente")
	s.Contains(wm, "## Nome de Tratamento")
	s.Contains(wm, "Stef")
	s.Equal("Stef", nomeTratamento)
	s.Equal("Comprar uma casa nova", objetivo)

	editedContent := "## Nome de Tratamento\n\nStefany\n\n## Objetivo Financeiro\n\nComprar uma casa nova"
	s.Require().NoError(repo.Upsert(ctx, resourceID, editedContent))
	s.Require().NoError(repo.UpsertMetadata(ctx, resourceID, map[string]any{"nome_tratamento": "Stefany"}))

	var wmAfterEdit, nomeAfterEdit, objetivoAfterEdit string
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT working_memory, metadata->>'nome_tratamento', metadata->>'objetivo_financeiro'
		   FROM mecontrola.platform_resources WHERE resource_id = $1`,
		resourceID,
	).Scan(&wmAfterEdit, &nomeAfterEdit, &objetivoAfterEdit))
	s.Contains(wmAfterEdit, "Stefany", "edição deve refletir o novo nome imediatamente (RF-09)")
	s.NotContains(wmAfterEdit, "Stef\n", "seção antiga deve ser substituída, não duplicada")
	s.Contains(wmAfterEdit, "## Objetivo Financeiro", "edição do nome não pode apagar o objetivo (não-clobber)")
	s.Contains(wmAfterEdit, "Comprar uma casa nova")
	s.Equal("Stefany", nomeAfterEdit)
	s.Equal("Comprar uma casa nova", objetivoAfterEdit, "objetivo do metadata permanece inalterado após edição do nome")

	var displayNameAfter string
	var updatedAtAfter time.Time
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT display_name, updated_at FROM mecontrola.users WHERE id = $1`, resourceID,
	).Scan(&displayNameAfter, &updatedAtAfter))
	s.Equal(displayNameBefore, displayNameAfter, "RF-10: display_name cadastral do módulo identity não pode ser afetado")
	s.Equal(updatedAtBefore, updatedAtAfter, "RF-10: nenhuma escrita em mecontrola.users deve ocorrer ao editar nome de tratamento")
}
