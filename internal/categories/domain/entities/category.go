package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type Category struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Kind           valueobjects.Kind
	ParentID       *uuid.UUID
	AllocationType valueobjects.AllocationType
	DeprecatedAt   *time.Time
}

func (c Category) IsRoot() bool {
	return c.ParentID == nil
}

func (c Category) IsActive() bool {
	return c.DeprecatedAt == nil
}
