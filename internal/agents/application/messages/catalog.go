package messages

import (
	"fmt"
	"hash/fnv"
	"strings"
)

type WriteKind uint8

const (
	WriteKindUnspecified WriteKind = iota
	WriteKindExpense
	WriteKindIncome
)

func (k WriteKind) String() string {
	switch k {
	case WriteKindExpense:
		return "expense"
	case WriteKindIncome:
		return "income"
	default:
		return "unspecified"
	}
}

func (k WriteKind) IsValid() bool {
	return k == WriteKindExpense || k == WriteKindIncome
}

type SummaryScenario uint8

const (
	SummaryScenarioUnspecified SummaryScenario = iota
	SummaryScenarioAvailable
	SummaryScenarioNearLimit
	SummaryScenarioExactLimit
	SummaryScenarioExceeded
)

func (s SummaryScenario) IsValid() bool {
	switch s {
	case SummaryScenarioAvailable, SummaryScenarioNearLimit, SummaryScenarioExactLimit, SummaryScenarioExceeded:
		return true
	default:
		return false
	}
}

type GeneralScenario uint8

const (
	GeneralScenarioUnspecified GeneralScenario = iota
	GeneralScenarioPositive
	GeneralScenarioAttention
	GeneralScenarioCritical
)

func (s GeneralScenario) IsValid() bool {
	switch s {
	case GeneralScenarioPositive, GeneralScenarioAttention, GeneralScenarioCritical:
		return true
	default:
		return false
	}
}

type MissingField uint8

const (
	MissingFieldUnspecified MissingField = iota
	MissingFieldAmount
	MissingFieldPaymentMethod
	MissingFieldCategory
	MissingFieldOrigin
	MissingFieldDescription
)

func (f MissingField) IsValid() bool {
	switch f {
	case MissingFieldAmount, MissingFieldPaymentMethod, MissingFieldCategory, MissingFieldOrigin, MissingFieldDescription:
		return true
	default:
		return false
	}
}

type MotivationSeed uint64

func NewMotivationSeed(raw string) MotivationSeed {
	h := fnv.New64a()
	_, _ = h.Write([]byte(raw))
	return MotivationSeed(h.Sum64())
}

func pick(seed MotivationSeed, phrases []string) string {
	if len(phrases) == 0 {
		return ""
	}
	return phrases[uint64(seed)%uint64(len(phrases))]
}

type ConfirmationView struct {
	AmountFormatted string
	PaymentMethod   string
	Category        string
	Origin          string
}

type CategoryEntryView struct {
	DateFormatted   string
	AmountFormatted string
	Subcategory     string
}

type CategoryView struct {
	Category           string
	Entries            []CategoryEntryView
	PlannedFormatted   string
	SpentFormatted     string
	AvailableFormatted string
	OverrunFormatted   string
	Scenario           SummaryScenario
}

type GeneralCategoryRowView struct {
	Category           string
	PlannedFormatted   string
	SpentFormatted     string
	AvailableFormatted string
}

type GeneralView struct {
	Categories              []GeneralCategoryRowView
	TotalPlannedFormatted   string
	TotalSpentFormatted     string
	TotalAvailableFormatted string
	Scenario                GeneralScenario
}

var expenseMotivationalPhrases = []string{
	"Você está no controle da sua grana! 💪",
	"Mais um passo rumo aos seus objetivos! 🚀",
	"Registrar é o primeiro passo pra prosperar! 🌱",
	"Consistência é o segredo — você está mandando bem! 🙌",
	"Cada lançamento te deixa mais perto da sua meta! 🎯",
}

var incomeMotivationalPhrases = []string{
	"Boas notícias merecem ser celebradas! 🎉",
	"Seu esforço está dando resultado! 💪",
	"Mais uma vitória pro seu bolso! 🚀",
	"Seguimos construindo sua estabilidade! 🌱",
	"Isso é motivo pra sorrir! 😄",
}

var categoryAvailablePhrases = []string{
	"Você está no controle dessa categoria! 💪",
	"Seguindo firme dentro do planejado! 🎯",
	"Mandando bem por aqui! 🙌",
}

var categoryNearLimitPhrases = []string{
	"Fique de olho, mas você ainda está no jogo! 👀",
	"Quase no limite — vamos com atenção daqui pra frente! ⚠️",
}

var categoryExactLimitPhrases = []string{
	"Você usou exatamente o planejado — controle em dia! 🎯",
	"No fio da navalha, mas dentro do combinado! ⚠️",
}

var categoryExceededPhrases = []string{
	"Passou do planejado, mas dá pra reorganizar! 🚨",
	"Hora de olhar com carinho pra essa categoria! 🚨",
}

var generalPositivePhrases = []string{
	"Seu orçamento está saudável — parabéns! 💚",
	"Você está no caminho certo! 🚀",
}

var generalAttentionPhrases = []string{
	"Vale a pena dar uma atenção especial esse mês! ⚠️",
	"Fique de olho pra fechar o mês no azul! ⚠️",
}

var generalCriticalPhrases = []string{
	"Vamos juntos reorganizar o que for preciso! 🚨",
	"Momento de replanejar — você consegue! 🚨",
}

var budgetManageMotivationalPhrases = []string{
	"Seu planejamento está cada vez melhor! 💪",
	"Mais um passo rumo aos seus objetivos! 🚀",
	"Você está no controle do seu orçamento! 🌱",
}

var cardManageMotivationalPhrases = []string{
	"Mais um passo pra organizar suas finanças! 💪",
	"Seus meios de pagamento sob controle! 🌱",
}

var goalEditMotivationalPhrases = []string{
	"Seu novo objetivo está guardado — vamos juntos até lá! 💚",
	"Motivação renovada é combustível pra prosperar! 🚀",
	"Cada objetivo claro te aproxima da conquista! 🌱",
}

func BudgetManageMotivation(seed MotivationSeed) string {
	return pick(seed, budgetManageMotivationalPhrases)
}

func CardManageMotivation(seed MotivationSeed) string {
	return pick(seed, cardManageMotivationalPhrases)
}

func GoalEditMotivation(seed MotivationSeed) string {
	return pick(seed, goalEditMotivationalPhrases)
}

func PendingConfirmationExists() string {
	return "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação."
}

func PendingCardCreationExists() string {
	return "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outro cadastro."
}

func TreatmentNameConfirmation(name string) string {
	return fmt.Sprintf("Combinado, %s! 💚 Vou te chamar assim daqui pra frente.", name)
}

func TreatmentNameEditQuestion() string {
	return "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚"
}

func TreatmentNameTooLong() string {
	return "Esse nome ficou um pouco longo. 😊 Pode me dizer uma forma mais curta pra eu te chamar? 💚"
}

func ExpenseConfirmationBlock(v ConfirmationView) string {
	return fmt.Sprintf(
		"✅ Encontrei este lançamento:\n\n💰 Valor: %s\n💳 Pagamento: %s\n📂 Categoria: %s\n\nPosso registrar?",
		v.AmountFormatted, v.PaymentMethod, v.Category,
	)
}

func IncomeConfirmationBlock(v ConfirmationView) string {
	return fmt.Sprintf(
		"✅ Encontrei esta entrada:\n\n💰 Valor: %s\n📥 Origem: %s\n\nPosso registrar?",
		v.AmountFormatted, v.Origin,
	)
}

func WriteSuccess(kind WriteKind, seed MotivationSeed) string {
	switch kind {
	case WriteKindIncome:
		return fmt.Sprintf("Boa notícia! 🎉\n\n%s 💚", pick(seed, incomeMotivationalPhrases))
	default:
		return fmt.Sprintf("Prontinho! ✅\n\n%s 💚", pick(seed, expenseMotivationalPhrases))
	}
}

func categoryScenarioLine(v CategoryView) string {
	switch v.Scenario {
	case SummaryScenarioNearLimit:
		return fmt.Sprintf("⚠️ Disponível: %s (perto do limite)\n\n%s", v.AvailableFormatted, pick(NewMotivationSeed(v.Category), categoryNearLimitPhrases))
	case SummaryScenarioExactLimit:
		return fmt.Sprintf("⚠️ Você atingiu exatamente o valor planejado para %s.\n\n%s", v.Category, pick(NewMotivationSeed(v.Category), categoryExactLimitPhrases))
	case SummaryScenarioExceeded:
		return fmt.Sprintf("🚨 Você ultrapassou em %s o valor planejado para %s.\n\n%s", v.OverrunFormatted, v.Category, pick(NewMotivationSeed(v.Category), categoryExceededPhrases))
	default:
		return fmt.Sprintf("✅ Disponível: %s\n\n%s", v.AvailableFormatted, pick(NewMotivationSeed(v.Category), categoryAvailablePhrases))
	}
}

func CategorySummaryBlock(v CategoryView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📊 Resumo de %s:\n\n", v.Category)
	for _, entry := range v.Entries {
		fmt.Fprintf(&b, "- %s | %s | %s\n", entry.DateFormatted, entry.AmountFormatted, entry.Subcategory)
	}
	fmt.Fprintf(&b, "\n💰 Planejado: %s\n💸 Gasto: %s\n\n", v.PlannedFormatted, v.SpentFormatted)
	b.WriteString(categoryScenarioLine(v))
	return b.String()
}

func generalScenarioLine(v GeneralView) string {
	switch v.Scenario {
	case GeneralScenarioAttention:
		return fmt.Sprintf("⚠️ %s", pick(NewMotivationSeed(v.TotalSpentFormatted), generalAttentionPhrases))
	case GeneralScenarioCritical:
		return fmt.Sprintf("🚨 %s", pick(NewMotivationSeed(v.TotalSpentFormatted), generalCriticalPhrases))
	default:
		return fmt.Sprintf("✅ %s", pick(NewMotivationSeed(v.TotalSpentFormatted), generalPositivePhrases))
	}
}

func GeneralSummaryBlock(v GeneralView) string {
	var b strings.Builder
	b.WriteString("📊 Panorama geral do orçamento:\n\n")
	for _, row := range v.Categories {
		fmt.Fprintf(&b, "*%s*\n💰 Planejado: %s\n💸 Gasto: %s\n✅ Disponível: %s\n\n", row.Category, row.PlannedFormatted, row.SpentFormatted, row.AvailableFormatted)
	}
	fmt.Fprintf(&b, "*Consolidado*\n💰 Total planejado: %s\n💸 Total gasto: %s\n✅ Total disponível: %s\n\n", v.TotalPlannedFormatted, v.TotalSpentFormatted, v.TotalAvailableFormatted)
	b.WriteString(generalScenarioLine(v))
	return b.String()
}

func CancelPlanInfo() string {
	return "Poxa, sentiremos sua falta! 💚 Segue o passo a passo para cancelar sua assinatura pela Kiwify:\n\n" +
		"1. Acesse sua conta na Kiwify\n" +
		"2. Vá em *Minhas Compras*\n" +
		"3. Localize a assinatura *MeControla*\n" +
		"4. Toque em *Gerenciar Assinatura*\n" +
		"5. Selecione *Cancelar Assinatura*\n" +
		"6. Confirme o cancelamento\n\n" +
		"Se mudar de ideia, as portas estarão sempre abertas! 🙂"
}

func SupportInfo() string {
	return "Estou aqui pra ajudar! 💬 Envie um e-mail para *contato@limateixeira.com.br* explicando o que aconteceu, " +
		"e nossa equipe responde em até 24 horas."
}

func ClarificationQuestion(field MissingField) string {
	switch field {
	case MissingFieldAmount:
		return "Qual foi o valor? 💰"
	case MissingFieldPaymentMethod:
		return "Como você pagou? (pix, débito, crédito, dinheiro...) 💳"
	case MissingFieldCategory:
		return "Em qual categoria isso se encaixa? 📂"
	case MissingFieldOrigin:
		return "Qual foi a origem dessa entrada? 📥"
	case MissingFieldDescription:
		return "Me conta rapidinho do que se trata esse lançamento? 📝"
	default:
		return "Pode me dar mais um detalhe pra eu continuar? 🙂"
	}
}
