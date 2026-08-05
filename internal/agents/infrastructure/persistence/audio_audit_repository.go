package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type audioAuditRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewAudioAuditRepository(db database.DBTX, o11y observability.Observability) usecases.AudioAuditRepository {
	return &audioAuditRepository{db: db, o11y: o11y}
}

func (r *audioAuditRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *audioAuditRepository) InsertProcessing(ctx context.Context, record usecases.AudioAuditRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.audio_audit.insert_processing")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.agents_whatsapp_audio_messages
			(wamid, user_id, peer, media_id, media_sha256, mime_type, size_bytes,
			 outcome, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.conn(ctx).ExecContext(ctx, q,
		record.WAMID,
		record.UserID,
		record.Peer,
		record.MediaID,
		record.MediaSHA256,
		record.MimeType,
		record.SizeBytes,
		record.Outcome.String(),
		record.Reason.String(),
		record.CreatedAt,
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return usecases.ErrAudioAuditAlreadyExists
		}
		return fmt.Errorf("agents.persistence.audio_audit.insert_processing: %w", err)
	}
	return nil
}

func (r *audioAuditRepository) FinalizeTerminal(ctx context.Context, record usecases.AudioAuditRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.audio_audit.finalize_terminal")
	defer span.End()

	const q = `
		UPDATE mecontrola.agents_whatsapp_audio_messages
		   SET mime_type            = $2,
		       size_bytes           = $3,
		       duration_ms          = $4,
		       stt_model            = $5,
		       outcome              = $6,
		       reason               = $7,
		       transcription        = $8,
		       transcription_sha256 = $9,
		       cost_microusd        = $10,
		       error_code           = $11,
		       completed_at         = $12
		 WHERE wamid = $1
		   AND outcome = $13`

	result, err := r.conn(ctx).ExecContext(ctx, q,
		record.WAMID,
		record.MimeType,
		record.SizeBytes,
		record.DurationMs,
		record.STTModel,
		record.Outcome.String(),
		record.Reason.String(),
		record.Transcription,
		record.TranscriptionSHA256,
		record.CostMicroUSD,
		record.ErrorCode,
		record.CompletedAt,
		usecases.AudioOutcomeProcessing.String(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.persistence.audio_audit.finalize_terminal: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("agents.persistence.audio_audit.finalize_terminal.rows_affected: %w", err)
	}
	if rows == 0 {
		return usecases.ErrAudioAuditNotFound
	}
	return nil
}

func (r *audioAuditRepository) FindByWAMID(ctx context.Context, wamid string) (usecases.AudioAuditRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.audio_audit.find_by_wamid")
	defer span.End()

	const q = `
		SELECT wamid, user_id, peer, media_id, media_sha256, mime_type, size_bytes, duration_ms,
		       stt_model, outcome, reason, transcription, transcription_sha256, cost_microusd,
		       error_code, created_at, completed_at
		  FROM mecontrola.agents_whatsapp_audio_messages
		 WHERE wamid = $1
		 LIMIT 1`

	var (
		record     usecases.AudioAuditRecord
		outcomeRaw string
		reasonRaw  string
	)
	err := r.conn(ctx).QueryRowContext(ctx, q, wamid).Scan(
		&record.WAMID,
		&record.UserID,
		&record.Peer,
		&record.MediaID,
		&record.MediaSHA256,
		&record.MimeType,
		&record.SizeBytes,
		&record.DurationMs,
		&record.STTModel,
		&outcomeRaw,
		&reasonRaw,
		&record.Transcription,
		&record.TranscriptionSHA256,
		&record.CostMicroUSD,
		&record.ErrorCode,
		&record.CreatedAt,
		&record.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return usecases.AudioAuditRecord{}, usecases.ErrAudioAuditNotFound
	}
	if err != nil {
		span.RecordError(err)
		return usecases.AudioAuditRecord{}, fmt.Errorf("agents.persistence.audio_audit.find_by_wamid: %w", err)
	}
	record.Outcome = usecases.AudioOutcome(outcomeRaw)
	record.Reason = usecases.AudioReason(reasonRaw)
	return record, nil
}

func (r *audioAuditRepository) UpdateOutcome(
	ctx context.Context,
	wamid string,
	outcome usecases.AudioOutcome,
	reason usecases.AudioReason,
	completedAt time.Time,
) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.audio_audit.update_outcome")
	defer span.End()

	const q = `
		UPDATE mecontrola.agents_whatsapp_audio_messages
		   SET outcome = $2, reason = $3, completed_at = $4
		 WHERE wamid = $1`

	result, err := r.conn(ctx).ExecContext(ctx, q, wamid, outcome.String(), reason.String(), completedAt)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.persistence.audio_audit.update_outcome: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("agents.persistence.audio_audit.update_outcome.rows_affected: %w", err)
	}
	if rows == 0 {
		return usecases.ErrAudioAuditNotFound
	}
	return nil
}
