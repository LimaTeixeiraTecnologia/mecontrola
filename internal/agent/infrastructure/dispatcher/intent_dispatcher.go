package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

var ErrIntentUnsupported = errors.New("agent.llm.dispatcher: intent module+action not yet implemented in MVP")

type IntentDispatcher struct {
	ports          interfaces.ModulePorts
	o11y           observability.Observability
	dispatchTotal  observability.Counter
	dispatchErrors observability.Counter
}

func NewIntentDispatcher(ports interfaces.ModulePorts, o11y observability.Observability) *IntentDispatcher {
	dispatchTotal := o11y.Metrics().Counter(
		"agent_llm_dispatch_total",
		"Total de intents despachados para use cases reais por module+action+outcome",
		"1",
	)
	dispatchErrors := o11y.Metrics().Counter(
		"agent_llm_dispatch_errors_total",
		"Total de erros ao despachar intents para use cases por module+action+reason",
		"1",
	)
	return &IntentDispatcher{
		ports:          ports,
		o11y:           o11y,
		dispatchTotal:  dispatchTotal,
		dispatchErrors: dispatchErrors,
	}
}

func (d *IntentDispatcher) Dispatch(ctx context.Context, userID uuid.UUID, outcome services.IntentOutcome) (interfaces.DispatchResult, error) {
	ctx, span := d.o11y.Tracer().Start(ctx, "agent.llm.dispatcher.dispatch")
	defer span.End()

	if outcome.Kind != services.IntentOutcomeRouted {
		return interfaces.DispatchResult{ReplyText: outcome.ResponseHint}, nil
	}

	intent := outcome.Intent
	module := intent.Module()
	action := intent.Action()

	span.SetAttributes(
		observability.String("module", module.String()),
		observability.String("action", action.String()),
	)

	reply, err := d.route(ctx, userID, module, action, intent.Filters(), intent.Payload())
	moduleLabel := observability.String("module", module.String())
	actionLabel := observability.String("action", action.String())

	if err != nil {
		d.dispatchErrors.Add(ctx, 1, moduleLabel, actionLabel, observability.String("reason", classifyError(err)))
		span.RecordError(err)
		if errors.Is(err, ErrIntentUnsupported) {
			fallback := fmt.Sprintf("Ainda nao consigo executar %s.%s, mas anotei seu pedido.",
				module.String(), action.String())
			d.dispatchTotal.Add(ctx, 1, moduleLabel, actionLabel, observability.String("outcome", "unsupported"))
			return interfaces.DispatchResult{ReplyText: fallback, WasApplied: false}, nil
		}
		d.dispatchTotal.Add(ctx, 1, moduleLabel, actionLabel, observability.String("outcome", "error"))
		return interfaces.DispatchResult{ReplyText: "Tive uma instabilidade para executar agora. Tente novamente em instantes."},
			fmt.Errorf("agent.llm.dispatcher: %s.%s: %w", module.String(), action.String(), err)
	}

	d.dispatchTotal.Add(ctx, 1, moduleLabel, actionLabel, observability.String("outcome", "applied"))
	return interfaces.DispatchResult{ReplyText: reply, WasApplied: true}, nil
}

func (d *IntentDispatcher) route(
	ctx context.Context,
	userID uuid.UUID,
	module valueobjects.IntentModule,
	action valueobjects.IntentAction,
	filters []byte,
	payload []byte,
) (string, error) {
	switch module.String() {
	case "categories":
		return d.routeCategories(ctx, action, filters)
	case "cards":
		return d.routeCards(ctx, userID, action, filters, payload)
	case "transactions":
		return d.routeTransactions(ctx, userID, action, filters, payload)
	case "budgets":
		return d.routeBudgets(ctx, userID, action, filters, payload)
	default:
		return "", ErrIntentUnsupported
	}
}

func (d *IntentDispatcher) routeBudgets(ctx context.Context, userID uuid.UUID, action valueobjects.IntentAction, filters []byte, payload []byte) (string, error) {
	switch action.String() {
	case "list":
		if d.ports.Budgets == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.Budgets.List(ctx, userID, filters)
	case "get":
		if d.ports.BudgetsGet == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.BudgetsGet.Get(ctx, userID, filters)
	case "create":
		if d.ports.BudgetsCreate == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.BudgetsCreate.Create(ctx, userID, payload)
	case "update":
		if d.ports.BudgetsUpdate == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.BudgetsUpdate.Update(ctx, userID, payload)
	case "delete":
		if d.ports.BudgetsDelete == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.BudgetsDelete.Delete(ctx, userID, payload)
	default:
		return "", ErrIntentUnsupported
	}
}

func (d *IntentDispatcher) routeCategories(ctx context.Context, action valueobjects.IntentAction, filters []byte) (string, error) {
	switch action.String() {
	case "list":
		if d.ports.CategoriesSearch != nil && looksLikeCategorySearch(filters) {
			return d.ports.CategoriesSearch.Search(ctx, filters)
		}
		if d.ports.CategoriesListDictionary != nil && looksLikeDictionaryList(filters) {
			return d.ports.CategoriesListDictionary.ListDictionary(ctx, filters)
		}
		if d.ports.Categories == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.Categories.List(ctx, filters)
	case "get":
		if d.ports.CategoriesGet == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.CategoriesGet.Get(ctx, filters)
	default:
		return "", ErrIntentUnsupported
	}
}

func (d *IntentDispatcher) routeCards(ctx context.Context, userID uuid.UUID, action valueobjects.IntentAction, filters []byte, payload []byte) (string, error) {
	switch action.String() {
	case "list":
		if d.ports.Cards == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.Cards.List(ctx, userID, filters)
	case "get":
		if d.ports.CardsGet == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.CardsGet.Get(ctx, userID, filters)
	case "create":
		if d.ports.CardsCreate == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.CardsCreate.Create(ctx, userID, payload)
	case "update":
		if d.ports.CardsUpdate == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.CardsUpdate.Update(ctx, userID, payload)
	case "delete":
		if d.ports.CardsDelete == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.CardsDelete.Delete(ctx, userID, payload)
	default:
		return "", ErrIntentUnsupported
	}
}

func (d *IntentDispatcher) routeTransactions(ctx context.Context, userID uuid.UUID, action valueobjects.IntentAction, filters []byte, payload []byte) (string, error) {
	switch action.String() {
	case "list":
		if d.ports.Transactions == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.Transactions.List(ctx, userID, filters)
	case "get":
		if d.ports.TransactionsGet == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.TransactionsGet.Get(ctx, userID, filters)
	case "create":
		if d.ports.TransactionsCreate == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.TransactionsCreate.Create(ctx, userID, payload)
	case "delete":
		if d.ports.TransactionsDelete == nil {
			return "", ErrIntentUnsupported
		}
		return d.ports.TransactionsDelete.Delete(ctx, userID, payload)
	default:
		return "", ErrIntentUnsupported
	}
}

func looksLikeCategorySearch(filters []byte) bool {
	return containsJSONHint(filters, `"query"`)
}

func looksLikeDictionaryList(filters []byte) bool {
	return containsJSONHint(filters, `"signal_type"`) || containsJSONHint(filters, `"category_id"`) || containsJSONHint(filters, `"page_size"`)
}

func containsJSONHint(raw []byte, needle string) bool {
	return len(raw) > 0 && strings.Contains(string(raw), needle)
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, ErrIntentUnsupported):
		return "unsupported"
	default:
		return "use_case"
	}
}
