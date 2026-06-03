package outbox

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// Storage é a porta de domínio do Outbox (D-14): define o contrato SQL sem expor
// detalhes de implementação (pgx, SQL, etc.) ao resto do pacote.
// A única implementação concreta no MVP é PgxStorage (storage_pgx.go).
//
// Regra de import: apenas storage_pgx.go pode importar pgx. Os demais arquivos
// dependem unicamente desta interface.
type Storage interface {
	// InsertEvent insere um registro em outbox_events dentro da transação fornecida.
	// O caller (Publisher) é responsável por Commit/Rollback — InsertEvent não abre tx.
	InsertEvent(ctx context.Context, tx database.DBTX, evt Event) error

	// InsertDeliveries insere uma linha em outbox_deliveries por cada SubscriptionName
	// fornecida, dentro da transação do caller.
	// Retorna ErrDuplicateSubscription (wrappado) se a constraint
	// uq_outbox_deliveries_event_subscription for violada.
	InsertDeliveries(ctx context.Context, tx database.DBTX, evtID events.EventID, subs []SubscriptionName) error

	// ClaimReady faz claim de até batchSize deliveries pendentes com next_retry_at <= now(),
	// ordenadas por id, usando FOR UPDATE SKIP LOCKED para coordenação multi-instância (RF-14).
	// Retorna os Claims preenchidos com o Event hidratado via SELECT em outbox_events.
	ClaimReady(ctx context.Context, batchSize int, instanceID string) ([]Claim, error)

	// MarkProcessed transita a delivery para StatusProcessed e registra processedAt.
	// Retorna erro se a transição for inválida (DeliveryStatus.CanTransitionTo).
	MarkProcessed(ctx context.Context, id ClaimID, processedAt time.Time) error

	// MarkFailed registra falha transitória: incrementa attempts, define lastErr e
	// calcula nextRetryAt para o próximo retry.
	MarkFailed(ctx context.Context, id ClaimID, lastErr string, nextAttempt Attempt, nextRetryAt time.Time) error

	// MarkDLQ transita a delivery para StatusDeadLetter registrando lastErr e deadLetterAt.
	MarkDLQ(ctx context.Context, id ClaimID, lastErr string, deadLetterAt time.Time) error

	// ReleaseStuck libera deliveries em StatusClaimed com claimed_at < olderThan,
	// revertendo-as para StatusPending (D-17, evita race com Dispatcher via SKIP LOCKED).
	// Retorna o número de linhas liberadas.
	ReleaseStuck(ctx context.Context, olderThan time.Time) (int64, error)

	// PurgeOlderThan apaga deliveries finalizadas (processed/dead_letter) com idade
	// superior a olderThan e, em segundo passo, remove eventos órfãos.
	// Retorna o total de linhas apagadas (deliveries + eventos).
	PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error)

	// Stats retorna contagens de deliveries por status e subscription para uso em gauges OTel.
	Stats(ctx context.Context) (Stats, error)
}
