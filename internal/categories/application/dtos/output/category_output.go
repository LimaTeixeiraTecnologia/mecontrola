package output

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoryOutput struct {
	ID             uuid.UUID  `json:"id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	Kind           string     `json:"kind"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	AllocationType string     `json:"allocation_type"`
	DeprecatedAt   *string    `json:"deprecated_at,omitempty"`
	Version        int64      `json:"version"`
}

type CategoryTreeOutput struct {
	ID             uuid.UUID        `json:"id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Kind           string           `json:"kind"`
	ParentID       *uuid.UUID       `json:"parent_id,omitempty"`
	AllocationType string           `json:"allocation_type"`
	DeprecatedAt   *string          `json:"deprecated_at,omitempty"`
	Subcategories  []CategoryOutput `json:"subcategories,omitempty"`
	Version        int64            `json:"version"`
}

type CategoryDetailOutput struct {
	ID             uuid.UUID        `json:"id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Kind           string           `json:"kind"`
	ParentID       *uuid.UUID       `json:"parent_id,omitempty"`
	AllocationType string           `json:"allocation_type"`
	DeprecatedAt   *string          `json:"deprecated_at,omitempty"`
	Path           string           `json:"path,omitempty"`
	Subcategories  []CategoryOutput `json:"subcategories,omitempty"`
	Version        int64            `json:"version"`
}

type ListCategoriesOutput struct {
	Categories []CategoryTreeOutput `json:"categories"`
	Version    int64                `json:"version"`
}

func NewCategoryOutputFromEntity(c interface{ IsActive() bool }, version int64) CategoryOutput {
	switch e := c.(type) {
	case categoryLike:
		return CategoryOutput{
			ID:             e.GetID(),
			Slug:           e.GetSlug(),
			Name:           e.GetName(),
			Kind:           e.GetKind().String(),
			ParentID:       e.GetParentID(),
			AllocationType: e.GetAllocationType().String(),
			DeprecatedAt:   e.GetDeprecatedAt(),
			Version:        version,
		}
	default:
		return CategoryOutput{}
	}
}

type categoryLike interface {
	IsActive() bool
	GetID() uuid.UUID
	GetSlug() string
	GetName() string
	GetKind() valueobjects.Kind
	GetParentID() *uuid.UUID
	GetAllocationType() valueobjects.AllocationType
	GetDeprecatedAt() *string
}
