package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	TreatmentNameEditWorkflowID  = "treatment-name-edit"
	stepTreatmentNameEditID      = "treatment-name-edit"
	TreatmentNameEditStaleAfter  = 20 * time.Minute
	TreatmentNameEditReaperBatch = 100

	maxTreatmentNameReprompts = 1

	treatmentNameSectionHeading = "## Nome de Tratamento"
	treatmentNameMetadataKey    = "nome_tratamento"
)

func TreatmentNameEditKey(resourceID, threadID string) string {
	return CorrelationKey(resourceID, threadID, TreatmentNameEditWorkflowID)
}

func BuildTreatmentNameEditWorkflow(workingMem memory.WorkingMemory, a agent.Agent) workflow.Definition[TreatmentNameEditState] {
	step := workflow.NewStepFunc(stepTreatmentNameEditID, buildTreatmentNameEditStep(workingMem, a))
	return workflow.Definition[TreatmentNameEditState]{
		ID:          TreatmentNameEditWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildTreatmentNameEditReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, TreatmentNameEditWorkflowID, TreatmentNameEditStaleAfter, TreatmentNameEditReaperBatch, o11y)
}

var treatmentNameExtractSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"hasName": map[string]any{"type": "boolean"},
		"name":    map[string]any{"type": "string"},
	},
	"required":             []any{"hasName", "name"},
	"additionalProperties": false,
}

type treatmentNameEditExtract struct {
	HasName bool   `json:"hasName"`
	Name    string `json:"name"`
}

const treatmentNameEditExtractSystemPrompt = "Extraia o nome ou apelido pelo qual o usuário deseja ser chamado a partir da mensagem a seguir. " +
	"Se a mensagem não trouxer um nome claro, retorne hasName=false."

func treatmentNameEditSuspend(state TreatmentNameEditState, prompt string) (workflow.StepOutput[TreatmentNameEditState], error) {
	state.ResponseText = prompt
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[TreatmentNameEditState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}, nil
}

func treatmentNameEditComplete(state TreatmentNameEditState) (workflow.StepOutput[TreatmentNameEditState], error) {
	return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func treatmentNameEditExpireStep(state TreatmentNameEditState) (workflow.StepOutput[TreatmentNameEditState], error) {
	state.Status = TreatmentNameEditExpired
	state.Expired = true
	state.ResponseText = ""
	state.ResumeText = ""
	return treatmentNameEditComplete(state)
}

func treatmentNameEditReprompt() string {
	return "Não entendi. Como você gostaria que eu te chamasse? 💚"
}

func treatmentNameEditCancelMessage() string {
	return "🚫 Tudo bem, vou continuar te chamando como antes."
}

func buildTreatmentNameEditStep(workingMem memory.WorkingMemory, a agent.Agent) func(context.Context, TreatmentNameEditState) (workflow.StepOutput[TreatmentNameEditState], error) {
	return func(ctx context.Context, state TreatmentNameEditState) (workflow.StepOutput[TreatmentNameEditState], error) {
		if newName, ok := DecideTreatmentName(state.ProvidedName != "", state.ProvidedName); ok {
			state.NewName = newName
			return executeTreatmentNameEdit(ctx, state, workingMem)
		}

		if state.ResumeText == "" {
			if DecideTreatmentNameTooLong(state.ProvidedName != "", state.ProvidedName) {
				return treatmentNameEditSuspend(state, messages.TreatmentNameTooLong())
			}
			return treatmentNameEditSuspend(state, messages.TreatmentNameEditQuestion())
		}

		if DecideTreatmentNameEditExpiry(state, time.Now().UTC()) {
			return treatmentNameEditExpireStep(state)
		}

		resumeText := state.ResumeText
		state.ResumeText = ""

		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: treatmentNameEditExtractSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "treatment_name_edit_extract", Strict: true, Schema: treatmentNameExtractSchema},
		})
		if err != nil {
			return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.treatment_name_edit: parse: %w", err)
		}

		var extract treatmentNameEditExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.treatment_name_edit: unmarshal: %w", err)
		}

		newName, ok := DecideTreatmentName(extract.HasName, extract.Name)
		if !ok {
			if state.RepromptCount >= maxTreatmentNameReprompts {
				state.Status = TreatmentNameEditCancelled
				state.ResponseText = treatmentNameEditCancelMessage()
				return treatmentNameEditComplete(state)
			}
			state.RepromptCount++
			if DecideTreatmentNameTooLong(extract.HasName, extract.Name) {
				return treatmentNameEditSuspend(state, messages.TreatmentNameTooLong())
			}
			return treatmentNameEditSuspend(state, treatmentNameEditReprompt())
		}

		state.NewName = newName
		return executeTreatmentNameEdit(ctx, state, workingMem)
	}
}

func executeTreatmentNameEdit(ctx context.Context, state TreatmentNameEditState, workingMem memory.WorkingMemory) (workflow.StepOutput[TreatmentNameEditState], error) {
	content, err := workingMem.Get(ctx, state.ResourceID)
	if err != nil {
		state.ResponseText = "Não consegui atualizar seu nome de tratamento. Tente novamente em breve."
		return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.treatment_name_edit: get_working_memory: %w", err)
	}

	if err := workingMem.UpsertMetadata(ctx, state.ResourceID, map[string]any{treatmentNameMetadataKey: state.NewName}); err != nil {
		state.ResponseText = "Não consegui atualizar seu nome de tratamento. Tente novamente em breve."
		return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.treatment_name_edit: upsert_metadata: %w", err)
	}

	updated := replaceWorkingMemorySection(content, treatmentNameSectionHeading, state.NewName)
	if err := workingMem.Upsert(ctx, state.ResourceID, updated); err != nil {
		state.ResponseText = "Não consegui atualizar seu nome de tratamento. Tente novamente em breve."
		return workflow.StepOutput[TreatmentNameEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.treatment_name_edit: upsert_working_memory: %w", err)
	}

	state.Status = TreatmentNameEditCompleted
	state.ResponseText = messages.TreatmentNameConfirmation(state.NewName)
	return treatmentNameEditComplete(state)
}

func ContinueTreatmentNameEdit(
	ctx context.Context,
	engine workflow.Engine[TreatmentNameEditState],
	def workflow.Definition[TreatmentNameEditState],
	key string,
	userMessage string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage})
	if err != nil {
		return false, "", fmt.Errorf("workflows.treatment_name_edit: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.treatment_name_edit: resume: %w", resumeErr)
	}

	if result.State.Expired {
		return false, "", nil
	}

	return true, result.State.ResponseText, nil
}
