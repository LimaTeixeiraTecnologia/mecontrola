package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	GoalEditWorkflowID  = "goal-edit"
	stepGoalEditID      = "goal-edit"
	GoalEditStaleAfter  = 20 * time.Minute
	GoalEditReaperBatch = 100

	goalEditSectionHeading = "## Objetivo Financeiro"
	goalEditMetadataKey    = "objetivo_financeiro"
)

func GoalEditKey(resourceID, threadID string) string {
	return CorrelationKey(resourceID, threadID, GoalEditWorkflowID)
}

func BuildGoalEditWorkflow(workingMem memory.WorkingMemory) workflow.Definition[GoalEditState] {
	step := workflow.NewStepFunc(stepGoalEditID, buildGoalEditStep(workingMem))
	return workflow.Definition[GoalEditState]{
		ID:          GoalEditWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildGoalEditReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, GoalEditWorkflowID, GoalEditStaleAfter, GoalEditReaperBatch, o11y)
}

func buildGoalEditStep(workingMem memory.WorkingMemory) func(context.Context, GoalEditState) (workflow.StepOutput[GoalEditState], error) {
	return func(ctx context.Context, state GoalEditState) (workflow.StepOutput[GoalEditState], error) {
		switch state.Awaiting {
		case GoalEditAwaitingConfirm:
			return handleGoalEditConfirmSlot(ctx, state, workingMem)
		default:
			return handleGoalEditGoalSlot(ctx, state, workingMem)
		}
	}
}

func goalEditSuspend(state GoalEditState, prompt string) (workflow.StepOutput[GoalEditState], error) {
	state.ResponseText = prompt
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[GoalEditState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}, nil
}

func goalEditComplete(state GoalEditState) (workflow.StepOutput[GoalEditState], error) {
	return workflow.StepOutput[GoalEditState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func goalEditExpireStep(state GoalEditState) (workflow.StepOutput[GoalEditState], error) {
	state.Status = GoalEditExpired
	state.Expired = true
	state.ResponseText = ""
	state.ResumeText = ""
	return goalEditComplete(state)
}

func goalEditGoalPrompt(previous string) string {
	if previous == "" {
		return "Qual é o seu objetivo financeiro? 🎯"
	}
	return fmt.Sprintf("Seu objetivo financeiro atual é: \"%s\"\n\nQual é o novo objetivo? 🎯", previous)
}

func goalEditGoalReprompt() string {
	return "Não entendi. Me conta em poucas palavras qual é o seu novo objetivo financeiro? 🎯"
}

func handleGoalEditGoalSlot(ctx context.Context, state GoalEditState, workingMem memory.WorkingMemory) (workflow.StepOutput[GoalEditState], error) {
	if state.PreviousGoal == "" && state.ResumeText == "" && state.SuspendedAt.IsZero() {
		content, err := workingMem.Get(ctx, state.ResourceID)
		if err != nil {
			return workflow.StepOutput[GoalEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.goal_edit.goal: get_working_memory: %w", err)
		}
		state.PreviousGoal = goalEditSectionBody(content, goalEditSectionHeading)
	}

	if state.ResumeText == "" {
		state.Awaiting = GoalEditAwaitingGoal
		return goalEditSuspend(state, goalEditGoalPrompt(state.PreviousGoal))
	}

	if isGoalEditExpired(state, time.Now().UTC()) {
		return goalEditExpireStep(state)
	}

	resumeText := state.ResumeText
	state.ResumeText = ""

	newGoal, ok := DecideGoalEditNewGoal(resumeText)
	if !ok {
		state.RepromptCount++
		return goalEditSuspend(state, goalEditGoalReprompt())
	}

	state.NewGoal = newGoal
	state.Awaiting = GoalEditAwaitingConfirm
	return goalEditConfirmSuspend(state)
}

func goalEditConfirmSuspend(state GoalEditState) (workflow.StepOutput[GoalEditState], error) {
	return goalEditSuspend(state, goalEditConfirmPrompt(state))
}

func goalEditConfirmPrompt(state GoalEditState) string {
	var b strings.Builder
	b.WriteString("Vamos revisar seu novo objetivo financeiro:\n\n")
	fmt.Fprintf(&b, "🎯 Novo objetivo: %s\n\n", state.NewGoal)
	b.WriteString("Posso atualizar? Responda \"sim\" para confirmar ou \"não\" para cancelar.")
	return b.String()
}

func handleGoalEditConfirmSlot(ctx context.Context, state GoalEditState, workingMem memory.WorkingMemory) (workflow.StepOutput[GoalEditState], error) {
	if state.ResumeText == "" {
		return goalEditConfirmSuspend(state)
	}

	msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
	now := time.Now().UTC()
	action := DecideGoalEditConfirmation(state, msg, now)
	state.ResumeText = ""

	switch action {
	case GoalEditActionAccept:
		state.MessageID = state.IncomingMessageID
		return executeGoalEdit(ctx, state, workingMem)
	case GoalEditActionCancel:
		state.Status = GoalEditCancelled
		state.ResponseText = "🚫 Alteração de objetivo cancelada conforme solicitado."
		return goalEditComplete(state)
	case GoalEditActionExpire:
		state.Status = GoalEditExpired
		state.Expired = true
		state.ResponseText = ""
		return goalEditComplete(state)
	case GoalEditActionReplay:
		state.Status = GoalEditCompleted
		return goalEditComplete(state)
	case GoalEditActionReprompt:
		state.RepromptCount++
		state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar."
		return goalEditSuspend(state, state.ResponseText)
	default:
		state.Status = GoalEditCancelled
		state.ResponseText = "🚫 Alteração de objetivo cancelada: resposta não reconhecida."
		return goalEditComplete(state)
	}
}

func executeGoalEdit(ctx context.Context, state GoalEditState, workingMem memory.WorkingMemory) (workflow.StepOutput[GoalEditState], error) {
	content, err := workingMem.Get(ctx, state.ResourceID)
	if err != nil {
		return workflow.StepOutput[GoalEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.goal_edit.confirm: get_working_memory: %w", err)
	}

	updated := goalEditReplaceSection(content, goalEditSectionHeading, state.NewGoal)
	if err := workingMem.Upsert(ctx, state.ResourceID, updated); err != nil {
		state.ResponseText = "Não consegui atualizar seu objetivo. Tente novamente em breve."
		return workflow.StepOutput[GoalEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.goal_edit.confirm: upsert_working_memory: %w", err)
	}

	if err := workingMem.UpsertMetadata(ctx, state.ResourceID, map[string]any{goalEditMetadataKey: state.NewGoal}); err != nil {
		return workflow.StepOutput[GoalEditState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.goal_edit.confirm: upsert_metadata: %w", err)
	}

	state.Status = GoalEditCompleted
	seed := messages.NewMotivationSeed(state.MessageID)
	state.ResponseText = fmt.Sprintf("✅ Objetivo atualizado com sucesso!\n\n%s", messages.GoalEditMotivation(seed))
	return goalEditComplete(state)
}

func goalEditSectionBody(content, heading string) string {
	sections := goalEditParseSections(content)
	for _, sec := range sections {
		if sec.heading == heading {
			return sec.body
		}
	}
	return ""
}

type goalEditSection struct {
	heading string
	body    string
}

func goalEditParseSections(content string) []goalEditSection {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	var sections []goalEditSection
	current := &goalEditSection{}
	var bodyLines []string
	flush := func() {
		if current == nil {
			return
		}
		current.body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		if current.heading == "" && current.body == "" {
			return
		}
		sections = append(sections, *current)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = &goalEditSection{heading: strings.TrimSpace(line)}
			bodyLines = nil
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	flush()
	return sections
}

func goalEditReplaceSection(content, heading, newBody string) string {
	sections := goalEditParseSections(content)
	found := false
	for i := range sections {
		if sections[i].heading == heading {
			sections[i].body = newBody
			found = true
			break
		}
	}
	if !found {
		sections = append(sections, goalEditSection{heading: heading, body: newBody})
	}

	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if sec.heading == "" {
			b.WriteString(sec.body)
			continue
		}
		b.WriteString(sec.heading)
		if sec.body != "" {
			b.WriteString("\n\n")
			b.WriteString(sec.body)
		}
	}
	return b.String()
}

func ContinueGoalEdit(
	ctx context.Context,
	engine workflow.Engine[GoalEditState],
	def workflow.Definition[GoalEditState],
	key string,
	userMessage string,
	messageID string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage, "incomingMessageId": messageID})
	if err != nil {
		return false, "", fmt.Errorf("workflows.goal_edit: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.goal_edit: resume: %w", resumeErr)
	}

	if result.State.Expired {
		return false, "", nil
	}

	return true, result.State.ResponseText, nil
}
