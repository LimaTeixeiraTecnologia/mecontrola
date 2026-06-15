package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dedup"
)

type updateRepository struct {
	o11y observability.Observability
	mgr  manager.Manager
}

func NewUpdateRepository(o11y observability.Observability, mgr manager.Manager) dedup.UpdateRepository {
	return &updateRepository{o11y: o11y, mgr: mgr}
}

const channelTelegram = "telegram"

func (r *updateRepository) InsertIfAbsent(ctx context.Context, botID, updateID int64) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "telegram.dedup.repository.insert_if_absent")
	defer span.End()

	messageID := strconv.FormatInt(botID, 10) + ":" + strconv.FormatInt(updateID, 10)

	const q = `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (channel, message_id) DO NOTHING`

	result, err := r.mgr.DBTX(ctx).ExecContext(ctx, q, channelTelegram, messageID)
	if err != nil {
		return false, fmt.Errorf("telegram.dedup: insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("telegram.dedup: rows affected: %w", err)
	}
	return rows > 0, nil
}
