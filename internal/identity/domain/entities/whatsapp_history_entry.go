package entities

import "time"

type WhatsAppHistoryEntry struct {
	id         string
	userID     string
	number     string
	active     bool
	linkedAt   time.Time
	unlinkedAt time.Time
	reason     string
}

func NewWhatsAppHistoryEntry(userID, number, reason string) WhatsAppHistoryEntry {
	return WhatsAppHistoryEntry{
		id:       NewID(),
		userID:   userID,
		number:   number,
		active:   true,
		linkedAt: time.Now().UTC(),
		reason:   reason,
	}
}

func (w WhatsAppHistoryEntry) ID() string            { return w.id }
func (w WhatsAppHistoryEntry) UserID() string        { return w.userID }
func (w WhatsAppHistoryEntry) Number() string        { return w.number }
func (w WhatsAppHistoryEntry) Active() bool          { return w.active }
func (w WhatsAppHistoryEntry) LinkedAt() time.Time   { return w.linkedAt }
func (w WhatsAppHistoryEntry) UnlinkedAt() time.Time { return w.unlinkedAt }
func (w WhatsAppHistoryEntry) Reason() string        { return w.reason }
