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

	name, err := valueobjects.NewCardName("Nubank Gold")
	s.Require().NoError(err)

	nick, err := valueobjects.NewNickname(fmt.Sprintf("nu-%d", time.Now().UnixNano()))
	s.Require().NoError(err)

	cycle, err := valueobjects.NewBillingCycle(10, 25)
	s.Require().NoError(err)

	return entities.NewCard(entities.NewCardInput{
		UserID:   uid,
		Name:     name,
		Nickname: nick,
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
				s.Assert().Equal(card.Name.String(), got.Name.String())
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

				name, err := valueobjects.NewCardName("Test Card")
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
							Name:     name,
							Nickname: nick,
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

				for i := range total {
					name, nameErr := valueobjects.NewCardName(fmt.Sprintf("Card %04d", i))
					s.Require().NoError(nameErr)
					nick, nickErr := valueobjects.NewNickname(fmt.Sprintf("nick-%04d-%d", i, time.Now().UnixNano()))
					s.Require().NoError(nickErr)

					card := entities.NewCard(entities.NewCardInput{
						UserID:   uid,
						Name:     name,
						Nickname: nick,
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

				for i := range 3 {
					name, nameErr := valueobjects.NewCardName(fmt.Sprintf("Pg Card %d", i))
					s.Require().NoError(nameErr)
					nick, nickErr := valueobjects.NewNickname(fmt.Sprintf("pg-nick-%d-%d", i, time.Now().UnixNano()))
					s.Require().NoError(nickErr)
					card := entities.NewCard(entities.NewCardInput{
						UserID:   uid,
						Name:     name,
						Nickname: nick,
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

func (s *CardRepositorySuite) TestPersistAndReadLimitCents() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "insert e leitura com limite positivo",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)

				name, err := valueobjects.NewCardName("Nubank Roxo")
				s.Require().NoError(err)
				nick, err := valueobjects.NewNickname(fmt.Sprintf("nu-lim-%d", time.Now().UnixNano()))
				s.Require().NoError(err)
				cycle, err := valueobjects.NewBillingCycle(10, 25)
				s.Require().NoError(err)

				card := entities.NewCard(entities.NewCardInput{
					UserID:     uid,
					Name:       name,
					Nickname:   nick,
					Cycle:      cycle,
					LimitCents: 500000,
				})

				s.Require().NoError(repo.Insert(ctx, card))

				got, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)
				s.Assert().Equal(int64(500000), got.LimitCents)
			},
		},
		{
			name: "insert e leitura com limite zero (default)",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)
				s.Require().NoError(repo.Insert(ctx, card))

				got, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)
				s.Assert().Equal(int64(0), got.LimitCents)
			},
		},
		{
			name: "update persiste novo limite",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)
				s.Require().NoError(repo.Insert(ctx, card))

				inserted, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)

				limit, err := valueobjects.NewCardLimit(1234500)
				s.Require().NoError(err)
				updated := inserted.UpdateLimit(limit, time.Now().UTC())

				persisted, err := repo.UpdateByIDForUser(ctx, updated)
				s.Require().NoError(err)
				s.Assert().Equal(int64(1234500), persisted.LimitCents)
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background(), s.newRepo())
		})
	}
}

func (s *CardRepositorySuite) TestUpdateLimitOptimisticConcurrency() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.CardRepository)
	}{
		{
			name: "primeiro update vence; segundo com versao estale retorna ErrCardLimitConflict",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)
				s.Require().NoError(repo.Insert(ctx, card))

				snapshot, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)
				staleVersion := snapshot.Version

				firstLimit, err := valueobjects.NewCardLimit(200000)
				s.Require().NoError(err)
				firstUpdated := snapshot.UpdateLimit(firstLimit, time.Now().UTC())
				persisted, err := repo.UpdateLimitByIDForUser(ctx, firstUpdated, staleVersion)
				s.Require().NoError(err)
				s.Assert().Equal(int64(200000), persisted.LimitCents)
				s.Assert().Equal(staleVersion+1, persisted.Version)

				secondLimit, err := valueobjects.NewCardLimit(300000)
				s.Require().NoError(err)
				stale := snapshot.UpdateLimit(secondLimit, time.Now().UTC())
				_, err = repo.UpdateLimitByIDForUser(ctx, stale, staleVersion)
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, carddomain.ErrCardLimitConflict))
			},
		},
		{
			name: "update_limit com versao correta apos bump permite seguinte update",
			expect: func(ctx context.Context, repo interfaces.CardRepository) {
				userID := s.insertTestUser(ctx)
				card := s.makeCard(userID)
				s.Require().NoError(repo.Insert(ctx, card))

				snapshot, err := repo.GetByIDForUser(ctx, card.ID.String(), userID)
				s.Require().NoError(err)

				limitA, _ := valueobjects.NewCardLimit(100000)
				afterA, err := repo.UpdateLimitByIDForUser(ctx, snapshot.UpdateLimit(limitA, time.Now().UTC()), snapshot.Version)
				s.Require().NoError(err)

				limitB, _ := valueobjects.NewCardLimit(400000)
				afterB, err := repo.UpdateLimitByIDForUser(ctx, afterA.UpdateLimit(limitB, time.Now().UTC()), afterA.Version)
				s.Require().NoError(err)
				s.Assert().Equal(int64(400000), afterB.LimitCents)
				s.Assert().Equal(afterA.Version+1, afterB.Version)
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

				newName, err := valueobjects.NewCardName("Updated Name")
				s.Require().NoError(err)
				newNick, err := valueobjects.NewNickname(fmt.Sprintf("updated-%d", time.Now().UnixNano()))
				s.Require().NoError(err)
				newCycle, err := valueobjects.NewBillingCycle(15, 28)
				s.Require().NoError(err)

				updatedCard := entities.HydrateCard(
					inserted.ID,
					inserted.UserID,
					newName,
					newNick,
					newCycle,
					inserted.LimitCents,
					inserted.CreatedAt,
					time.Now().UTC(),
					nil,
				)

				result, err := repo.UpdateByIDForUser(ctx, updatedCard)
				s.Require().NoError(err)

				s.Assert().True(inserted.CreatedAt.Equal(result.CreatedAt),
					"created_at deve ser preservado: want=%v got=%v", inserted.CreatedAt, result.CreatedAt)
				s.Assert().Equal(newName.String(), result.Name.String())
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

				name, err := valueobjects.NewCardName("Ghost")
				s.Require().NoError(err)
				nick, err := valueobjects.NewNickname("ghost-nick")
				s.Require().NoError(err)
				cycle, err := valueobjects.NewBillingCycle(1, 10)
				s.Require().NoError(err)

				ghost := entities.HydrateCard(uuid.New(), uid, name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
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
