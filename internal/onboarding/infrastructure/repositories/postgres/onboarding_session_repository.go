package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type onboardingSessionRepository struct {
	o11y         observability.Observability
	db           database.DBTX
	driftCounter observability.Counter
}

func NewOnboardingSessionRepository(o11y observability.Observability, db database.DBTX) appinterfaces.OnboardingSessionRepository {
	return &onboardingSessionRepository{
		o11y:         o11y,
		db:           db,
		driftCounter: o11y.Metrics().Counter("onboarding_state_drift_total", "Sessoes com estado active sem completed_at", "1"),
	}
}

type onboardingTurnJSON struct {
	Role       string    `json:"role"`
	Text       string    `json:"text"`
	OccurredAt time.Time `json:"occurred_at"`
}

type onboardingSessionPayloadJSON struct {
	IncomeCents      int64                           `json:"income_cents"`
	Cards            []onboardingCardDraftJSON       `json:"cards"`
	PendingCard      onboardingCardDraftJSON         `json:"pending_card"`
	HasPending       bool                            `json:"has_pending"`
	Split            []onboardingSplitEntryJSON      `json:"split"`
	Objective        string                          `json:"objective,omitempty"`
	CustomSplit      []onboardingAllocationEntryJSON `json:"custom_split,omitempty"`
	FirstTxRecorded  bool                            `json:"first_tx_recorded,omitempty"`
	Phase            string                          `json:"phase,omitempty"`
	RecentTurns      []onboardingTurnJSON            `json:"recent_turns,omitempty"`
	WelcomeSentAt    *time.Time                      `json:"welcome_sent_at,omitempty"`
	CompletedAt      *time.Time                      `json:"completed_at,omitempty"`
	ObjectiveProfile string                          `json:"objective_profile,omitempty"`
}

type onboardingCardDraftJSON struct {
	Name       string `json:"name"`
	LimitCents int64  `json:"limit_cents"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

type onboardingSplitEntryJSON struct {
	Kind    string `json:"kind"`
	Percent int    `json:"percent"`
}

type onboardingAllocationEntryJSON struct {
	Kind        string `json:"kind"`
	BasisPoints int    `json:"basis_points"`
}

func (r *onboardingSessionRepository) Find(ctx context.Context, userID uuid.UUID) (entities.OnboardingSession, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.session.find")
	defer span.End()

	const query = `
		SELECT user_id, channel, state, payload, updated_at
		  FROM mecontrola.onboarding_sessions
		 WHERE user_id = $1
	`

	var (
		uid       uuid.UUID
		channel   string
		state     string
		payload   []byte
		updatedAt time.Time
	)
	row := r.db.QueryRowContext(ctx, query, userID)
	if err := row.Scan(&uid, &channel, &state, &payload, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound
		}
		return entities.OnboardingSession{}, fmt.Errorf("onboarding: session_repository.find: %w", err)
	}

	parsedChannel, err := entities.ParseOnboardingChannel(channel)
	if err != nil {
		return entities.OnboardingSession{}, fmt.Errorf("onboarding: session_repository.find: %w", err)
	}

	var pj onboardingSessionPayloadJSON
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &pj); err != nil {
			return entities.OnboardingSession{}, fmt.Errorf("onboarding: session_repository.find: unmarshal payload: %w", err)
		}
	}

	if state == valueobjects.OnboardingStateActive && pj.CompletedAt == nil {
		r.driftCounter.Add(ctx, 1)
		r.o11y.Logger().Warn(ctx, "onboarding.repository.state_drift",
			observability.String("session_id", uid.String()),
		)
	}

	phase, phaseErr := valueobjects.ParseOnboardingPhase(pj.Phase)
	if phaseErr != nil {
		r.o11y.Logger().Warn(ctx, "onboarding.repository.phase_migration_reset",
			observability.String("session_id", uid.String()),
			observability.String("phase_raw", pj.Phase),
			observability.Error(phaseErr),
		)
		if state == valueobjects.OnboardingStateInProgress {
			pj = onboardingSessionPayloadJSON{Phase: valueobjects.PhaseWelcome.String()}
		}
		phase = valueobjects.PhaseWelcome
	}

	domainPayload := entities.OnboardingSessionPayload{
		IncomeCents:      pj.IncomeCents,
		Cards:            fromCardsJSON(pj.Cards),
		PendingCard:      fromCardJSON(pj.PendingCard),
		HasPending:       pj.HasPending,
		Split:            fromSplitJSON(pj.Split),
		Objective:        pj.Objective,
		CustomSplit:      fromAllocationJSON(pj.CustomSplit),
		FirstTxRecorded:  pj.FirstTxRecorded,
		Phase:            phase,
		RecentTurns:      fromTurnsJSON(pj.RecentTurns),
		WelcomeSentAt:    pj.WelcomeSentAt,
		CompletedAt:      pj.CompletedAt,
		ObjectiveProfile: pj.ObjectiveProfile,
	}

	return entities.HydrateOnboardingSession(uid, parsedChannel, domainPayload, updatedAt), nil
}

func (r *onboardingSessionRepository) Upsert(ctx context.Context, session entities.OnboardingSession) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.session.upsert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.onboarding_sessions (user_id, channel, state, payload, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		   SET channel = EXCLUDED.channel,
		       state = EXCLUDED.state,
		       payload = EXCLUDED.payload,
		       updated_at = EXCLUDED.updated_at
	`

	p := session.Payload()
	pj := onboardingSessionPayloadJSON{
		IncomeCents:      p.IncomeCents,
		Cards:            toCardsJSON(p.Cards),
		PendingCard:      toCardJSON(p.PendingCard),
		HasPending:       p.HasPending,
		Split:            toSplitJSON(p.Split),
		Objective:        p.Objective,
		CustomSplit:      toAllocationJSON(p.CustomSplit),
		FirstTxRecorded:  p.FirstTxRecorded,
		Phase:            p.Phase.String(),
		RecentTurns:      toTurnsJSON(p.RecentTurns),
		WelcomeSentAt:    p.WelcomeSentAt,
		CompletedAt:      p.CompletedAt,
		ObjectiveProfile: p.ObjectiveProfile,
	}
	raw, err := json.Marshal(pj)
	if err != nil {
		return fmt.Errorf("onboarding: session_repository.upsert: marshal payload: %w", err)
	}

	stateColumn := valueobjects.OnboardingStateInProgress
	if session.IsActive() {
		stateColumn = valueobjects.OnboardingStateActive
	}

	_, err = r.db.ExecContext(ctx, query,
		session.UserID(),
		session.Channel().String(),
		stateColumn,
		raw,
		session.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("onboarding: session_repository.upsert: %w", err)
	}
	return nil
}

func (r *onboardingSessionRepository) MarkActive(ctx context.Context, userID uuid.UUID) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.session.mark_active")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_sessions
		   SET state = 'active', updated_at = now()
		 WHERE user_id = $1
	`

	res, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("onboarding: session_repository.mark_active: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("onboarding: session_repository.mark_active: rows affected: %w", err)
	}
	if rows == 0 {
		return appinterfaces.ErrOnboardingSessionNotFound
	}
	return nil
}

func toCardsJSON(in []entities.OnboardingCardDraft) []onboardingCardDraftJSON {
	out := make([]onboardingCardDraftJSON, 0, len(in))
	for _, c := range in {
		out = append(out, toCardJSON(c))
	}
	return out
}

func toCardJSON(c entities.OnboardingCardDraft) onboardingCardDraftJSON {
	return onboardingCardDraftJSON{
		Name:       c.Name,
		LimitCents: c.LimitCents,
		ClosingDay: c.ClosingDay,
		DueDay:     c.DueDay,
	}
}

func fromCardsJSON(in []onboardingCardDraftJSON) []entities.OnboardingCardDraft {
	out := make([]entities.OnboardingCardDraft, 0, len(in))
	for _, c := range in {
		out = append(out, fromCardJSON(c))
	}
	return out
}

func fromCardJSON(c onboardingCardDraftJSON) entities.OnboardingCardDraft {
	return entities.OnboardingCardDraft{
		Name:       c.Name,
		LimitCents: c.LimitCents,
		ClosingDay: c.ClosingDay,
		DueDay:     c.DueDay,
	}
}

func toSplitJSON(in []entities.OnboardingCardSplitEntry) []onboardingSplitEntryJSON {
	out := make([]onboardingSplitEntryJSON, 0, len(in))
	for _, e := range in {
		out = append(out, onboardingSplitEntryJSON{Kind: e.Kind, Percent: e.Percent})
	}
	return out
}

func fromSplitJSON(in []onboardingSplitEntryJSON) []entities.OnboardingCardSplitEntry {
	out := make([]entities.OnboardingCardSplitEntry, 0, len(in))
	for _, e := range in {
		out = append(out, entities.OnboardingCardSplitEntry{Kind: e.Kind, Percent: e.Percent})
	}
	return out
}

func toAllocationJSON(in []entities.OnboardingBudgetAllocationEntry) []onboardingAllocationEntryJSON {
	out := make([]onboardingAllocationEntryJSON, 0, len(in))
	for _, e := range in {
		out = append(out, onboardingAllocationEntryJSON{Kind: e.Kind, BasisPoints: e.BasisPoints})
	}
	return out
}

func fromAllocationJSON(in []onboardingAllocationEntryJSON) []entities.OnboardingBudgetAllocationEntry {
	out := make([]entities.OnboardingBudgetAllocationEntry, 0, len(in))
	for _, e := range in {
		out = append(out, entities.OnboardingBudgetAllocationEntry{Kind: e.Kind, BasisPoints: e.BasisPoints})
	}
	return out
}

func toTurnsJSON(in []entities.OnboardingTurn) []onboardingTurnJSON {
	if len(in) == 0 {
		return nil
	}
	out := make([]onboardingTurnJSON, 0, len(in))
	for _, t := range in {
		out = append(out, onboardingTurnJSON{Role: t.Role, Text: t.Text, OccurredAt: t.OccurredAt})
	}
	return out
}

func fromTurnsJSON(in []onboardingTurnJSON) []entities.OnboardingTurn {
	if len(in) == 0 {
		return nil
	}
	out := make([]entities.OnboardingTurn, 0, len(in))
	for _, t := range in {
		out = append(out, entities.OnboardingTurn{Role: t.Role, Text: t.Text, OccurredAt: t.OccurredAt})
	}
	return out
}
