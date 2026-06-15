package observability_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	cardobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/observability"
)

func TestRedactCardLogFields_NoPII(t *testing.T) {
	name, err := valueobjects.NewCardName("Nubank")
	require.NoError(t, err)

	nickname, err := valueobjects.NewNickname("Nu")
	require.NoError(t, err)

	cycle, err := valueobjects.NewBillingCycle(15, 22)
	require.NoError(t, err)

	card := entities.HydrateCard(
		uuid.New(),
		uuid.New(),
		name,
		nickname,
		cycle,
		time.Now().UTC(),
		time.Now().UTC(),
		nil,
	)

	fields := cardobs.Redactor{}.RedactCardLogFields(card)

	keys := make(map[string]bool, len(fields))
	for _, f := range fields {
		keys[f.Key] = true
	}

	require.False(t, keys["name"], "field 'name' must not appear in log fields (PII)")
	require.False(t, keys["nickname"], "field 'nickname' must not appear in log fields (PII)")

	require.True(t, keys["card_id"], "field 'card_id' must be present")
	require.True(t, keys["user_id"], "field 'user_id' must be present")
	require.True(t, keys["closing_day"], "field 'closing_day' must be present")
	require.True(t, keys["due_day"], "field 'due_day' must be present")
}

func TestRedactCardLogFields_Values(t *testing.T) {
	cardID := uuid.New()
	userID := uuid.New()

	name, _ := valueobjects.NewCardName("Itau")
	nickname, _ := valueobjects.NewNickname("It")
	cycle, _ := valueobjects.NewBillingCycle(10, 20)

	card := entities.HydrateCard(cardID, userID, name, nickname, cycle, time.Now().UTC(), time.Now().UTC(), nil)

	fields := cardobs.Redactor{}.RedactCardLogFields(card)

	fieldMap := make(map[string]string, len(fields))
	for _, f := range fields {
		fieldMap[f.Key] = f.StringValue()
	}

	require.Equal(t, cardID.String(), fieldMap["card_id"])
	require.Equal(t, userID.String(), fieldMap["user_id"])
}
