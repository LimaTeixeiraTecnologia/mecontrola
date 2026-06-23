package services

import (
	"errors"
	"fmt"
	"strings"

	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

const rootSlugMetas = "expense.metas"

func pickMostRecent(items []TransactionView) (TransactionView, bool, error) {
	if len(items) == 0 {
		return TransactionView{}, false, nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if moreRecent(item, latest) {
			latest = item
		}
	}
	return latest, true, nil
}

func moreRecent(candidate, current TransactionView) bool {
	if candidate.CreatedAt.Equal(current.CreatedAt) {
		return candidate.ID > current.ID
	}
	return candidate.CreatedAt.After(current.CreatedAt)
}

func resolveCardByName(list cardoutput.CardList, name string) (cardoutput.Card, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return cardoutput.Card{}, false
	}
	for _, item := range list.Items {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return item, true
		}
		if strings.EqualFold(strings.TrimSpace(item.Nickname), name) {
			return item, true
		}
	}
	for _, item := range list.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			return item, true
		}
		if strings.Contains(strings.ToLower(item.Nickname), target) {
			return item, true
		}
	}
	return cardoutput.Card{}, false
}

func formatBRL(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	reaisStr := formatThousands(reais)
	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("R$ %s%s,%02d", sign, reaisStr, centavos)
}

func formatThousands(value int64) string {
	raw := fmt.Sprintf("%d", value)
	if len(raw) <= 3 {
		return raw
	}
	var out strings.Builder
	prefix := len(raw) % 3
	if prefix > 0 {
		out.WriteString(raw[:prefix])
		if len(raw) > prefix {
			out.WriteString(".")
		}
	}
	for idx := prefix; idx < len(raw); idx += 3 {
		out.WriteString(raw[idx : idx+3])
		if idx+3 < len(raw) {
			out.WriteString(".")
		}
	}
	return out.String()
}

func formatPersistedExpense(amountCents int64, merchant, categoryPath string) string {
	var sb strings.Builder
	sb.WriteString("💸 *Transação realizada!*\n*")
	sb.WriteString(formatBRL(amountCents))
	sb.WriteString("*")
	if strings.TrimSpace(merchant) != "" {
		sb.WriteString(" em *")
		sb.WriteString(merchant)
		sb.WriteString("*")
	}
	if strings.TrimSpace(categoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(categoryPath)
	}
	sb.WriteString("\n🔔 *Atualizando seu orçamento automaticamente...*")
	return sb.String()
}

func formatPersistedIncome(amountCents int64, source, categoryPath string) string {
	var sb strings.Builder
	sb.WriteString("💰 *Recebimento registrado!*\n*")
	sb.WriteString(formatBRL(amountCents))
	sb.WriteString("*")
	if strings.TrimSpace(source) != "" {
		sb.WriteString(" de *")
		sb.WriteString(source)
		sb.WriteString("*")
	}
	if strings.TrimSpace(categoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(categoryPath)
	}
	sb.WriteString("\n✅ Anotei na sua conta.")
	return sb.String()
}

func registerFailedText(amountCents int64, merchant string) string {
	var sb strings.Builder
	sb.WriteString("😕 Não consegui registrar ")
	sb.WriteString(formatBRL(amountCents))
	if strings.TrimSpace(merchant) != "" {
		sb.WriteString(" em ")
		sb.WriteString(merchant)
	}
	sb.WriteString(" agora. Pode tentar de novo em instantes? Se quiser, me diga a categoria pra eu organizar certinho.")
	return sb.String()
}

func formatMonthlySummary(summary budgetsoutput.MonthlySummaryOutput) string {
	var sb strings.Builder
	sb.WriteString("📊 *Resumo de ")
	sb.WriteString(summary.Competence)
	sb.WriteString("*\n")
	sb.WriteString("• Gasto total: ")
	sb.WriteString(formatBRL(summary.TotalSpentCents))
	if summary.TotalPlannedCents != nil {
		sb.WriteString(" / planejado ")
		sb.WriteString(formatBRL(*summary.TotalPlannedCents))
	}
	sb.WriteString("\n")
	for _, allocation := range summary.Allocations {
		if allocation.SpentCents == 0 && (allocation.PlannedCents == nil || *allocation.PlannedCents == 0) {
			continue
		}
		sb.WriteString("• ")
		sb.WriteString(rootSlugLabel(allocation.RootSlug))
		sb.WriteString(": ")
		sb.WriteString(formatBRL(allocation.SpentCents))
		if allocation.PlannedCents != nil {
			sb.WriteString(" / ")
			sb.WriteString(formatBRL(*allocation.PlannedCents))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatCategoryAllocation(summary budgetsoutput.MonthlySummaryOutput, categoryName string) string {
	target := strings.ToLower(strings.TrimSpace(categoryName))
	for _, allocation := range summary.Allocations {
		if strings.ToLower(allocation.RootSlug) == target {
			var sb strings.Builder
			sb.WriteString("📊 *")
			sb.WriteString(rootSlugLabel(allocation.RootSlug))
			sb.WriteString("* (")
			sb.WriteString(summary.Competence)
			sb.WriteString("): ")
			sb.WriteString(formatBRL(allocation.SpentCents))
			if allocation.PlannedCents != nil {
				sb.WriteString(" de ")
				sb.WriteString(formatBRL(*allocation.PlannedCents))
				sb.WriteString(" planejados")
				if allocation.PercentageSpent != nil {
					_, _ = fmt.Fprintf(&sb, " (%.0f%% da meta)", *allocation.PercentageSpent)
				}
			}
			sb.WriteString(".")
			return sb.String()
		}
	}
	return fmt.Sprintf("Não encontrei dados para a categoria %q em %s.", categoryName, summary.Competence)
}

func rootSlugLabel(slug string) string {
	switch slug {
	case "expense.custo_fixo":
		return "Custo Fixo"
	case "expense.conhecimento":
		return "Conhecimento"
	case "expense.prazeres":
		return "Prazeres"
	case "expense.metas":
		return "Metas"
	case "expense.liberdade_financeira":
		return "Liberdade Financeira"
	default:
		return slug
	}
}

func formatGoalUnavailable(goalName string) string {
	if strings.TrimSpace(goalName) == "" {
		return "Consultar metas ainda não está disponível, mas anotei seu pedido."
	}
	return fmt.Sprintf("Consultar a meta %q ainda não está disponível, mas anotei seu pedido.", goalName)
}

func formatGoalProgress(summary budgetsoutput.MonthlySummaryOutput, goalName string) string {
	for _, allocation := range summary.Allocations {
		if allocation.RootSlug == rootSlugMetas {
			var sb strings.Builder
			sb.WriteString("🎯 ")
			if strings.TrimSpace(goalName) != "" {
				sb.WriteString("*")
				sb.WriteString(goalName)
				sb.WriteString("*: ")
			}
			sb.WriteString("você já guardou ")
			sb.WriteString(formatBRL(allocation.SpentCents))
			if allocation.PlannedCents != nil && *allocation.PlannedCents > 0 {
				sb.WriteString(" de ")
				sb.WriteString(formatBRL(*allocation.PlannedCents))
				sb.WriteString(" previstos em Metas")
				if allocation.PercentageSpent != nil {
					_, _ = fmt.Fprintf(&sb, " (%.0f%%)", *allocation.PercentageSpent)
				}
			} else {
				sb.WriteString(" em Metas")
			}
			sb.WriteString(".")
			return sb.String()
		}
	}
	if strings.TrimSpace(goalName) != "" {
		return fmt.Sprintf("Não encontrei dados de metas para %q em %s.", goalName, summary.Competence)
	}
	return fmt.Sprintf("Não encontrei dados de metas em %s.", summary.Competence)
}

func formatCardList(list cardoutput.CardList) string {
	if len(list.Items) == 0 {
		return "💳 Você ainda não tem cartões cadastrados. Quer cadastrar um agora?"
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "💳 *Seus cartões* (%d)\n", len(list.Items))
	for _, card := range list.Items {
		name := strings.TrimSpace(card.Nickname)
		if name == "" {
			name = strings.TrimSpace(card.Name)
		}
		if name == "" {
			name = "Cartão sem nome"
		}
		sb.WriteString("• *")
		sb.WriteString(name)
		sb.WriteString("*")
		if card.LimitCents > 0 {
			sb.WriteString(" — limite ")
			sb.WriteString(formatBRL(card.LimitCents))
		}
		_, _ = fmt.Fprintf(&sb, " (fecha dia %d, vence dia %d)", card.ClosingDay, card.DueDay)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatCreatedCard(result CardCreatorResult) string {
	label := strings.TrimSpace(result.Nickname)
	if label == "" {
		label = strings.TrimSpace(result.Name)
	}
	if label == "" {
		label = "novo cartão"
	}
	var sb strings.Builder
	sb.WriteString("💳 *Cartão cadastrado!*\n*")
	sb.WriteString(label)
	sb.WriteString("*")
	if result.LimitCents > 0 {
		sb.WriteString(" — limite ")
		sb.WriteString(formatBRL(result.LimitCents))
	}
	_, _ = fmt.Fprintf(&sb, "\n📅 Fecha dia %d, vence dia %d.", result.ClosingDay, result.DueDay)
	return sb.String()
}

func formatUpdatedCard(result CardUpdaterResult) string {
	label := strings.TrimSpace(result.Nickname)
	if label == "" {
		label = strings.TrimSpace(result.Name)
	}
	if label == "" {
		label = "cartão"
	}
	var sb strings.Builder
	sb.WriteString("💳 *Cartão atualizado!*\n*")
	sb.WriteString(label)
	sb.WriteString("*")
	if result.LimitCents > 0 {
		sb.WriteString(" — limite ")
		sb.WriteString(formatBRL(result.LimitCents))
	}
	_, _ = fmt.Fprintf(&sb, "\n📅 Fecha dia %d, vence dia %d.", result.ClosingDay, result.DueDay)
	return sb.String()
}

func formatDeletedCard(result CardDeleterResult) string {
	label := strings.TrimSpace(result.Name)
	if label == "" {
		return "🗑️ Cartão apagado com sucesso."
	}
	return fmt.Sprintf("🗑️ *Cartão apagado!*\nRemovi o *%s* do seu cadastro.", label)
}

func formatCategoryPercentageUpdated(categoryName string, percentage int) string {
	label := strings.TrimSpace(categoryName)
	if label == "" {
		label = "a categoria"
	}
	return fmt.Sprintf("🎯 *Orçamento ajustado!*\nDefini *%s* com *%d%%* do seu planejamento. As outras categorias foram rebalanceadas pra somar 100%%.", label, percentage)
}

func formatCardAmbiguous(cardName string) string {
	if strings.TrimSpace(cardName) == "" {
		return "💳 Encontrei mais de um cartão parecido. Me diz o nome exato do cartão pra eu não errar? 🙂"
	}
	return fmt.Sprintf("💳 Encontrei mais de um cartão parecido com %q. Me diz o nome exato pra eu não errar? 🙂", cardName)
}

func formatCardCount(total int64) string {
	switch total {
	case 0:
		return "💳 Você ainda não tem cartões cadastrados. Quer cadastrar um agora?"
	case 1:
		return "💳 Você tem *1 cartão* cadastrado."
	default:
		return fmt.Sprintf("💳 Você tem *%d cartões* cadastrados.", total)
	}
}

func createCardErrorText(err error) string {
	switch {
	case errors.Is(err, carddomain.ErrNicknameConflict):
		return "💳 Você já tem um cartão com esse apelido. Que tal escolher outro nome?"
	case errors.Is(err, carddomain.ErrInvalidClosingDay) || (strings.Contains(err.Error(), "closing_day") && !strings.Contains(err.Error(), "due_day")):
		return "💳 O dia de fechamento precisa estar entre 1 e 31. Me confirma o dia certo?"
	case errors.Is(err, carddomain.ErrInvalidDueDay) || (strings.Contains(err.Error(), "due_day") && !strings.Contains(err.Error(), "closing_day")):
		return "💳 O dia de vencimento precisa estar entre 1 e 31. Me confirma o dia certo?"
	case strings.Contains(err.Error(), "closing_day") && strings.Contains(err.Error(), "due_day"):
		return "💳 Para cadastrar o cartão preciso saber o dia de fechamento e o dia de vencimento da fatura. Me informa os dois?"
	case errors.Is(err, carddomain.ErrInvalidNickname):
		return "💳 O apelido do cartão precisa ter entre 1 e 32 caracteres. Pode me passar um nome mais curto?"
	case errors.Is(err, carddomain.ErrInvalidCardName):
		return "💳 O nome do cartão precisa ter entre 1 e 64 caracteres. Pode reformular?"
	case errors.Is(err, carddomain.ErrCardLimitTooLarge):
		return "💳 Esse limite passou do máximo que consigo registrar (R$ 1.000.000,00). Confere o valor?"
	case errors.Is(err, carddomain.ErrCardLimitNegative):
		return "💳 O limite não pode ser negativo. Me passa um valor válido?"
	default:
		return "😕 Não consegui cadastrar o cartão agora. Pode tentar de novo em instantes?"
	}
}

func formatCardNotFound(cardName string) string {
	if strings.TrimSpace(cardName) == "" {
		return "💳 Não encontrei esse cartão no seu cadastro. Que tal cadastrá-lo primeiro pra eu cuidar da fatura pra você?"
	}
	return fmt.Sprintf("💳 Não encontrei um cartão chamado %q no seu cadastro. Quer cadastrá-lo primeiro pra eu acompanhar a fatura?", cardName)
}

func formatCardInvoice(card cardoutput.Card, invoice cardoutput.Invoice) string {
	name := strings.TrimSpace(card.Name)
	if name == "" {
		name = strings.TrimSpace(card.Nickname)
	}
	base := fmt.Sprintf("💳 *Fatura do cartão %s*\nFechamento em %s, vencimento em %s.",
		name, invoice.ClosingDate, invoice.DueDate)
	if card.LimitCents > 0 {
		return base + "\nLimite: " + formatBRL(card.LimitCents) + "."
	}
	return base
}

func formatHowAmIDoing(summary budgetsoutput.MonthlySummaryOutput) string {
	alert := summary.PercentageTotal != nil && *summary.PercentageTotal >= 80
	var sb strings.Builder
	if alert {
		sb.WriteString("⚠️ *Atenção Proativa* (")
	} else {
		sb.WriteString("📊 *Como você está* (")
	}
	sb.WriteString(summary.Competence)
	sb.WriteString(")\nVocê gastou ")
	sb.WriteString(formatBRL(summary.TotalSpentCents))
	if summary.TotalPlannedCents != nil && *summary.TotalPlannedCents > 0 {
		sb.WriteString(" de ")
		sb.WriteString(formatBRL(*summary.TotalPlannedCents))
		sb.WriteString(" planejados")
		if summary.PercentageTotal != nil {
			_, _ = fmt.Fprintf(&sb, " (%.0f%%)", *summary.PercentageTotal)
			if *summary.PercentageTotal >= 90 {
				sb.WriteString(". Você está bem próximo do limite do mês. Vamos manter o foco nos seus sonhos? 🎯")
				return sb.String()
			}
			if *summary.PercentageTotal >= 80 {
				sb.WriteString(". Dá pra segurar o ritmo até o fim do mês. Vamos juntos? 🎯")
				return sb.String()
			}
			if *summary.PercentageTotal <= 50 {
				sb.WriteString(". Está em ritmo tranquilo até aqui.")
				return sb.String()
			}
		}
		sb.WriteString(".")
		return sb.String()
	}
	sb.WriteString(". Defina um planejamento para eu te ajudar a acompanhar melhor.")
	return sb.String()
}

func formatCardPurchaseCardMissing(cardHint string) string {
	if strings.TrimSpace(cardHint) == "" {
		return "💳 Pra registrar a compra parcelada eu preciso saber em qual cartão foi. Me diz o nome do cartão? (ex: nubank, itaú)"
	}
	return fmt.Sprintf("💳 Não encontrei um cartão chamado %q no seu cadastro. Quer cadastrá-lo primeiro pra eu registrar a compra parcelada?", cardHint)
}

func formatPersistedCardPurchase(result CardPurchaseLoggerResult) string {
	var sb strings.Builder
	sb.WriteString("💳 *Compra parcelada registrada!*\n*")
	sb.WriteString(formatBRL(result.AmountCents))
	sb.WriteString("*")
	_, _ = fmt.Fprintf(&sb, " em *%dx*", result.Installments)
	if strings.TrimSpace(result.CardName) != "" {
		sb.WriteString(" no *")
		sb.WriteString(result.CardName)
		sb.WriteString("*")
	}
	if strings.TrimSpace(result.CategoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(result.CategoryPath)
	}
	sb.WriteString("\n✅ Anotei nas suas faturas.")
	return sb.String()
}

func formatTransactionList(list TransactionListResult) string {
	if len(list.Transactions) == 0 {
		return fmt.Sprintf("📭 Você não tem lançamentos em %s ainda.", list.RefMonth)
	}
	totalIn := int64(0)
	totalOut := int64(0)
	for _, t := range list.Transactions {
		switch t.Direction {
		case "income":
			totalIn += t.AmountCents
		case "outcome":
			totalOut += t.AmountCents
		}
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "📋 *Lançamentos de %s* (%d)\n", list.RefMonth, len(list.Transactions))
	sb.WriteString("• Entradas: ")
	sb.WriteString(formatBRL(totalIn))
	sb.WriteString("\n• Saídas: ")
	sb.WriteString(formatBRL(totalOut))
	return sb.String()
}

func formatDeletedTransaction(view TransactionView) string {
	var sb strings.Builder
	sb.WriteString("🗑️ *Lançamento excluído!*\n")
	sb.WriteString(formatBRL(view.AmountCents))
	if strings.TrimSpace(view.Description) != "" {
		sb.WriteString(" — ")
		sb.WriteString(view.Description)
	}
	sb.WriteString(" (")
	sb.WriteString(view.OccurredAt.Format("02/01/2006"))
	sb.WriteString(")")
	return sb.String()
}

func formatEditedTransaction(result EditTransactionResult) string {
	var sb strings.Builder
	sb.WriteString("✏️ *Lançamento atualizado!*\n")
	sb.WriteString("De ")
	sb.WriteString(formatBRL(result.OldAmount))
	sb.WriteString(" para *")
	sb.WriteString(formatBRL(result.NewAmount))
	sb.WriteString("*")
	if strings.TrimSpace(result.Description) != "" {
		sb.WriteString(" — ")
		sb.WriteString(result.Description)
	}
	return sb.String()
}

func formatPersistedRecurring(result RecurringCreatorResult) string {
	var sb strings.Builder
	sb.WriteString("🔁 *Recorrência criada!*\n*")
	sb.WriteString(formatBRL(result.AmountCents))
	sb.WriteString("*")
	if result.Direction == "income" {
		sb.WriteString(" de entrada")
	} else {
		sb.WriteString(" de saída")
	}
	sb.WriteString(" ")
	sb.WriteString(frequencyLabel(result.Frequency))
	_, _ = fmt.Fprintf(&sb, " (dia %d)", result.DayOfMonth)
	if strings.TrimSpace(result.Description) != "" {
		sb.WriteString("\n📝 ")
		sb.WriteString(result.Description)
	}
	if strings.TrimSpace(result.CategoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(result.CategoryPath)
	}
	return sb.String()
}

func formatRecurringList(items []RecurringView) string {
	if len(items) == 0 {
		return "🔁 Você ainda não tem lançamentos recorrentes cadastrados."
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "🔁 *Recorrências* (%d)\n", len(items))
	for _, item := range items {
		sb.WriteString("• ")
		sb.WriteString(formatBRL(item.AmountCents))
		sb.WriteString(" ")
		sb.WriteString(frequencyLabel(item.Frequency))
		_, _ = fmt.Fprintf(&sb, " (dia %d)", item.DayOfMonth)
		if strings.TrimSpace(item.Description) != "" {
			sb.WriteString(" — ")
			sb.WriteString(item.Description)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func frequencyLabel(frequency string) string {
	switch frequency {
	case "monthly":
		return "mensal"
	case "yearly":
		return "anual"
	default:
		return frequency
	}
}
