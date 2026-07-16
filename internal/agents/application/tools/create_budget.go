package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	createBudgetOutcomeStarted               = "started"
	createBudgetOutcomeClarify               = "clarify"
	createBudgetOutcomePendingCreationExists = "pending_creation_exists"
)

var errCreateBudgetMonthRefKindRequired = errors.New("monthRefKind: obrigatório")

type CreateBudgetToolInput struct {
	MonthRefKind string `json:"monthRefKind"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
	TotalCents   int64  `json:"totalCents,omitempty"`
}

func (i *CreateBudgetToolInput) Validate() error {
	var errs []error
	if i.MonthRefKind == "" {
		errs = append(errs, errCreateBudgetMonthRefKindRequired)
	} else if _, err := budgetsvo.ParseMonthRefKind(i.MonthRefKind); err != nil {
		errs = append(errs, fmt.Errorf("monthRefKind: %w", err))
	}
	if i.TotalCents < 0 {
		errs = append(errs, errors.New("totalCents: não pode ser negativo"))
	}
	return errors.Join(errs...)
}

type CreateBudgetToolOutput struct {
	Outcome            string `json:"outcome"`
	Competence         string `json:"competence"`
	ConfirmationPrompt string `json:"confirmationPrompt"`
	ClarifyPrompt      string `json:"clarifyPrompt"`
}

func BuildCreateBudgetTool(engine wf.Engine[workflows.BudgetManageState], def wf.Definition[workflows.BudgetManageState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "create_budget_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"monthRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
					"description": "Classificação da referência de mês citada pelo usuário. Use named_without_year sempre que um nome de mês (ex.: junho, março) for citado SEM um ano junto — mesmo que esse mês já tenha passado no ano corrente. NUNCA use current quando um nome de mês foi citado, mesmo sem ano.",
				},
				"year":       map[string]any{"type": "integer", "description": "Ano numérico, apenas quando o usuário citou explicitamente o ano junto ao mês (monthRefKind=explicit). Omitir para named_without_year."},
				"month":      map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "Mês numérico (1-12), quando monthRefKind=explicit ou named_without_year."},
				"totalCents": map[string]any{"type": "integer", "minimum": 0},
			},
			"required":             []string{"monthRefKind"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "create_budget_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":            map[string]any{"type": "string"},
				"competence":         map[string]any{"type": "string"},
				"confirmationPrompt": map[string]any{"type": "string"},
				"clarifyPrompt":      map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "competence", "confirmationPrompt", "clarifyPrompt"},
			"additionalProperties": false,
		},
	}
	exec := buildCreateBudgetExec(engine, def)
	return tool.NewTool[CreateBudgetToolInput, CreateBudgetToolOutput]("create_budget", "Inicia a criação conversacional de um orçamento mensal (inclusive retroativo), coletando total e distribuição por categoria até a confirmação.", in, out, exec)
}

func buildCreateBudgetExec(engine wf.Engine[workflows.BudgetManageState], def wf.Definition[workflows.BudgetManageState]) func(context.Context, CreateBudgetToolInput) (CreateBudgetToolOutput, error) {
	return func(ctx context.Context, in CreateBudgetToolInput) (CreateBudgetToolOutput, error) {
		if err := in.Validate(); err != nil {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: input inválido: %w", err)
		}

		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: parse resource uuid: %w", err)
		}

		ref, err := mapCreateBudgetMonthReference(in)
		if err != nil {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: mapear referência de mês: %w", err)
		}

		loc, locErr := time.LoadLocation("America/Sao_Paulo")
		if locErr != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)

		competence, clarifyReason, err := budgetsvo.DecideCompetence(ref, now)
		if err != nil {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: resolver competência: %w", err)
		}
		if clarifyReason != budgetsvo.ClarifyNone {
			return CreateBudgetToolOutput{
				Outcome:       createBudgetOutcomeClarify,
				ClarifyPrompt: createBudgetClarifyPrompt(clarifyReason),
			}, nil
		}

		state := workflows.BudgetManageState{
			Status:     workflows.BudgetManageActive,
			Operation:  workflows.BudgetManageOpCreateRetroactive,
			UserID:     userID,
			Competence: competence.String(),
			TotalCents: in.TotalCents,
			MessageID:  req.MessageID,
		}

		key := workflows.BudgetManageKey(req.ResourceID, req.ThreadID)
		result, startErr := engine.Start(ctx, def, key, state)
		if startErr != nil && !errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return CreateBudgetToolOutput{}, fmt.Errorf("agents.tool.create_budget: iniciar workflow: %w", startErr)
		}
		if errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return CreateBudgetToolOutput{
				Outcome:       createBudgetOutcomePendingCreationExists,
				Competence:    competence.String(),
				ClarifyPrompt: "Há uma criação de orçamento em andamento. Por favor, responda a pergunta anterior antes de solicitar outra.",
			}, nil
		}

		return CreateBudgetToolOutput{
			Outcome:            createBudgetOutcomeStarted,
			Competence:         competence.String(),
			ConfirmationPrompt: result.State.ResponseText,
		}, nil
	}
}

func mapCreateBudgetMonthReference(in CreateBudgetToolInput) (budgetsvo.MonthReference, error) {
	kind, err := budgetsvo.ParseMonthRefKind(in.MonthRefKind)
	if err != nil {
		return budgetsvo.MonthReference{}, err
	}
	return budgetsvo.MonthReference{
		Kind:  kind,
		Year:  in.Year,
		Month: in.Month,
	}, nil
}

func createBudgetClarifyPrompt(reason budgetsvo.ClarifyReason) string {
	switch reason {
	case budgetsvo.ClarifyMissingYear:
		return "De qual ano é esse mês? Preciso do ano para criar o orçamento certo."
	case budgetsvo.ClarifyUnrecognized:
		return "Não entendi de qual mês você está falando. Pode me dizer o mês (e o ano, se for diferente do atual)?"
	default:
		return "Pode me confirmar de qual mês é esse orçamento?"
	}
}
