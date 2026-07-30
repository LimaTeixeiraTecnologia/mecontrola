package guards

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubListRecurrencesTool struct {
	invoked    bool
	candidates []recurrenceManageCandidate
}

func (s *stubListRecurrencesTool) ID() string                 { return "list_recurrences" }
func (s *stubListRecurrencesTool) Description() string        { return "" }
func (s *stubListRecurrencesTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubListRecurrencesTool) Invoke(_ context.Context, _ []byte) ([]byte, string, error) {
	s.invoked = true
	payload := struct {
		Recurrences []recurrenceManageCandidate `json:"recurrences"`
	}{Recurrences: s.candidates}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return raw, "", nil
}

func userCandidates() []recurrenceManageCandidate {
	return []recurrenceManageCandidate{
		{ID: "2dc23397-5551-494a-8085-854fa96858a3", Description: "aluguel", DayOfMonth: 5, Version: 1},
		{ID: "a83be4e3-cb07-435e-96fc-a53ded2ef65b", Description: "salário", DayOfMonth: 5, Version: 1},
		{ID: "047884cc-bc8a-445a-8290-0015378b5348", Description: "salário", DayOfMonth: 31, Version: 1},
		{ID: "f7e70454-3662-472a-bacb-a2932682b5bb", Description: "pensão", DayOfMonth: 30, Version: 1},
		{ID: "6de625e6-33d0-4043-b1f7-e7c5ac9f886d", Description: "internet", DayOfMonth: 10, Version: 1},
	}
}

func TestRecurrenceManageShortcutGuardIncidentPhrases(t *testing.T) {
	scenarios := []struct {
		name         string
		input        string
		wantTool     string
		wantTemplate string
		wantVersion  int64
		wantExtra    map[string]any
	}{
		{
			name:         "muda o aluguel para 1600",
			input:        "muda o aluguel para 1600",
			wantTool:     "update_recurrence",
			wantTemplate: "2dc23397-5551-494a-8085-854fa96858a3",
			wantVersion:  1,
			wantExtra:    map[string]any{"amountCents": float64(160000)},
		},
		{
			name:         "cancela a recorrência da pensão",
			input:        "cancela a recorrência da pensão",
			wantTool:     "delete_recurrence",
			wantTemplate: "f7e70454-3662-472a-bacb-a2932682b5bb",
			wantVersion:  1,
		},
		{
			name:         "cancela a recorrência do salário do dia 5",
			input:        "cancela a recorrência do salário do dia 5",
			wantTool:     "delete_recurrence",
			wantTemplate: "a83be4e3-cb07-435e-96fc-a53ded2ef65b",
			wantVersion:  1,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			list := &stubListRecurrencesTool{candidates: userCandidates()}
			update := &stubRecurrenceTool{id: "update_recurrence", message: "confirma atualizacao"}
			del := &stubRecurrenceTool{id: "delete_recurrence", message: "confirma exclusao"}
			guard := NewRecurrenceManageShortcutGuard(update, del, list, nil)

			decision := guard.Inspect(context.Background(), agent.Request{
				Messages: []llm.Message{{Role: "user", Content: scenario.input}},
			})

			if !decision.Handled {
				t.Fatalf("esperava short-circuit determinístico para %q", scenario.input)
			}
			if !list.invoked {
				t.Fatal("esperava invocação de list_recurrences")
			}

			var invoked *stubRecurrenceTool
			switch scenario.wantTool {
			case "update_recurrence":
				invoked = update
			case "delete_recurrence":
				invoked = del
			}
			if !invoked.invoked {
				t.Fatalf("esperava invocação de %s", scenario.wantTool)
			}

			var args map[string]any
			if err := json.Unmarshal(invoked.args, &args); err != nil {
				t.Fatalf("unmarshal args: %v", err)
			}
			if args["templateId"] != scenario.wantTemplate {
				t.Fatalf("templateId = %v; want %v", args["templateId"], scenario.wantTemplate)
			}
			if args["version"] != float64(scenario.wantVersion) {
				t.Fatalf("version = %v; want %v", args["version"], scenario.wantVersion)
			}
			for k, v := range scenario.wantExtra {
				if args[k] != v {
					t.Fatalf("%s = %v; want %v", k, args[k], v)
				}
			}

			if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != scenario.wantTool {
				t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
			}
			if decision.Result.ToolOutcome != agent.ToolOutcomeClarify {
				t.Fatalf("tool outcome = %v; want clarify", decision.Result.ToolOutcome)
			}
		})
	}
}

func TestRecurrenceManageShortcutGuardNoCandidateDelegatesToLLM(t *testing.T) {
	list := &stubListRecurrencesTool{candidates: userCandidates()}
	update := &stubRecurrenceTool{id: "update_recurrence"}
	del := &stubRecurrenceTool{id: "delete_recurrence"}
	guard := NewRecurrenceManageShortcutGuard(update, del, list, nil)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "cancela a recorrência da academia"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar termo sem candidato correspondente")
	}
	if update.invoked || del.invoked {
		t.Fatal("não deveria invocar tool de escrita sem candidato resolvido")
	}
}

func TestRecurrenceManageShortcutGuardAmbiguousCandidateDelegatesToLLM(t *testing.T) {
	list := &stubListRecurrencesTool{candidates: userCandidates()}
	del := &stubRecurrenceTool{id: "delete_recurrence"}
	guard := NewRecurrenceManageShortcutGuard(nil, del, list, nil)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "cancela a recorrência do salário"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar termo ambíguo (duas recorrências de salário) sem o dia")
	}
	if del.invoked {
		t.Fatal("não deveria invocar delete_recurrence com candidato ambíguo")
	}
}

func TestRecurrenceManageShortcutGuardRecordsUnresolvedMetric(t *testing.T) {
	scenarios := []struct {
		name        string
		input       string
		wantOutcome string
	}{
		{name: "sem candidato", input: "cancela a recorrência da academia", wantOutcome: recurrenceManageResolveOutcomeNoMatch},
		{name: "candidato ambiguo", input: "cancela a recorrência do salário", wantOutcome: recurrenceManageResolveOutcomeAmbiguous},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			obs := fake.NewProvider()
			list := &stubListRecurrencesTool{candidates: userCandidates()}
			del := &stubRecurrenceTool{id: "delete_recurrence"}
			guard := NewRecurrenceManageShortcutGuard(nil, del, list, obs)

			decision := guard.Inspect(context.Background(), agent.Request{
				Messages: []llm.Message{{Role: "user", Content: scenario.input}},
			})
			if decision.Handled {
				t.Fatal("não deveria interceptar cenário sem candidato único")
			}

			metrics, ok := obs.Metrics().(*fake.FakeMetrics)
			if !ok {
				t.Fatal("esperava *fake.FakeMetrics")
			}
			counter := metrics.GetCounter("agent_recurrence_manage_shortcut_unresolved_total")
			if counter == nil {
				t.Fatal("esperava métrica agent_recurrence_manage_shortcut_unresolved_total registrada")
			}
			values := counter.GetValues()
			if len(values) != 1 {
				t.Fatalf("esperava 1 incremento; obteve %d", len(values))
			}
			var outcome string
			for _, f := range values[0].Fields {
				if f.Key == "outcome" {
					outcome = f.StringValue()
				}
			}
			if outcome != scenario.wantOutcome {
				t.Fatalf("outcome = %v; want %v", outcome, scenario.wantOutcome)
			}
		})
	}
}

func TestRecurrenceManageShortcutGuardBlockedByOtherDomainWord(t *testing.T) {
	list := &stubListRecurrencesTool{candidates: userCandidates()}
	update := &stubRecurrenceTool{id: "update_recurrence"}
	guard := NewRecurrenceManageShortcutGuard(update, nil, list, nil)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "muda o lançamento do aluguel para 1600"}},
	})

	if decision.Handled {
		t.Fatal("frase citando lançamento sem a palavra recorrência não deveria disparar o guard")
	}
	if list.invoked {
		t.Fatal("não deveria chamar list_recurrences quando bloqueado por palavra de outro domínio")
	}
}

func TestRecurrenceManageShortcutGuardIgnoresUnrelatedExpense(t *testing.T) {
	list := &stubListRecurrencesTool{candidates: userCandidates()}
	update := &stubRecurrenceTool{id: "update_recurrence"}
	guard := NewRecurrenceManageShortcutGuard(update, nil, list, nil)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "gastei 30 no mercado"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar despesa avulsa sem verbo de edição/cancelamento de recorrência")
	}
	if list.invoked || update.invoked {
		t.Fatal("não deveria chamar nenhuma tool para despesa avulsa")
	}
}

func TestResolveRecurrenceManageCandidate(t *testing.T) {
	candidates := userCandidates()

	scenarios := []struct {
		name   string
		term   string
		wantOK bool
		wantID string
	}{
		{name: "match unico por substring", term: "aluguel", wantOK: true, wantID: "2dc23397-5551-494a-8085-854fa96858a3"},
		{name: "match com acento normalizado", term: "pensao", wantOK: true, wantID: "f7e70454-3662-472a-bacb-a2932682b5bb"},
		{name: "match desambiguado por dia", term: "salário do dia 31", wantOK: true, wantID: "047884cc-bc8a-445a-8290-0015378b5348"},
		{name: "sem candidato", term: "academia", wantOK: false},
		{name: "ambiguo sem dia", term: "salário", wantOK: false},
		{name: "termo vazio", term: "", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got, _, ok := resolveRecurrenceManageCandidate(candidates, scenario.term)
			if ok != scenario.wantOK {
				t.Fatalf("resolveRecurrenceManageCandidate(%q) ok = %v; want %v", scenario.term, ok, scenario.wantOK)
			}
			if scenario.wantOK && got.ID != scenario.wantID {
				t.Fatalf("id = %v; want %v", got.ID, scenario.wantID)
			}
		})
	}
}

func TestNormalizeRecurrenceManageTerm(t *testing.T) {
	scenarios := []struct {
		input string
		want  string
	}{
		{input: "Pensão", want: "pensao"},
		{input: "  Salário   do dia 5  ", want: "salario do dia 5"},
		{input: "ALUGUEL", want: "aluguel"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.input, func(t *testing.T) {
			if got := normalizeRecurrenceManageTerm(scenario.input); got != scenario.want {
				t.Fatalf("normalizeRecurrenceManageTerm(%q) = %q; want %q", scenario.input, got, scenario.want)
			}
		})
	}
}
