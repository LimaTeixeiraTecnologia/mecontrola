package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

var ErrIntentSafetyMissingLookupField = errors.New("agent.llm.safety: missing lookup field")

var ErrIntentSafetyMissingRequiredField = errors.New("agent.llm.safety: missing required field")

var ErrIntentSafetyMissingMutationVerb = errors.New("agent.llm.safety: missing explicit mutation verb")

var ErrIntentSafetyMissingTarget = errors.New("agent.llm.safety: missing mutation target")

var ErrIntentSafetyMissingConfirmation = errors.New("agent.llm.safety: missing destructive confirmation")

var ErrIntentSafetyInvalidOperation = errors.New("agent.llm.safety: invalid operation")

type IntentSafetyError struct {
	reason string
	err    error
}

func (e *IntentSafetyError) Error() string {
	return fmt.Sprintf("agent.llm.safety: %s: %v", e.reason, e.err)
}

func (e *IntentSafetyError) Unwrap() error {
	return e.err
}

func (e *IntentSafetyError) Reason() string {
	return e.reason
}

type IntentSafetyGuard struct{}

func NewIntentSafetyGuard() IntentSafetyGuard {
	return IntentSafetyGuard{}
}

func (g IntentSafetyGuard) Validate(userText string, intent entities.IntentResult) error {
	if intent.IsError() {
		return nil
	}
	if err := g.validateLookups(intent); err != nil {
		return err
	}
	if !intent.Action().IsMutation() {
		return nil
	}
	payload, err := g.decodeObject(intent.Payload())
	if err != nil {
		return err
	}
	normalizedText := strings.ToLower(strings.TrimSpace(userText))
	switch intent.Module().String() {
	case "cards":
		return g.validateCardsMutation(normalizedText, intent.Action().String(), payload)
	case "budgets":
		return g.validateBudgetsMutation(normalizedText, intent.Action().String(), payload)
	case "transactions":
		return g.validateTransactionsMutation(normalizedText, intent.Action().String(), payload)
	default:
		return nil
	}
}

func (g IntentSafetyGuard) validateLookups(intent entities.IntentResult) error {
	switch intent.Module().String() {
	case "cards", "categories", "transactions":
		if intent.Action().String() != "get" {
			return nil
		}
		filters, err := g.decodeObject(intent.Filters())
		if err != nil {
			return err
		}
		if !g.hasNonEmptyString(filters, "id") {
			return g.newSafetyError("missing_lookup_field", ErrIntentSafetyMissingLookupField)
		}
	}
	return nil
}

func (g IntentSafetyGuard) validateCardsMutation(text string, action string, payload map[string]json.RawMessage) error {
	if !g.containsAny(text, "cartao", "cartão", "fatura", "limite") {
		return g.newSafetyError("missing_target", ErrIntentSafetyMissingTarget)
	}
	switch action {
	case "create":
		if !g.containsAny(text, "criar", "cadastrar", "adicionar", "novo") {
			return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
		}
		if !g.hasNonEmptyString(payload, "nickname") || !g.hasIntInRange(payload, "closing_day", 1, 31) || !g.hasIntInRange(payload, "due_day", 1, 31) {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
	case "update":
		if !g.containsAny(text, "alterar", "atualizar", "mudar", "ajustar") {
			return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
		}
		if !g.hasNonEmptyString(payload, "id") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		if !g.hasAnyField(payload, "name", "nickname", "closing_day", "due_day", "limit_cents") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
	case "delete":
		if !g.containsAny(text, "excluir", "apagar", "remover", "deletar") {
			return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
		}
		if !g.containsAny(text, "confirmo", "confirmar", "pode apagar", "pode excluir", "sim, excluir", "sim, apagar") {
			return g.newSafetyError("missing_confirmation", ErrIntentSafetyMissingConfirmation)
		}
		if !g.hasNonEmptyString(payload, "id") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
	}
	return nil
}

func (g IntentSafetyGuard) validateBudgetsMutation(text string, action string, payload map[string]json.RawMessage) error {
	operation := g.stringValue(payload, "operation")
	if operation == "" {
		return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
	}
	switch action {
	case "create":
		return g.validateBudgetCreateOperation(text, operation, payload)
	case "update":
		return g.validateBudgetUpdateOperation(text, operation, payload)
	case "delete":
		return g.validateBudgetDeleteOperation(text, operation, payload)
	}
	return nil
}

func (g IntentSafetyGuard) validateBudgetCreateOperation(text string, operation string, payload map[string]json.RawMessage) error {
	switch operation {
	case "budget":
		if err := g.requireTargetAndVerb(text, []string{"orcamento", "orçamento"}, []string{"criar", "montar", "configurar", "definir", "planejar"}); err != nil {
			return err
		}
		if !g.hasNonEmptyString(payload, "competence") || !g.hasPositiveInt(payload, "total_cents") || !g.hasNonEmptyArray(payload, "allocations") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		return nil
	case "recurrence":
		if err := g.requireTargetAndVerb(text, []string{"orcamento", "orçamento", "recorrencia", "recorrência"}, []string{"copiar", "replicar", "repetir", "recorrer", "recorrencia", "recorrência"}); err != nil {
			return err
		}
		if !g.hasNonEmptyString(payload, "source_competence") || !g.hasPositiveInt(payload, "months") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		return nil
	case "expense":
		if err := g.requireTargetAndVerb(text, []string{"gasto", "despesa", "lancamento", "lançamento"}, []string{"registrar", "lancar", "lançar", "adicionar", "criar"}); err != nil {
			return err
		}
		if !g.hasNonEmptyString(payload, "subcategory_id") || !g.hasPositiveInt(payload, "amount_cents") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		return nil
	default:
		return g.newSafetyError("invalid_operation", ErrIntentSafetyInvalidOperation)
	}
}

func (g IntentSafetyGuard) validateBudgetUpdateOperation(text string, operation string, payload map[string]json.RawMessage) error {
	if operation != "activate_budget" {
		return g.newSafetyError("invalid_operation", ErrIntentSafetyInvalidOperation)
	}
	if err := g.requireTargetAndVerb(text, []string{"orcamento", "orçamento"}, []string{"ativar", "publicar", "liberar"}); err != nil {
		return err
	}
	if !g.hasNonEmptyString(payload, "competence") {
		return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
	}
	return nil
}

func (g IntentSafetyGuard) validateBudgetDeleteOperation(text string, operation string, payload map[string]json.RawMessage) error {
	if !g.containsAny(text, "excluir", "apagar", "remover", "deletar") {
		return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
	}
	if !g.containsAny(text, "confirmo", "confirmar", "pode apagar", "pode excluir", "sim, excluir", "sim, apagar") {
		return g.newSafetyError("missing_confirmation", ErrIntentSafetyMissingConfirmation)
	}
	switch operation {
	case "draft_budget":
		if !g.containsAny(text, "orcamento", "orçamento", "rascunho") {
			return g.newSafetyError("missing_target", ErrIntentSafetyMissingTarget)
		}
		if !g.hasNonEmptyString(payload, "competence") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		return nil
	case "expense":
		if !g.containsAny(text, "gasto", "despesa", "lancamento", "lançamento") {
			return g.newSafetyError("missing_target", ErrIntentSafetyMissingTarget)
		}
		if !g.hasNonEmptyString(payload, "external_transaction_id") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
		return nil
	default:
		return g.newSafetyError("invalid_operation", ErrIntentSafetyInvalidOperation)
	}
}

func (g IntentSafetyGuard) validateTransactionsMutation(text string, action string, payload map[string]json.RawMessage) error {
	if action == "create" {
		if !g.containsAny(text, "registrar", "lancar", "lançar", "adicionar", "criar") {
			return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
		}
		if !g.containsAny(text, "transacao", "transação", "lancamento", "lançamento", "gasto", "despesa", "entrada", "receita") {
			return g.newSafetyError("missing_target", ErrIntentSafetyMissingTarget)
		}
		if !g.hasPositiveNumber(payload, "amount", "amount_cents") || !g.hasAnyStringValue(payload, "type", "direction") || !g.hasNonEmptyString(payload, "category_id") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
	}
	if action == "delete" {
		if !g.containsAny(text, "excluir", "apagar", "remover", "deletar") {
			return g.newSafetyError("missing_mutation_verb", ErrIntentSafetyMissingMutationVerb)
		}
		if !g.containsAny(text, "confirmo", "confirmar", "pode apagar", "pode excluir", "sim, excluir", "sim, apagar") {
			return g.newSafetyError("missing_confirmation", ErrIntentSafetyMissingConfirmation)
		}
		if !g.containsAny(text, "transacao", "transação", "lancamento", "lançamento", "gasto", "despesa", "entrada", "receita") {
			return g.newSafetyError("missing_target", ErrIntentSafetyMissingTarget)
		}
		if !g.hasNonEmptyString(payload, "id") {
			return g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
		}
	}
	return nil
}

func (g IntentSafetyGuard) mutationReason(text string, targets ...string) string {
	if !g.containsAny(text, targets...) {
		return "missing_target"
	}
	return "missing_mutation_verb"
}

func (g IntentSafetyGuard) mutationError(text string, targets ...string) error {
	if !g.containsAny(text, targets...) {
		return ErrIntentSafetyMissingTarget
	}
	return ErrIntentSafetyMissingMutationVerb
}

func (g IntentSafetyGuard) requireTargetAndVerb(text string, targets []string, verbs []string) error {
	if g.containsAny(text, targets...) && g.containsAny(text, verbs...) {
		return nil
	}
	return g.newSafetyError(g.mutationReason(text, targets...), g.mutationError(text, targets...))
}

func (g IntentSafetyGuard) newSafetyError(reason string, err error) error {
	return &IntentSafetyError{reason: reason, err: err}
}

func (g IntentSafetyGuard) decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, g.newSafetyError("missing_required_field", ErrIntentSafetyMissingRequiredField)
	}
	return out, nil
}

func (g IntentSafetyGuard) containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func (g IntentSafetyGuard) hasAnyField(payload map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		if g.hasField(payload, key) {
			return true
		}
	}
	return false
}

func (g IntentSafetyGuard) hasField(payload map[string]json.RawMessage, key string) bool {
	raw, ok := payload[key]
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func (g IntentSafetyGuard) hasNonEmptyString(payload map[string]json.RawMessage, key string) bool {
	return g.stringValue(payload, key) != ""
}

func (g IntentSafetyGuard) hasAnyStringValue(payload map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		if g.stringValue(payload, key) != "" {
			return true
		}
	}
	return false
}

func (g IntentSafetyGuard) stringValue(payload map[string]json.RawMessage, key string) string {
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (g IntentSafetyGuard) hasIntInRange(payload map[string]json.RawMessage, key string, min int64, max int64) bool {
	value, ok := g.int64Value(payload, key)
	return ok && value >= min && value <= max
}

func (g IntentSafetyGuard) hasPositiveInt(payload map[string]json.RawMessage, key string) bool {
	value, ok := g.int64Value(payload, key)
	return ok && value > 0
}

func (g IntentSafetyGuard) hasPositiveNumber(payload map[string]json.RawMessage, primary string, fallback string) bool {
	if value, ok := g.float64Value(payload, primary); ok && value > 0 {
		return true
	}
	value, ok := g.int64Value(payload, fallback)
	return ok && value > 0
}

func (g IntentSafetyGuard) hasNonEmptyArray(payload map[string]json.RawMessage, key string) bool {
	raw, ok := payload[key]
	if !ok {
		return false
	}
	var value []json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return len(value) > 0
}

func (g IntentSafetyGuard) int64Value(payload map[string]json.RawMessage, key string) (int64, bool) {
	raw, ok := payload[key]
	if !ok {
		return 0, false
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, false
	}
	value, err := number.Int64()
	if err != nil {
		return 0, false
	}
	return value, true
}

func (g IntentSafetyGuard) float64Value(payload map[string]json.RawMessage, key string) (float64, bool) {
	raw, ok := payload[key]
	if !ok {
		return 0, false
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, false
	}
	value, err := number.Float64()
	if err != nil {
		return 0, false
	}
	return value, true
}
