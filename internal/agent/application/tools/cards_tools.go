package tools

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type ListCards struct {
	recorder *Recorder
	lister   CardLister
	o11y     observability.Observability
}

func NewListCards(recorder *Recorder, lister CardLister, o11y observability.Observability) *ListCards {
	return &ListCards{recorder: recorder, lister: lister, o11y: o11y}
}

func (t *ListCards) Name() string { return "list_cards" }

func (t *ListCards) Descriptor() ToolSpec {
	return ToolSpec{Name: "list_cards", IntentKind: intent.KindListCards, Description: "list_cards"}
}

func (t *ListCards) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindListCards
	if t.lister == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	cards, err := WithReadRetry(ctx, func(ctx context.Context) (cardoutput.CardList, error) {
		return t.lister.Execute(ctx, cardinput.ListCards{UserID: in.UserID, Limit: defaultListCardsLimit})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.list_cards_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCardList(cards), Outcome: OutcomeRouted, Kind: kind}, nil
}

type CreateCard struct {
	recorder *Recorder
	creator  CardCreator
	o11y     observability.Observability
}

func NewCreateCard(recorder *Recorder, creator CardCreator, o11y observability.Observability) *CreateCard {
	return &CreateCard{recorder: recorder, creator: creator, o11y: o11y}
}

func (t *CreateCard) Name() string { return "create_card" }

func (t *CreateCard) Descriptor() ToolSpec {
	return ToolSpec{Name: "create_card", IntentKind: intent.KindCreateCard, Description: "create_card"}
}

func (t *CreateCard) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindCreateCard
	if t.creator == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.creator.Execute(ctx, in.UserID, in.Intent)
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.create_card_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: createCardErrorText(err), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCreatedCard(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type CountCards struct {
	recorder *Recorder
	counter  CardCounter
	o11y     observability.Observability
}

func NewCountCards(recorder *Recorder, counter CardCounter, o11y observability.Observability) *CountCards {
	return &CountCards{recorder: recorder, counter: counter, o11y: o11y}
}

func (t *CountCards) Name() string { return "count_cards" }

func (t *CountCards) Descriptor() ToolSpec {
	return ToolSpec{Name: "count_cards", IntentKind: intent.KindCountCards, Description: "count_cards"}
}

func (t *CountCards) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindCountCards
	if t.counter == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	total, err := WithReadRetry(ctx, func(ctx context.Context) (int64, error) {
		return t.counter.Execute(ctx, in.UserID)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.count_cards_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCardCount(total), Outcome: OutcomeRouted, Kind: kind}, nil
}

type UpdateCard struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	updater       CardUpdater
	o11y          observability.Observability
}

func NewUpdateCard(recorder *Recorder, clarification *ClarificationResolver, updater CardUpdater, o11y observability.Observability) *UpdateCard {
	return &UpdateCard{recorder: recorder, clarification: clarification, updater: updater, o11y: o11y}
}

func (t *UpdateCard) Name() string { return "update_card" }

func (t *UpdateCard) Descriptor() ToolSpec {
	return ToolSpec{Name: "update_card", IntentKind: intent.KindUpdateCard, Description: "update_card"}
}

func (t *UpdateCard) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindUpdateCard
	if t.updater == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.updater.Execute(ctx, in.UserID, in.Intent)
	if err != nil {
		if clarify, ok := t.clarification.ResolveCard(ctx, in.Channel, kind, in.Intent.CardName(), err); ok {
			return clarify, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.update_card_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: createCardErrorText(err), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatUpdatedCard(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type DeleteCard struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	deleter       CardDeleter
	o11y          observability.Observability
}

func NewDeleteCard(recorder *Recorder, clarification *ClarificationResolver, deleter CardDeleter, o11y observability.Observability) *DeleteCard {
	return &DeleteCard{recorder: recorder, clarification: clarification, deleter: deleter, o11y: o11y}
}

func (t *DeleteCard) Name() string { return "delete_card" }

func (t *DeleteCard) Descriptor() ToolSpec {
	return ToolSpec{Name: "delete_card", IntentKind: intent.KindDeleteCard, Description: "delete_card"}
}

func (t *DeleteCard) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindDeleteCard
	if t.deleter == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.deleter.Execute(ctx, in.UserID, in.Intent.CardName())
	if err != nil {
		if clarify, ok := t.clarification.ResolveCard(ctx, in.Channel, kind, in.Intent.CardName(), err); ok {
			return clarify, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.delete_card_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatDeletedCard(result), Outcome: OutcomeRouted, Kind: kind}, nil
}
