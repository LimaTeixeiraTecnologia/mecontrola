package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

var (
	ErrInvalidUserID          = errors.New("identity: user id deve ser uuid v4")
	ErrUserRequiresNumber     = errors.New("identity: user requer whatsapp number válido")
	ErrUserRequiresTimestamps = errors.New("identity: user requer created_at e updated_at")
	ErrUserAlreadyDeleted     = errors.New("identity: user já está soft-deleted")
)

// UserID encapsula um UUID v4 imutável que identifica o agregado User.
type UserID struct{ value string }

// NewUserID valida que v é um UUID v4 canônico e retorna um UserID.
// UUID de outras versões (v1, v3, v5) são rejeitados.
func NewUserID(v string) (UserID, error) {
	parsed, err := uuid.Parse(v)
	if err != nil {
		return UserID{}, ErrInvalidUserID
	}
	if parsed.Version() != 4 {
		return UserID{}, ErrInvalidUserID
	}
	return UserID{value: parsed.String()}, nil
}

func (u UserID) String() string { return u.value }

// User é o agregado raiz do módulo identity.
// Campos não exportados — mutações apenas via métodos com intenção de negócio.
type User struct {
	id          UserID
	number      valueobjects.WhatsAppNumber
	displayName string
	email       *valueobjects.Email
	isAdmin     bool
	status      valueobjects.UserStatus
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   *time.Time
}

// NewUserParams agrupa os campos obrigatórios para construção de um novo User.
type NewUserParams struct {
	ID          UserID
	Number      valueobjects.WhatsAppNumber
	DisplayName string
	Email       *valueobjects.Email
	IsAdmin     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewUser constrói um User validando invariantes: Number não pode ser zero e
// timestamps não podem ser zero.
func NewUser(p NewUserParams) (*User, error) {
	if p.Number.IsZero() {
		return nil, ErrUserRequiresNumber
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return nil, ErrUserRequiresTimestamps
	}
	return &User{
		id:          p.ID,
		number:      p.Number,
		displayName: p.DisplayName,
		email:       p.Email,
		isAdmin:     p.IsAdmin,
		status:      valueobjects.UserStatusActive,
		createdAt:   p.CreatedAt,
		updatedAt:   p.UpdatedAt,
	}, nil
}

func (u *User) ID() UserID                                  { return u.id }
func (u *User) WhatsAppNumber() valueobjects.WhatsAppNumber { return u.number }
func (u *User) Email() *valueobjects.Email                  { return u.email }
func (u *User) IsAdmin() bool                               { return u.isAdmin }
func (u *User) Status() valueobjects.UserStatus             { return u.status }
func (u *User) DeletedAt() *time.Time                       { return u.deletedAt }
func (u *User) IsDeleted() bool                             { return u.deletedAt != nil }

// MarkAsAdmin promove o usuário a administrador e registra o instante da alteração.
func (u *User) MarkAsAdmin(at time.Time) {
	u.isAdmin = true
	u.updatedAt = at
}

// RevokeAdmin remove a permissão de administrador e registra o instante da alteração.
func (u *User) RevokeAdmin(at time.Time) {
	u.isAdmin = false
	u.updatedAt = at
}

// UpdateEmail substitui o e-mail associado ao usuário e registra o instante da alteração.
func (u *User) UpdateEmail(e valueobjects.Email, at time.Time) {
	u.email = &e
	u.updatedAt = at
}

// SoftDelete marca o usuário como deletado, transita o status para DELETED e registra
// o instante da deleção. Retorna ErrUserAlreadyDeleted sem mutação se já deletado.
func (u *User) SoftDelete(at time.Time) error {
	if u.deletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.deletedAt = &at
	u.status = valueobjects.UserStatusDeleted
	u.updatedAt = at
	return nil
}

// RehydrateUserParams agrupa todos os campos necessários para reconstruir um User
// a partir de uma row Postgres. Uso restrito ao mapper de infrastructure.
type RehydrateUserParams struct {
	ID          UserID
	Number      valueobjects.WhatsAppNumber
	DisplayName string
	Email       *valueobjects.Email
	IsAdmin     bool
	Status      valueobjects.UserStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// RehydrateUser é o construtor exclusivo de reidratação (mapper de infrastructure).
// Diferente de NewUser, aceita Status e DeletedAt arbitrários — esses já foram
// validados pelo banco via CK constraint e índice parcial. Não publicar para application.
//
// Uso restrito ao mapper de infrastructure.
func RehydrateUser(p RehydrateUserParams) *User {
	return &User{
		id:          p.ID,
		number:      p.Number,
		displayName: p.DisplayName,
		email:       p.Email,
		isAdmin:     p.IsAdmin,
		status:      p.Status,
		createdAt:   p.CreatedAt,
		updatedAt:   p.UpdatedAt,
		deletedAt:   p.DeletedAt,
	}
}
