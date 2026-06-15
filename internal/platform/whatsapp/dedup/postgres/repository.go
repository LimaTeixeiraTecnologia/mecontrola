package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
)

type messageRepository struct {
	o11y observability.Observability
	mgr  manager.Manager
}

func NewMessageRepository(o11y observability.Observability, mgr manager.Manager) dedup.MessageRepository {
	return &messageRepository{o11y: o11y, mgr: mgr}
}

const channelWhatsApp = "whatsapp"

func (r *messageRepository) InsertIfAbsent(ctx context.Context, wamid string) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "whatsapp.dedup.repository.insert_if_absent")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (channel, message_id) DO NOTHING`

	result, err := r.mgr.DBTX(ctx).ExecContext(ctx, q, channelWhatsApp, wamid)
	if err != nil {
		return false, fmt.Errorf("whatsapp.dedup: insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("whatsapp.dedup: rows affected: %w", err)
	}
	return rows > 0, nil
}
