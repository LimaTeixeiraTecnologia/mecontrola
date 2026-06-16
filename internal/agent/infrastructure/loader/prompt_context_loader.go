package loader

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

const (
	categoriesPreviewLimit = 30
	cardsPreviewLimit      = 20
)

type ListCategoriesUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error)
}

type ListCardsUseCase interface {
	Execute(ctx context.Context, in cardinput.ListCards) (cardoutput.CardList, error)
}

type PromptContextLoader struct {
	categories    ListCategoriesUseCase
	cards         ListCardsUseCase
	o11y          observability.Observability
	loaderFailure observability.Counter
}

func NewPromptContextLoader(
	categories ListCategoriesUseCase,
	cards ListCardsUseCase,
	o11y observability.Observability,
) *PromptContextLoader {
	failure := o11y.Metrics().Counter(
		"agent_llm_prompt_loader_failures_total",
		"Falhas ao carregar contexto para o prompt LLM por origem",
		"1",
	)
	return &PromptContextLoader{
		categories:    categories,
		cards:         cards,
		o11y:          o11y,
		loaderFailure: failure,
	}
}

func (l *PromptContextLoader) Load(ctx context.Context, userID uuid.UUID, channel string) (interfaces.PromptSeed, error) {
	ctx, span := l.o11y.Tracer().Start(ctx, "agent.llm.prompt_loader.load")
	defer span.End()

	seed := interfaces.PromptSeed{Permissions: []string{"read", "write"}}

	if l.categories != nil {
		out, err := l.categories.Execute(ctx, &categoriesinput.ListCategoriesInput{IncludeDeprecated: false})
		if err != nil {
			l.loaderFailure.Add(ctx, 1, observability.String("source", "categories"))
			l.o11y.Logger().Warn(ctx, "agent.llm.prompt_loader.categories_failed",
				observability.String("user_id", userID.String()),
				observability.Error(err),
			)
		} else if out != nil {
			seed.Categories = collectCategorySeeds(out.Categories)
		}
	}

	if l.cards != nil {
		out, err := l.cards.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: cardsPreviewLimit})
		if err != nil {
			l.loaderFailure.Add(ctx, 1, observability.String("source", "cards"))
			l.o11y.Logger().Warn(ctx, "agent.llm.prompt_loader.cards_failed",
				observability.String("user_id", userID.String()),
				observability.Error(err),
			)
		} else {
			seed.Cards = collectCardSeeds(out.Items)
		}
	}

	span.SetAttributes(
		observability.Int("categories_count", len(seed.Categories)),
		observability.Int("cards_count", len(seed.Cards)),
		observability.String("channel", channel),
	)
	return seed, nil
}

func collectCategorySeeds(items []categoriesoutput.CategoryTreeOutput) []interfaces.CategorySeed {
	count := min(len(items), categoriesPreviewLimit)
	out := make([]interfaces.CategorySeed, 0, count)
	for i := range count {
		item := items[i]
		if item.Name == "" {
			continue
		}
		out = append(out, interfaces.CategorySeed{
			ID:   item.ID.String(),
			Name: item.Name,
		})
	}
	return out
}

func collectCardSeeds(items []cardoutput.Card) []interfaces.CardSeed {
	out := make([]interfaces.CardSeed, 0, len(items))
	for _, item := range items {
		nickname := item.Nickname
		if nickname == "" {
			nickname = item.Name
		}
		out = append(out, interfaces.CardSeed{
			ID:         item.ID,
			Name:       item.Name,
			Nickname:   nickname,
			ClosingDay: item.ClosingDay,
			DueDay:     item.DueDay,
			LimitCents: item.LimitCents,
		})
	}
	return out
}
