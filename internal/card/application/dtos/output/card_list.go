package output

type CardList struct {
	Items      []Card  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}
