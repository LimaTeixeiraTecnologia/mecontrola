package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

var ErrValidatorForbiddenField = errors.New("agent.llm.validator: forbidden field present in LLM output")

var ErrValidatorInvalidJSON = errors.New("agent.llm.validator: payload or filters is not valid JSON object")

var ErrValidatorEmptyOutput = errors.New("agent.llm.validator: LLM output is empty")

var forbiddenKeys = []string{
	"user_id", "userid", "userId", "user-id", "user.id",
	"tenant_id", "tenantid", "tenantId", "tenant-id", "tenant.id",
	"principal_id", "principalid", "principalId", "principal-id",
	"customer_id", "customerid", "customerId", "customer-id",
}

type IntentValidator struct{}

func NewIntentValidator() IntentValidator {
	return IntentValidator{}
}

func (v IntentValidator) Validate(raw []byte) (entities.IntentResult, error) {
	if len(raw) == 0 {
		return entities.IntentResult{}, ErrValidatorEmptyOutput
	}

	cleaned := stripJSONFences(raw)

	var rawIntent entities.RawIntent
	if err := json.Unmarshal(cleaned, &rawIntent); err != nil {
		return entities.IntentResult{}, fmt.Errorf("agent.llm.validator: unmarshal: %w", err)
	}

	if rawIntent.Error != "" {
		return entities.NewIntentResultFromError(entities.IntentError{
			Code:    strings.TrimSpace(rawIntent.Error),
			Message: strings.TrimSpace(rawIntent.Message),
		}), nil
	}

	module, err := valueobjects.NewIntentModule(rawIntent.Module)
	if err != nil {
		return entities.IntentResult{}, fmt.Errorf("agent.llm.validator: module: %w", err)
	}
	action, err := valueobjects.NewIntentAction(rawIntent.Action)
	if err != nil {
		return entities.IntentResult{}, fmt.Errorf("agent.llm.validator: action: %w", err)
	}

	if err := checkForbiddenKeys(rawIntent.Payload); err != nil {
		return entities.IntentResult{}, fmt.Errorf("agent.llm.validator: payload: %w", err)
	}
	if err := checkForbiddenKeys(rawIntent.Filters); err != nil {
		return entities.IntentResult{}, fmt.Errorf("agent.llm.validator: filters: %w", err)
	}

	return entities.NewIntentResult(module, action, rawIntent.Filters, rawIntent.Payload, rawIntent.ResponseHint)
}

func stripJSONFences(raw []byte) []byte {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "```") {
		return []byte(trimmed)
	}
	cleaned := strings.TrimPrefix(trimmed, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return []byte(strings.TrimSpace(cleaned))
}

func checkForbiddenKeys(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return fmt.Errorf("%w: %v", ErrValidatorInvalidJSON, err)
		}
		for key, value := range obj {
			normalized := strings.ToLower(strings.TrimSpace(key))
			for _, forbidden := range forbiddenKeys {
				if normalized == strings.ToLower(forbidden) {
					return fmt.Errorf("agent.llm.validator: %q: %w", key, ErrValidatorForbiddenField)
				}
			}
			if nestedErr := checkForbiddenKeys(value); nestedErr != nil {
				return nestedErr
			}
		}
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return fmt.Errorf("%w: %v", ErrValidatorInvalidJSON, err)
		}
		for _, item := range arr {
			if itemErr := checkForbiddenKeys(item); itemErr != nil {
				return itemErr
			}
		}
		return nil
	}
	return nil
}
