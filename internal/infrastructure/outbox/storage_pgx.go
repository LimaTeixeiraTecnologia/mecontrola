package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	devkitdb "github.com/JailtonJunior94/devkit-go/pkg/database"
	devkitmanager "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// PgxStorage implementa Storage usando pgx/v5 via devkit-go Manager.
// Zero estado mutável fora da conexão: todas as operações recebem ctx e usam
// o pool via manager.DBTX(ctx) para queries fora de transação, ou a tx passada
// pelo caller para inserts dentro de UnitOfWork.
type PgxStorage struct {
	mgr devkitmanager.Manager
}

// NewPgxStorage cria um PgxStorage a partir do Manager interno do database.Manager.
//
// Uso típico:
//
//	storage := outbox.NewPgxStorage(dbManager.Inner())
func NewPgxStorage(mgr devkitmanager.Manager) *PgxStorage {
	return &PgxStorage{mgr: mgr}
}

// InsertEvent insere um registro em outbox_events dentro da transação fornecida.
func (s *PgxStorage) InsertEvent(ctx context.Context, tx devkitdb.DBTX, evt Event) error {
	headersJSON, err := json.Marshal(evt.Headers())
	if err != nil {
		return fmt.Errorf("outbox: InsertEvent: marshal headers: %w", err)
	}

	const query = `
		INSERT INTO outbox_events
			(id, event_type, event_version, aggregate_type, aggregate_id, partition_key, payload, headers, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, execErr := tx.ExecContext(ctx, query,
		evt.ID().String(),
		evt.Type().String(),
		evt.Version(),
		evt.AggregateType(),
		evt.AggregateID(),
		evt.PartitionKey(),
		[]byte(evt.Payload()),
		headersJSON,
		evt.OccurredAt(),
	)
	if execErr != nil {
		return fmt.Errorf("outbox: InsertEvent: %w", execErr)
	}
	return nil
}

// InsertDeliveries insere uma linha em outbox_deliveries por cada SubscriptionName.
// Retorna ErrDuplicateSubscription quando a constraint uq_outbox_deliveries_event_subscription
// for violada (RF-04).
func (s *PgxStorage) InsertDeliveries(ctx context.Context, tx devkitdb.DBTX, evtID events.EventID, subs []SubscriptionName) error {
	if len(subs) == 0 {
		return nil
	}

	const query = `
		INSERT INTO outbox_deliveries (event_id, subscription_name, status, next_retry_at)
		VALUES ($1, $2, 'pending', now())
	`
	for _, sub := range subs {
		if _, err := tx.ExecContext(ctx, query, evtID.String(), sub.String()); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: event_id=%s subscription=%s", ErrDuplicateSubscription, evtID.String(), sub.String())
			}
			return fmt.Errorf("outbox: InsertDeliveries: %w", err)
		}
	}
	return nil
}

// ClaimReady faz claim de até batchSize deliveries pendentes usando FOR UPDATE SKIP LOCKED.
func (s *PgxStorage) ClaimReady(ctx context.Context, batchSize int, instanceID string) ([]Claim, error) {
	dbtx := s.mgr.DBTX(ctx)

	const claimQuery = `
		UPDATE outbox_deliveries d
		   SET status     = 'claimed',
		       claimed_at = now(),
		       claimed_by = $1,
		       attempts   = d.attempts + 1,
		       updated_at = now()
		 WHERE d.id IN (
		       SELECT id FROM outbox_deliveries
		        WHERE status = 'pending'
		          AND next_retry_at <= now()
		        ORDER BY id
		        LIMIT $2
		        FOR UPDATE SKIP LOCKED
		 )
		 RETURNING d.id, d.event_id, d.subscription_name, d.attempts, d.claimed_at
	`
	rows, err := dbtx.QueryContext(ctx, claimQuery, instanceID, batchSize)
	if err != nil {
		return nil, fmt.Errorf("outbox: ClaimReady: update: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type claimRow struct {
		id               ClaimID
		eventID          string
		subscriptionName string
		attempts         uint8
		claimedAt        time.Time
	}

	var claimed []claimRow
	for rows.Next() {
		var cr claimRow
		if scanErr := rows.Scan(&cr.id, &cr.eventID, &cr.subscriptionName, &cr.attempts, &cr.claimedAt); scanErr != nil {
			return nil, fmt.Errorf("outbox: ClaimReady: scan claim row: %w", scanErr)
		}
		claimed = append(claimed, cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox: ClaimReady: rows error: %w", err)
	}

	if len(claimed) == 0 {
		return nil, nil
	}

	// Sort claimed rows by delivery ID to ensure stable ordering (ORDER BY id).
	// UPDATE ... RETURNING does not guarantee any order.
	sort.Slice(claimed, func(i, j int) bool {
		return int64(claimed[i].id) < int64(claimed[j].id)
	})

	// Collect unique event IDs to hydrate in one query.
	eventIDs := uniqueStrings(func() []string {
		out := make([]string, len(claimed))
		for i, cr := range claimed {
			out[i] = cr.eventID
		}
		return out
	}())

	eventMap, err := s.fetchEventsByIDs(ctx, dbtx, eventIDs)
	if err != nil {
		return nil, err
	}

	claims := make([]Claim, 0, len(claimed))
	for _, cr := range claimed {
		evt, ok := eventMap[cr.eventID]
		if !ok {
			return nil, fmt.Errorf("outbox: ClaimReady: event %s not found after claim", cr.eventID)
		}
		subName, subErr := NewSubscriptionName(cr.subscriptionName)
		if subErr != nil {
			return nil, fmt.Errorf("outbox: ClaimReady: invalid subscription name %q: %w", cr.subscriptionName, subErr)
		}
		claims = append(claims, Claim{
			ID:               cr.id,
			Event:            evt,
			SubscriptionName: subName,
			Attempt:          NewAttempt(cr.attempts),
			ClaimedAt:        cr.claimedAt,
		})
	}
	return claims, nil
}

// MarkProcessed transita a delivery para processed.
func (s *PgxStorage) MarkProcessed(ctx context.Context, id ClaimID, processedAt time.Time) error {
	dbtx := s.mgr.DBTX(ctx)

	const query = `
		UPDATE outbox_deliveries
		   SET status       = 'processed',
		       processed_at = $1,
		       updated_at   = now()
		 WHERE id = $2
		   AND status = 'claimed'
	`
	result, err := dbtx.ExecContext(ctx, query, processedAt, int64(id))
	if err != nil {
		return fmt.Errorf("outbox: MarkProcessed: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("outbox: MarkProcessed: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("outbox: MarkProcessed: delivery %d not found or invalid transition", id)
	}
	return nil
}

// MarkFailed registra falha transitória atualizando attempts, lastErr e nextRetryAt.
func (s *PgxStorage) MarkFailed(ctx context.Context, id ClaimID, lastErr string, nextAttempt Attempt, nextRetryAt time.Time) error {
	dbtx := s.mgr.DBTX(ctx)

	const query = `
		UPDATE outbox_deliveries
		   SET status        = 'pending',
		       attempts      = $1,
		       last_error    = $2,
		       next_retry_at = $3,
		       claimed_at    = NULL,
		       claimed_by    = NULL,
		       updated_at    = now()
		 WHERE id = $4
		   AND status = 'claimed'
	`
	result, err := dbtx.ExecContext(ctx, query, nextAttempt.Value(), lastErr, nextRetryAt, int64(id))
	if err != nil {
		return fmt.Errorf("outbox: MarkFailed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("outbox: MarkFailed: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("outbox: MarkFailed: delivery %d not found or invalid transition", id)
	}
	return nil
}

// MarkDLQ transita a delivery para dead_letter.
func (s *PgxStorage) MarkDLQ(ctx context.Context, id ClaimID, lastErr string, deadLetterAt time.Time) error {
	dbtx := s.mgr.DBTX(ctx)

	const query = `
		UPDATE outbox_deliveries
		   SET status        = 'dead_letter',
		       last_error    = $1,
		       dead_letter_at = $2,
		       updated_at    = now()
		 WHERE id = $3
		   AND status = 'claimed'
	`
	result, err := dbtx.ExecContext(ctx, query, lastErr, deadLetterAt, int64(id))
	if err != nil {
		return fmt.Errorf("outbox: MarkDLQ: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("outbox: MarkDLQ: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("outbox: MarkDLQ: delivery %d not found or invalid transition", id)
	}
	return nil
}

// ReleaseStuck libera deliveries claimed há mais de olderThan, revertendo para pending (D-17).
func (s *PgxStorage) ReleaseStuck(ctx context.Context, olderThan time.Time) (int64, error) {
	dbtx := s.mgr.DBTX(ctx)

	const query = `
		UPDATE outbox_deliveries
		   SET status     = 'pending',
		       claimed_by = NULL,
		       claimed_at = NULL,
		       updated_at = now()
		 WHERE id IN (
		       SELECT id FROM outbox_deliveries
		        WHERE status = 'claimed'
		          AND claimed_at < $1
		        ORDER BY id
		        FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id
	`
	rows, err := dbtx.QueryContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("outbox: ReleaseStuck: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var count int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return 0, fmt.Errorf("outbox: ReleaseStuck: scan: %w", scanErr)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("outbox: ReleaseStuck: rows error: %w", err)
	}
	return count, nil
}

// PurgeOlderThan apaga deliveries finalizadas e eventos órfãos anteriores a olderThan.
func (s *PgxStorage) PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	dbtx := s.mgr.DBTX(ctx)

	const delDeliveries = `
		DELETE FROM outbox_deliveries
		 WHERE status IN ('processed','dead_letter')
		   AND COALESCE(processed_at, dead_letter_at) < $1
	`
	r1, err := dbtx.ExecContext(ctx, delDeliveries, olderThan)
	if err != nil {
		return 0, fmt.Errorf("outbox: PurgeOlderThan: delete deliveries: %w", err)
	}
	n1, err := r1.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: PurgeOlderThan: rows affected deliveries: %w", err)
	}

	const delOrphans = `
		DELETE FROM outbox_events e
		 WHERE NOT EXISTS (SELECT 1 FROM outbox_deliveries d WHERE d.event_id = e.id)
		   AND e.created_at < $1
	`
	r2, err := dbtx.ExecContext(ctx, delOrphans, olderThan)
	if err != nil {
		return 0, fmt.Errorf("outbox: PurgeOlderThan: delete orphan events: %w", err)
	}
	n2, err := r2.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: PurgeOlderThan: rows affected events: %w", err)
	}

	return n1 + n2, nil
}

// Stats retorna contagens de deliveries por status e subscription para gauges OTel.
func (s *PgxStorage) Stats(ctx context.Context) (Stats, error) {
	dbtx := s.mgr.DBTX(ctx)

	const query = `
		SELECT subscription_name, status, COUNT(*)
		  FROM outbox_deliveries
		 WHERE status IN ('pending','dead_letter')
		 GROUP BY subscription_name, status
	`
	rows, err := dbtx.QueryContext(ctx, query)
	if err != nil {
		return Stats{}, fmt.Errorf("outbox: Stats: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	st := Stats{
		Pending:    make(map[SubscriptionName]int64),
		DeadLetter: make(map[SubscriptionName]int64),
	}

	for rows.Next() {
		var subStr, statusStr string
		var count int64
		if scanErr := rows.Scan(&subStr, &statusStr, &count); scanErr != nil {
			return Stats{}, fmt.Errorf("outbox: Stats: scan: %w", scanErr)
		}
		sub, subErr := NewSubscriptionName(subStr)
		if subErr != nil {
			continue // skip invalid names silently — telemetry best-effort
		}
		switch statusStr {
		case StatusPending.String():
			st.Pending[sub] = count
		case StatusDeadLetter.String():
			st.DeadLetter[sub] = count
		}
	}
	if err := rows.Err(); err != nil {
		return Stats{}, fmt.Errorf("outbox: Stats: rows error: %w", err)
	}

	// Oldest pending at.
	const oldestQuery = `
		SELECT MIN(oe.occurred_at)
		  FROM outbox_deliveries od
		  JOIN outbox_events oe ON oe.id = od.event_id
		 WHERE od.status = 'pending'
	`
	row := dbtx.QueryRowContext(ctx, oldestQuery)
	var oldestPtr *time.Time
	if scanErr := row.Scan(&oldestPtr); scanErr != nil {
		return Stats{}, fmt.Errorf("outbox: Stats: oldest pending query: %w", scanErr)
	}
	if oldestPtr != nil {
		st.OldestPendingAt = *oldestPtr
	}

	return st, nil
}

// fetchEventsByIDs executa um SELECT em outbox_events para os IDs fornecidos e
// retorna um mapa eventID → Event.
func (s *PgxStorage) fetchEventsByIDs(ctx context.Context, dbtx devkitdb.DBTX, ids []string) (map[string]Event, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, event_type, event_version, aggregate_type, aggregate_id,
		       partition_key, payload, headers, occurred_at
		  FROM outbox_events
		 WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := dbtx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("outbox: fetchEventsByIDs: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]Event, len(ids))
	for rows.Next() {
		var (
			idStr         string
			eventTypeStr  string
			version       uint16
			aggregateType string
			aggregateID   string
			partitionKey  *string
			payloadRaw    []byte
			headersRaw    []byte
			occurredAt    time.Time
		)
		if scanErr := rows.Scan(
			&idStr, &eventTypeStr, &version, &aggregateType, &aggregateID,
			&partitionKey, &payloadRaw, &headersRaw, &occurredAt,
		); scanErr != nil {
			return nil, fmt.Errorf("outbox: fetchEventsByIDs: scan: %w", scanErr)
		}

		var hdrs Headers
		if unmarshalErr := json.Unmarshal(headersRaw, &hdrs); unmarshalErr != nil {
			hdrs = make(Headers)
		}

		eventID, parseErr := events.NewEventID(idStr)
		if parseErr != nil {
			return nil, fmt.Errorf("outbox: fetchEventsByIDs: parse event id %q: %w", idStr, parseErr)
		}

		eventName, nameErr := events.NewEventName(eventTypeStr)
		if nameErr != nil {
			return nil, fmt.Errorf("outbox: fetchEventsByIDs: parse event name %q: %w", eventTypeStr, nameErr)
		}

		evt, evtErr := NewEvent(NewEventParams{
			ID:            eventID,
			EventType:     eventName,
			Version:       version,
			AggregateType: aggregateType,
			AggregateID:   aggregateID,
			PartitionKey:  partitionKey,
			Payload:       json.RawMessage(payloadRaw),
			Headers:       hdrs,
			OccurredAt:    occurredAt,
		})
		if evtErr != nil {
			return nil, fmt.Errorf("outbox: fetchEventsByIDs: reconstruct event: %w", evtErr)
		}
		result[idStr] = evt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox: fetchEventsByIDs: rows error: %w", err)
	}
	return result, nil
}

// isUniqueViolation retorna true quando o erro é uma violação de constraint UNIQUE no PostgreSQL.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// uniqueStrings retorna uma slice sem duplicatas preservando a ordem.
func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
