package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type messageRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewMessageRepository(db database.DBTX, o11y observability.Observability) memory.MessageStore {
	return &messageRepository{db: db, o11y: o11y}
}

func (r *messageRepository) Append(ctx context.Context, threadPK uuid.UUID, m memory.Message) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.message.append")
	defer span.End()

	parts := m.Parts
	if len(parts) == 0 {
		parts = []byte("[]")
	}

	const q = `
		INSERT INTO mecontrola.platform_messages (id, thread_pk, resource_id, role, content, parts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, q,
		m.ID,
		threadPK,
		m.ResourceID,
		m.Role.String(),
		m.Content,
		parts,
		m.CreatedAt,
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "platform.memory.repository.message.append.failed",
			observability.String("thread_pk", threadPK.String()),
			observability.Error(err),
		)
		return fmt.Errorf("platform.memory.repository.message.append: %w", err)
	}

	return nil
}

func (r *messageRepository) Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]memory.Message, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.message.recent")
	defer span.End()

	const q = `
		SELECT id, thread_pk, resource_id, role, content, parts, created_at
		  FROM mecontrola.platform_messages
		 WHERE thread_pk = $1
		 ORDER BY created_at DESC
		 LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, threadPK, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("platform.memory.repository.message.recent: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var msgs []memory.Message
	for rows.Next() {
		var (
			id         uuid.UUID
			thrPK      uuid.UUID
			resourceID string
			role       string
			content    string
			parts      []byte
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &thrPK, &resourceID, &role, &content, &parts, &createdAt); err != nil {
			return nil, fmt.Errorf("platform.memory.repository.message.recent.scan: %w", err)
		}
		parsedRole, _ := memory.ParseMessageRole(role)
		msgs = append(msgs, memory.Message{
			ID:         id,
			ThreadPK:   thrPK,
			ResourceID: resourceID,
			Role:       parsedRole,
			Content:    content,
			Parts:      parts,
			CreatedAt:  createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform.memory.repository.message.recent.rows: %w", err)
	}

	return msgs, nil
}
