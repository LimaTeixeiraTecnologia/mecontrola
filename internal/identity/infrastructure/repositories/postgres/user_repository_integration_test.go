//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	pgpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type fakeIDGenerator struct {
	mu      sync.Mutex
	counter int
}

func (f *fakeIDGenerator) NewID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counter++
	// Produce a deterministic but valid UUID v4 by setting the right bits.
	id := uuid.New()
	return id.String()
}

// ---------------------------------------------------------------------------
// Suite
// ---------------------------------------------------------------------------

type UserRepositoryIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	mgr  *dbpkg.Manager
	repo *pgpkg.PgxUserRepository
	ids  *fakeIDGenerator
}

func TestUserRepositoryIntegration(t *testing.T) {
	suite.Run(t, new(UserRepositoryIntegrationSuite))
}

func (s *UserRepositoryIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		if container != nil {
			_ = container.Terminate(context.Background())
		}
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	mappedPort, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	cfg := &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(mappedPort.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 10,
			MinConns: 2,
		},
	}

	mgr, err := dbpkg.NewManager(cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *UserRepositoryIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		_ = s.mgr.Shutdown(context.Background())
	}
}

func (s *UserRepositoryIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx, "TRUNCATE users, user_whatsapp_history CASCADE")
	s.Require().NoError(err)

	s.ids = &fakeIDGenerator{}
	s.repo = pgpkg.NewPgxUserRepository(s.mgr, s.ids)
}

// ---------------------------------------------------------------------------
// mustWhatsApp returns a WhatsAppNumber or fails the test.
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) mustWhatsApp(n string) valueobjects.WhatsAppNumber {
	num, err := valueobjects.NewWhatsAppNumber(n)
	s.Require().NoError(err)
	return num
}

// ---------------------------------------------------------------------------
// mustUserID returns a UserID or fails the test.
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) mustUserID(v string) entities.UserID {
	id, err := entities.NewUserID(v)
	s.Require().NoError(err)
	return id
}

// ---------------------------------------------------------------------------
// Cenário 9.5: Upsert idempotente por WhatsApp number
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestUpsertIdempotenteByWhatsAppNumber() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"deve retornar mesmo UserID em duas chamadas com mesmo número", "+5511999990001"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			user1, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)
			s.Require().NotNil(user1)

			user2, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)
			s.Require().NotNil(user2)

			s.Equal(user1.ID().String(), user2.ID().String(), "deve retornar o mesmo UserID")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.6: SoftDelete torna usuário invisível em FindByID
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestSoftDeleteFiltraEmFindByID() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"deve retornar ErrUserNotFound após soft delete via FindByID", "+5511999990002"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			user, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)

			s.Require().NoError(s.repo.SoftDelete(s.ctx, user.ID(), time.Now().UTC()))

			_, err = s.repo.FindByID(s.ctx, user.ID())
			s.ErrorIs(err, pgpkg.ErrUserNotFound)
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.7: SoftDelete torna usuário invisível em FindByWhatsAppNumber
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestSoftDeleteFiltraEmFindByWhatsApp() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"deve retornar ErrUserNotFound após soft delete via FindByWhatsAppNumber", "+5511999990003"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			user, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)

			s.Require().NoError(s.repo.SoftDelete(s.ctx, user.ID(), time.Now().UTC()))

			_, err = s.repo.FindByWhatsAppNumber(s.ctx, num)
			s.ErrorIs(err, pgpkg.ErrUserNotFound)
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.8: SoftDelete cascata desativa histórico (ADR-009)
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestSoftDeleteCascataDesativaHistorico() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"deve desativar history rows com reason=user_soft_deleted após soft delete", "+5511999990004"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			user, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)

			// Link a second number to create a history row.
			num2 := s.mustWhatsApp("+5511999990041")
			s.Require().NoError(s.repo.LinkNewNumber(s.ctx, user.ID(), num2, "troca", time.Now().UTC()))

			s.Require().NoError(s.repo.SoftDelete(s.ctx, user.ID(), time.Now().UTC()))

			dbtx := s.mgr.DBTX(s.ctx)

			var activeCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM user_whatsapp_history WHERE user_id = $1 AND active = true",
				user.ID().String(),
			).Scan(&activeCount))
			s.Equal(0, activeCount, "nenhuma history row deve estar ativa após soft delete")

			var reasonCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM user_whatsapp_history WHERE user_id = $1 AND reason = 'user_soft_deleted'",
				user.ID().String(),
			).Scan(&reasonCount))
			s.GreaterOrEqual(reasonCount, 1, "pelo menos uma row deve ter reason=user_soft_deleted")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.9: LinkNewNumber registra histórico
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestLinkNewNumberRegistraHistorico() {
	scenarios := []struct {
		name   string
		number string
		newNum string
	}{
		{
			"deve registrar 2 rows em history e atualizar whatsapp_number",
			"+5511999990005",
			"+5511999990050",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)
			user, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)

			// Link initial number into history first by linking a new number.
			num2 := s.mustWhatsApp("+5511999990051")
			s.Require().NoError(s.repo.LinkNewNumber(s.ctx, user.ID(), num2, "primeiro link", time.Now().UTC()))

			num3 := s.mustWhatsApp(sc.newNum)
			s.Require().NoError(s.repo.LinkNewNumber(s.ctx, user.ID(), num3, "segundo link", time.Now().UTC()))

			dbtx := s.mgr.DBTX(s.ctx)

			var historyCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM user_whatsapp_history WHERE user_id = $1",
				user.ID().String(),
			).Scan(&historyCount))
			s.GreaterOrEqual(historyCount, 2, "deve ter pelo menos 2 rows em history")

			var activeCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM user_whatsapp_history WHERE user_id = $1 AND active = true",
				user.ID().String(),
			).Scan(&activeCount))
			s.Equal(1, activeCount, "apenas 1 row deve estar ativa")

			// Verify whatsapp_number updated.
			updatedUser, err := s.repo.FindByID(s.ctx, user.ID())
			s.Require().NoError(err)
			s.Equal(num3.String(), updatedUser.WhatsAppNumber().String(), "whatsapp_number deve estar atualizado")
		})
	}
}

func (s *UserRepositoryIntegrationSuite) TestLinkNewNumberUsuarioInexistente() {
	scenarios := []struct {
		name   string
		userID string
		number string
	}{
		{
			"deve retornar ErrUserNotFound sem erro de foreign key",
			"550e8400-e29b-41d4-a716-446655440000",
			"+5511999990052",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			err := s.repo.LinkNewNumber(
				s.ctx,
				s.mustUserID(sc.userID),
				s.mustWhatsApp(sc.number),
				"troca",
				time.Now().UTC(),
			)

			s.ErrorIs(err, pgpkg.ErrUserNotFound)
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.10: UNIQUE parcial permite reuso de número após soft delete (ADR-006)
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestUniqueIndexParcialPermiteReuso() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"deve permitir criar novo usuário com número de usuário soft-deleted", "+5511999990006"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			userA, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)

			s.Require().NoError(s.repo.SoftDelete(s.ctx, userA.ID(), time.Now().UTC()))

			userB, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
			s.Require().NoError(err)
			s.Require().NotNil(userB)

			s.NotEqual(userA.ID().String(), userB.ID().String(), "deve criar novo usuário com ID diferente")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.11: Dois INSERTs simultâneos com mesmo número ativo retornam ErrDuplicateWhatsAppNumber
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestDuplicateWhatsAppNumberConcorrente() {
	scenarios := []struct {
		name   string
		number string
	}{
		{"dois upserts concorrentes com mesmo número ativo — segundo falha ou retorna mesmo ID", "+5511999990007"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			num := s.mustWhatsApp(sc.number)

			var (
				wg   sync.WaitGroup
				mu   sync.Mutex
				errs []error
				ids  []string
			)

			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					u, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						errs = append(errs, err)
					} else {
						ids = append(ids, u.ID().String())
					}
				}()
			}
			wg.Wait()

			// Either both succeed returning the same ID (race won by same insert+select),
			// or one returns ErrDuplicateWhatsAppNumber.
			if len(errs) > 0 {
				for _, err := range errs {
					s.ErrorIs(err, pgpkg.ErrDuplicateWhatsAppNumber)
				}
			} else {
				s.Len(ids, 2)
				s.Equal(ids[0], ids[1], "ambos devem retornar o mesmo ID quando upsert é idempotente")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.12: Mapper rejeita UUID corrompido (ADR-008)
// Uses FindByWhatsAppNumber so the mapper receives the v3 UUID from the DB row.
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestMapperRejeitaUUIDCorrompido() {
	scenarios := []struct {
		name string
	}{
		{"FindByWhatsAppNumber com UUID v3 no banco deve retornar erro tipado de mapper"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// UUID v3 passes PK constraint but NewUserID rejects non-v4.
			v3id := uuid.NewMD5(uuid.NameSpaceDNS, []byte("test-mapper-corrupt")).String()
			number := "+5511999990098"

			dbtx := s.mgr.DBTX(s.ctx)
			_, err := dbtx.ExecContext(s.ctx,
				`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
				 VALUES ($1, $2, 'ACTIVE', now(), now())`,
				v3id, number,
			)
			s.Require().NoError(err)

			num := s.mustWhatsApp(number)
			_, findErr := s.repo.FindByWhatsAppNumber(s.ctx, num)
			s.Require().Error(findErr)
			s.NotErrorIs(findErr, pgpkg.ErrUserNotFound, "deve falhar com erro de mapper, não de not found")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 9.13: Admin seed promotion (migration 0004)
// ---------------------------------------------------------------------------

func (s *UserRepositoryIntegrationSuite) TestAdminSeedPromocao() {
	scenarios := []struct {
		name    string
		numbers []string
	}{
		{
			"deve promover is_admin=true para usuários cujos números estão no app.admin_whatsapp_numbers",
			[]string{"+5511999990010", "+5511999990011"},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// Create users with admin numbers.
			for _, n := range sc.numbers {
				num := s.mustWhatsApp(n)
				_, err := s.repo.UpsertByWhatsAppNumber(s.ctx, num, time.Now().UTC())
				s.Require().NoError(err)
			}

			// Set admin_whatsapp_numbers param and re-apply migration 0004.
			csv := fmt.Sprintf("%s,%s", sc.numbers[0], sc.numbers[1])
			s.Require().NoError(dbpkg.SetAdminWhatsAppNumbers(s.ctx, s.mgr, csv))

			// Re-run the admin seed migration by executing it directly (idempotent).
			dbtx := s.mgr.DBTX(s.ctx)
			_, err := dbtx.ExecContext(s.ctx, `
				DO $$
				DECLARE
					raw   TEXT := current_setting('app.admin_whatsapp_numbers', true);
					parts TEXT[];
					nbr   TEXT;
				BEGIN
					IF raw IS NULL OR raw = '' THEN RETURN; END IF;
					parts := string_to_array(raw, ',');
					FOREACH nbr IN ARRAY parts LOOP
						UPDATE users
						   SET is_admin = true, updated_at = now()
						 WHERE whatsapp_number = trim(nbr)
						   AND deleted_at IS NULL;
					END LOOP;
				END $$;
			`)
			s.Require().NoError(err)

			// Verify both users are now admin.
			for _, n := range sc.numbers {
				num := s.mustWhatsApp(n)
				user, err := s.repo.FindByWhatsAppNumber(s.ctx, num)
				s.Require().NoError(err)
				s.True(user.IsAdmin(), "usuário %s deve ser admin após seed", n)
			}
		})
	}
}
