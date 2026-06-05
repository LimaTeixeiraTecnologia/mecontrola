package entities

import (
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type Status string

const (
	StatusActive  Status = "ACTIVE"
	StatusDeleted Status = "DELETED"
)

type User struct {
	id          string
	whatsapp    valueobjects.WhatsAppNumber
	email       valueobjects.Email
	displayName string
	status      Status
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   time.Time
}

type Option func(*User)

func WithEmail(e valueobjects.Email) Option {
	return func(u *User) {
		u.email = e
	}
}

func WithDisplayName(name string) Option {
	return func(u *User) {
		u.displayName = name
	}
}

func New(whatsapp valueobjects.WhatsAppNumber, opts ...Option) User {
	u := User{
		id:        NewID(),
		whatsapp:  whatsapp,
		status:    StatusActive,
		createdAt: time.Now().UTC(),
		updatedAt: time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(&u)
	}
	return u
}

func Hydrate(id, whatsapp, email, displayName, status string, createdAt, updatedAt, deletedAt time.Time) (User, error) {
	wa, err := valueobjects.NewWhatsAppNumber(whatsapp)
	if err != nil {
		return User{}, fmt.Errorf("identity: hydrate whatsapp: %w", err)
	}
	var em valueobjects.Email
	if email != "" {
		em, err = valueobjects.NewEmail(email)
		if err != nil {
			return User{}, fmt.Errorf("identity: hydrate email: %w", err)
		}
	}
	return User{
		id:          id,
		whatsapp:    wa,
		email:       em,
		displayName: displayName,
		status:      Status(status),
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		deletedAt:   deletedAt,
	}, nil
}

func (u User) ID() string                            { return u.id }
func (u User) WhatsApp() valueobjects.WhatsAppNumber { return u.whatsapp }
func (u User) Email() valueobjects.Email             { return u.email }
func (u User) DisplayName() string                   { return u.displayName }
func (u User) Status() Status                        { return u.status }
func (u User) CreatedAt() time.Time                  { return u.createdAt }
func (u User) UpdatedAt() time.Time                  { return u.updatedAt }
func (u User) DeletedAt() time.Time                  { return u.deletedAt }

func (u *User) MarkDeleted(now time.Time) {
	u.status = StatusDeleted
	u.deletedAt = now
	u.updatedAt = now
}

func (u *User) Reanimate(now time.Time) {
	u.status = StatusActive
	u.deletedAt = time.Time{}
	u.email = valueobjects.Email{}
	u.displayName = ""
	u.updatedAt = now
}

func (u User) CanReanimate(now time.Time) bool {
	if u.deletedAt.IsZero() {
		return false
	}
	return now.Sub(u.deletedAt) <= domain.ReanimationWindow
}

func (u *User) SetDisplayNameIfEmpty(name string) {
	if u.displayName == "" {
		u.displayName = name
	}
}

func (u *User) SetEmailIfEmpty(e valueobjects.Email) {
	if u.email.String() == "" {
		u.email = e
	}
}
