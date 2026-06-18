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

func validUpdateRecurringTemplateRaw() commands.RawUpdateRecurringTemplate {
	return commands.RawUpdateRecurringTemplate{
		TemplateID:        uuid.New().String(),
		Direction:         "outcome",
		PaymentMethod:     "pix",
		AmountCents:       1500,
		Description:       "Streaming",
		CategoryID:        uuid.New().String(),
		Frequency:         "monthly",
		DayOfMonth:        10,
		StartedAt:         time.Now(),
		InstallmentsTotal: 1,
		Version:           1,
	}
}

func TestNewUpdateRecurringTemplate_Valid(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	userID := uuid.New()

	cmd, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cmd.UserID.UUID())
	assert.Equal(t, valueobjects.DirectionOutcome, cmd.Direction)
	assert.Equal(t, valueobjects.FrequencyMonthly, cmd.Frequency)
	assert.Equal(t, 10, cmd.DayOfMonth.Value())
	assert.Equal(t, int64(1500), cmd.Amount.Cents())
}

func TestNewUpdateRecurringTemplate_InvalidTemplateID(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.TemplateID = "not-a-uuid"
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
}

func TestNewUpdateRecurringTemplate_InvalidAmountCents(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.AmountCents = 0
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewUpdateRecurringTemplate_InvalidDayOfMonth(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.DayOfMonth = 0
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewUpdateRecurringTemplate_DayOfMonthAbove28(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.DayOfMonth = 29
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewUpdateRecurringTemplate_InvalidFrequency(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.Frequency = "daily"
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrFrequencyUnknown))
}

func TestNewUpdateRecurringTemplate_CreditCardRequiresCardID(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.PaymentMethod = "credit_card"
	raw.CardID = ""
	userID := uuid.New()

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresCardID))
}

func TestNewUpdateRecurringTemplate_AccumulatesMultipleErrors(t *testing.T) {
	raw := commands.RawUpdateRecurringTemplate{
		TemplateID:    "bad-uuid",
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

	_, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDirectionUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
	assert.True(t, errors.Is(err, valueobjects.ErrFrequencyUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrDayOfMonthOutOfRange))
}

func TestNewUpdateRecurringTemplate_WithEndedAt(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	end := time.Now().AddDate(0, 6, 0)
	raw.EndedAt = &end
	userID := uuid.New()

	cmd, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.EndedAt.IsPresent())
}

func TestNewUpdateRecurringTemplate_WithSubcategory(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.SubcategoryID = uuid.New().String()
	userID := uuid.New()

	cmd, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.SubcategoryID.IsPresent())
}

func TestNewUpdateRecurringTemplate_YearlyFrequency(t *testing.T) {
	raw := validUpdateRecurringTemplateRaw()
	raw.Frequency = "yearly"
	userID := uuid.New()

	cmd, err := commands.NewUpdateRecurringTemplate(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, valueobjects.FrequencyYearly, cmd.Frequency)
}
