package status

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const statusFailed = "failed"

type StatusRecord struct {
	MessageID   string
	Status      string
	RecipientID string
	ErrorCode   string
	ErrorTitle  string
	StatusAt    time.Time
}

type MessageStatusRepository interface {
	Record(ctx context.Context, record StatusRecord) (stored bool, err error)
}

type RecordMessageStatus struct {
	repo        MessageStatusRepository
	o11y        observability.Observability
	statusTotal observability.Counter
}

func NewRecordMessageStatus(
	repo MessageStatusRepository,
	o11y observability.Observability,
) *RecordMessageStatus {
	statusTotal := o11y.Metrics().Counter(
		"whatsapp_message_status_total",
		"Total de callbacks de status de mensagens WhatsApp persistidos por status",
		"1",
	)
	return &RecordMessageStatus{
		repo:        repo,
		o11y:        o11y,
		statusTotal: statusTotal,
	}
}

func (u *RecordMessageStatus) Execute(ctx context.Context, statuses []MessageStatus) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "whatsapp.status.usecase.record_message_status")
	defer span.End()

	for _, st := range statuses {
		record := StatusRecord{
			MessageID:   st.MessageID,
			Status:      st.Status,
			RecipientID: st.RecipientID,
			ErrorCode:   st.ErrorCode,
			ErrorTitle:  st.ErrorTitle,
			StatusAt:    parseStatusAt(st.Timestamp),
		}

		stored, err := u.repo.Record(ctx, record)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "whatsapp.status.record_failed",
				observability.Error(err),
				observability.String("status", st.Status),
			)
			return fmt.Errorf("whatsapp.status.usecase.record_message_status record: %w", err)
		}

		if !stored {
			continue
		}

		u.statusTotal.Increment(ctx, observability.String("status", st.Status))

		if st.Status == statusFailed {
			u.o11y.Logger().Warn(ctx, "whatsapp.status.message_failed",
				observability.String("status", st.Status),
				observability.String("error_code", st.ErrorCode),
				observability.String("error_title", st.ErrorTitle),
			)
		}
	}

	return nil
}

func parseStatusAt(raw string) time.Time {
	ts, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Unix(ts, 0).UTC()
}
