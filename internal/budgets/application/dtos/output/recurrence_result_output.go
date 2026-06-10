package output

type RecurrenceStatus string

const (
	RecurrenceStatusCreated            RecurrenceStatus = "created"
	RecurrenceStatusUpdated            RecurrenceStatus = "updated"
	RecurrenceStatusCompletedFromDraft RecurrenceStatus = "completed_from_draft"
	RecurrenceStatusConflict           RecurrenceStatus = "conflict"
	RecurrenceStatusFailure            RecurrenceStatus = "failure"
)

type RecurrenceResultEntry struct {
	Competence string
	Status     RecurrenceStatus
	Reason     string
}

type RecurrenceResultOutput struct {
	SourceCompetence string
	Results          []RecurrenceResultEntry
}
