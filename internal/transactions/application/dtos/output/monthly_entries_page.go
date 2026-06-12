package output

type MonthlyEntriesPage struct {
	Items      []any  `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
