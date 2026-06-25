package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PlanInput struct {
	UserID       uuid.UUID
	Channel      string
	MessageID    string
	Text         string
	LLMModel     string
	PromptSHA256 string
	DirectReply  string
	RawResponse  string
	Plan         PlanSteps
}

type PlanSteps struct {
	Steps []PlanStepItem
}

type PlanStepItem struct {
	Intent     intent.Intent
	Confidence float64
	Index      int
}

type PlanDispatchInput struct {
	UserID       uuid.UUID
	Channel      string
	MessageID    string
	Text         string
	StepIndex    int
	Intent       intent.Intent
	Confidence   float64
	LLMModel     string
	PromptSHA256 string
	DirectReply  string
	RawResponse  string
	ResumeText   string
	Resuming     bool
}

type PlanStepDispatcher func(ctx context.Context, in PlanDispatchInput) (tools.ToolResult, error)

type PlanExecutor struct {
	engine     platform.Engine[PlanState]
	dispatcher PlanStepDispatcher
	o11y       observability.Observability
	stepsTotal observability.Counter
}

func NewPlanExecutor(engine platform.Engine[PlanState], dispatcher PlanStepDispatcher, o11y observability.Observability) (*PlanExecutor, error) {
	if engine == nil {
		return nil, fmt.Errorf("agent.workflow.plan_executor: engine is nil")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("agent.workflow.plan_executor: dispatcher is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.workflow.plan_executor: o11y is nil")
	}
	stepsTotal := o11y.Metrics().Counter("agent_plan_steps_total", "Total plan steps executed", "1")
	return &PlanExecutor{
		engine:     engine,
		dispatcher: dispatcher,
		o11y:       o11y,
		stepsTotal: stepsTotal,
	}, nil
}

func (pe *PlanExecutor) buildDefinition(durable bool) platform.Definition[PlanState] {
	return platform.Definition[PlanState]{
		ID:      "plan_executor",
		Durable: durable,
		Root:    pe.buildStep(),
	}
}

func (pe *PlanExecutor) buildStep() platform.Step[PlanState] {
	return platform.NewStepFunc("plan_execute", func(ctx context.Context, state PlanState) (platform.StepOutput[PlanState], error) {
		resumeCursor := state.Cursor
		resumeText := state.ResumeText
		state.ResumeText = ""
		for state.Cursor < len(state.Steps) {
			step := state.Steps[state.Cursor]
			in, err := deserializePlanStep(step)
			if err != nil {
				state.Failed = true
				return platform.StepOutput[PlanState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("agent.workflow.plan_executor: deserialize step %d: %w", state.Cursor, err)
			}
			userID, parseErr := uuid.Parse(state.UserID)
			if parseErr != nil {
				state.Failed = true
				return platform.StepOutput[PlanState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("agent.workflow.plan_executor: parse user_id: %w", parseErr)
			}
			resuming := resumeText != "" && state.Cursor == resumeCursor
			dispatch := PlanDispatchInput{
				UserID:       userID,
				Channel:      state.Channel,
				MessageID:    state.MessageID,
				Text:         state.Text,
				StepIndex:    step.Index,
				Intent:       in,
				Confidence:   step.Confidence,
				LLMModel:     state.LLMModel,
				PromptSHA256: state.PromptSHA256,
				DirectReply:  state.DirectReply,
				RawResponse:  state.RawResponse,
				Resuming:     resuming,
			}
			if resuming {
				dispatch.ResumeText = resumeText
			}
			result, dispatchErr := pe.dispatcher(ctx, dispatch)
			if dispatchErr != nil {
				pe.stepsTotal.Add(ctx, 1, observability.String("outcome", "error"))
				state.Failed = true
				return platform.StepOutput[PlanState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("agent.workflow.plan_executor: step %d dispatch: %w", state.Cursor, dispatchErr)
			}
			pe.stepsTotal.Add(ctx, 1, observability.String("outcome", result.Outcome.String()))
			switch planStepDispositionFor(result.Outcome) {
			case planStepSuspend:
				return platform.StepOutput[PlanState]{
					State:   state,
					Status:  platform.StepStatusSuspended,
					Suspend: &platform.Suspension{Reason: platform.SuspendAwaitingInput, Prompt: result.Reply},
				}, nil
			case planStepShortCircuit:
				state.Failed = true
				if result.Reply != "" {
					state.Replies = append(state.Replies, result.Reply)
				}
				return platform.StepOutput[PlanState]{State: state, Status: platform.StepStatusFailed}, nil
			default:
				if result.Reply != "" {
					state.Replies = append(state.Replies, result.Reply)
				}
				state.Cursor++
			}
		}
		return platform.StepOutput[PlanState]{State: state, Status: platform.StepStatusCompleted}, nil
	})
}

func (pe *PlanExecutor) Execute(ctx context.Context, in PlanInput) (tools.ToolResult, error) {
	if len(in.Plan.Steps) == 0 {
		return tools.ToolResult{}, fmt.Errorf("agent.workflow.plan_executor: empty plan")
	}
	serialized := make([]PlanStepSerialized, len(in.Plan.Steps))
	for i, step := range in.Plan.Steps {
		serialized[i] = serializePlanStep(step.Intent, step.Confidence, step.Index)
	}
	initial := PlanState{
		UserID:       in.UserID.String(),
		Channel:      in.Channel,
		MessageID:    in.MessageID,
		Text:         in.Text,
		LLMModel:     in.LLMModel,
		PromptSHA256: in.PromptSHA256,
		DirectReply:  in.DirectReply,
		RawResponse:  in.RawResponse,
		Steps:        serialized,
		Cursor:       0,
		Replies:      nil,
	}
	hasWrite := false
	for _, step := range in.Plan.Steps {
		if step.Intent.Kind().IsWrite() {
			hasWrite = true
			break
		}
	}
	def := pe.buildDefinition(hasWrite)
	result, err := pe.engine.Start(ctx, def, planCorrelationKey(in.UserID.String(), in.Channel), initial)
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("agent.workflow.plan_executor: engine start: %w", err)
	}
	return planResultToToolResult(result), nil
}

func (pe *PlanExecutor) Resume(ctx context.Context, userID uuid.UUID, channel, resumeText string) (tools.ToolResult, bool, error) {
	def := pe.buildDefinition(true)
	resumeBytes, err := json.Marshal(struct {
		ResumeText string `json:"resume_text"`
	}{ResumeText: resumeText})
	if err != nil {
		return tools.ToolResult{}, false, fmt.Errorf("agent.workflow.plan_executor: encode resume: %w", err)
	}
	result, err := pe.engine.Resume(ctx, def, planCorrelationKey(userID.String(), channel), resumeBytes)
	if err != nil {
		return tools.ToolResult{}, false, fmt.Errorf("agent.workflow.plan_executor: engine resume: %w", err)
	}
	if result.Status == platform.RunStatusRunning || result.RunID == uuid.Nil {
		return tools.ToolResult{}, false, nil
	}
	return planResultToToolResult(result), true, nil
}

func planCorrelationKey(userID, channel string) string {
	return fmt.Sprintf("plan:%s:%s", userID, channel)
}

func planResultToToolResult(result platform.RunResult[PlanState]) tools.ToolResult {
	if result.Status == platform.RunStatusSuspended && result.Suspend != nil {
		return tools.ToolResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify}
	}
	reply := joinReplies(result.State.Replies)
	if result.State.Failed {
		return tools.ToolResult{Reply: reply, Outcome: tools.OutcomeUsecaseError}
	}
	return tools.ToolResult{Reply: reply, Outcome: tools.OutcomeRouted}
}
