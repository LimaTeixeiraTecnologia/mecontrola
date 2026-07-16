package messages

import (
	"fmt"
	"strings"
)

var editMotivationalPhrases = []string{
	"Registro atualizado, tudo em ordem! 💪",
	"Correção feita — seus dados seguem confiáveis! 🎯",
	"Ajuste concluído, você no controle! 🙌",
}

var recurrenceMotivationalPhrases = []string{
	"Recorrência configurada — no piloto automático! 🚀",
	"Agora isso se cuida sozinho todo mês! 🙌",
	"Mais organização para sua rotina financeira! 🌱",
}

type EditConfirmationView struct {
	PreviousAmountFormatted string
	PreviousCategory        string
	PreviousPaymentMethod   string
	NewAmountFormatted      string
	NewCategory             string
	NewPaymentMethod        string
	AmountChanged           bool
	CategoryChanged         bool
	PaymentChanged          bool
}

func EditConfirmationBlock(v EditConfirmationView) string {
	changes := make([]string, 0, 3)
	if v.AmountChanged {
		changes = append(changes, fmt.Sprintf("🔄 Novo valor: %s", v.NewAmountFormatted))
	}
	if v.CategoryChanged {
		changes = append(changes, fmt.Sprintf("🔄 Nova categoria: *%s*", v.NewCategory))
	}
	if v.PaymentChanged {
		changes = append(changes, fmt.Sprintf("🔄 Novo pagamento: %s", v.NewPaymentMethod))
	}
	if len(changes) == 0 {
		changes = append(changes, fmt.Sprintf("🔄 Novo valor: %s", v.NewAmountFormatted))
	}
	return fmt.Sprintf(
		"✏️ Lançamento atual: %s em *%s* (%s)\n%s\n\nPosso atualizar?",
		v.PreviousAmountFormatted, v.PreviousCategory, v.PreviousPaymentMethod, strings.Join(changes, "\n"),
	)
}

func EditSuccess(seed MotivationSeed) string {
	return fmt.Sprintf("Prontinho, atualizei! ✅\n\n%s 💚", pick(seed, editMotivationalPhrases))
}

type RecurrenceConfirmationView struct {
	AmountFormatted string
	Category        string
	Frequency       string
}

func RecurrenceConfirmationBlock(v RecurrenceConfirmationView) string {
	return fmt.Sprintf(
		"✅ Encontrei esta recorrência:\n\n💰 Valor: %s\n📂 Categoria: %s\n🔁 Frequência: %s\n\nPosso configurar?",
		v.AmountFormatted, v.Category, v.Frequency,
	)
}

func RecurrenceSuccess(seed MotivationSeed) string {
	return fmt.Sprintf("Prontinho! ✅\n\n%s 💚", pick(seed, recurrenceMotivationalPhrases))
}

func WriteCancelled() string {
	return "Tudo certo, o registro foi cancelado. 🙂"
}

func WriteExpired() string {
	return "O registro expirou. Para continuar, envie a informação completa novamente. 🙂"
}

func ConfirmationReprompt() string {
	return "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar."
}

func PaymentMethodMigrationBlocked() string {
	return "Essa forma de pagamento não pode migrar de/para crédito nesta edição. Por favor, escolha outra forma de pagamento ou mantenha a atual."
}

func NoEditCandidateFound() string {
	return "Não encontrei um lançamento compatível para editar. Pode me dar mais detalhes (valor, categoria ou descrição)?"
}

func EditCandidatesPrompt(paths []string) string {
	list := ""
	for i, p := range paths {
		if i > 0 {
			list += "\n"
		}
		list += fmt.Sprintf("%d. %s", i+1, p)
	}
	return "Encontrei mais de um lançamento compatível. Qual deles você quer editar?\n\n" + list
}

func WriteFailure() string {
	return "Não consegui registrar. Tente novamente em breve."
}

func CardPrompt() string {
	return "Qual 💳 foi utilizado?"
}

func DatePrompt() string {
	return "Qual foi a data do lançamento?"
}

func ActiveWriteExists() string {
	return "Ainda tenho um lançamento em aberto aguardando você. Me responda para concluí-lo ou envie \"cancelar\" para descartá-lo, aí seguimos com o próximo. 🙂"
}
