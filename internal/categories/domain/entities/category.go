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

func (c Category) GetID() uuid.UUID {
	return c.ID
}

func (c Category) GetSlug() string {
	return c.Slug
}

func (c Category) GetName() string {
	return c.Name
}

func (c Category) GetKind() valueobjects.Kind {
	return c.Kind
}

func (c Category) GetParentID() *uuid.UUID {
	return c.ParentID
}

func (c Category) GetAllocationType() valueobjects.AllocationType {
	return c.AllocationType
}

func (c Category) GetDeprecatedAt() *string {
	if c.DeprecatedAt == nil {
		return nil
	}
	ts := c.DeprecatedAt.Format("2006-01-02T15:04:05Z")
	return &ts
}
