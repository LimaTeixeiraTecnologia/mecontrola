//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	cardrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type InvoiceDuePhase5Suite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestInvoiceDuePhase5Suite(t *testing.T) {
	suite.Run(t, new(InvoiceDuePhase5Suite))
}

func (s *InvoiceDuePhase5Suite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = cardrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *InvoiceDuePhase5Suite) SetupTest() {}

func (s *InvoiceDuePhase5Suite) cardRepo() interfaces.CardRepository {
	return s.factory.CardRepository(s.db)
}

func (s *InvoiceDuePhase5Suite) ledgerRepo() interfaces.InvoiceDueAlertSentRepository {
	return s.factory.InvoiceDueAlertSentRepository(s.db)
}

func (s *InvoiceDuePhase5Suite) insertUser(ctx context.Context) string {
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

func (s *InvoiceDuePhase5Suite) insertCardWithDueDay(ctx context.Context, userID string, dueDay int) entities.Card {
	repo := s.cardRepo()
	uid, err := uuid.Parse(userID)
	s.Require().NoError(err)

	bank, err := valueobjects.NewBankCode("Nubank")
	s.Require().NoError(err)
	nick, err := valueobjects.NewNickname(fmt.Sprintf("due-%d-%d", dueDay, time.Now().UnixNano()))
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(5, dueDay)
	s.Require().NoError(err)

	card := entities.NewCard(entities.NewCardInput{
		UserID:   uid,
		Nickname: nick,
		Bank:     bank,
		Cycle:    cycle,
	})
	s.Require().NoError(repo.Insert(ctx, card))
	return card
}

func (s *InvoiceDuePhase5Suite) softDeleteCard(ctx context.Context, card entities.Card, userID string) {
	repo := s.cardRepo()
	s.Require().NoError(repo.SoftDeleteByIDForUser(ctx, card.ID.String(), userID, time.Now().UTC()))
}

func dueDayWindowSet(now time.Time, windowDays int) map[int]struct{} {
	set := make(map[int]struct{}, windowDays+2)
	for i := 0; i <= windowDays+1; i++ {
		set[now.AddDate(0, 0, i).Day()] = struct{}{}
	}
	return set
}

func pickDueDays(now time.Time, windowDays int) (inWindow int, outWindow int) {
	set := dueDayWindowSet(now, windowDays)
	inWindow = now.AddDate(0, 0, 1).Day()
	for d := 1; d <= 28; d++ {
		if _, ok := set[d]; !ok {
			outWindow = d
			break
		}
	}
	return inWindow, outWindow
}

func (s *InvoiceDuePhase5Suite) TestFindCardsWithInvoiceDueWithin() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "retorna apenas cartoes com due_day dentro da janela e ignora soft-deleted",
			expect: func(ctx context.Context) {
				const windowDays = 3
				now := time.Now().UTC()
				inDay, outDay := pickDueDays(now, windowDays)
				s.Require().NotZero(outDay, "deve existir um due_day fora da janela")

				userID := s.insertUser(ctx)
				inCard := s.insertCardWithDueDay(ctx, userID, inDay)
				_ = s.insertCardWithDueDay(ctx, userID, outDay)
				deletedCard := s.insertCardWithDueDay(ctx, userID, inDay)
				s.softDeleteCard(ctx, deletedCard, userID)

				repo := s.cardRepo()
				got, err := repo.FindCardsWithInvoiceDueWithin(ctx, windowDays, 100)
				s.Require().NoError(err)

				ids := make(map[string]bool, len(got))
				for _, c := range got {
					ids[c.ID.String()] = true
				}
				s.Assert().True(ids[inCard.ID.String()], "cartao dentro da janela deve retornar")
				s.Assert().False(ids[deletedCard.ID.String()], "cartao soft-deleted nao pode retornar")
				for _, c := range got {
					s.Assert().Nil(c.DeletedAt, "nenhum cartao retornado pode estar soft-deleted")
				}
			},
		},
		{
			name: "respeita o limit retornando no maximo N linhas",
			expect: func(ctx context.Context) {
				const windowDays = 5
				now := time.Now().UTC()
				inDay, _ := pickDueDays(now, windowDays)

				userID := s.insertUser(ctx)
				for range 4 {
					s.insertCardWithDueDay(ctx, userID, inDay)
				}

				repo := s.cardRepo()
				got, err := repo.FindCardsWithInvoiceDueWithin(ctx, windowDays, 2)
				s.Require().NoError(err)
				s.Assert().LessOrEqual(len(got), 2, "limit deve limitar o numero de linhas")
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background())
		})
	}
}

func (s *InvoiceDuePhase5Suite) TestInsertSentIdempotent() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "InsertSent duas vezes na mesma PK nao duplica nem erra (ON CONFLICT DO NOTHING)",
			expect: func(ctx context.Context) {
				const windowDays = 0
				userID := s.insertUser(ctx)
				inDay, _ := pickDueDays(time.Now().UTC(), windowDays)
				card := s.insertCardWithDueDay(ctx, userID, inDay)

				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)
				refDue := time.Now().UTC().Truncate(24 * time.Hour)

				ledger := s.ledgerRepo()
				s.Require().NoError(ledger.InsertSent(ctx, uid, card.ID, refDue))
				s.Require().NoError(ledger.InsertSent(ctx, uid, card.ID, refDue))

				records, err := ledger.ListSentForDueDates(ctx, []time.Time{refDue})
				s.Require().NoError(err)

				count := 0
				for _, rec := range records {
					if rec.UserID == uid && rec.CardID == card.ID {
						count++
					}
				}
				s.Assert().Equal(1, count, "PK duplicada deve resultar em exatamente 1 linha")
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background())
		})
	}
}

func (s *InvoiceDuePhase5Suite) TestIsNotifiedTransitionsViaMarkNotified() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "IsNotified false antes e true depois de MarkNotified",
			expect: func(ctx context.Context) {
				const windowDays = 0
				userID := s.insertUser(ctx)
				inDay, _ := pickDueDays(time.Now().UTC(), windowDays)
				card := s.insertCardWithDueDay(ctx, userID, inDay)

				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)
				refDue := time.Now().UTC().Truncate(24 * time.Hour)

				ledger := s.ledgerRepo()
				s.Require().NoError(ledger.InsertSent(ctx, uid, card.ID, refDue))

				notified, err := ledger.IsNotified(ctx, uid, card.ID, refDue)
				s.Require().NoError(err)
				s.Assert().False(notified, "deve iniciar nao notificado")

				marked, err := ledger.MarkNotified(ctx, uid, card.ID, refDue, "whatsapp", time.Now().UTC())
				s.Require().NoError(err)
				s.Assert().True(marked, "MarkNotified deve afetar exatamente a linha pendente")

				notified, err = ledger.IsNotified(ctx, uid, card.ID, refDue)
				s.Require().NoError(err)
				s.Assert().True(notified, "deve estar notificado apos MarkNotified")
			},
		},
		{
			name: "MarkNotified em linha ja notificada retorna false (filtro notified_at IS NULL)",
			expect: func(ctx context.Context) {
				const windowDays = 0
				userID := s.insertUser(ctx)
				inDay, _ := pickDueDays(time.Now().UTC(), windowDays)
				card := s.insertCardWithDueDay(ctx, userID, inDay)

				uid, err := uuid.Parse(userID)
				s.Require().NoError(err)
				refDue := time.Now().UTC().Truncate(24 * time.Hour)

				ledger := s.ledgerRepo()
				s.Require().NoError(ledger.InsertSent(ctx, uid, card.ID, refDue))

				first, err := ledger.MarkNotified(ctx, uid, card.ID, refDue, "whatsapp", time.Now().UTC())
				s.Require().NoError(err)
				s.Assert().True(first)

				second, err := ledger.MarkNotified(ctx, uid, card.ID, refDue, "whatsapp", time.Now().UTC())
				s.Require().NoError(err)
				s.Assert().False(second, "segunda marcacao nao deve afetar linha ja notificada")
			},
		},
		{
			name: "IsNotified em registro inexistente retorna ErrInvoiceDueAlertRecordMissing",
			expect: func(ctx context.Context) {
				ledger := s.ledgerRepo()
				_, err := ledger.IsNotified(ctx, uuid.New(), uuid.New(), time.Now().UTC().Truncate(24*time.Hour))
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, interfaces.ErrInvoiceDueAlertRecordMissing))
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect(context.Background())
		})
	}
}
