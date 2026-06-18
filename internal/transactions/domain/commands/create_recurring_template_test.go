package commands_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func validCreateRecurringTemplateRaw() commands.RawCreateRecurringTemplate {
	return commands.RawCreateRecurringTemplate{
		Direction:         "income",
		PaymentMethod:     "pix",
		AmountCents:       2000,
		Description:       "Aluguel",
		CategoryID:        uuid.New().String(),
		Frequency:         "monthly",
		DayOfMonth:        5,
		StartedAt:         time.Now(),
		InstallmentsTotal: 1,
	}
}

func TestNewCreateRecurringTemplate_Valid(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	userID := uuid.New()

	cmd, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cmd.UserID.UUID())
	assert.Equal(t, valueobjects.DirectionIncome, cmd.Direction)
	assert.Equal(t, valueobjects.FrequencyMonthly, cmd.Frequency)
	assert.Equal(t, 5, cmd.DayOfMonth.Value())
}

func TestNewCreateRecurringTemplate_InvalidAmountCents(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.AmountCents = 0
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewCreateRecurringTemplate_InvalidDayOfMonth(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.DayOfMonth = 0
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewCreateRecurringTemplate_DayOfMonthAbove28(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.DayOfMonth = 29
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewCreateRecurringTemplate_InvalidFrequency(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.Frequency = "weekly"
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrFrequencyUnknown))
}

func TestNewCreateRecurringTemplate_AccumulatesMultipleErrors(t *testing.T) {
	raw := commands.RawCreateRecurringTemplate{
		Direction:     "invalid",
		PaymentMethod: "pix",
		AmountCents:   -1,
		Description:   "",
		CategoryID:    uuid.New().String(),
		Frequency:     "weekly",
		DayOfMonth:    0,
		StartedAt:     time.Now(),
	}
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDirectionUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
	assert.True(t, errors.Is(err, valueobjects.ErrFrequencyUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewCreateRecurringTemplate_CreditCardRequiresCardID(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.PaymentMethod = "credit_card"
	raw.CardID = ""
	userID := uuid.New()

	_, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresCardID))
}

func TestNewCreateRecurringTemplate_WithEndedAt(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	end := time.Now().AddDate(1, 0, 0)
	raw.EndedAt = &end
	userID := uuid.New()

	cmd, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.EndedAt.IsPresent())
}

func TestNewCreateRecurringTemplate_WithSubcategory(t *testing.T) {
	raw := validCreateRecurringTemplateRaw()
	raw.SubcategoryID = uuid.New().String()
	userID := uuid.New()

	cmd, err := commands.NewCreateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.SubcategoryID.IsPresent())
}
