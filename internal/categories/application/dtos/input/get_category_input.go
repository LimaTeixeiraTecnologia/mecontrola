package input

import "github.com/google/uuid"

type GetCategoryInput struct {
	ID                uuid.UUID
	IncludeDeprecated bool
}
