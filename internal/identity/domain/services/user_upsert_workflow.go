package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UpsertAction interface {
	isUpsertAction()
}

type UpsertCreateNew struct {
	Candidate entities.User
}

type UpsertUpdateExisting struct {
	Existing entities.User
}

type UpsertReanimate struct {
	Deleted entities.User
}

func (UpsertCreateNew) isUpsertAction()      {}
func (UpsertUpdateExisting) isUpsertAction() {}
func (UpsertReanimate) isUpsertAction()      {}

type UserUpsertWorkflow struct{}

func (UserUpsertWorkflow) DecideUpsertAction(
	activeFound *entities.User,
	deletedFound *entities.User,
	whatsapp valueobjects.WhatsAppNumber,
	email valueobjects.Email,
	displayName string,
	now time.Time,
) UpsertAction {
	if activeFound != nil {
		existing := *activeFound
		existing.SetDisplayNameIfEmpty(displayName)
		existing.SetEmailIfEmpty(email)
		return UpsertUpdateExisting{Existing: existing}
	}

	if deletedFound != nil && deletedFound.CanReanimate(now) {
		deleted := *deletedFound
		deleted.Reanimate(now)
		deleted.SetDisplayNameIfEmpty(displayName)
		deleted.SetEmailIfEmpty(email)
		return UpsertReanimate{Deleted: deleted}
	}

	candidate := entities.New(whatsapp,
		entities.WithEmail(email),
		entities.WithDisplayName(displayName),
	)
	return UpsertCreateNew{Candidate: candidate}
}
