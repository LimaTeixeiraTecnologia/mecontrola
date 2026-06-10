package output

type Invoice struct {
	ClosingDate string `json:"closing_date"`
	DueDate     string `json:"due_date"`
}
