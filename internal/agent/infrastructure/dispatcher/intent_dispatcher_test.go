package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
)

type stubCategories struct {
	called bool
	reply  string
	err    error
}

func (s *stubCategories) List(_ context.Context, _ json.RawMessage) (string, error) {
	s.called = true
	return s.reply, s.err
}

type stubCards struct {
	called  bool
	gotUser uuid.UUID
	reply   string
	err     error
}

func (s *stubCards) List(_ context.Context, userID uuid.UUID, _ json.RawMessage) (string, error) {
	s.called = true
	s.gotUser = userID
	return s.reply, s.err
}

type stubTransactions struct {
	called     bool
	gotUser    uuid.UUID
	gotFilters string
	reply      string
	err        error
}

func (s *stubTransactions) List(_ context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	s.called = true
	s.gotUser = userID
	s.gotFilters = string(rawFilters)
	return s.reply, s.err
}

func mustIntent(t *testing.T, module valueobjects.IntentModule, action valueobjects.IntentAction) entities.IntentResult {
	t.Helper()
	r, err := entities.NewIntentResult(module, action, json.RawMessage(`{}`), json.RawMessage(`{}`), "ok")
	require.NoError(t, err)
	return r
}

func newOutcome(t *testing.T, module valueobjects.IntentModule, action valueobjects.IntentAction) services.IntentOutcome {
	t.Helper()
	return services.IntentOutcome{
		Kind:       services.IntentOutcomeRouted,
		Intent:     mustIntent(t, module, action),
		Provider:   valueobjects.ModelSlugGeminiFlashLite(),
		EventID:    uuid.New(),
		OccurredAt: time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
	}
}

func TestIntentDispatcher_Categories_List_RoutesToAdapter(t *testing.T) {
	categories := &stubCategories{reply: "Voce tem 3 categorias: x."}
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{Categories: categories}, noop.NewProvider())

	outcome := newOutcome(t, valueobjects.IntentModuleCategories(), valueobjects.IntentActionList())
	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)

	require.NoError(t, err)
	assert.True(t, categories.called)
	assert.True(t, res.WasApplied)
	assert.Contains(t, res.ReplyText, "categorias")
}

func TestIntentDispatcher_Cards_List_RoutesToAdapter(t *testing.T) {
	cards := &stubCards{reply: "Voce tem 1 cartao(oes): nubank."}
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{Cards: cards}, noop.NewProvider())

	userID := uuid.New()
	outcome := newOutcome(t, valueobjects.IntentModuleCards(), valueobjects.IntentActionList())
	res, err := sut.Dispatch(context.Background(), userID, outcome)

	require.NoError(t, err)
	assert.True(t, cards.called)
	assert.Equal(t, userID, cards.gotUser)
	assert.True(t, res.WasApplied)
	assert.Contains(t, res.ReplyText, "cartao")
}

func TestIntentDispatcher_Transactions_List_RoutesToAdapter(t *testing.T) {
	tx := &stubTransactions{reply: "Em 2026-06 voce tem 5 lancamentos."}
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{Transactions: tx}, noop.NewProvider())

	userID := uuid.New()
	outcome := newOutcome(t, valueobjects.IntentModuleTransactions(), valueobjects.IntentActionList())
	res, err := sut.Dispatch(context.Background(), userID, outcome)

	require.NoError(t, err)
	assert.True(t, tx.called)
	assert.Equal(t, userID, tx.gotUser)
	assert.True(t, res.WasApplied)
}

func TestIntentDispatcher_NonRoutedOutcome_PassThroughHint(t *testing.T) {
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{}, noop.NewProvider())
	outcome := services.IntentOutcome{
		Kind:         services.IntentOutcomeStructuredError,
		ResponseHint: "Nao encontrei essa categoria.",
	}

	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)
	require.NoError(t, err)
	assert.False(t, res.WasApplied)
	assert.Equal(t, "Nao encontrei essa categoria.", res.ReplyText)
}

func TestIntentDispatcher_UnsupportedModule_ReturnsFriendlyMessage(t *testing.T) {
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{}, noop.NewProvider())
	outcome := newOutcome(t, valueobjects.IntentModuleBudgets(), valueobjects.IntentActionCreate())

	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)
	require.NoError(t, err)
	assert.False(t, res.WasApplied)
	assert.True(t, strings.Contains(res.ReplyText, "budgets.create"))
}

func TestIntentDispatcher_UnsupportedAction_ReturnsFriendlyMessage(t *testing.T) {
	categories := &stubCategories{}
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{Categories: categories}, noop.NewProvider())

	outcome := newOutcome(t, valueobjects.IntentModuleCategories(), valueobjects.IntentActionGet())
	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)

	require.NoError(t, err)
	assert.False(t, categories.called)
	assert.False(t, res.WasApplied)
	assert.Contains(t, res.ReplyText, "categories.get")
}

func TestIntentDispatcher_NilPort_ReturnsUnsupported(t *testing.T) {
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{}, noop.NewProvider())
	outcome := newOutcome(t, valueobjects.IntentModuleCategories(), valueobjects.IntentActionList())

	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)
	require.NoError(t, err)
	assert.False(t, res.WasApplied)
}

func TestIntentDispatcher_AdapterError_PropagatesAndReturnsFriendlyText(t *testing.T) {
	categories := &stubCategories{err: errors.New("db down")}
	sut := dispatcher.NewIntentDispatcher(interfaces.ModulePorts{Categories: categories}, noop.NewProvider())

	outcome := newOutcome(t, valueobjects.IntentModuleCategories(), valueobjects.IntentActionList())
	res, err := sut.Dispatch(context.Background(), uuid.New(), outcome)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "categories.list")
	assert.True(t, categories.called)
	assert.NotEmpty(t, res.ReplyText)
}
