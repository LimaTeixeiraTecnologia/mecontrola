package input

import "github.com/google/uuid"

type GetCategoryInput struct {
	ID                uuid.UUID
	IncludeDeprecated bool
}

func (i *GetCategoryInput) Validate() error {
	if i.ID == uuid.Nil {
		return ErrCategoryIDRequired
	}
	return nil
}
