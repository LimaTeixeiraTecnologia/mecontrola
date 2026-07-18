package scorers

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

var mecontrolaWriteTools = []string{
	"register_expense",
	"register_income",
	"create_recurrence",
	"update_recurrence",
	"delete_recurrence",
	"adjust_allocation",
	"edit_entry",
	"delete_entry",
	"update_card",
}

var mecontrolaMonthRefTools = []string{
	"query_month",
	"query_plan",
	"create_budget",
}

var mecontrolaRequiredArgsByTool = map[string][]string{
	"register_expense":  {"amountCents", "description"},
	"register_income":   {"amountCents", "description"},
	"create_recurrence": {"direction", "paymentMethod", "amountCents", "description", "frequency", "dayOfMonth"},
	"adjust_allocation": {"competence", "rootSlug", "percentage"},
	"edit_entry":        {"entryId"},
	"delete_entry":      {"entryId", "entryKind", "version"},
	"update_card":       {"cardId", "version"},
	"update_recurrence": {"templateId", "version"},
	"delete_recurrence": {"templateId", "version"},
}

var mecontrolaSuccessMarkers = []string{
	"registrei",
	"salvei",
	"atualizei",
	"removi",
	"exclu",
	"cadastrei",
	"criei",
}

var mecontrolaInternalTerms = []string{
	"usecase",
	"uuid",
	"nil pointer",
	"internal server error",
	"panic",
	"goroutine",
	"stack trace",
	"nullpointer",
	"sql:",
	"context.context",
	"register_expense",
	"register_income",
	"create_recurrence",
	"adjust_allocation",
	"edit_entry",
	"delete_entry",
	"update_card",
	"query_plan",
	"query_month",
}

var mecontrolaValidMonthRefKinds = map[string]bool{
	"current":            true,
	"previous":           true,
	"next":               true,
	"explicit":           true,
	"named_without_year": true,
	"unknown":            true,
}

var mecontrolaToneEmojis = []string{"✅", "💰", "💳", "📂", "📥", "📊", "⚠️", "🚨", "🎉", "💚"}

type noEmptyAnswerScorer struct{}

func (s *noEmptyAnswerScorer) ID() string              { return "no_empty_answer" }
func (s *noEmptyAnswerScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *noEmptyAnswerScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	if strings.TrimSpace(sample.Output) == "" {
		return scorer.ScoreResult{Score: 0.0, Reason: "resposta vazia"}, nil
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "resposta não vazia"}, nil
}

type whatsappFormatScorer struct{}

func (s *whatsappFormatScorer) ID() string              { return "whatsapp_format" }
func (s *whatsappFormatScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *whatsappFormatScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	for line := range strings.SplitSeq(sample.Output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return scorer.ScoreResult{Score: 0.0, Reason: "contém header markdown (#) não suportado pelo WhatsApp"}, nil
		}
		if strings.Contains(trimmed, "|---") || strings.Contains(trimmed, "| ---") {
			return scorer.ScoreResult{Score: 0.0, Reason: "contém tabela markdown não suportada pelo WhatsApp"}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "formato compatível com WhatsApp"}, nil
}

type noInternalTermsScorer struct{}

func (s *noInternalTermsScorer) ID() string              { return "no_internal_terms" }
func (s *noInternalTermsScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *noInternalTermsScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	lower := strings.ToLower(sample.Output)
	for _, term := range mecontrolaInternalTerms {
		if strings.Contains(lower, term) {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("termo interno vazado: %q", term),
				Metadata: map[string]any{"term": term},
			}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "sem termos internos vazados"}, nil
}

type verbatimRequiredScorer struct{}

func (s *verbatimRequiredScorer) ID() string              { return "verbatim_required" }
func (s *verbatimRequiredScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *verbatimRequiredScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	verbatimText, ok := sample.Metadata["verbatim_text"].(string)
	if !ok || verbatimText == "" {
		return scorer.ScoreResult{Score: 1.0, Reason: "nenhum texto verbatim esperado"}, nil
	}
	if strings.Contains(sample.Output, verbatimText) {
		return scorer.ScoreResult{Score: 1.0, Reason: "resposta contém o texto verbatim esperado"}, nil
	}
	return scorer.ScoreResult{
		Score:  0.0,
		Reason: "resposta não contém o texto verbatim esperado",
	}, nil
}

type noDuplicateWriteScorer struct{}

func (s *noDuplicateWriteScorer) ID() string              { return "no_duplicate_write" }
func (s *noDuplicateWriteScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *noDuplicateWriteScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	writeSet := make(map[string]bool, len(mecontrolaWriteTools))
	for _, t := range mecontrolaWriteTools {
		writeSet[t] = true
	}
	seen := make(map[string]bool)
	for _, tc := range sample.ToolCalls {
		if !writeSet[tc.Name] {
			continue
		}
		key := tc.Name + ":" + argsFingerprint(tc.Args)
		if seen[key] {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("write-tool duplicada com os mesmos argumentos: %s", tc.Name),
				Metadata: map[string]any{"tool": tc.Name},
			}, nil
		}
		seen[key] = true
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "sem escrita duplicada"}, nil
}

func argsFingerprint(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, args[k]))
	}
	return strings.Join(parts, "&")
}

type noHallucinationScorer struct{}

func (s *noHallucinationScorer) ID() string              { return "no_hallucination" }
func (s *noHallucinationScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *noHallucinationScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	lower := strings.ToLower(sample.Output)
	hasMarker := false
	for _, marker := range mecontrolaSuccessMarkers {
		if strings.Contains(lower, marker) {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return scorer.ScoreResult{Score: 1.0, Reason: "sem marcador de sucesso na resposta"}, nil
	}
	writeSet := make(map[string]bool, len(mecontrolaWriteTools))
	for _, t := range mecontrolaWriteTools {
		writeSet[t] = true
	}
	for _, tc := range sample.ToolCalls {
		if writeSet[tc.Name] && mecontrolaEffectiveWriteOutcomes[tc.Outcome] {
			return scorer.ScoreResult{Score: 1.0, Reason: "marcador de sucesso respaldado por write-tool efetivada"}, nil
		}
	}
	return scorer.ScoreResult{
		Score:  0.0,
		Reason: "marcador de sucesso sem write-tool efetivada correspondente",
	}, nil
}

type requiredArgsScorer struct{}

func (s *requiredArgsScorer) ID() string              { return "required_args" }
func (s *requiredArgsScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *requiredArgsScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	for _, tc := range sample.ToolCalls {
		required, ok := mecontrolaRequiredArgsByTool[tc.Name]
		if !ok {
			continue
		}
		missing := make([]string, 0)
		for _, field := range required {
			v, present := tc.Args[field]
			if !present || isZeroArgValue(v) {
				missing = append(missing, field)
			}
		}
		if len(missing) > 0 {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("write-tool %s sem args obrigatórios: %v", tc.Name, missing),
				Metadata: map[string]any{"tool": tc.Name, "missing": missing},
			}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "args obrigatórios presentes em todas as write-tools chamadas"}, nil
}

var rePaymentLaunchVerbs = regexp.MustCompile(`(?i)\b(gastei|paguei|comprei|parcelei)\b`)

var paymentMethodEvidenceTerms = map[string][]string{
	"pix":              {"pix"},
	"cash":             {"dinheiro", "especie", "espécie"},
	"debit_card":       {"debito", "débito"},
	"debit_in_account": {"debito em conta", "débito em conta", "conta"},
	"credit_card":      {"credito", "crédito", "cartao", "cartão", "parcel"},
	"boleto":           {"boleto"},
	"ted":              {"ted"},
	"doc":              {"doc"},
	"transferencia":    {"transfer"},
	"vale_refeicao":    {"vale", "vr", "refeicao", "refeição"},
	"vale_alimentacao": {"vale", "va", "alimentacao", "alimentação"},
	"apple_pay":        {"apple"},
	"google_pay":       {"google"},
	"picpay":           {"picpay"},
	"mercado_pago":     {"mercado pago", "mercadopago"},
	"cheque":           {"cheque"},
}

type paymentMethodProvenanceScorer struct{}

func (s *paymentMethodProvenanceScorer) ID() string              { return "payment_method_provenance" }
func (s *paymentMethodProvenanceScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *paymentMethodProvenanceScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	if priorTurn, ok := sample.Metadata["payment_method_prior_turn"].(bool); ok && priorTurn {
		return scorer.ScoreResult{Score: 1.0, Reason: "forma de pagamento veio de turno anterior (metadata)"}, nil
	}
	input := strings.ToLower(sample.Input)
	if !rePaymentLaunchVerbs.MatchString(input) {
		return scorer.ScoreResult{Score: 1.0, Reason: "sem verbo de lançamento no turno; proveniência não avaliada"}, nil
	}
	for _, tc := range sample.ToolCalls {
		if tc.Name != "register_expense" {
			continue
		}
		raw, present := tc.Args["paymentMethod"]
		if !present {
			continue
		}
		method, ok := raw.(string)
		if !ok || method == "" {
			continue
		}
		if !hasPaymentEvidence(input, method) {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("paymentMethod inventado: %q sem evidência no texto do usuário", method),
				Metadata: map[string]any{"tool": tc.Name, "paymentMethod": method},
			}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "paymentMethod ausente ou com evidência textual no input"}, nil
}

func hasPaymentEvidence(input, method string) bool {
	terms, ok := paymentMethodEvidenceTerms[method]
	if !ok {
		return false
	}
	words := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	for _, term := range terms {
		if len(term) <= 3 {
			if slices.Contains(words, term) {
				return true
			}
			continue
		}
		if strings.Contains(input, term) {
			return true
		}
	}
	return false
}

func isZeroArgValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case float64:
		return val == 0
	default:
		return false
	}
}

type monthReferenceCorrectnessScorer struct{}

func (s *monthReferenceCorrectnessScorer) ID() string { return "month_reference_correctness" }
func (s *monthReferenceCorrectnessScorer) Kind() scorer.ScorerKind {
	return scorer.ScorerKindCodeBased
}

func (s *monthReferenceCorrectnessScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	monthRefToolSet := make(map[string]bool, len(mecontrolaMonthRefTools))
	for _, t := range mecontrolaMonthRefTools {
		monthRefToolSet[t] = true
	}
	for _, tc := range sample.ToolCalls {
		if !monthRefToolSet[tc.Name] {
			continue
		}
		kindRaw, present := tc.Args["monthRefKind"]
		if !present {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("tool de mês %s chamada sem monthRefKind", tc.Name),
				Metadata: map[string]any{"tool": tc.Name},
			}, nil
		}
		kind, ok := kindRaw.(string)
		if !ok || !mecontrolaValidMonthRefKinds[kind] {
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("tool de mês %s chamada com monthRefKind inválido: %v", tc.Name, kindRaw),
				Metadata: map[string]any{"tool": tc.Name, "monthRefKind": kindRaw},
			}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "monthRefKind presente e consistente nas tools de mês chamadas"}, nil
}

type verbatimToneAdherenceScorer struct{}

func (s *verbatimToneAdherenceScorer) ID() string              { return "verbatim_tone_adherence" }
func (s *verbatimToneAdherenceScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *verbatimToneAdherenceScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	output := sample.Output
	if strings.TrimSpace(output) == "" {
		return scorer.ScoreResult{Score: 1.0, Reason: "resposta vazia; sem tom a avaliar"}, nil
	}
	if strings.Contains(output, "**") {
		return scorer.ScoreResult{Score: 0.0, Reason: "usa negrito duplo (**) em vez de asterisco simples do Tom de Voz"}, nil
	}
	if strings.Count(output, "*")%2 != 0 {
		return scorer.ScoreResult{
			Score:  0.0,
			Reason: "número ímpar de asteriscos; marcação de negrito mal formada",
		}, nil
	}
	requiresEmoji, ok := sample.Metadata["requires_brand_emoji"].(bool)
	if ok && requiresEmoji {
		hasEmoji := false
		for _, e := range mecontrolaToneEmojis {
			if strings.Contains(output, e) {
				hasEmoji = true
				break
			}
		}
		if !hasEmoji {
			return scorer.ScoreResult{
				Score:  0.0,
				Reason: "resposta não contém nenhum emoji oficial do Tom de Voz",
				Metadata: map[string]any{
					"expected_any_of": mecontrolaToneEmojis,
				},
			}, nil
		}
	}
	return scorer.ScoreResult{Score: 1.0, Reason: "aderente ao Tom de Voz (negrito simples e emojis oficiais)"}, nil
}

type expectedToolOracleScorer struct{}

func (s *expectedToolOracleScorer) ID() string              { return "expected_tool" }
func (s *expectedToolOracleScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *expectedToolOracleScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	expectedTool, ok := sample.Metadata["expected_tool"].(string)
	if !ok || expectedTool == "" {
		return scorer.ScoreResult{Score: 1.0, Reason: "nenhuma tool esperada no gabarito"}, nil
	}
	for _, tc := range sample.ToolCalls {
		if tc.Name == expectedTool {
			return scorer.ScoreResult{
				Score:    1.0,
				Reason:   fmt.Sprintf("chamou a tool esperada: %s", expectedTool),
				Metadata: map[string]any{"expected_tool": expectedTool},
			}, nil
		}
	}
	called := make([]string, len(sample.ToolCalls))
	for i, tc := range sample.ToolCalls {
		called[i] = tc.Name
	}
	return scorer.ScoreResult{
		Score:  0.0,
		Reason: fmt.Sprintf("tool esperada %q não foi chamada; chamadas: %v", expectedTool, called),
		Metadata: map[string]any{
			"expected_tool": expectedTool,
			"called":        called,
		},
	}, nil
}

func NewNoEmptyAnswerScorer() scorer.Scorer { return &noEmptyAnswerScorer{} }

func NewWhatsAppFormatScorer() scorer.Scorer { return &whatsappFormatScorer{} }

func NewNoInternalTermsScorer() scorer.Scorer { return &noInternalTermsScorer{} }

func NewVerbatimRequiredScorer() scorer.Scorer { return &verbatimRequiredScorer{} }

func NewNoDuplicateWriteScorer() scorer.Scorer { return &noDuplicateWriteScorer{} }

func NewNoHallucinationScorer() scorer.Scorer { return &noHallucinationScorer{} }

func NewRequiredArgsScorer() scorer.Scorer { return &requiredArgsScorer{} }

func NewPaymentMethodProvenanceScorer() scorer.Scorer { return &paymentMethodProvenanceScorer{} }

func NewMonthReferenceCorrectnessScorer() scorer.Scorer { return &monthReferenceCorrectnessScorer{} }

func NewExpectedToolOracleScorer() scorer.Scorer { return &expectedToolOracleScorer{} }

func NewVerbatimToneAdherenceScorer() scorer.Scorer { return &verbatimToneAdherenceScorer{} }
