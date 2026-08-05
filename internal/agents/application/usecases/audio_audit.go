package usecases

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrAudioAuditAlreadyExists = errors.New("agents.persistence: audio audit already exists for wamid")

var ErrAudioAuditNotFound = errors.New("agents.persistence: audio audit not found for wamid")

type AudioAuditRecord struct {
	WAMID               string
	UserID              uuid.UUID
	Peer                string
	MediaID             string
	MediaSHA256         string
	MimeType            string
	SizeBytes           *int64
	DurationMs          *int
	STTModel            *string
	Outcome             AudioOutcome
	Reason              AudioReason
	Transcription       *string
	TranscriptionSHA256 *string
	CostMicroUSD        *int64
	ErrorCode           *string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

type AudioAuditRepository interface {
	InsertProcessing(ctx context.Context, record AudioAuditRecord) error
	FinalizeTerminal(ctx context.Context, record AudioAuditRecord) error
	FindByWAMID(ctx context.Context, wamid string) (AudioAuditRecord, error)
	UpdateOutcome(ctx context.Context, wamid string, outcome AudioOutcome, reason AudioReason, completedAt time.Time) error
}
