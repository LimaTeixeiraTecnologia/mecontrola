package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	robfigcron "github.com/robfig/cron/v3"
)

// CronDeps agrupa as dependências do Cron (DI via construtor, R6.6).
// Todos os campos são obrigatórios exceto Logger (default: slog.Default()).
type CronDeps struct {
	// Storage é a porta de acesso ao banco de dados para housekeeping e reaper.
	Storage Storage
	// Metrics é a fachada OTel para registro de counters de housekeeping e reaper.
	Metrics *OutboxMetrics
	// Logger é o logger estruturado. Se nil, usa slog.Default().
	Logger *slog.Logger
	// Clock é a fonte de tempo injetável (permite testes determinísticos).
	Clock Clock
	// HousekeepingSchedule é a expressão cron para o housekeeping (ex: "@daily").
	HousekeepingSchedule string
	// ReaperInterval é a expressão cron para o reaper (ex: "@every 1m").
	ReaperInterval string
	// RetentionDays é o número de dias de retenção para o housekeeping.
	RetentionDays int
	// ReaperStuckAfter é o tempo após o qual uma delivery claimed é considerada travada.
	ReaperStuckAfter time.Duration
}

// Cron orquestra dois jobs periódicos via robfig/cron/v3 (D-04):
//   - housekeeping (OUTBOX_HOUSEKEEPING_SCHEDULE, default @daily): apaga deliveries antigas.
//   - reaper (OUTBOX_REAPER_INTERVAL, default @every 1m): libera deliveries claimed travadas.
//
// Regra de import: cron.go NÃO importa pgx. Toda persistência passa por Storage.
// Regra de segurança: payload jamais aparece em chamada slog.* (R-SEC-001).
type Cron struct {
	inner             *robfigcron.Cron
	storage           Storage
	metrics           *OutboxMetrics
	logger            *slog.Logger
	clock             Clock
	housekeepingSched string
	reaperInterval    string
	retentionDays     int
	reaperStuckAfter  time.Duration
}

// NewCron cria um Cron validado a partir de CronDeps.
// Retorna erro se Storage for nil, se RetentionDays <= 0, ou se ReaperStuckAfter <= 0.
// Não valida o parse dos schedules aqui — isso é feito em Start (defensive, além do boot em config.go).
func NewCron(deps CronDeps) (*Cron, error) {
	if deps.Storage == nil {
		return nil, fmt.Errorf("outbox: cron: storage é obrigatório")
	}
	if deps.RetentionDays <= 0 {
		return nil, fmt.Errorf("outbox: cron: retention_days deve ser > 0")
	}
	if deps.ReaperStuckAfter <= 0 {
		return nil, fmt.Errorf("outbox: cron: reaper_stuck_after deve ser > 0")
	}
	if deps.HousekeepingSchedule == "" {
		deps.HousekeepingSchedule = "@daily"
	}
	if deps.ReaperInterval == "" {
		deps.ReaperInterval = "@every 1m"
	}
	if deps.Clock == nil {
		deps.Clock = RealClock()
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	inner := robfigcron.New(robfigcron.WithLogger(robfigcron.DiscardLogger))

	return &Cron{
		inner:             inner,
		storage:           deps.Storage,
		metrics:           deps.Metrics,
		logger:            deps.Logger,
		clock:             deps.Clock,
		housekeepingSched: deps.HousekeepingSchedule,
		reaperInterval:    deps.ReaperInterval,
		retentionDays:     deps.RetentionDays,
		reaperStuckAfter:  deps.ReaperStuckAfter,
	}, nil
}

// Start registra os dois jobs e inicia o cron scheduler.
// Valida os schedules defensivamente (parse-check) antes de iniciar.
// Retorna erro se qualquer schedule for inválido.
func (c *Cron) Start(ctx context.Context) error {
	// Validação defensiva dos schedules (além do boot em config.go).
	if _, err := robfigcron.ParseStandard(c.housekeepingSched); err != nil {
		return fmt.Errorf("outbox: cron: housekeeping schedule inválido %q: %w", c.housekeepingSched, err)
	}
	if _, err := robfigcron.ParseStandard(c.reaperInterval); err != nil {
		return fmt.Errorf("outbox: cron: reaper interval inválido %q: %w", c.reaperInterval, err)
	}

	if _, err := c.inner.AddFunc(c.housekeepingSched, func() {
		c.runHousekeeping(ctx)
	}); err != nil {
		return fmt.Errorf("outbox: cron: registrando housekeeping: %w", err)
	}

	if _, err := c.inner.AddFunc(c.reaperInterval, func() {
		c.runReaper(ctx)
	}); err != nil {
		return fmt.Errorf("outbox: cron: registrando reaper: %w", err)
	}

	c.inner.Start()
	c.logger.InfoContext(ctx, "outbox: cron iniciado",
		slog.String("housekeeping_schedule", c.housekeepingSched),
		slog.String("reaper_interval", c.reaperInterval),
		slog.Int("retention_days", c.retentionDays),
		slog.Duration("reaper_stuck_after", c.reaperStuckAfter),
	)
	return nil
}

// Stop aguarda jobs in-flight terminarem ou o ctx expirar (o que ocorrer primeiro).
// Usa select com cron.Stop(ctx).Done() e ctx.Done() para não estourar deadline de shutdown
// (Riscos Conhecidos em techspec — cron.Stop(ctx) espera jobs in-flight).
func (c *Cron) Stop(ctx context.Context) error {
	stopCtx := c.inner.Stop()
	select {
	case <-stopCtx.Done():
		c.logger.InfoContext(ctx, "outbox: cron parado — todos os jobs drenados")
		return nil
	case <-ctx.Done():
		c.logger.WarnContext(ctx, "outbox: cron stop timeout — jobs in-flight podem ter sido abandonados")
		return ctx.Err()
	}
}

// Entries retorna a lista de entries registradas no cron scheduler.
// Útil para testes que verificam o número de jobs registrados.
func (c *Cron) Entries() []robfigcron.Entry {
	return c.inner.Entries()
}

// RunHousekeepingForTest expõe runHousekeeping para testes unitários.
// Permite verificar os argumentos passados a Storage.PurgeOlderThan sem depender do scheduler real.
// Não deve ser usado em produção.
func (c *Cron) RunHousekeepingForTest(ctx context.Context) {
	c.runHousekeeping(ctx)
}

// RunReaperForTest expõe runReaper para testes unitários.
// Permite verificar os argumentos passados a Storage.ReleaseStuck sem depender do scheduler real.
// Não deve ser usado em produção.
func (c *Cron) RunReaperForTest(ctx context.Context) {
	c.runReaper(ctx)
}

// runHousekeeping executa o job de limpeza periódica.
// Chama Storage.PurgeOlderThan com now - retention e registra o counter OTel.
// Payload jamais aparece em chamada slog.* (R-SEC-001).
func (c *Cron) runHousekeeping(ctx context.Context) {
	olderThan := c.clock.Now().Add(-time.Duration(c.retentionDays) * 24 * time.Hour)

	n, err := c.storage.PurgeOlderThan(ctx, olderThan)
	if err != nil {
		c.logger.ErrorContext(ctx, "outbox: housekeeping falhou",
			slog.String("error", err.Error()),
			slog.Int("retention_days", c.retentionDays),
		)
		return
	}

	if c.metrics != nil {
		c.metrics.RecordHousekeepingDeleted(ctx, n)
	}

	c.logger.InfoContext(ctx, "outbox.housekeeping.purged",
		slog.Int64("count", n),
		slog.Int("retention_days", c.retentionDays),
	)
}

// runReaper executa o job de liberação de deliveries travadas.
// Chama Storage.ReleaseStuck com now - stuckAfter e registra o counter OTel.
// Loga apenas se N > 0 (evitar ruído em operação normal).
// Payload jamais aparece em chamada slog.* (R-SEC-001).
func (c *Cron) runReaper(ctx context.Context) {
	olderThan := c.clock.Now().Add(-c.reaperStuckAfter)

	n, err := c.storage.ReleaseStuck(ctx, olderThan)
	if err != nil {
		c.logger.ErrorContext(ctx, "outbox: reaper falhou",
			slog.String("error", err.Error()),
			slog.Duration("stuck_after", c.reaperStuckAfter),
		)
		return
	}

	if c.metrics != nil {
		c.metrics.RecordReaperReleased(ctx, n)
	}

	if n > 0 {
		c.logger.WarnContext(ctx, "outbox.reaper.released",
			slog.Int64("count", n),
			slog.Duration("older_than", c.reaperStuckAfter),
		)
	}
}
