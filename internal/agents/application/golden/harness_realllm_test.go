//go:build integration

package golden

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const goldenGateThreshold = 0.90

var errGoldenSimulatedToolFailure = errors.New("golden: falha de persistência simulada")

func buildGoldenHarnessProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}
	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(45*time.Second),
	)
	require.NoError(t, err)
	model := os.Getenv("AGENT_HARNESS_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          model,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-golden-harness",
		MaxTokens:      1536,
		Temperature:    0,
		RequestTimeout: 45 * time.Second,
	}, fake.NewProvider())
}

var goldenPaymentMethodEnum = []string{"pix", "debit_card", "debit_in_account", "cash", "boleto", "ted", "credit_card", "doc", "vale_refeicao", "vale_alimentacao", "transferencia", "apple_pay", "google_pay", "picpay", "mercado_pago", "cheque"}

func goldenSchemaWithPaymentEnum(fields ...string) map[string]any {
	schema := goldenBaseSchema(fields...)
	props := schema["properties"].(map[string]any)
	props["paymentMethod"] = map[string]any{"type": "string", "enum": goldenPaymentMethodEnum}
	return schema
}

func goldenBaseSchema(fields ...string) map[string]any {
	props := map[string]any{}
	for _, f := range fields {
		props[f] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
}

func goldenCaptureTool(name, description string, schema map[string]any, sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: name + "_input", Strict: false, Schema: schema}
	out := llm.Schema{
		Name:   name + "_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"ok": map[string]any{"type": "boolean"}},
			"required":             []string{"ok"},
			"additionalProperties": false,
		},
	}
	type output struct {
		OK bool `json:"ok"`
	}
	return tool.NewTool[map[string]any, output](name, description, in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink(name, in)
			return output{OK: true}, nil
		},
	)
}

func goldenFailingTool(name, description string, schema map[string]any) tool.ToolHandle {
	in := llm.Schema{Name: name + "_input", Strict: false, Schema: schema}
	return tool.NewTool[map[string]any, map[string]any](name, description, in, llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errGoldenSimulatedToolFailure
		},
	)
}

func goldenMonthRefSchema(extra ...string) map[string]any {
	props := map[string]any{
		"monthRefKind": map[string]any{
			"type":        "string",
			"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
			"description": "Classificação da referência de mês citada pelo usuário. Regra obrigatória: se o usuário citou um nome de mês SEM ano (ex.: 'orçamento de junho', 'gastos de março'), o valor DEVE ser named_without_year — mesmo que esse mês já tenha passado no ano corrente e mesmo que você ache que sabe qual ano é. Exemplo: 'quero criar o orçamento de junho' -> monthRefKind=named_without_year (SEM year, SEM month). Use explicit SOMENTE quando o usuário citar mês E ano explicitamente juntos, ex.: 'orçamento de junho de 2026' -> monthRefKind=explicit, year=2026, month=6. NUNCA use current quando um nome de mês foi citado.",
		},
		"year":  map[string]any{"type": "integer", "description": "Ano numérico, apenas quando o usuário citou explicitamente o ano junto ao mês (monthRefKind=explicit). Omitir para named_without_year."},
		"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "Mês numérico (1-12), quando monthRefKind=explicit ou named_without_year."},
	}
	for _, f := range extra {
		props[f] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
}

func goldenMonthAskYearTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "query_month_ask_year_input", Strict: false, Schema: goldenMonthRefSchema("refMonth")}
	out := llm.Schema{
		Name:   "query_month_ask_year_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"outcome": map[string]any{"type": "string"}, "clarifyPrompt": map[string]any{"type": "string"}},
			"required":             []string{"outcome", "clarifyPrompt"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome       string `json:"outcome"`
		ClarifyPrompt string `json:"clarifyPrompt"`
	}
	return tool.NewTool[map[string]any, output]("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário; use para 'quanto gastei?', 'como foi meu mês?', 'última transação'. Quando monthRefKind=named_without_year, a tool pede o ano antes de responder.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("query_month", in)
			return output{Outcome: "clarify", ClarifyPrompt: MonthClarifyPrompt}, nil
		},
	)
}

func goldenRegisterIncomeConfirmTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "register_income_confirm_input", Strict: false, Schema: goldenBaseSchema("description", "amountCents", "occurredAt")}
	out := llm.Schema{
		Name:   "register_income_confirm_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"outcome": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
			"required":             []string{"outcome", "message"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome string `json:"outcome"`
		Message string `json:"message"`
	}
	return tool.NewTool[map[string]any, output]("register_income", "Registra uma receita; pode retornar pedido de confirmação antes de efetivar.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("register_income", in)
			return output{Outcome: "clarify", Message: RegisterExpenseConfirmMessage}, nil
		},
	)
}

func goldenQueryPlanNotFoundTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "query_plan_not_found_input", Strict: false, Schema: goldenMonthRefSchema("refMonth")}
	out := llm.Schema{
		Name:   "query_plan_not_found_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"outcome": map[string]any{"type": "string"}, "offerCreatePrompt": map[string]any{"type": "string"}},
			"required":             []string{"outcome", "offerCreatePrompt"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome           string `json:"outcome"`
		OfferCreatePrompt string `json:"offerCreatePrompt"`
	}
	return tool.NewTool[map[string]any, output]("query_plan", "Consulta o plano orçamentário mensal e alertas do usuário; use para 'como está meu orçamento?', 'panorama do mês', retrospectiva e 'orçamento completo'.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("query_plan", in)
			return output{
				Outcome:           "not_found",
				OfferCreatePrompt: "Você ainda não tem um orçamento para esse mês. Posso te ajudar a criar um?",
			}, nil
		},
	)
}

func goldenCreateBudgetTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "create_budget_input", Strict: false, Schema: goldenMonthRefSchema("totalCents")}
	out := llm.Schema{
		Name:   "create_budget_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"outcome": map[string]any{"type": "string"}, "clarifyPrompt": map[string]any{"type": "string"}, "confirmationPrompt": map[string]any{"type": "string"}},
			"required":             []string{"outcome", "clarifyPrompt", "confirmationPrompt"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome            string `json:"outcome"`
		ClarifyPrompt      string `json:"clarifyPrompt"`
		ConfirmationPrompt string `json:"confirmationPrompt"`
	}
	return tool.NewTool[map[string]any, output]("create_budget", "Inicia a criação conversacional de um orçamento mensal (inclusive retroativo), coletando total e distribuição por categoria até a confirmação. Quando monthRefKind=named_without_year, a tool pede o ano antes de prosseguir.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("create_budget", in)
			kind, _ := in["monthRefKind"].(string)
			if kind == "named_without_year" {
				return output{Outcome: "clarify", ClarifyPrompt: MonthClarifyPrompt}, nil
			}
			return output{Outcome: "ok", ConfirmationPrompt: "Vamos criar seu orçamento. Qual é o valor total?"}, nil
		},
	)
}

func goldenResolveCardNotFoundTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "resolve_card_not_found_input", Strict: false, Schema: goldenBaseSchema("nickname")}
	out := llm.Schema{
		Name:   "resolve_card_not_found_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"found": map[string]any{"type": "boolean"}},
			"required":             []string{"found"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Found bool `json:"found"`
	}
	return tool.NewTool[map[string]any, output]("resolve_card_not_found", "Resolve o 💳 de crédito do usuário pelo apelido informado; use como etapa obrigatória antes de registrar compra no crédito. Se não encontrar o 💳, peça ao usuário para escolher entre os 💳 cadastrados via list_cards — nunca prossiga com um cardId inventado.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("resolve_card_not_found", in)
			return output{Found: false}, nil
		},
	)
}

func goldenEditTreatmentNameTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "edit_treatment_name_input", Strict: false, Schema: goldenBaseSchema("name")}
	out := llm.Schema{
		Name:   "edit_treatment_name_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"status": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
			"required":             []string{"status", "message"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	return tool.NewTool[map[string]any, output]("edit_treatment_name", "Inicia a alteração do nome de tratamento do usuário. SEMPRE chame esta ferramenta quando o usuário pedir para trocar como é chamado, mesmo que o campo name venha vazio — NUNCA responda essa pergunta diretamente sem chamar a ferramenta, pois é ela quem persiste o estado necessário para aplicar o nome na resposta seguinte. Aplica imediatamente quando o nome já vier na mensagem (name preenchido) ou pergunta o novo nome antes de gravar na memória de trabalho (name vazio).", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("edit_treatment_name", in)
			name, _ := in["name"].(string)
			if name == "" {
				return output{Status: "started", Message: TreatmentNameEditQuestionMessage}, nil
			}
			return output{Status: "started", Message: messages.TreatmentNameConfirmation(name)}, nil
		},
	)
}

func goldenRegisterExpensePaymentClarifyTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "register_expense_payment_clarify_input", Strict: false, Schema: goldenSchemaWithPaymentEnum("description", "amountCents", "paymentMethod", "occurredAt")}
	out := llm.Schema{
		Name:   "register_expense_payment_clarify_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"outcome": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
			"required":             []string{"outcome", "message"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome string `json:"outcome"`
		Message string `json:"message"`
	}
	return tool.NewTool[map[string]any, output]("register_expense", "Registra uma despesa; quando faltar a forma de pagamento, retorna a pergunta pronta em message para ser repassada verbatim.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("register_expense", in)
			return output{Outcome: "clarify", Message: messages.ClarificationQuestion(messages.MissingFieldPaymentMethod)}, nil
		},
	)
}

var goldenToolCatalog = map[string]func(sink ToolCaptureSink) tool.ToolHandle{
	"register_expense_payment_clarify": goldenRegisterExpensePaymentClarifyTool,
	"register_expense": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("register_expense", "Registra uma despesa", goldenSchemaWithPaymentEnum("description", "amountCents", "paymentMethod", "occurredAt", "categoryId", "cardId"), sink)
	},
	"register_income": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("register_income", "Registra uma receita", goldenBaseSchema("description", "amountCents", "occurredAt"), sink)
	},
	"create_recurrence": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("create_recurrence", "Cria template recorrente", goldenBaseSchema("description", "amountCents", "frequency", "dayOfMonth", "direction"), sink)
	},
	"query_month": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário; use para 'quanto gastei?', 'como foi meu mês?', 'última transação' e 'últimas N transações'.", goldenMonthRefSchema("refMonth"), sink)
	},
	"get_transaction": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("get_transaction", "Retorna os detalhes de um lançamento de transação pelo ID.", goldenBaseSchema("txId"), sink)
	},
	"search_transactions": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("search_transactions", "Pesquisa lançamentos do usuário por termo explícito no mês informado; use APENAS quando o usuário fornecer uma palavra-chave ou termo de busca específico.", goldenBaseSchema("query", "refMonth"), sink)
	},
	"list_cards": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("list_cards", "Lista os 💳 cadastrados do usuário; use apenas quando o usuário pedir explicitamente para ver, listar ou saber quais são seus 💳.", goldenBaseSchema(), sink)
	},
	"get_card": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("get_card", "Busca dados de um 💳 pelo ID", goldenBaseSchema("cardId"), sink)
	},
	"count_cards": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("count_cards", "Conta 💳 do usuário", goldenBaseSchema(), sink)
	},
	"best_purchase_day": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("best_purchase_day", "Calcula melhor dia de compra", goldenBaseSchema("bank", "dueDay"), sink)
	},
	"query_card_invoice": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("query_card_invoice", "Consulta fatura do 💳", goldenBaseSchema("cardId", "refMonth"), sink)
	},
	"list_recurrences": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("list_recurrences", "Lista templates de recorrência", goldenBaseSchema(), sink)
	},
	"update_recurrence": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("update_recurrence", "Solicita atualização de recorrência", goldenBaseSchema("recurrenceId", "description"), sink)
	},
	"delete_recurrence": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("delete_recurrence", "Solicita exclusão de recorrência", goldenBaseSchema("recurrenceId"), sink)
	},
	"list_categories": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("list_categories", "Lista categorias disponíveis", goldenBaseSchema(), sink)
	},
	"classify_category": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("classify_category", "Classifica lançamento por categoria", goldenBaseSchema("description", "direction"), sink)
	},
	"query_plan": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("query_plan", "Consulta o plano orçamentário mensal e alertas do usuário; use para 'como está meu orçamento?', 'panorama do mês' e 'orçamento completo'.", goldenMonthRefSchema("refMonth"), sink)
	},
	"adjust_allocation": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("adjust_allocation", "Ajusta alocação de categoria", goldenBaseSchema("rootSlug", "basisPoints"), sink)
	},
	"suggest_allocation": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("suggest_allocation", "Sugere distribuição de alocação", goldenBaseSchema("totalCents"), sink)
	},
	"edit_entry": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("edit_entry", "Inicia a edição de um lançamento pelo ID", goldenBaseSchema("entryId", "entryKind"), sink)
	},
	"delete_entry": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("delete_entry", "Solicita exclusão de lançamento ou 💳; entryId DEVE ser o id real do lançamento ou o cardId real do 💳 (obtido via resolve_card quando o 💳 for identificado por apelido), nunca um valor inventado", goldenBaseSchema("entryId", "entryKind", "version"), sink)
	},
	"update_card": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("update_card", "Solicita atualização de 💳", goldenBaseSchema("cardId", "nickname", "dueDay"), sink)
	},
	"resolve_card": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("resolve_card", "Resolve o 💳 de crédito do usuário pelo apelido informado, retornando o cardId; use como etapa obrigatória antes de registrar compra no crédito, antes de consultar a fatura do 💳, OU antes de excluir um 💳 identificado por apelido via delete_entry.", goldenBaseSchema("nickname"), sink)
	},
	"resolve_card_not_found":  goldenResolveCardNotFoundTool,
	"create_budget":           goldenCreateBudgetTool,
	"query_plan_not_found":    goldenQueryPlanNotFoundTool,
	"register_income_confirm": goldenRegisterIncomeConfirmTool,
	"query_month_ask_year":    goldenMonthAskYearTool,
	"create_card": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("create_card", "Cadastra um novo 💳 de crédito pela conversa. Requer confirmação humana explícita antes de criar.", goldenBaseSchema("nickname", "bank", "dueDay", "closingDay"), sink)
	},
	"edit_budget_total": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("edit_budget_total", "Altera o valor total do orçamento mensal ativo, reescalando a distribuição proporcionalmente. Requer confirmação humana explícita.", goldenBaseSchema("refMonth", "newTotalCents"), sink)
	},
	"edit_goal": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("edit_goal", "Consulta e atualiza o objetivo financeiro do usuário, mantido como memória de trabalho. Requer confirmação humana explícita antes de reescrever.", goldenBaseSchema("newGoal"), sink)
	},
	"cancel_plan_info": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("cancel_plan_info", "Retorna o passo a passo oficial e verbatim de cancelamento de assinatura na Kiwify; não altera o estado da assinatura.", goldenBaseSchema(), sink)
	},
	"support_info": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("support_info", "Retorna as informações verbatim de contato do suporte (e-mail e prazo de resposta).", goldenBaseSchema(), sink)
	},
	goldenCategoryDetailWithDataKey: goldenCategoryDetailWithDataTool,
	"category_detail": func(sink ToolCaptureSink) tool.ToolHandle {
		return goldenCaptureTool("category_detail", "Detalha os lançamentos, planejado, gasto e disponível/excedente de uma categoria do orçamento no mês.", goldenMonthRefSchema("rootSlug"), sink)
	},
	"edit_treatment_name": goldenEditTreatmentNameTool,
}

const goldenCategoryDetailWithDataKey = "category_detail_com_dados"

func goldenCategoryDetailWithDataTool(sink ToolCaptureSink) tool.ToolHandle {
	in := llm.Schema{Name: "category_detail_input", Strict: false, Schema: goldenMonthRefSchema("rootSlug")}
	out := llm.Schema{
		Name:   "category_detail_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":        map[string]any{"type": "string"},
				"category":       map[string]any{"type": "string"},
				"plannedCents":   map[string]any{"type": "integer"},
				"spentCents":     map[string]any{"type": "integer"},
				"availableCents": map[string]any{"type": "integer"},
			},
			"required":             []string{"outcome", "category", "plannedCents", "spentCents", "availableCents"},
			"additionalProperties": false,
		},
	}
	type output struct {
		Outcome        string `json:"outcome"`
		Category       string `json:"category"`
		PlannedCents   int64  `json:"plannedCents"`
		SpentCents     int64  `json:"spentCents"`
		AvailableCents int64  `json:"availableCents"`
	}
	return tool.NewTool[map[string]any, output]("category_detail", "Detalha os lançamentos, planejado, gasto e disponível/excedente de uma categoria do orçamento no mês.", in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			sink("category_detail", in)
			return output{Outcome: "ok", Category: "geral", PlannedCents: 1387440, SpentCents: 25000, AvailableCents: 1362440}, nil
		},
	)
}

func goldenToolsFor(c Case, sink ToolCaptureSink) []tool.ToolHandle {
	names := c.ToolSubset
	if len(names) == 0 {
		names = make([]string, 0, len(goldenToolCatalog))
		for name := range goldenToolCatalog {
			if name == goldenCategoryDetailWithDataKey {
				continue
			}
			names = append(names, name)
		}
	}
	tools := make([]tool.ToolHandle, 0, len(names))
	for _, name := range names {
		factory, ok := goldenToolCatalog[name]
		if !ok {
			continue
		}
		tools = append(tools, factory(sink))
	}
	return tools
}

type GoldenRealLLMSuite struct {
	suite.Suite
	provider llm.Provider
}

func TestGoldenRealLLMSuite(t *testing.T) {
	suite.Run(t, new(GoldenRealLLMSuite))
}

func (s *GoldenRealLLMSuite) SetupSuite() {
	s.provider = buildGoldenHarnessProvider(s.T())
}

func (s *GoldenRealLLMSuite) executorFor(tools []tool.ToolHandle) AgentExecutor {
	provider := s.provider
	return func(ctx context.Context, messages []llm.Message) (agent.Result, error) {
		obs := fake.NewProvider()
		a := agentsapp.BuildMeControlaAgent(provider, tools, nil, obs, 0)
		if len(messages) > 0 && messages[0].Role == "system" {
			messages[0].Content = a.Instructions() + "\n\n" + messages[0].Content
		}
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		return a.Execute(ctx, agent.Request{
			AgentID:   agentsapp.MecontrolaAgentID,
			Messages:  messages,
			MaxTokens: 1024,
		})
	}
}

const goldenRepeatsPerCase = 3

func (s *GoldenRealLLMSuite) TestGoldenSetGate() {
	t := s.T()
	var outcomes []CaseOutcome

	for _, c := range AllCases() {
		if c.Category == CategoryToolError {
			continue
		}
		for attempt := 1; attempt <= goldenRepeatsPerCase; attempt++ {
			s.Run(fmt.Sprintf("%s/%d", c.Name, attempt), func() {
				var captured []CapturedToolCall
				sink := func(name string, args map[string]any) {
					captured = append(captured, CapturedToolCall{Tool: name, Args: args})
				}
				executor := s.executorFor(goldenToolsFor(c, sink))
				outcome := EvaluateCaseWithCapture(context.Background(), executor, c, func() []CapturedToolCall { return captured })
				outcomes = append(outcomes, outcome)
				if !outcome.Passed {
					t.Logf("[%s/%s attempt=%d] FALHOU: %s", c.Category, c.Name, attempt, outcome.Detail)
				}
			})
		}
	}

	results := AggregateByCategory(outcomes)
	failed := false
	for _, r := range results {
		t.Logf("categoria=%s hits=%d total=%d ratio=%.4f", r.Category, r.Hits, r.Total, r.Ratio())
		if !r.PassesGate(goldenGateThreshold) {
			failed = true
			t.Logf("categoria=%s ABAIXO do gate %.2f; falhas=%v", r.Category, goldenGateThreshold, r.Failures)
		}
	}
	require.False(t, failed, "RF-29: uma ou mais categorias do golden set ficaram abaixo do gate %.2f", goldenGateThreshold)
}

func (s *GoldenRealLLMSuite) TestGoldenSetGate_ToolErrorCategory() {
	t := s.T()
	cases := CasesByCategory(CategoryToolError)
	require.NotEmpty(t, cases, "categoria tool_error deve ter ao menos um caso golden")

	var outcomes []CaseOutcome
	for _, c := range cases {
		s.Run(c.Name, func() {
			tools := []tool.ToolHandle{
				goldenFailingTool("register_expense", "Registra uma despesa", goldenBaseSchema("description", "amountCents", "paymentMethod")),
			}
			executor := s.executorFor(tools)
			outcome := EvaluateCase(context.Background(), executor, c)
			outcomes = append(outcomes, outcome)
			if !outcome.Passed {
				t.Logf("[%s/%s] FALHOU: %s", c.Category, c.Name, outcome.Detail)
			}
		})
	}

	results := AggregateByCategory(outcomes)
	for _, r := range results {
		t.Logf("categoria=%s hits=%d total=%d ratio=%.4f", r.Category, r.Hits, r.Total, r.Ratio())
		require.True(t, r.PassesGate(goldenGateThreshold), "RF-29: categoria %s abaixo do gate %.2f; falhas=%v", r.Category, goldenGateThreshold, r.Failures)
	}
}
