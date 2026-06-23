package binding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type ThreadGatewayAdapter struct {
	factory appinterfaces.AgentThreadRepositoryFactory
	unit    uow.UnitOfWork
}

func NewThreadGatewayAdapter(factory appinterfaces.AgentThreadRepositoryFactory, unit uow.UnitOfWork) *ThreadGatewayAdapter {
	return &ThreadGatewayAdapter{factory: factory, unit: unit}
}

func (a *ThreadGatewayAdapter) GetOrCreate(ctx context.Context, userID uuid.UUID, channel string) (entities.Thread, error) {
	var resolved entities.Thread
	op := func(ctx context.Context, db database.DBTX) error {
		candidate, err := entities.NewThread(userID, channel)
		if err != nil {
			return err
		}
		persisted, err := a.factory.AgentThreadRepository(db).Upsert(ctx, candidate)
		if err != nil {
			return err
		}
		resolved = persisted
		return nil
	}
	if err := a.unit.Do(ctx, op); err != nil {
		return entities.Thread{}, fmt.Errorf("agent: thread get_or_create: %w", err)
	}
	return resolved, nil
}

type RunGatewayAdapter struct {
	factory appinterfaces.AgentRunRepositoryFactory
	unit    uow.UnitOfWork
}

func NewRunGatewayAdapter(factory appinterfaces.AgentRunRepositoryFactory, unit uow.UnitOfWork) *RunGatewayAdapter {
	return &RunGatewayAdapter{factory: factory, unit: unit}
}

func (a *RunGatewayAdapter) Insert(ctx context.Context, run entities.Run) error {
	op := func(ctx context.Context, db database.DBTX) error {
		return a.factory.AgentRunRepository(db).Insert(ctx, run)
	}
	if err := a.unit.Do(ctx, op); err != nil {
		return fmt.Errorf("agent: run insert: %w", err)
	}
	return nil
}

func (a *RunGatewayAdapter) Finish(ctx context.Context, run entities.Run) error {
	op := func(ctx context.Context, db database.DBTX) error {
		return a.factory.AgentRunRepository(db).UpdateOnFinish(ctx, run)
	}
	if err := a.unit.Do(ctx, op); err != nil {
		return fmt.Errorf("agent: run finish: %w", err)
	}
	return nil
}
