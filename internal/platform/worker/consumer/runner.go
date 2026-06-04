package consumer

import (
	"context"
	"log/slog"
)

type runnerImpl struct {
	source   Source
	registry Registry
	logger   *slog.Logger
}

func NewRunner(source Source, registry Registry, logger *slog.Logger) Runner {
	return &runnerImpl{
		source:   source,
		registry: registry,
		logger:   logger,
	}
}

func (r *runnerImpl) Start(ctx context.Context) error {
	return r.source.Start(ctx, func(msgCtx context.Context, msg Message) error {
		if err := r.registry.Dispatch(msgCtx, msg.EventType, msg.Params, msg.Body); err != nil {
			r.logger.ErrorContext(msgCtx, "dispatch error", "event_type", msg.EventType, "error", err)
			return err
		}
		return nil
	})
}

func (r *runnerImpl) Stop(ctx context.Context) error {
	return r.source.Stop(ctx)
}
