package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

type userRow struct {
	ID             string
	WhatsAppNumber string
	DisplayName    sql.NullString
	Email          sql.NullString
	IsAdmin        bool
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      sql.NullTime
}

type rowMapper struct{}

func (rowMapper) HydrateUser(row userRow) (*entities.User, error) {
	userID, err := entities.NewUserID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("postgres user mapper: id corrompido: %w", err)
	}

	number, err := valueobjects.NewWhatsAppNumber(row.WhatsAppNumber)
	if err != nil {
		return nil, fmt.Errorf("postgres user mapper: whatsapp corrompido: %w", err)
	}

	var emailPtr *valueobjects.Email
	if row.Email.Valid && row.Email.String != "" {
		email, err := valueobjects.NewEmail(row.Email.String)
		if err != nil {
			return nil, fmt.Errorf("postgres user mapper: email corrompido: %w", err)
		}
		emailPtr = &email
	}

	status, _ := valueobjects.ParseUserStatus(row.Status)

	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		t := row.DeletedAt.Time
		deletedAt = &t
	}

	return entities.RehydrateUser(entities.RehydrateUserParams{
		ID:          userID,
		Number:      number,
		DisplayName: row.DisplayName.String,
		Email:       emailPtr,
		IsAdmin:     row.IsAdmin,
		Status:      status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   deletedAt,
	}), nil
}
