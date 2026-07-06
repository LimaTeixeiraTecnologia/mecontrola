package valueobjects_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func validEvidenceInput() valueobjects.CategoryWriteEvidenceInput {
	return valueobjects.CategoryWriteEvidenceInput{
		RootCategoryID:   uuid.New(),
		SubcategoryID:    uuid.New(),
		Kind:             "expense",
		Path:             "expense/food",
		Outcome:          "matched",
		Score:            0.95,
		Confidence:       "high",
		Quality:          "exact",
		SignalType:       "canonical_name",
		MatchedTerm:      "food",
		MatchReason:      "exact match on canonical name",
		Source:           valueobjects.CategoryDecisionSourceAutoMatched,
		EditorialVersion: 1,
		DecidedAt:        time.Now().UTC(),
	}
}

func validManualEvidenceInput() valueobjects.CategoryWriteEvidenceInput {
	rootID := uuid.New()
	subID := uuid.New()
	return valueobjects.CategoryWriteEvidenceInput{
		RootCategoryID:   rootID,
		SubcategoryID:    subID,
		Kind:             "expense",
		Path:             "expense/food",
		Outcome:          "matched",
		Score:            1.0,
		Confidence:       "manual_confirmed",
		Quality:          "manual_canonical",
		SignalType:       "manual_canonical",
		MatchedTerm:      "food-slug",
		MatchReason:      "manual canonical id validated",
		Source:           valueobjects.CategoryDecisionSourceManualCanonicalID,
		EditorialVersion: 1,
		DecidedAt:        time.Now().UTC(),
	}
}

func TestNewCategoryWriteEvidence_Valid(t *testing.T) {
	in := validEvidenceInput()
	ev, err := valueobjects.NewCategoryWriteEvidence(in)
	require.NoError(t, err)
	assert.Equal(t, in.RootCategoryID, ev.RootCategoryID())
	assert.Equal(t, in.SubcategoryID, ev.SubcategoryID())
	assert.Equal(t, in.Kind, ev.Kind())
	assert.Equal(t, in.Path, ev.Path())
	assert.Equal(t, in.Outcome, ev.Outcome())
	assert.InDelta(t, in.Score, ev.Score(), 1e-9)
	assert.Equal(t, in.Confidence, ev.Confidence())
	assert.Equal(t, in.Quality, ev.Quality())
	assert.Equal(t, in.SignalType, ev.SignalType())
	assert.Equal(t, in.MatchedTerm, ev.MatchedTerm())
	assert.Equal(t, in.MatchReason, ev.MatchReason())
	assert.Equal(t, in.Source, ev.Source())
	assert.Equal(t, in.EditorialVersion, ev.EditorialVersion())
	assert.False(t, ev.IsZero())
}

func TestNewCategoryWriteEvidence_ZeroValueInvalid(t *testing.T) {
	var ev valueobjects.CategoryWriteEvidence
	assert.True(t, ev.IsZero())
}

func TestNewCategoryWriteEvidence_InvalidSource(t *testing.T) {
	in := validEvidenceInput()
	in.Source = 0
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCategoryDecisionSource))
}

func TestNewCategoryWriteEvidence_OutcomeMustBeMatched(t *testing.T) {
	cases := []struct {
		name    string
		outcome string
	}{
		{"no_match", "no_match"},
		{"ambiguous", "ambiguous"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validEvidenceInput()
			in.Outcome = tc.outcome
			_, err := valueobjects.NewCategoryWriteEvidence(in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
		})
	}
}

func TestNewCategoryWriteEvidence_ScoreOutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		score float64
	}{
		{"negative", -0.01},
		{"above_one", 1.01},
		{"way_above", 2.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validEvidenceInput()
			in.Score = tc.score
			_, err := valueobjects.NewCategoryWriteEvidence(in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
		})
	}
}

func TestNewCategoryWriteEvidence_ScoreBoundaries(t *testing.T) {
	for _, score := range []float64{0.0, 1.0} {
		in := validEvidenceInput()
		in.Score = score
		_, err := valueobjects.NewCategoryWriteEvidence(in)
		require.NoError(t, err)
	}
}

func TestNewCategoryWriteEvidence_InvalidConfidence(t *testing.T) {
	in := validEvidenceInput()
	in.Confidence = "very_high"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_InvalidQuality(t *testing.T) {
	in := validEvidenceInput()
	in.Quality = "perfect"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_InvalidSignalType(t *testing.T) {
	in := validEvidenceInput()
	in.SignalType = "unknown_signal"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_RootUUIDZero(t *testing.T) {
	in := validEvidenceInput()
	in.RootCategoryID = uuid.Nil
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryRootWithoutLeaf))
}

func TestNewCategoryWriteEvidence_LeafUUIDZero(t *testing.T) {
	in := validEvidenceInput()
	in.SubcategoryID = uuid.Nil
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryRootWithoutLeaf))
}

func TestNewCategoryWriteEvidence_RootEqualsLeaf(t *testing.T) {
	in := validEvidenceInput()
	same := uuid.New()
	in.RootCategoryID = same
	in.SubcategoryID = same
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryRootWithoutLeaf))
}

func TestNewCategoryWriteEvidence_InvalidKind(t *testing.T) {
	cases := []string{"", "other", "debit"}
	for _, kind := range cases {
		t.Run(kind, func(t *testing.T) {
			in := validEvidenceInput()
			in.Kind = kind
			_, err := valueobjects.NewCategoryWriteEvidence(in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, valueobjects.ErrCategoryKindDirectionMismatch))
		})
	}
}

func TestNewCategoryWriteEvidence_VersionZeroOrNegative(t *testing.T) {
	cases := []int64{0, -1, -100}
	for _, v := range cases {
		in := validEvidenceInput()
		in.EditorialVersion = v
		_, err := valueobjects.NewCategoryWriteEvidence(in)
		require.Error(t, err)
		assert.True(t, errors.Is(err, valueobjects.ErrCategoryVersionChanged))
	}
}

func TestNewCategoryWriteEvidence_EmptyPath(t *testing.T) {
	in := validEvidenceInput()
	in.Path = ""
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_Valid(t *testing.T) {
	in := validManualEvidenceInput()
	ev, err := valueobjects.NewCategoryWriteEvidence(in)
	require.NoError(t, err)
	assert.Equal(t, valueobjects.CategoryDecisionSourceManualCanonicalID, ev.Source())
	assert.InDelta(t, 1.0, ev.Score(), 1e-9)
	assert.Equal(t, "manual_confirmed", ev.Confidence())
	assert.Equal(t, "manual_canonical", ev.Quality())
	assert.Equal(t, "manual_canonical", ev.SignalType())
	assert.Equal(t, "manual canonical id validated", ev.MatchReason())
}

func TestNewCategoryWriteEvidence_ManualDeterministic_ScoreNot1(t *testing.T) {
	in := validManualEvidenceInput()
	in.Score = 0.9
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_ConfidenceWrong(t *testing.T) {
	in := validManualEvidenceInput()
	in.Confidence = "high"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_QualityWrong(t *testing.T) {
	in := validManualEvidenceInput()
	in.Quality = "exact"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_SignalTypeWrong(t *testing.T) {
	in := validManualEvidenceInput()
	in.SignalType = "alias"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_MatchReasonWrong(t *testing.T) {
	in := validManualEvidenceInput()
	in.MatchReason = "wrong reason"
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ManualDeterministic_EmptyMatchedTerm(t *testing.T) {
	in := validManualEvidenceInput()
	in.MatchedTerm = ""
	_, err := valueobjects.NewCategoryWriteEvidence(in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrCategoryWriteBlocked))
}

func TestNewCategoryWriteEvidence_ValidConfidences(t *testing.T) {
	for _, c := range []string{"high", "medium", "low", "manual_confirmed"} {
		in := validEvidenceInput()
		in.Confidence = c
		if c == "manual_confirmed" {
			in.Source = valueobjects.CategoryDecisionSourceManualCanonicalID
			in.Score = 1.0
			in.Quality = "manual_canonical"
			in.SignalType = "manual_canonical"
			in.MatchReason = "manual canonical id validated"
			in.MatchedTerm = "some-slug"
		}
		_, err := valueobjects.NewCategoryWriteEvidence(in)
		require.NoError(t, err, "confidence %q should be valid", c)
	}
}

func TestNewCategoryWriteEvidence_ValidQualities(t *testing.T) {
	for _, q := range []string{"exact", "token", "fuzzy"} {
		in := validEvidenceInput()
		in.Quality = q
		_, err := valueobjects.NewCategoryWriteEvidence(in)
		require.NoError(t, err, "quality %q should be valid", q)
	}
}

func TestNewCategoryWriteEvidence_ValidKinds(t *testing.T) {
	for _, k := range []string{"expense", "income"} {
		in := validEvidenceInput()
		in.Kind = k
		_, err := valueobjects.NewCategoryWriteEvidence(in)
		require.NoError(t, err, "kind %q should be valid", k)
	}
}

func TestParseCategoryDecisionSource(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    valueobjects.CategoryDecisionSource
		wantErr error
	}{
		{"auto_matched", "auto_matched", valueobjects.CategoryDecisionSourceAutoMatched, nil},
		{"user_selected_candidate", "user_selected_candidate", valueobjects.CategoryDecisionSourceUserSelectedCandidate, nil},
		{"manual_canonical_id", "manual_canonical_id", valueobjects.CategoryDecisionSourceManualCanonicalID, nil},
		{"system_migration", "system_migration", valueobjects.CategoryDecisionSourceSystemMigration, nil},
		{"invalid", "invalid_source", 0, valueobjects.ErrInvalidCategoryDecisionSource},
		{"empty", "", 0, valueobjects.ErrInvalidCategoryDecisionSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := valueobjects.ParseCategoryDecisionSource(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, s)
			assert.Equal(t, tc.input, s.String())
		})
	}
}

func TestCategoryDecisionSource_IsValid(t *testing.T) {
	assert.True(t, valueobjects.CategoryDecisionSourceAutoMatched.IsValid())
	assert.True(t, valueobjects.CategoryDecisionSourceUserSelectedCandidate.IsValid())
	assert.True(t, valueobjects.CategoryDecisionSourceManualCanonicalID.IsValid())
	assert.True(t, valueobjects.CategoryDecisionSourceSystemMigration.IsValid())
	assert.False(t, valueobjects.CategoryDecisionSource(0).IsValid())
	assert.False(t, valueobjects.CategoryDecisionSource(99).IsValid())
}

func TestCategoryDecisionSource_IsManual(t *testing.T) {
	assert.True(t, valueobjects.CategoryDecisionSourceManualCanonicalID.IsManual())
	assert.False(t, valueobjects.CategoryDecisionSourceAutoMatched.IsManual())
	assert.False(t, valueobjects.CategoryDecisionSourceUserSelectedCandidate.IsManual())
	assert.False(t, valueobjects.CategoryDecisionSourceSystemMigration.IsManual())
}

func TestCategoryDecisionSource_String(t *testing.T) {
	assert.Equal(t, "auto_matched", valueobjects.CategoryDecisionSourceAutoMatched.String())
	assert.Equal(t, "user_selected_candidate", valueobjects.CategoryDecisionSourceUserSelectedCandidate.String())
	assert.Equal(t, "manual_canonical_id", valueobjects.CategoryDecisionSourceManualCanonicalID.String())
	assert.Equal(t, "system_migration", valueobjects.CategoryDecisionSourceSystemMigration.String())
	assert.Equal(t, "", valueobjects.CategoryDecisionSource(0).String())
}

func TestCategoryDecisionSource_Iota(t *testing.T) {
	assert.Equal(t, valueobjects.CategoryDecisionSource(1), valueobjects.CategoryDecisionSourceAutoMatched)
	assert.Equal(t, valueobjects.CategoryDecisionSource(2), valueobjects.CategoryDecisionSourceUserSelectedCandidate)
	assert.Equal(t, valueobjects.CategoryDecisionSource(3), valueobjects.CategoryDecisionSourceManualCanonicalID)
	assert.Equal(t, valueobjects.CategoryDecisionSource(4), valueobjects.CategoryDecisionSourceSystemMigration)
}
