package valueobjects

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCategoryWriteBlocked          = errors.New("transactions: category write blocked")
	ErrCategoryVersionChanged        = errors.New("transactions: category editorial version changed")
	ErrCategoryEvidenceRequired      = errors.New("transactions: category write evidence required")
	ErrCategoryRootWithoutLeaf       = errors.New("transactions: category root without leaf subcategory")
	ErrCategoryDeprecated            = errors.New("transactions: category or subcategory is deprecated")
	ErrCategoryKindDirectionMismatch = errors.New("transactions: category kind does not match transaction direction")
)

type CategoryWriteEvidence struct {
	rootCategoryID   uuid.UUID
	subcategoryID    uuid.UUID
	kind             string
	path             string
	outcome          string
	score            float64
	confidence       string
	quality          string
	signalType       string
	matchedTerm      string
	matchReason      string
	source           CategoryDecisionSource
	editorialVersion int64
	decidedAt        time.Time
}

type CategoryWriteEvidenceInput struct {
	RootCategoryID   uuid.UUID
	SubcategoryID    uuid.UUID
	Kind             string
	Path             string
	Outcome          string
	Score            float64
	Confidence       string
	Quality          string
	SignalType       string
	MatchedTerm      string
	MatchReason      string
	Source           CategoryDecisionSource
	EditorialVersion int64
	DecidedAt        time.Time
}

var (
	validConfidences = map[string]struct{}{
		"high":             {},
		"medium":           {},
		"low":              {},
		"manual_confirmed": {},
	}
	validQualities = map[string]struct{}{
		"exact":            {},
		"token":            {},
		"fuzzy":            {},
		"manual_canonical": {},
	}
	validSignalTypes = map[string]struct{}{
		"canonical_name":   {},
		"alias":            {},
		"phrase":           {},
		"merchant":         {},
		"segment":          {},
		"manual_canonical": {},
	}
	validKinds = map[string]struct{}{
		"expense": {},
		"income":  {},
	}
)

func NewCategoryWriteEvidence(in CategoryWriteEvidenceInput) (CategoryWriteEvidence, error) {
	if err := errors.Join(validateEvidenceInput(in)...); err != nil {
		return CategoryWriteEvidence{}, err
	}
	return CategoryWriteEvidence{
		rootCategoryID:   in.RootCategoryID,
		subcategoryID:    in.SubcategoryID,
		kind:             in.Kind,
		path:             in.Path,
		outcome:          in.Outcome,
		score:            in.Score,
		confidence:       in.Confidence,
		quality:          in.Quality,
		signalType:       in.SignalType,
		matchedTerm:      in.MatchedTerm,
		matchReason:      in.MatchReason,
		source:           in.Source,
		editorialVersion: in.EditorialVersion,
		decidedAt:        in.DecidedAt,
	}, nil
}

func validateEvidenceInput(in CategoryWriteEvidenceInput) []error {
	var errs []error

	if !in.Source.IsValid() {
		errs = append(errs, fmt.Errorf("source: %w", ErrInvalidCategoryDecisionSource))
	}
	if in.Outcome != "matched" {
		errs = append(errs, fmt.Errorf("outcome: deve ser 'matched', recebido %q: %w", in.Outcome, ErrCategoryWriteBlocked))
	}
	if in.Score < 0 || in.Score > 1 {
		errs = append(errs, fmt.Errorf("score: deve estar em [0,1], recebido %v: %w", in.Score, ErrCategoryWriteBlocked))
	}
	if _, ok := validConfidences[in.Confidence]; !ok {
		errs = append(errs, fmt.Errorf("confidence: valor invalido %q: %w", in.Confidence, ErrCategoryWriteBlocked))
	}
	if _, ok := validQualities[in.Quality]; !ok {
		errs = append(errs, fmt.Errorf("quality: valor invalido %q: %w", in.Quality, ErrCategoryWriteBlocked))
	}
	if _, ok := validSignalTypes[in.SignalType]; !ok {
		errs = append(errs, fmt.Errorf("signal_type: valor invalido %q: %w", in.SignalType, ErrCategoryWriteBlocked))
	}
	errs = append(errs, validateEvidenceIDs(in)...)
	if _, ok := validKinds[in.Kind]; !ok {
		errs = append(errs, fmt.Errorf("kind: valor invalido %q: %w", in.Kind, ErrCategoryKindDirectionMismatch))
	}
	if in.EditorialVersion <= 0 {
		errs = append(errs, fmt.Errorf("editorial_version: deve ser > 0, recebido %d: %w", in.EditorialVersion, ErrCategoryVersionChanged))
	}
	if in.Path == "" {
		errs = append(errs, fmt.Errorf("path: nao pode ser vazio: %w", ErrCategoryWriteBlocked))
	}
	if in.DecidedAt.IsZero() {
		errs = append(errs, fmt.Errorf("decided_at: nao pode ser zero: %w", ErrCategoryWriteBlocked))
	}
	if in.Source.IsValid() && in.Source.IsManual() {
		errs = append(errs, validateManualEvidence(in)...)
	}
	return errs
}

func validateEvidenceIDs(in CategoryWriteEvidenceInput) []error {
	var errs []error
	if in.RootCategoryID == uuid.Nil {
		errs = append(errs, fmt.Errorf("root_category_id: uuid zero invalido: %w", ErrCategoryRootWithoutLeaf))
	}
	if in.SubcategoryID == uuid.Nil {
		errs = append(errs, fmt.Errorf("subcategory_id: uuid zero invalido: %w", ErrCategoryRootWithoutLeaf))
	}
	if in.RootCategoryID != uuid.Nil && in.SubcategoryID != uuid.Nil && in.RootCategoryID == in.SubcategoryID {
		errs = append(errs, fmt.Errorf("root_category_id e subcategory_id nao podem ser iguais: %w", ErrCategoryRootWithoutLeaf))
	}
	return errs
}

func validateManualEvidence(in CategoryWriteEvidenceInput) []error {
	var errs []error
	if in.Score != 1.0 {
		errs = append(errs, fmt.Errorf("score manual deve ser 1.0, recebido %v: %w", in.Score, ErrCategoryWriteBlocked))
	}
	if in.Confidence != "manual_confirmed" {
		errs = append(errs, fmt.Errorf("confidence manual deve ser 'manual_confirmed', recebido %q: %w", in.Confidence, ErrCategoryWriteBlocked))
	}
	if in.Quality != "manual_canonical" {
		errs = append(errs, fmt.Errorf("quality manual deve ser 'manual_canonical', recebido %q: %w", in.Quality, ErrCategoryWriteBlocked))
	}
	if in.SignalType != "manual_canonical" {
		errs = append(errs, fmt.Errorf("signal_type manual deve ser 'manual_canonical', recebido %q: %w", in.SignalType, ErrCategoryWriteBlocked))
	}
	if in.MatchReason != "manual canonical id validated" {
		errs = append(errs, fmt.Errorf("match_reason manual deve ser 'manual canonical id validated', recebido %q: %w", in.MatchReason, ErrCategoryWriteBlocked))
	}
	if in.MatchedTerm == "" {
		errs = append(errs, fmt.Errorf("matched_term manual deve ser o slug da subcategoria: %w", ErrCategoryWriteBlocked))
	}
	return errs
}

func (e CategoryWriteEvidence) RootCategoryID() uuid.UUID      { return e.rootCategoryID }
func (e CategoryWriteEvidence) SubcategoryID() uuid.UUID       { return e.subcategoryID }
func (e CategoryWriteEvidence) Kind() string                   { return e.kind }
func (e CategoryWriteEvidence) Path() string                   { return e.path }
func (e CategoryWriteEvidence) Outcome() string                { return e.outcome }
func (e CategoryWriteEvidence) Score() float64                 { return e.score }
func (e CategoryWriteEvidence) Confidence() string             { return e.confidence }
func (e CategoryWriteEvidence) Quality() string                { return e.quality }
func (e CategoryWriteEvidence) SignalType() string             { return e.signalType }
func (e CategoryWriteEvidence) MatchedTerm() string            { return e.matchedTerm }
func (e CategoryWriteEvidence) MatchReason() string            { return e.matchReason }
func (e CategoryWriteEvidence) Source() CategoryDecisionSource { return e.source }
func (e CategoryWriteEvidence) EditorialVersion() int64        { return e.editorialVersion }
func (e CategoryWriteEvidence) DecidedAt() time.Time           { return e.decidedAt }

func (e CategoryWriteEvidence) IsZero() bool {
	return e.rootCategoryID == uuid.Nil && e.subcategoryID == uuid.Nil
}

func ReconstituteEvidence(
	rootCategoryID uuid.UUID,
	subcategoryID uuid.UUID,
	kind string,
	path string,
	outcome string,
	score float64,
	confidence string,
	quality string,
	signalType string,
	matchedTerm string,
	matchReason string,
	source CategoryDecisionSource,
	editorialVersion int64,
	decidedAt time.Time,
) CategoryWriteEvidence {
	return CategoryWriteEvidence{
		rootCategoryID:   rootCategoryID,
		subcategoryID:    subcategoryID,
		kind:             kind,
		path:             path,
		outcome:          outcome,
		score:            score,
		confidence:       confidence,
		quality:          quality,
		signalType:       signalType,
		matchedTerm:      matchedTerm,
		matchReason:      matchReason,
		source:           source,
		editorialVersion: editorialVersion,
		decidedAt:        decidedAt,
	}
}
