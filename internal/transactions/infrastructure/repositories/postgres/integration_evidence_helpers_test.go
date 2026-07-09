//go:build integration

package postgres_test

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var (
	seedExpenseRootID = uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62")
	seedExpenseLeafID = uuid.MustParse("c2fda6a3-c329-52c8-81ea-771b6ea4f365")
	seedIncomeRootID  = uuid.MustParse("86dd34b0-7342-525a-9a30-b1b5a76b109f")
	seedIncomeLeafID  = uuid.MustParse("98455e74-b1f3-5b9c-a8d8-05db0cdb465d")
)

const seedEditorialVersion = int64(9)

func expenseEvidence() valueobjects.CategoryWriteEvidence {
	return valueobjects.ReconstituteEvidence(
		seedExpenseRootID,
		seedExpenseLeafID,
		"expense",
		"custo-fixo/aluguel",
		"matched",
		1.0,
		"high",
		"exact",
		"canonical_name",
		"aluguel",
		"matched canonical_name aluguel",
		valueobjects.CategoryDecisionSourceAutoMatched,
		seedEditorialVersion,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)
}

func incomeEvidence() valueobjects.CategoryWriteEvidence {
	return valueobjects.ReconstituteEvidence(
		seedIncomeRootID,
		seedIncomeLeafID,
		"income",
		"salario/decimo-terceiro",
		"matched",
		1.0,
		"high",
		"exact",
		"canonical_name",
		"salário",
		"matched canonical_name salário",
		valueobjects.CategoryDecisionSourceAutoMatched,
		seedEditorialVersion,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)
}
