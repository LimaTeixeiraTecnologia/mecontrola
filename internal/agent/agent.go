package agent

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

type AgentHandler interface {
	HandleMessage(ctx context.Context, msg payload.Message) error
}
