//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

type MigrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
	dsn string
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationSuite))
}

func (s *MigrationSuite) SetupSuite() {
	s.ctx = context.Background()

	db, dsn := testcontainer.Postgres(s.T())
	s.db = db
	s.dsn = dsn
}

func (s *MigrationSuite) SetupTest() {}

func (s *MigrationSuite) newMigrator() *migrate.Migrate {
	_, err := s.db.ExecContext(s.ctx, `CREATE SCHEMA IF NOT EXISTS mecontrola`)
	s.Require().NoError(err)
	driver, err := migratepgx.WithInstance(s.db.DB, &migratepgx.Config{
		MigrationsTable: migratepgx.DefaultMigrationsTable,
		SchemaName:      "mecontrola",
	})
	s.Require().NoError(err)
	src, err := iofs.New(migrations.FS, ".")
	s.Require().NoError(err)
	migrator, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	s.Require().NoError(err)
	return migrator
}

func (s *MigrationSuite) applyBaseline(migrator *migrate.Migrate) {
	upErr := migrator.Up()
	s.Require().True(
		upErr == nil || errors.Is(upErr, migrate.ErrNoChange),
		"up deve ser idempotente: %v",
		upErr,
	)
}

func (s *MigrationSuite) downToVersion(migrator *migrate.Migrate, target uint) {
	for {
		version, dirty, err := migrator.Version()
		if err != nil {
			return
		}
		s.False(dirty)
		if version <= target {
			return
		}
		s.Require().NoError(migrator.Steps(-1))
	}
}

func (s *MigrationSuite) TestBaselineUpDownUp() {
	migrator := s.newMigrator()

	s.applyBaseline(migrator)

	s.assertTablePresent("mecontrola.schema_migrations")
	s.assertTablePresent("mecontrola.billing_plans")
	s.assertTablePresent("mecontrola.categories")
	s.assertTablePresent("mecontrola.category_dictionary")
	s.assertTablePresent("mecontrola.idempotency_keys")
	s.assertTablePresent("mecontrola.cards")
	s.assertTablePresent("mecontrola.banks")
	s.assertTablePresent("mecontrola.budgets")
	s.assertTablePresent("mecontrola.transactions")
	s.assertTablePresent("mecontrola.user_identities")
	s.assertTablePresent("mecontrola.channel_processed_messages")
	s.assertTablePresent("mecontrola.budget_alerts_sent")
	s.assertTablePresent("mecontrola.onboarding_tokens")
	s.assertTablePresent("mecontrola.onboarding_activation_nomatch_throttle")
	s.assertTablePresent("mecontrola.onboarding_welcome_processed")
	s.assertTableMissing("mecontrola.onboarding_sessions")
	s.assertTablePresent("mecontrola.agents_write_ledger")
	s.assertIndexPresent("mecontrola", "onboarding_tokens_mobile_activable_idx")
	s.assertIndexPresent("mecontrola", "outbox_events_user_pending_occurred_idx")
	s.assertIndexPresent("mecontrola", "outbox_events_user_inflight_uidx")

	s.assertTableMissing("mecontrola.meta_processed_messages")
	s.assertTableMissing("mecontrola.telegram_processed_updates")

	s.assertSeededPlans()
	s.assertBanksSeed()
	s.assertExpenseCategoriesCount()
	s.assertIncomeCategoriesCount()
	s.assertDictionaryCanonicalsCount()
	s.assertDictionaryAliasesCount()
	s.assertUnaccentAvailable()
	s.assertPgcryptoAvailable()
	s.assertPgTrgmAvailable()

	s.downToVersion(migrator, 0)

	s.assertTablePresent("mecontrola.schema_migrations")
	s.assertTableMissing("mecontrola.billing_plans")
	s.assertTableMissing("mecontrola.banks")
	s.assertTableMissing("mecontrola.categories")
	s.assertTableMissing("mecontrola.transactions")
	s.assertTableMissing("mecontrola.users")
	s.assertTableMissing("mecontrola.onboarding_activation_nomatch_throttle")
	s.assertTableMissing("mecontrola.onboarding_welcome_processed")
	s.assertIndexMissing("mecontrola", "onboarding_tokens_mobile_activable_idx")
	s.assertIndexMissing("mecontrola", "outbox_events_user_pending_occurred_idx")
	s.assertIndexMissing("mecontrola", "outbox_events_user_inflight_uidx")

	s.applyBaseline(migrator)

	s.assertTablePresent("mecontrola.billing_plans")
	s.assertTablePresent("mecontrola.banks")
	s.assertTablePresent("mecontrola.channel_processed_messages")
	s.assertIndexPresent("mecontrola", "outbox_events_user_pending_occurred_idx")
	s.assertIndexPresent("mecontrola", "outbox_events_user_inflight_uidx")
}

func (s *MigrationSuite) TestFinalSchemaColumnsAndConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertColumnPresent("mecontrola.outbox_events", "aggregate_user_id")
	s.assertColumnPresent("mecontrola.auth_events", "request_id")
	s.assertColumnPresent("mecontrola.auth_events", "client_ip")
	s.assertColumnMissing("mecontrola.cards", "limit_cents")
	s.assertColumnMissing("mecontrola.cards", "name")
	s.assertColumnPresent("mecontrola.cards", "bank")
	s.assertColumnPresent("mecontrola.cards", "version")
	s.assertColumnMissing("mecontrola.onboarding_tokens", "telegram_external_id")
	s.assertColumnPresent("mecontrola.onboarding_tokens", "email_sent_at")
	s.assertColumnPresent("mecontrola.onboarding_tokens", "page_opened_at")
	s.assertColumnPresent("mecontrola.onboarding_tokens", "activation_started_at")
	s.assertColumnPresent("mecontrola.onboarding_tokens", "whatsapp_opened_at")
	s.assertIndexPresent("mecontrola", "outbox_events_user_pending_occurred_idx")
	s.assertIndexPresent("mecontrola", "outbox_events_user_inflight_uidx")
	s.assertIndexPresent("mecontrola", "onboarding_tokens_mobile_activable_idx")

	invalidSourceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, occurred_at)
		VALUES ($1, 'principal_established', 'telegram', now())
	`, uuid.NewString())
	s.Require().Error(invalidSourceErr)
	s.Contains(invalidSourceErr.Error(), "auth_events_source_check")

	validGatewayErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, reason, request_id, client_ip, occurred_at)
		VALUES ($1, 'failed', 'gateway', 'stale_webhook', 'req-1', '127.0.0.1', now())
	`, uuid.NewString())
	s.Require().NoError(validGatewayErr)
}

func (s *MigrationSuite) renamePlatformColumnsToLegacy() {
	stmts := []string{
		`ALTER INDEX mecontrola.platform_messages_platform_thread_id_created_idx RENAME TO platform_messages_thread_created_idx`,
		`ALTER TABLE mecontrola.platform_messages RENAME CONSTRAINT platform_messages_platform_thread_id_fkey TO platform_messages_thread_fkey`,
		`ALTER TABLE mecontrola.platform_messages RENAME COLUMN platform_thread_id TO thread_pk`,
		`ALTER INDEX mecontrola.platform_runs_platform_thread_id_started_idx RENAME TO platform_runs_thread_started_idx`,
		`ALTER TABLE mecontrola.platform_runs RENAME CONSTRAINT platform_runs_platform_thread_id_fkey TO platform_runs_thread_fkey`,
		`ALTER TABLE mecontrola.platform_runs RENAME COLUMN platform_thread_id TO thread_pk`,
		`ALTER TABLE mecontrola.platform_embeddings RENAME COLUMN source_message_id TO source_message_pk`,
	}
	for _, stmt := range stmts {
		s.Require().NoError(execSQL(s.db, s.ctx, stmt))
	}
}

func (s *MigrationSuite) TestReconcilePlatformThreadColumnsFromLegacy() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.renamePlatformColumnsToLegacy()
	s.Require().NoError(migrator.Force(1))

	s.assertColumnPresent("mecontrola.platform_messages", "thread_pk")
	s.assertColumnMissing("mecontrola.platform_messages", "platform_thread_id")
	s.assertColumnPresent("mecontrola.platform_runs", "thread_pk")
	s.assertColumnPresent("mecontrola.platform_embeddings", "source_message_pk")
	s.assertIndexPresent("mecontrola", "platform_messages_thread_created_idx")

	upErr := migrator.Up()
	s.Require().True(upErr == nil || errors.Is(upErr, migrate.ErrNoChange), "up 000002: %v", upErr)

	s.assertColumnPresent("mecontrola.platform_messages", "platform_thread_id")
	s.assertColumnMissing("mecontrola.platform_messages", "thread_pk")
	s.assertColumnPresent("mecontrola.platform_runs", "platform_thread_id")
	s.assertColumnMissing("mecontrola.platform_runs", "thread_pk")
	s.assertColumnPresent("mecontrola.platform_embeddings", "source_message_id")
	s.assertColumnMissing("mecontrola.platform_embeddings", "source_message_pk")
	s.assertIndexPresent("mecontrola", "platform_messages_platform_thread_id_created_idx")
	s.assertIndexPresent("mecontrola", "platform_runs_platform_thread_id_started_idx")
	s.assertIndexMissing("mecontrola", "platform_messages_thread_created_idx")

	version, dirty, err := migrator.Version()
	s.Require().NoError(err)
	s.Equal(uint(8), version)
	s.False(dirty)
}

func (s *MigrationSuite) TestReconcileIsNoopOnFreshBaseline() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertColumnPresent("mecontrola.platform_messages", "platform_thread_id")
	s.assertColumnMissing("mecontrola.platform_messages", "thread_pk")
	s.assertColumnPresent("mecontrola.platform_runs", "platform_thread_id")
	s.assertColumnPresent("mecontrola.platform_embeddings", "source_message_id")
	s.assertColumnMissing("mecontrola.platform_embeddings", "source_message_pk")

	version, dirty, err := migrator.Version()
	s.Require().NoError(err)
	s.Equal(uint(8), version)
	s.False(dirty)
}

func (s *MigrationSuite) TestChannelDedupAndUserIdentitiesConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	insertMessageErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ('whatsapp', 'wamid-1', now())
	`)
	s.Require().NoError(insertMessageErr)

	dupMessageErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ('whatsapp', 'wamid-1', now())
	`)
	s.Require().Error(dupMessageErr)
	s.Contains(dupMessageErr.Error(), "channel_processed_messages_pkey")

	userID := "12121212-1212-1212-1212-121212121212"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999991212', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	insertIdentityErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at)
		VALUES ($1, $2, 'whatsapp', 'external-1', now(), now())
	`, uuid.NewString(), userID)
	s.Require().NoError(insertIdentityErr)

	dupIdentityErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at)
		VALUES ($1, $2, 'whatsapp', 'external-1', now(), now())
	`, uuid.NewString(), userID)
	s.Require().Error(dupIdentityErr)
	s.Contains(dupIdentityErr.Error(), "user_identities_channel_external_active_uniq_idx")

	telegramIdentityErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at)
		VALUES ($1, $2, 'telegram', 'tg-1', now(), now())
	`, uuid.NewString(), userID)
	s.Require().Error(telegramIdentityErr)
	s.Contains(telegramIdentityErr.Error(), "user_identities_channel_check")
}

func (s *MigrationSuite) TestIdempotencyKeysConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	validHash := "a948904f2f0f479b8f936f443923b14a04f830de30e39fa93ef91b5c6a8c5e10"

	statusErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 199, '', now() + interval '1 day')
	`, "key-status-test", userID, validHash)
	s.Require().Error(statusErr)
	s.Contains(statusErr.Error(), "idempotency_keys_status_chk")

	bodyErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, $4, now() + interval '1 day')
	`, "key-body-test", userID, validHash, make([]byte, 65537))
	s.Require().Error(bodyErr)
	s.Contains(bodyErr.Error(), "idempotency_keys_body_size_chk")

	longKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa129x"
	keyErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, '', now() + interval '1 day')
	`, longKey, userID, validHash)
	s.Require().Error(keyErr)
	s.Contains(keyErr.Error(), "idempotency_keys_key_len_chk")
}

func (s *MigrationSuite) TestCardsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999990099', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	closingZeroErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Nubank', 'mycard', 0, 5)
	`, userID)
	s.Require().Error(closingZeroErr)
	s.Contains(closingZeroErr.Error(), "cards_closing_day_chk")

	closing32Err := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Nubank', 'mycard', 32, 5)
	`, userID)
	s.Require().Error(closing32Err)
	s.Contains(closing32Err.Error(), "cards_closing_day_chk")

	emptyNicknameErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Nubank', '', 10, 15)
	`, userID)
	s.Require().Error(emptyNicknameErr)
	s.Contains(emptyNicknameErr.Error(), "cards_nickname_len_chk")

	longBankErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, $2, 'mycard', 10, 15)
	`, userID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	s.Require().Error(longBankErr)
	s.Contains(longBankErr.Error(), "cards_bank_len_chk")

	insertCardErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Nubank', 'mycard', 10, 15)
	`, userID)
	s.Require().NoError(insertCardErr)

	dupNicknameErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Itaú', 'mycard', 15, 20)
	`, userID)
	s.Require().Error(dupNicknameErr)
	s.Contains(dupNicknameErr.Error(), "cards_user_nickname_active_uniq_idx")
}

func (s *MigrationSuite) TestCardsNoPCIColumns() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	prohibitedTerms := []string{"pan", "cvv", "cvc", "track", "pin"}
	for _, term := range prohibitedTerms {
		var count int64
		err := s.db.QueryRowContext(s.ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'mecontrola'
			  AND table_name = 'cards'
			  AND lower(column_name) LIKE '%' || $1 || '%'
		`, term).Scan(&count)
		s.Require().NoError(err)
		s.Equal(int64(0), count)
	}
}

func (s *MigrationSuite) TestCategoriesAndDictionarySeed() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertExpenseCategoriesCount()
	s.assertIncomeCategoriesCount()
	s.assertDeterministicCategoryIDs()
	s.assertAllocationTypesCorrect()
	s.assertMaxDepthTwo()
	s.assertEditorialVersionIncremented()
	s.assertDictionaryCanonicalsCount()
	s.assertDictionaryAliasesCount()
	s.assertDictionaryUniqueness()
	s.assertDictionaryUnaccentNormalization()
}

func (s *MigrationSuite) TestCategoryDictionarySeedV2_aliases() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	probes := []string{"ifood", "mercado", "uber", "gasolina", "curso", "viagem"}

	var presentCount int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary
		WHERE deprecated_at IS NULL
		  AND term_normalized = ANY($1)
	`, probes).Scan(&presentCount)
	s.Require().NoError(err)
	s.GreaterOrEqual(presentCount, int64(6))

	for _, term := range probes {
		var categoryID sql.NullString
		queryErr := s.db.QueryRowContext(s.ctx, `
			SELECT category_id::text FROM mecontrola.category_dictionary
			WHERE deprecated_at IS NULL AND term_normalized = $1
			ORDER BY confidence DESC
			LIMIT 1
		`, term).Scan(&categoryID)
		s.Require().NoErrorf(queryErr, "term=%s", term)
		s.Truef(categoryID.Valid, "term=%s sem category_id", term)
		s.NotEmptyf(categoryID.String, "term=%s mapeamento vazio", term)
	}

	var seedV2Count int64
	seedErr := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary
		WHERE id::text LIKE 'a1b00001-0000-5007-0000-%'
	`).Scan(&seedV2Count)
	s.Require().NoError(seedErr)
	s.GreaterOrEqualf(seedV2Count, int64(60), "seed v2 inseriu %d entradas", seedV2Count)
	s.LessOrEqualf(seedV2Count, int64(200), "seed v2 inseriu %d entradas", seedV2Count)

	var dupCount int64
	dupErr := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM (
			SELECT matched_term FROM (
				SELECT term_normalized AS matched_term, COUNT(*) AS c
				FROM mecontrola.category_dictionary
				WHERE deprecated_at IS NULL
				GROUP BY kind, category_id, term_normalized
				HAVING COUNT(*) > 1
			) inner_t
		) outer_t
	`).Scan(&dupCount)
	s.Require().NoError(dupErr)
	s.Equal(int64(0), dupCount)
}

func (s *MigrationSuite) TestSalarioBaseLeafSeed() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	const salarioRootID = "86dd34b0-7342-525a-9a30-b1b5a76b109f"
	const salarioBaseLeafID = "a1742a1d-85ef-5f94-af85-940e27e32178"
	const decimoTerceiroLeafID = "98455e74-b1f3-5b9c-a8d8-05db0cdb465d"

	var slug, name, kind string
	var parentID sql.NullString
	var allocationType string
	var deprecatedAt sql.NullTime
	err := s.db.QueryRowContext(s.ctx, `
		SELECT slug, name, kind, parent_id::text, allocation_type, deprecated_at
		FROM mecontrola.categories WHERE id = $1::uuid
	`, salarioBaseLeafID).Scan(&slug, &name, &kind, &parentID, &allocationType, &deprecatedAt)
	s.Require().NoError(err)
	s.Equal("salario-base", slug)
	s.Equal("Salário", name)
	s.Equal("income", kind)
	s.True(parentID.Valid)
	s.Equal(salarioRootID, parentID.String)
	s.Equal("consumption", allocationType)
	s.False(deprecatedAt.Valid)

	s.assertDictionaryTermResolvesTo("salario", "income", salarioBaseLeafID)
	s.assertDictionaryTermResolvesTo("meu salario", "income", salarioBaseLeafID)
	s.assertDictionaryTermResolvesTo("recebi salario", "income", salarioBaseLeafID)
	s.assertDictionaryTermResolvesTo("recebi meu salario", "income", salarioBaseLeafID)

	s.assertDictionaryTermResolvesTo("decimo terceiro salario", "income", decimoTerceiroLeafID)
	s.assertDictionaryTermResolvesTo("13 salario", "income", decimoTerceiroLeafID)

	var editorialVersion int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT version FROM mecontrola.category_editorial_version`,
	).Scan(&editorialVersion))
	s.GreaterOrEqual(editorialVersion, int64(5))

	upErrRerun := migrator.Up()
	s.Require().True(
		upErrRerun == nil || errors.Is(upErrRerun, migrate.ErrNoChange),
		"reexecucao da migracao de seed deve ser idempotente: %v",
		upErrRerun,
	)

	var leafCountAfterRerun int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.categories WHERE id = $1::uuid`,
		salarioBaseLeafID,
	).Scan(&leafCountAfterRerun))
	s.Equal(int64(1), leafCountAfterRerun)

	var dictCountAfterRerun int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE category_id = $1::uuid AND deprecated_at IS NULL`,
		salarioBaseLeafID,
	).Scan(&dictCountAfterRerun))
	s.Equal(int64(4), dictCountAfterRerun)
}

func (s *MigrationSuite) TestFixDecimoTerceiroDictionaryCollision() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	const salarioBaseLeafID = "a1742a1d-85ef-5f94-af85-940e27e32178"
	const decimoTerceiroLeafID = "98455e74-b1f3-5b9c-a8d8-05db0cdb465d"

	var salarioBaseSignalType string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT signal_type FROM mecontrola.category_dictionary
		WHERE id = '1382e2c5-db89-5abf-9793-93fb89053937'::uuid
	`).Scan(&salarioBaseSignalType)
	s.Require().NoError(err)
	s.Equal("alias", salarioBaseSignalType,
		"RF-05: termo canonico 'salario' da folha-base deve ser rebaixado para 'alias' para nao vencer aliases de Decimo Terceiro na busca por token")

	s.assertDictionaryTermResolvesTo("recebi meu 13º salario", "income", decimoTerceiroLeafID)
	s.assertDictionaryTermResolvesTo("13o salario", "income", decimoTerceiroLeafID)
	s.assertDictionaryTermResolvesTo("recebi 13º salario", "income", decimoTerceiroLeafID)
	s.assertDictionaryTermResolvesTo("recebi decimo terceiro", "income", decimoTerceiroLeafID)

	s.assertDictionaryTermResolvesTo("salario", "income", salarioBaseLeafID)

	upErrRerun := migrator.Up()
	s.Require().True(
		upErrRerun == nil || errors.Is(upErrRerun, migrate.ErrNoChange),
		"reexecucao da migracao 000007 deve ser idempotente: %v",
		upErrRerun,
	)

	var dictCountAfterRerun int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE category_id = $1::uuid AND deprecated_at IS NULL`,
		decimoTerceiroLeafID,
	).Scan(&dictCountAfterRerun))
	s.GreaterOrEqual(dictCountAfterRerun, int64(9))
}

func (s *MigrationSuite) assertDictionaryTermResolvesTo(termNormalized, kind, expectedCategoryID string) {
	var categoryID string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT category_id::text FROM mecontrola.category_dictionary
		WHERE deprecated_at IS NULL AND kind = $1 AND term_normalized = $2
		LIMIT 1
	`, kind, termNormalized).Scan(&categoryID)
	s.Require().NoErrorf(err, "term_normalized=%s", termNormalized)
	s.Equalf(expectedCategoryID, categoryID, "term_normalized=%s resolveu para categoria inesperada", termNormalized)
}

func (s *MigrationSuite) TestAuthEventsResolvePathAndCorrelationBackfill() {
	migrator := s.newMigrator()

	migrateErr := migrator.Migrate(7)
	s.Require().True(
		migrateErr == nil || errors.Is(migrateErr, migrate.ErrNoChange),
		"baseline ate 000007 deve aplicar: %v",
		migrateErr,
	)

	const legacyThreadID = "11111111-2222-3333-4444-555555555555"
	const legacyRunEmptyID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	const legacyRunFilledID = "ffffffff-1111-2222-3333-444444444444"

	insertThreadErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id)
		VALUES ($1, 'resource-legacy', 'thread-legacy')
	`, legacyThreadID)
	s.Require().NoError(insertThreadErr)

	insertEmptyRunErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.platform_runs (id, platform_thread_id, resource_id, thread_id, status, correlation_key)
		VALUES ($1, $2, 'resource-legacy', 'thread-legacy', 'succeeded', '')
	`, legacyRunEmptyID, legacyThreadID)
	s.Require().NoError(insertEmptyRunErr)

	insertFilledRunErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.platform_runs (id, platform_thread_id, resource_id, thread_id, status, correlation_key)
		VALUES ($1, $2, 'resource-legacy', 'thread-legacy', 'succeeded', 'wamid.EXISTING')
	`, legacyRunFilledID, legacyThreadID)
	s.Require().NoError(insertFilledRunErr)

	stepErr := migrator.Steps(1)
	s.Require().NoError(stepErr, "000008 up deve aplicar com runs legados vazios (backfill antes do CHECK)")

	version, dirty, versionErr := migrator.Version()
	s.Require().NoError(versionErr)
	s.False(dirty)
	s.Equal(uint(8), version)

	s.assertColumnPresent("mecontrola.auth_events", "resolve_path")
	s.assertConstraintPresent("auth_events", "auth_events_resolve_path_chk")
	s.assertConstraintPresent("auth_events", "auth_events_reason_check")
	s.assertConstraintPresent("platform_runs", "platform_runs_correlation_len_chk")

	var emptyCount int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_runs WHERE correlation_key = ''`,
	).Scan(&emptyCount))
	s.Equal(int64(0), emptyCount, "backfill deve eliminar todo correlation_key vazio")

	var backfilledKey string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT correlation_key FROM mecontrola.platform_runs WHERE id = $1::uuid`, legacyRunEmptyID,
	).Scan(&backfilledKey))
	s.Equal("legacy:"+legacyRunEmptyID, backfilledKey, "run legado vazio deve migrar para 'legacy:' || id")

	var untouchedKey string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT correlation_key FROM mecontrola.platform_runs WHERE id = $1::uuid`, legacyRunFilledID,
	).Scan(&untouchedKey))
	s.Equal("wamid.EXISTING", untouchedKey, "run com correlation_key preenchido nao deve ser tocado pelo backfill")

	validPathErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, resolve_path)
		VALUES (gen_random_uuid(), 'principal_established', 'whatsapp', 'legacy')
	`)
	s.Require().NoError(validPathErr)

	nullPathErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, resolve_path)
		VALUES (gen_random_uuid(), 'principal_established', 'whatsapp', NULL)
	`)
	s.Require().NoError(nullPathErr, "resolve_path NULL deve ser aceito (coluna nullable)")

	invalidPathErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, resolve_path)
		VALUES (gen_random_uuid(), 'principal_established', 'whatsapp', 'bogus')
	`)
	s.Require().Error(invalidPathErr)
	s.Contains(invalidPathErr.Error(), "auth_events_resolve_path_chk")

	failedReasonPreservedErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, reason)
		VALUES (gen_random_uuid(), 'principal_established', 'whatsapp', 'invalid_signature')
	`)
	s.Require().Error(failedReasonPreservedErr)
	s.Contains(failedReasonPreservedErr.Error(), "auth_events_reason_check")

	emptyCorrelationErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.platform_runs (id, platform_thread_id, resource_id, thread_id, status, correlation_key)
		VALUES (gen_random_uuid(), $1, 'resource-legacy', 'thread-legacy', 'running', '')
	`, legacyThreadID)
	s.Require().Error(emptyCorrelationErr)
	s.Contains(emptyCorrelationErr.Error(), "platform_runs_correlation_len_chk")

	longCorrelationErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.platform_runs (id, platform_thread_id, resource_id, thread_id, status, correlation_key)
		VALUES (gen_random_uuid(), $1, 'resource-legacy', 'thread-legacy', 'running', $2)
	`, legacyThreadID, strings.Repeat("x", 257))
	s.Require().Error(longCorrelationErr)
	s.Contains(longCorrelationErr.Error(), "platform_runs_correlation_len_chk")

	downErr := migrator.Steps(-1)
	s.Require().NoError(downErr, "000008 down deve reverter")

	s.assertColumnMissing("mecontrola.auth_events", "resolve_path")
	s.assertConstraintMissing("auth_events", "auth_events_resolve_path_chk")
	s.assertConstraintMissing("platform_runs", "platform_runs_correlation_len_chk")
	s.assertConstraintPresent("auth_events", "auth_events_reason_check")

	var emptyAfterDownCount int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_runs WHERE correlation_key = ''`,
	).Scan(&emptyAfterDownCount))
	s.Equal(int64(0), emptyAfterDownCount, "down nao desfaz o backfill (idempotente)")

	reUpErr := migrator.Steps(1)
	s.Require().NoError(reUpErr, "000008 up reaplicado deve ser reentrante")

	s.assertColumnPresent("mecontrola.auth_events", "resolve_path")
	s.assertConstraintPresent("auth_events", "auth_events_resolve_path_chk")
	s.assertConstraintPresent("platform_runs", "platform_runs_correlation_len_chk")
}

func (s *MigrationSuite) assertConstraintPresent(table, constraint string) {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'mecontrola'
		  AND t.relname = $1
		  AND c.conname = $2
	`, table, constraint).Scan(&count)
	s.Require().NoError(err)
	s.Equalf(int64(1), count, "constraint %s.%s ausente", table, constraint)
}

func (s *MigrationSuite) assertConstraintMissing(table, constraint string) {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'mecontrola'
		  AND t.relname = $1
		  AND c.conname = $2
	`, table, constraint).Scan(&count)
	s.Require().NoError(err)
	s.Equalf(int64(0), count, "constraint %s.%s deveria ter sido removida", table, constraint)
}

func (s *MigrationSuite) TestTransactionsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "aaaaaaaa-1111-1111-1111-aaaaaaaaaaaa"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880010', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	invalidAmountErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, category_name_snapshot, ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 1, 1, 0, 'teste',
			gen_random_uuid(), 'cat', '2026-06', now(), now(), now()
		)
	`, userID)
	s.Require().Error(invalidAmountErr)

	cardID := uuid.New()
	cardInvoiceID := uuid.New()
	insertInvoiceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_card_invoices (
			id, user_id, card_id, ref_month, closing_at, due_at, created_at, updated_at
		) VALUES ($1, $2, $3, '2026-06', now(), now(), now(), now())
	`, cardInvoiceID, userID, cardID)
	s.Require().NoError(insertInvoiceErr)

	dupInvoiceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_card_invoices (
			id, user_id, card_id, ref_month, closing_at, due_at, created_at, updated_at
		) VALUES (gen_random_uuid(), $1, $2, '2026-06', now(), now(), now(), now())
	`, userID, cardID)
	s.Require().Error(dupInvoiceErr)
	s.Contains(dupInvoiceErr.Error(), "transactions_card_invoices_uk")
}

func (s *MigrationSuite) TestBudgetsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertBudgetsUniqueConstraint()
	s.assertBudgetsExpensesIdentityUnique()
	s.assertBudgetsExpensesPartialIndex()
	s.assertBudgetsThresholdStatesConstraints()
	s.assertBudgetsPendingEventIdempotency()
}

func (s *MigrationSuite) assertSeededPlans() {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT code, duration_days
		FROM mecontrola.billing_plans
		ORDER BY duration_days
	`)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		s.Require().NoError(rows.Close())
	})

	type planRow struct {
		code         string
		durationDays int
	}

	var plans []planRow
	for rows.Next() {
		var row planRow
		s.Require().NoError(rows.Scan(&row.code, &row.durationDays))
		plans = append(plans, row)
	}
	s.Require().NoError(rows.Err())
	s.Require().Equal([]planRow{
		{code: "MONTHLY", durationDays: 30},
		{code: "QUARTERLY", durationDays: 90},
		{code: "ANNUAL", durationDays: 365},
	}, plans)
}

func (s *MigrationSuite) assertTablePresent(name string) {
	var regclass sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.True(regclass.Valid)
}

func (s *MigrationSuite) assertTableMissing(name string) {
	var regclass sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.False(regclass.Valid)
}

func (s *MigrationSuite) assertColumnPresent(table, column string) {
	parts := splitTableName(table)
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name = $3
	`, parts[0], parts[1], column).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(1), count)
}

func (s *MigrationSuite) assertColumnMissing(table, column string) {
	parts := splitTableName(table)
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name = $3
	`, parts[0], parts[1], column).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}

func (s *MigrationSuite) assertUnaccentAvailable() {
	var result bool
	err := s.db.QueryRowContext(s.ctx, `SELECT mecontrola.unaccent('á') = 'a'`).Scan(&result)
	s.Require().NoError(err)
	s.True(result)
}

func (s *MigrationSuite) assertPgcryptoAvailable() {
	var result bool
	err := s.db.QueryRowContext(s.ctx, `SELECT gen_random_uuid() IS NOT NULL`).Scan(&result)
	s.Require().NoError(err)
	s.True(result)
}

func (s *MigrationSuite) assertPgTrgmAvailable() {
	var result bool
	err := s.db.QueryRowContext(s.ctx, `SELECT similarity('mercado', 'mercadinho') > 0`).Scan(&result)
	s.Require().NoError(err)
	s.True(result)
}

func (s *MigrationSuite) assertExpenseCategoriesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.categories WHERE kind = 'expense'`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(88))
}

func (s *MigrationSuite) assertIncomeCategoriesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.categories WHERE kind = 'income'`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(21))
}

func (s *MigrationSuite) assertDeterministicCategoryIDs() {
	rows, err := s.db.QueryContext(s.ctx, `SELECT slug, kind, id FROM mecontrola.categories`)
	s.Require().NoError(err)
	defer rows.Close()

	for rows.Next() {
		var slug string
		var kind string
		var id string
		s.Require().NoError(rows.Scan(&slug, &kind, &id))
	}
	s.Require().NoError(rows.Err())
}

func (s *MigrationSuite) assertAllocationTypesCorrect() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.categories
		WHERE kind = 'expense' AND parent_id IS NOT NULL AND allocation_type = 'asset_allocation'
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(1))
}

func (s *MigrationSuite) assertMaxDepthTwo() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.categories c1
		JOIN mecontrola.categories c2 ON c1.parent_id = c2.id
		WHERE c2.parent_id IS NOT NULL
	`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}

func (s *MigrationSuite) assertEditorialVersionIncremented() {
	var version int64
	err := s.db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version`).Scan(&version)
	s.Require().NoError(err)
	s.GreaterOrEqual(version, int64(4))
}

func (s *MigrationSuite) assertDictionaryCanonicalsCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE signal_type = 'canonical_name'
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(60))
}

func (s *MigrationSuite) assertDictionaryAliasesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE signal_type IN ('alias', 'phrase')
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(10))
}

func (s *MigrationSuite) assertDictionaryUniqueness() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM (
			SELECT kind, category_id, term_normalized, COUNT(*) AS c
			FROM mecontrola.category_dictionary
			WHERE deprecated_at IS NULL
			GROUP BY kind, category_id, term_normalized
			HAVING COUNT(*) > 1
		) t
	`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}

func (s *MigrationSuite) assertDictionaryUnaccentNormalization() {
	var result string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT term_normalized FROM mecontrola.category_dictionary WHERE term = 'gás encanado' LIMIT 1
	`).Scan(&result)
	s.Require().NoError(err)
	s.Equal("gas encanado", result)
}

func (s *MigrationSuite) assertBudgetsUniqueConstraint() {
	userID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, '2026-01', 0, 1, now(), now())
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, '2026-01', 0, 1, now(), now())
	`, userID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_user_comp_uk")
}

func (s *MigrationSuite) assertBudgetsExpensesIdentityUnique() {
	userID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expenses (
			id, user_id, source, external_transaction_id, subcategory_id,
			root_slug, competence, amount_cents, occurred_at, version,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 'api', 'ext-001',
			gen_random_uuid(), 'expense.custo_fixo', '2026-01',
			1000, now(), 1, now(), now()
		)
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expenses (
			id, user_id, source, external_transaction_id, subcategory_id,
			root_slug, competence, amount_cents, occurred_at, version,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 'api', 'ext-001',
			gen_random_uuid(), 'expense.custo_fixo', '2026-01',
			2000, now(), 1, now(), now()
		)
	`, userID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_expenses_identity_uk")
}

func (s *MigrationSuite) assertBudgetsExpensesPartialIndex() {
	rows, err := s.db.QueryContext(s.ctx, `
		EXPLAIN SELECT root_slug, SUM(amount_cents)
		FROM mecontrola.budgets_expenses
		WHERE user_id = 'dddddddd-dddd-dddd-dddd-dddddddddddd'
		  AND competence = '2026-01'
		  AND deleted_at IS NULL
		GROUP BY root_slug
	`)
	s.Require().NoError(err)
	defer rows.Close()

	var hasRows bool
	for rows.Next() {
		hasRows = true
	}
	s.Require().NoError(rows.Err())
	s.True(hasRows)
}

func (s *MigrationSuite) assertBudgetsThresholdStatesConstraints() {
	userID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-01', 'expense.custo_fixo', 80, false, 0)
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-01', 'expense.custo_fixo', 80, false, 0)
	`, userID)
	s.Require().Error(dupErr)

	invalidThresholdErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-02', 'expense.custo_fixo', 50, false, 0)
	`, userID)
	s.Require().Error(invalidThresholdErr)
	s.Contains(invalidThresholdErr.Error(), "budgets_threshold_states_threshold_chk")
}

func (s *MigrationSuite) assertBudgetsPendingEventIdempotency() {
	eventID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expense_events_pending (
			id, event_id, source, user_id, external_transaction_id,
			expected_version, mutation_kind, payload, state, received_at
		) VALUES (
			gen_random_uuid(), $1, 'api',
			'aaaaaaaa-0000-0000-0000-000000000001', 'ext-pending-001',
			1, 1, '{}', 1, now()
		)
	`, eventID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expense_events_pending (
			id, event_id, source, user_id, external_transaction_id,
			expected_version, mutation_kind, payload, state, received_at
		) VALUES (
			gen_random_uuid(), $1, 'api',
			'aaaaaaaa-0000-0000-0000-000000000001', 'ext-pending-001',
			1, 1, '{}', 1, now()
		)
	`, eventID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_expense_events_pending_event_uk")
}

func (s *MigrationSuite) assertBanksSeed() {
	type bankRow struct {
		code          string
		daysBeforeDue int
	}

	rows, err := s.db.QueryContext(s.ctx, `
		SELECT code, days_before_due
		FROM mecontrola.banks
		ORDER BY code
	`)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = rows.Close()
	})

	var banks []bankRow
	for rows.Next() {
		var row bankRow
		s.Require().NoError(rows.Scan(&row.code, &row.daysBeforeDue))
		banks = append(banks, row)
	}
	s.Require().NoError(rows.Err())
	s.Require().Len(banks, 8)

	expected := map[string]int{
		"nubank":          7,
		"itau":            8,
		"santander":       8,
		"bradesco":        7,
		"banco-do-brasil": 7,
		"caixa":           7,
		"inter":           7,
		"c6-bank":         7,
	}
	for _, b := range banks {
		days, ok := expected[b.code]
		s.Truef(ok, "banco inesperado: %s", b.code)
		s.Equalf(days, b.daysBeforeDue, "banco %s: days_before_due incorreto", b.code)
	}
}

func (s *MigrationSuite) TestActivationJourneyThrottleTableConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	fixedWindow := "2026-01-01 00:00:00+00"

	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.onboarding_activation_nomatch_throttle (mobile_e164, window_start)
		VALUES ('+5511999990001', $1)
	`, fixedWindow)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.onboarding_activation_nomatch_throttle (mobile_e164, window_start)
		VALUES ('+5511999990001', $1)
	`, fixedWindow)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "onboarding_activation_nomatch_throttle_pkey")
}

func (s *MigrationSuite) TestClaimParticionadoInflightUniqueBackstop() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := uuid.New()

	insertFirstErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.outbox_events (
			id, event_type, aggregate_type, aggregate_id, aggregate_user_id,
			payload, status, attempts, max_attempts, occurred_at, next_attempt_at,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), 'test.event', 'whatsapp', gen_random_uuid(), $1,
			'{}', 2, 0, 3, now(), now(), now(), now()
		)
	`, userID)
	s.Require().NoError(insertFirstErr)

	insertDupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.outbox_events (
			id, event_type, aggregate_type, aggregate_id, aggregate_user_id,
			payload, status, attempts, max_attempts, occurred_at, next_attempt_at,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), 'test.event', 'whatsapp', gen_random_uuid(), $1,
			'{}', 2, 0, 3, now(), now(), now(), now()
		)
	`, userID)
	s.Require().Error(insertDupErr)
	s.Contains(insertDupErr.Error(), "outbox_events_user_inflight_uidx")
}

func (s *MigrationSuite) assertIndexPresent(schema, indexName string) {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = $1
		  AND indexname = $2
	`, schema, indexName).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(1), count, "index %s.%s should be present", schema, indexName)
}

func (s *MigrationSuite) assertIndexMissing(schema, indexName string) {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = $1
		  AND indexname = $2
	`, schema, indexName).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count, "index %s.%s should be absent", schema, indexName)
}

func (s *MigrationSuite) TestCategoryWriteGateBaseline() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	var expenseRootID, expenseLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL LIMIT 1`,
	).Scan(&expenseRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseLeafID))

	var incomeRootID, incomeLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='income' AND parent_id IS NULL LIMIT 1`,
	).Scan(&incomeRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='income' AND parent_id=$1::uuid LIMIT 1`,
		incomeRootID,
	).Scan(&incomeLeafID))

	var expenseRoot2ID, expenseLeaf2ID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL AND id<>$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseRoot2ID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRoot2ID,
	).Scan(&expenseLeaf2ID))

	var currentVersion int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT version FROM mecontrola.category_editorial_version`,
	).Scan(&currentVersion))

	userID := "bb222222-2222-2222-2222-222222222222"
	s.Require().NoError(execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880200', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID))

	err1 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'despesa valida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().NoError(err1)

	err2 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 1, 1, 2000, 'receita valida',
			$2::uuid, $3::uuid,
			'renda', 'salario cat',
			'income', 'income/root', 'matched', 1.0,
			'high', 'exact', 'canonical_name',
			'salario', 'canonical match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, incomeRootID, incomeLeafID, currentVersion)
	s.Require().NoError(err2)

	err3 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategoria igual raiz',
			$2::uuid, $2::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$3, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, currentVersion)
	s.Require().Error(err3)

	err4 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'outcome invalido',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'no_match', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err4)
	s.Contains(err4.Error(), "transactions_category_outcome_chk")

	err5 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'score invalido',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 1.5,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err5)
	s.Contains(err5.Error(), "transactions_category_score_chk")

	err6 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'confidence invalida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'invalid', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err6)
	s.Contains(err6.Error(), "transactions_category_confidence_chk")

	err7 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'match_quality invalida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'invalid', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err7)
	s.Contains(err7.Error(), "transactions_category_match_quality_chk")

	err8 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'signal_type invalido',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'invalid',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err8)
	s.Contains(err8.Error(), "transactions_category_signal_type_chk")

	err9 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'decision_source invalida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'invalid',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err9)
	s.Contains(err9.Error(), "transactions_category_decision_source_chk")

	err10 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'editorial_version zero',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			0, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID)
	s.Require().Error(err10)
	s.Contains(err10.Error(), "category_editorial_version")

	err11 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'matched_term vazio',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err11)
	s.Contains(err11.Error(), "transactions_category_matched_term_chk")

	err12 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'match_reason vazio',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', '', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err12)
	s.Contains(err12.Error(), "transactions_category_match_reason_chk")

	err13 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_path vazio',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', '', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err13)
	s.Contains(err13.Error(), "transactions_category_path_chk")

	err14 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_id eh folha',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseLeafID, expenseRootID, currentVersion)
	s.Require().Error(err14)
	s.Contains(err14.Error(), "category_must_be_root")

	err15 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory de raiz diferente',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeaf2ID, currentVersion)
	s.Require().Error(err15)
	s.Contains(err15.Error(), "subcategory_must_be_direct_leaf")

	err16 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'income com direction expense',
			$2::uuid, $3::uuid,
			'renda', 'salario cat',
			'income', 'income/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'salario', 'canonical match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, incomeRootID, incomeLeafID, currentVersion)
	s.Require().Error(err16)
	s.Contains(err16.Error(), "category_direction_kind_mismatch")

	err17 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'editorial version errada',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion+1)
	s.Require().Error(err17)
	s.Contains(err17.Error(), "category_editorial_version_drift")

	err18 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_kind drift',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'income', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err18)
	s.Contains(err18.Error(), "category_kind_column_drift")

	err22 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_name_snapshot vazio',
			$2::uuid, $3::uuid,
			'', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err22)
	s.Contains(err22.Error(), "transactions_category_name_snapshot_chk")

	err23 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory_name_snapshot vazio',
			$2::uuid, $3::uuid,
			'categoria', '',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err23)
	s.Contains(err23.Error(), "subcategory_name_snapshot_chk")

	err24 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 3, 1, 1000, 'direction invalida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err24)
	s.Contains(err24.Error(), "transactions_direction_chk")

	_, errDepRoot := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.categories SET deprecated_at = now() WHERE id = $1::uuid`,
		expenseRootID,
	)
	s.Require().NoError(errDepRoot)
	defer func() {
		_, _ = s.db.ExecContext(s.ctx,
			`UPDATE mecontrola.categories SET deprecated_at = NULL WHERE id = $1::uuid`,
			expenseRootID,
		)
	}()
	err19 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'raiz deprecated',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err19)
	s.Contains(err19.Error(), "root_category_deprecated")

	_, errDepLeaf := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.categories SET deprecated_at = now() WHERE id = $1::uuid`,
		incomeLeafID,
	)
	s.Require().NoError(errDepLeaf)
	defer func() {
		_, _ = s.db.ExecContext(s.ctx,
			`UPDATE mecontrola.categories SET deprecated_at = NULL WHERE id = $1::uuid`,
			incomeLeafID,
		)
	}()
	err20 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 1, 1, 2000, 'folha deprecated',
			$2::uuid, $3::uuid,
			'renda', 'salario cat',
			'income', 'income/root', 'matched', 1.0,
			'high', 'exact', 'canonical_name',
			'salario', 'canonical match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, incomeRootID, incomeLeafID, currentVersion)
	s.Require().Error(err20)
	s.Contains(err20.Error(), "leaf_category_deprecated")

	err21 := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'fk inexistente',
			gen_random_uuid(), $2::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$3, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseLeafID, currentVersion)
	s.Require().Error(err21)
	s.True(
		strings.Contains(err21.Error(), "category_must_be_root") || strings.Contains(err21.Error(), "foreign key"),
		"expected trigger or FK to reject non-existent category UUID, got: %s", err21.Error(),
	)
}

func (s *MigrationSuite) TestCategoryWriteGateRecurringTemplates() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	var expenseRootID, expenseLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL LIMIT 1`,
	).Scan(&expenseRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseLeafID))

	var expenseRoot2ID, expenseLeaf2ID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL AND id<>$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseRoot2ID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRoot2ID,
	).Scan(&expenseLeaf2ID))

	var incomeRootID, incomeLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='income' AND parent_id IS NULL LIMIT 1`,
	).Scan(&incomeRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='income' AND parent_id=$1::uuid LIMIT 1`,
		incomeRootID,
	).Scan(&incomeLeafID))

	var currentVersion int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT version FROM mecontrola.category_editorial_version`,
	).Scan(&currentVersion))

	userID := "cc333333-3333-3333-3333-333333333333"
	s.Require().NoError(execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880300', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID))

	errValid := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'recorrencia valida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().NoError(errValid)

	errCrossRoot := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory raiz diferente',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeaf2ID, currentVersion)
	s.Require().Error(errCrossRoot)
	s.Contains(errCrossRoot.Error(), "subcategory_must_be_direct_leaf")

	errOutcome := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'outcome invalido',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'no_match', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errOutcome)
	s.Contains(errOutcome.Error(), "transactions_rt_category_outcome_chk")

	errVersionDrift := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'editorial version errada',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion+1)
	s.Require().Error(errVersionDrift)
	s.Contains(errVersionDrift.Error(), "category_editorial_version_drift")

	errKindMismatch := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'income com direction expense',
			$2::uuid, $3::uuid,
			'renda', 'salario cat',
			'income', 'income/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'salario', 'canonical match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, incomeRootID, incomeLeafID, currentVersion)
	s.Require().Error(errKindMismatch)
	s.Contains(errKindMismatch.Error(), "category_direction_kind_mismatch")

	errKindDrift := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_kind drift',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'income', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errKindDrift)
	s.Contains(errKindDrift.Error(), "category_kind_column_drift")

	errMustBeRoot := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_id eh folha',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseLeafID, expenseRootID, currentVersion)
	s.Require().Error(errMustBeRoot)
	s.Contains(errMustBeRoot.Error(), "category_must_be_root")

	errNameSnapshot := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'category_name_snapshot vazio',
			$2::uuid, $3::uuid,
			'', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errNameSnapshot)
	s.Contains(errNameSnapshot.Error(), "transactions_rt_category_name_snapshot_chk")

	errSubNameSnapshot := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory_name_snapshot vazio',
			$2::uuid, $3::uuid,
			'categoria', '',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errSubNameSnapshot)
	s.Contains(errSubNameSnapshot.Error(), "transactions_rt_subcategory_name_snapshot_chk")

	errDirection := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 3, 1, 1000, 'direction invalida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errDirection)
	s.Contains(errDirection.Error(), "transactions_rt_direction_chk")

	_, errDepRoot := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.categories SET deprecated_at = now() WHERE id = $1::uuid`,
		expenseRootID,
	)
	s.Require().NoError(errDepRoot)
	defer func() {
		_, _ = s.db.ExecContext(s.ctx,
			`UPDATE mecontrola.categories SET deprecated_at = NULL WHERE id = $1::uuid`,
			expenseRootID,
		)
	}()
	errRootDeprecated := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'raiz deprecated',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(errRootDeprecated)
	s.Contains(errRootDeprecated.Error(), "root_category_deprecated")

	_, errDepLeaf := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.categories SET deprecated_at = now() WHERE id = $1::uuid`,
		incomeLeafID,
	)
	s.Require().NoError(errDepLeaf)
	defer func() {
		_, _ = s.db.ExecContext(s.ctx,
			`UPDATE mecontrola.categories SET deprecated_at = NULL WHERE id = $1::uuid`,
			incomeLeafID,
		)
	}()
	errLeafDeprecated := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 1, 1, 2000, 'folha deprecated',
			$2::uuid, $3::uuid,
			'renda', 'salario cat',
			'income', 'income/root', 'matched', 1.0,
			'high', 'exact', 'canonical_name',
			'salario', 'canonical match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, incomeRootID, incomeLeafID, currentVersion)
	s.Require().Error(errLeafDeprecated)
	s.Contains(errLeafDeprecated.Error(), "leaf_category_deprecated")
}

func splitTableName(qualified string) [2]string {
	for i := 0; i < len(qualified); i++ {
		if qualified[i] == '.' {
			return [2]string{qualified[:i], qualified[i+1:]}
		}
	}
	return [2]string{"public", qualified}
}

func execSQL(db database.DBTX, ctx context.Context, query string, args ...any) error {
	_, err := db.ExecContext(ctx, query, args...)
	return err
}
