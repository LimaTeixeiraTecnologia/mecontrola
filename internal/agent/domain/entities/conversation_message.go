package entities

import "time"

type ConversationMessage struct {
	Role    string
	Content string
	At      time.Time
}
