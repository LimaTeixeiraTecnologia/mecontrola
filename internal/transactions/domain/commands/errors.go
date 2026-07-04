package commands

import "errors"

var ErrCommandInvalidUserID = errors.New("transactions/commands: user_id inválido")
var ErrCommandInvalidCardID = errors.New("transactions/commands: card_id inválido")
var ErrCommandInvalidCategoryID = errors.New("transactions/commands: category_id inválido")
var ErrCommandInvalidDirection = errors.New("transactions/commands: direction inválido")
var ErrCommandInvalidPaymentMethod = errors.New("transactions/commands: payment_method inválido")
var ErrCommandInvalidAmount = errors.New("transactions/commands: amount_cents inválido")
var ErrCommandInvalidDescription = errors.New("transactions/commands: description inválida")
var ErrCommandInvalidInstallments = errors.New("transactions/commands: installments_total inválido")
var ErrCommandInvalidFrequency = errors.New("transactions/commands: frequency inválida")
var ErrCommandInvalidDayOfMonth = errors.New("transactions/commands: day_of_month inválido")
var ErrCommandMissingOccurredAt = errors.New("transactions/commands: occurred_at obrigatório")
var ErrCommandCreditCardRequiresCardID = errors.New("transactions/commands: credit_card requer card_id")
var ErrCommandCreditCardRequiresOutcome = errors.New("transactions/commands: credit_card requer direction outcome")
