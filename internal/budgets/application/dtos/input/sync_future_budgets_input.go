package input

import "errors"

var ErrInputInvalidSourceCompetence = errors.New("source_competence: inválida")

type SyncFutureBudgetsInput struct {
	UserID           string `json:"user_id"`
	SourceCompetence string `json:"source_competence"`
}

func (i SyncFutureBudgetsInput) Validate() error {
	if i.UserID == "" {
		return ErrInputInvalidUserID
	}
	if i.SourceCompetence == "" {
		return ErrInputInvalidSourceCompetence
	}
	return nil
}
