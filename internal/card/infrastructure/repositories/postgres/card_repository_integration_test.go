//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	cardrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type CardRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestCardRepositorySuite(t *testing.T) {
	suite.Run(t, new(CardRepositorySuite))
}

func (s *CardRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = cardrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *CardRepositorySuite) SetupTest() {}

func (s *CardRepositorySuite) newRepo() interfaces.CardRepository {
	return s.factory.CardRepository(s.db)
}

func (s *CardRepositorySuite) insertTestUser(ctx context.Context) string {
	userID := uuid.New().String()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *CardRepositorySuite) makeCard(userID string) entities.Card {
	uid, err := uuid.Parse(userID)
	s.Require().NoError(err)

	nick, err := valueobjects.NewNickname(fmt.Sprintf("nu-%d", time.Now().UnixNano()))
	s.Require().NoError(err)

	bank, err := valueobjects.NewBankCode("Nubank")
	s.Require().NoError(err)

	cycle, err := valueobjects.NewBillingCycle(10, 25)
	s.Require().NoError(err)

	return entities.NewCard(entities.NewCardInput{
		UserID:   uid,
		Nickname: nick,
		Bank:     bank,
		Cycle:    cycle,
	})
}

func (s *CardRepositorySuite) TestInsertAndGetHappyPath() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "deve inserir cartão e recuperar pelo id e user_id",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)

				err := repo.Insert(ctx, card)
				s.Require().NoError(err)

				got, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)
				s.Assert().Equal(card.ID, got.ID)
				s.Assert().Equal(card.UserID, got.UserID)
				s.Assert().Equal(card.Bank.String(), got.Bank.String())
				s.Assert().Equal(card.Nickname.String(), got.Nickname.String())
				s.Assert().Equal(card.Cycle.ClosingDay, got.Cycle.ClosingDay)
				s.Assert().Equal(card.Cycle.DueDay, got.Cycle.DueDay)
				s.Assert().Nil(got.DeletedAt)
			},
		},
		{
			name: "deve retornar ErrCardNotFound para id inexistente",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				_, err := repo.GetByIDForUser(ctx, uuid.New().String(), userID)
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, carddomain.ErrCardNotFound))
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}

func (s *CardRepositorySuite) TestSoftDeleteThenReadReturnsNotFound() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "deve retornar ErrCardNotFound após soft-delete",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)

				s.Require().NoError(repo.Insert(ctx, card))

				now := time.Now().UTC()
				err := repo.SoftDeleteByIDForUser(ctx, card.ID.String(), userID, now)
				s.Require().NoError(err)

				_, err = repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, carddomain.ErrCardNotFound))
			},
		},
		{
			name: "deve retornar ErrCardNotFound ao soft-deletar id inexistente",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				err := repo.SoftDeleteByIDForUser(ctx, uuid.New().String(), userID, time.Now().UTC())
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, carddomain.ErrCardNotFound))
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}

func (s *CardRepositorySuite) TestConcurrentInsertSameNickname() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "deve ter exatamente 1 sucesso e 9 ErrNicknameConflict em 10 goroutines",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)

				nick, err := valueobjects.NewNickname("shared-nick")
				s.Require().NoError(err)

				bank, err := valueobjects.NewBankCode("Nubank")
				s.Require().NoError(err)

				cycle, err := valueobjects.NewBillingCycle(5, 15)
				s.Require().NoError(err)

				const goroutines = 10
				errs := make([]error, goroutines)
				var wg sync.WaitGroup
				wg.Add(goroutines)

				for i := range goroutines {
					go func(idx int) {
						defer wg.Done()
						r := s.factory.CardRepository(s.db)
						card := entities.NewCard(entities.NewCardInput{
							UserID:   uid,
							Nickname: nick,
							Bank:     bank,
							Cycle:    cycle,
						})
						errs[idx] = r.Insert(ctx, card)
					}(i)
				}
				wg.Wait()

				successCount := 0
				conflictCount := 0
				for _, e := range errs {
					if e == nil {
						successCount++
					} else if errors.Is(e, carddomain.ErrNicknameConflict) {
						conflictCount++
					} else {
						s.Failf("unexpected error", "erro inesperado: %v", e)
					}
				}

				s.Assert().Equal(1, successCount, "exatamente 1 insert deve ter sucesso")
				s.Assert().Equal(goroutines-1, conflictCount, "os demais devem retornar ErrNicknameConflict")
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}

func (s *CardRepositorySuite) TestKeysetPaginationStable() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "deve paginar 250 cartões sem duplicatas e sem perdas",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)

				const total = 250
				cycle, err := valueobjects.NewBillingCycle(10, 20)
				s.Require().NoError(err)

				bank, err := valueobjects.NewBankCode("Nubank")
				s.Require().NoError(err)

				for i := range total {
					nick, nickErr := valueobjects.NewNickname(fmt.Sprintf("nick-%04d-%d", i, time.Now().UnixNano()))
					s.Require().NoError(nickErr)

					card := entities.NewCard(entities.NewCardInput{
						UserID:   uid,
						Nickname: nick,
						Bank:     bank,
						Cycle:    cycle,
					})
					s.Require().NoError(repo.Insert(ctx, card))
				}

				const pageSize = 50
				seen := make(map[string]bool)
				cursor := ""

				for {
					page, nextCursor, listErr := repo.ListByUser(ctx, userID, cursor, pageSize)
					s.Require().NoError(listErr)

					for _, c := range page {
						s.Assert().False(seen[c.ID.String()], "duplicata detectada: %s", c.ID)
						seen[c.ID.String()] = true
					}

					if nextCursor == "" {
						break
					}
					cursor = nextCursor
				}

				s.Assert().Equal(total, len(seen), "deve ter recuperado todos os %d cartões", total)
			},
		},
		{
			name: "deve retornar next_cursor nulo na última página",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)

				cycle, err := valueobjects.NewBillingCycle(1, 15)
				s.Require().NoError(err)

				bank, err := valueobjects.NewBankCode("Nubank")
				s.Require().NoError(err)

				for i := range 3 {
					nick, nickErr := valueobjects.NewNickname(fmt.Sprintf("pg-nick-%d-%d", i, time.Now().UnixNano()))
					s.Require().NoError(nickErr)
					card := entities.NewCard(entities.NewCardInput{
						UserID:   uid,
						Nickname: nick,
						Bank:     bank,
						Cycle:    cycle,
					})
					s.Require().NoError(repo.Insert(ctx, card))
				}

				_, nextCursor, listErr := repo.ListByUser(ctx, userID, "", 10)
				s.Require().NoError(listErr)
				s.Assert().Empty(nextCursor, "next_cursor deve ser vazio quando todos os itens couberam na página")
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}

func (s *CardRepositorySuite) TestUpdatePreservesCreatedAt() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "update deve preservar created_at e recalcular updated_at",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)

				s.Require().NoError(repo.Insert(ctx, card))

				inserted, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)

				newBank, err := valueobjects.NewBankCode("Itau")
				s.Require().NoError(err)
				newNick, err := valueobjects.NewNickname(fmt.Sprintf("updated-%d", time.Now().UnixNano()))
				s.Require().NoError(err)
				newCycle, err := valueobjects.NewBillingCycle(15, 28)
				s.Require().NoError(err)

				updatedCard := entities.HydrateCard(
					inserted.ID,
					inserted.UserID,
					newNick,
					newBank,
					newCycle,
					inserted.CreatedAt,
					time.Now().UTC(),
					nil,
				)

				result, err := repo.UpdateByIDForUser(ctx, updatedCard)
				s.Require().NoError(err)

				s.Assert().True(inserted.CreatedAt.Equal(result.CreatedAt),
					"created_at deve ser preservado: want=%v got=%v", inserted.CreatedAt, result.CreatedAt)
				s.Assert().Equal(newBank.String(), result.Bank.String())
				s.Assert().Equal(newNick.String(), result.Nickname.String())
				s.Assert().Equal(15, result.Cycle.ClosingDay)
				s.Assert().Equal(28, result.Cycle.DueDay)
			},
		},
		{
			name: "update em cartão inexistente deve retornar ErrCardNotFound",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)

				bank, err := valueobjects.NewBankCode("Ghost Bank")
				s.Require().NoError(err)
				nick, err := valueobjects.NewNickname("ghost-nick")
				s.Require().NoError(err)
				cycle, err := valueobjects.NewBillingCycle(1, 10)
				s.Require().NoError(err)

				ghost := entities.HydrateCard(uuid.New(), uid, nick, bank, cycle, time.Now().UTC(), time.Now().UTC(), nil)
				_, err = repo.UpdateByIDForUser(ctx, ghost)
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, carddomain.ErrCardNotFound))
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}
