package postdeploy

var RegisteredTools = []string{
	"register_expense",
	"register_income",
	"query_month",
	"query_plan",
	"edit_entry",
	"delete_entry",
	"adjust_allocation",
	"classify_category",
	"update_recurrence",
	"delete_recurrence",
	"update_card",
	"list_cards",
	"get_card",
	"resolve_card",
	"count_cards",
	"best_purchase_day",
	"query_card_invoice",
	"get_transaction",
	"search_transactions",
	"list_recurrences",
	"create_recurrence",
	"suggest_allocation",
	"list_categories",
	"create_card",
	"create_budget",
}

var RegisteredWorkflows = []string{
	"budget-creation",
	"card-create-confirm",
	"destructive-confirm",
	"onboarding-workflow",
	"pending-entry",
}

var RegisteredScorers = []string{
	"tool-call-accuracy",
	"completeness",
	"categorization",
	"no_empty_answer",
	"whatsapp_format",
	"no_internal_terms",
	"verbatim_required",
	"no_duplicate_write",
	"no_hallucination",
	"required_args",
	"month_reference_correctness",
}

var CoveredExistingFlows = []string{
	"registro_despesa",
	"registro_receita",
	"consulta_mensal",
	"orcamento",
	"fatura",
	"ultima_transacao",
	"busca_transacoes",
	"cartoes",
	"recorrencias",
	"categorias",
	"onboarding",
	"pendencias",
	"confirmacao_destrutiva",
	"criacao_cartao",
	"criacao_orcamento",
	"memoria",
	"scorers",
	"entrega_whatsapp",
}

func MissingFrom(expected, actual []string) []string {
	actualSet := make(map[string]bool, len(actual))
	for _, a := range actual {
		actualSet[a] = true
	}
	var missing []string
	for _, e := range expected {
		if !actualSet[e] {
			missing = append(missing, e)
		}
	}
	return missing
}

func ExtraIn(expected, actual []string) []string {
	expectedSet := make(map[string]bool, len(expected))
	for _, e := range expected {
		expectedSet[e] = true
	}
	var extra []string
	for _, a := range actual {
		if !expectedSet[a] {
			extra = append(extra, a)
		}
	}
	return extra
}
