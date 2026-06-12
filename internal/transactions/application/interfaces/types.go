package interfaces

import "github.com/google/uuid"

type Cursor struct {
	Value string
}

type CategorySnapshot struct {
	ID         uuid.UUID
	Name       string
	ParentID   *uuid.UUID
	ParentName string
}

type MonthlySummaryKey struct {
	UserID   uuid.UUID
	RefMonth string
}
