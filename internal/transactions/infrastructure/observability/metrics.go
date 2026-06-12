package observability

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type TransactionsMetrics struct {
	TransactionCreatedTotal                observability.Counter
	TransactionUpdatedTotal                observability.Counter
	TransactionDeletedTotal                observability.Counter
	CardPurchaseCreatedTotal               observability.Counter
	CardPurchaseUpdatedTotal               observability.Counter
	CardPurchaseDeletedTotal               observability.Counter
	RecurringTemplateCreated               observability.Counter
	RecurringMaterializeAttemptTotal       observability.Counter
	RecurringMaterializeSkippedTotal       observability.Counter
	RecurringMaterializeDurationSeconds    observability.Histogram
	WriteDurationSeconds                   observability.Histogram
	ReadDurationSeconds                    observability.Histogram
	MonthlySummaryRecomputeDurationSeconds observability.Histogram
	MonthlySummaryCoalesceFactor           observability.Histogram
	MonthlySummaryDriftTotal               observability.Counter
	OutboxConsumerLagSeconds               observability.Histogram
	OutboxDeadLetterTotal                  observability.Counter
	IdempotencyReplayTotal                 observability.Counter
	CardLookupFailureTotal                 observability.Counter
}

func NewTransactionsMetrics(m observability.Metrics) *TransactionsMetrics {
	return &TransactionsMetrics{
		TransactionCreatedTotal: m.Counter(
			"transactions_transactions_created_total",
			"Total de transacoes criadas",
			"1",
		),
		TransactionUpdatedTotal: m.Counter(
			"transactions_transactions_updated_total",
			"Total de transacoes atualizadas",
			"1",
		),
		TransactionDeletedTotal: m.Counter(
			"transactions_transactions_deleted_total",
			"Total de transacoes removidas",
			"1",
		),
		CardPurchaseCreatedTotal: m.Counter(
			"transactions_card_purchases_created_total",
			"Total de compras de cartao criadas",
			"1",
		),
		CardPurchaseUpdatedTotal: m.Counter(
			"transactions_card_purchases_updated_total",
			"Total de compras de cartao atualizadas",
			"1",
		),
		CardPurchaseDeletedTotal: m.Counter(
			"transactions_card_purchases_deleted_total",
			"Total de compras de cartao removidas",
			"1",
		),
		RecurringTemplateCreated: m.Counter(
			"transactions_recurring_template_created_total",
			"Total de templates recorrentes criados",
			"1",
		),
		RecurringMaterializeAttemptTotal: m.Counter(
			"transactions_recurring_materialize_attempt_total",
			"Total de tentativas de materializacao de recorrencias",
			"1",
		),
		RecurringMaterializeSkippedTotal: m.Counter(
			"transactions_recurring_materialize_skipped_total",
			"Total de materializacoes de recorrencias ignoradas",
			"1",
		),
		RecurringMaterializeDurationSeconds: m.HistogramWithBuckets(
			"transactions_recurring_materialize_duration_seconds",
			"Duracao da materializacao de recorrencias",
			"s",
			[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		),
		WriteDurationSeconds: m.HistogramWithBuckets(
			"transactions_write_duration_seconds",
			"Duracao das operacoes de escrita",
			"s",
			[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		),
		ReadDurationSeconds: m.HistogramWithBuckets(
			"transactions_read_duration_seconds",
			"Duracao das operacoes de leitura",
			"s",
			[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		),
		MonthlySummaryRecomputeDurationSeconds: m.HistogramWithBuckets(
			"transactions_monthly_summary_recompute_duration_seconds",
			"Duracao do recompute do resumo mensal",
			"s",
			[]float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		),
		MonthlySummaryCoalesceFactor: m.HistogramWithBuckets(
			"transactions_monthly_summary_coalesce_factor",
			"Fator de coalescing do resumo mensal",
			"1",
			[]float64{1, 2, 5, 10, 20, 50},
		),
		MonthlySummaryDriftTotal: m.Counter(
			"transactions_monthly_summary_drift_total",
			"Total de desvios detectados no resumo mensal",
			"1",
		),
		OutboxConsumerLagSeconds: m.HistogramWithBuckets(
			"transactions_outbox_consumer_lag_seconds",
			"Lag do consumer do outbox em segundos",
			"s",
			[]float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		),
		OutboxDeadLetterTotal: m.Counter(
			"transactions_outbox_dead_letter_total",
			"Total de eventos enviados para dead-letter",
			"1",
		),
		IdempotencyReplayTotal: m.Counter(
			"transactions_idempotency_replay_total",
			"Total de replays de idempotencia",
			"1",
		),
		CardLookupFailureTotal: m.Counter(
			"transactions_card_lookup_failure_total",
			"Total de falhas na busca de cartao",
			"1",
		),
	}
}
