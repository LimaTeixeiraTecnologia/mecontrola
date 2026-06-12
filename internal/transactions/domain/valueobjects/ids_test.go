package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestParseUserID(t *testing.T) {
	valid := uuid.New().String()
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid", input: valid},
		{name: "invalid", input: "not-a-uuid", wantErr: valueobjects.ErrInvalidUserID},
		{name: "empty", input: "", wantErr: valueobjects.ErrInvalidUserID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := valueobjects.ParseUserID(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.input, id.UUID().String())
		})
	}
}

func TestParseCardID(t *testing.T) {
	valid := uuid.New().String()
	_, err := valueobjects.ParseCardID(valid)
	require.NoError(t, err)

	_, err = valueobjects.ParseCardID("bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCardID))
}

func TestParseCategoryID(t *testing.T) {
	valid := uuid.New().String()
	_, err := valueobjects.ParseCategoryID(valid)
	require.NoError(t, err)

	_, err = valueobjects.ParseCategoryID("bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCategoryID))
}

func TestParseSubcategoryID(t *testing.T) {
	valid := uuid.New().String()
	_, err := valueobjects.ParseSubcategoryID(valid)
	require.NoError(t, err)

	_, err = valueobjects.ParseSubcategoryID("bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidSubcategoryID))
}
