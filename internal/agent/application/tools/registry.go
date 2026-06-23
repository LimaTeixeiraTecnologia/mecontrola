package tools

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var (
	ErrToolNameEmpty        = errors.New("agent.application.tools: tool name is empty")
	ErrToolDescriptionEmpty = errors.New("agent.application.tools: tool description is empty")
	ErrDuplicateToolName    = errors.New("agent.application.tools: duplicate tool name")
	ErrDuplicateIntentKind  = errors.New("agent.application.tools: duplicate intent kind")
	ErrEmptyRegistry        = errors.New("agent.application.tools: registry has no tools")
)

const (
	toolSystemHeader = `Você é o MeControla, parceiro financeiro conversacional em PT-BR. Responda sempre curto, claro e acolhedor.

Use uma ferramenta SOMENTE quando o usuário pedir uma AÇÃO concreta que ela executa:`

	toolSystemFooter = `Regras invioláveis:
- Se faltar um dado obrigatório da ação, NÃO chame a ferramenta: responda em TEXTO fazendo UMA pergunta objetiva para obter o dado.
- Nunca invente valores, datas ou nomes que o usuário não informou.
- Antes de qualquer escrita sensível, confirme com o usuário em texto quando houver ambiguidade.
- Para conversa, dúvida ou pedido fora dessas ações, responda em TEXTO curto e gentil, sem chamar ferramenta.`
)

type ToolSpec struct {
	Name        string
	IntentKind  intent.Kind
	Description string
}

type Registry struct {
	specs    []ToolSpec
	byIntent map[intent.Kind]ToolSpec
}

func NewRegistry(specs ...ToolSpec) (*Registry, error) {
	if len(specs) == 0 {
		return nil, ErrEmptyRegistry
	}
	ordered := make([]ToolSpec, 0, len(specs))
	byIntent := make(map[intent.Kind]ToolSpec, len(specs))
	seenName := make(map[string]struct{}, len(specs))
	var errs []error
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			errs = append(errs, fmt.Errorf("name=%q: %w", spec.Name, ErrToolNameEmpty))
			continue
		}
		if strings.TrimSpace(spec.Description) == "" {
			errs = append(errs, fmt.Errorf("name=%q: %w", name, ErrToolDescriptionEmpty))
			continue
		}
		if _, exists := seenName[name]; exists {
			errs = append(errs, fmt.Errorf("name=%q: %w", name, ErrDuplicateToolName))
			continue
		}
		if _, exists := byIntent[spec.IntentKind]; exists {
			errs = append(errs, fmt.Errorf("name=%q intent=%q: %w", name, spec.IntentKind.String(), ErrDuplicateIntentKind))
			continue
		}
		seenName[name] = struct{}{}
		byIntent[spec.IntentKind] = spec
		ordered = append(ordered, spec)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Registry{specs: ordered, byIntent: byIntent}, nil
}

func (r *Registry) SpecByIntent(kind intent.Kind) (ToolSpec, bool) {
	spec, ok := r.byIntent[kind]
	return spec, ok
}

func (r *Registry) Specs() []ToolSpec {
	out := make([]ToolSpec, len(r.specs))
	copy(out, r.specs)
	return out
}

func (r *Registry) RenderSystemPrompt() (string, error) {
	if len(r.specs) == 0 {
		return "", ErrEmptyRegistry
	}
	var buf strings.Builder
	buf.WriteString(toolSystemHeader)
	for _, spec := range r.specs {
		buf.WriteString("\n- ")
		buf.WriteString(spec.Name)
		buf.WriteString(": ")
		buf.WriteString(spec.Description)
	}
	buf.WriteString("\n\n")
	buf.WriteString(toolSystemFooter)
	return buf.String(), nil
}

func DefaultRegistry() (*Registry, error) {
	return NewRegistry(
		ToolSpec{
			Name:        "record_transaction",
			IntentKind:  intent.KindRecordExpense,
			Description: `registrar um gasto ou recebimento (ex: "gastei 58 no iFood", "recebi 5000 de salário").`,
		},
		ToolSpec{
			Name:        "monthly_summary",
			IntentKind:  intent.KindMonthlySummary,
			Description: "mostrar o resumo do mês / orçamento.",
		},
		ToolSpec{
			Name:        "list_cards",
			IntentKind:  intent.KindListCards,
			Description: "listar os cartões cadastrados.",
		},
		ToolSpec{
			Name:        "create_card",
			IntentKind:  intent.KindCreateCard,
			Description: "cadastrar um novo cartão (apelido, fechamento, vencimento, limite).",
		},
		ToolSpec{
			Name:        "count_cards",
			IntentKind:  intent.KindCountCards,
			Description: "dizer quantos cartões o usuário tem.",
		},
		ToolSpec{
			Name:        "configure_budget",
			IntentKind:  intent.KindConfigureBudget,
			Description: "iniciar a configuração do orçamento mensal.",
		},
	)
}
