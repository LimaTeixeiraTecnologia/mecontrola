package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type InstallmentSplitter struct{}

func (s InstallmentSplitter) Split(total valueobjects.Money, n valueobjects.InstallmentCount) []valueobjects.Money {
	count := n.Value()
	base := total.Cents() / int64(count)
	remainder := total.Cents() % int64(count)

	result := make([]valueobjects.Money, count)
	for i := range count {
		cents := base
		if int64(i) < remainder {
			cents++
		}
		result[i], _ = valueobjects.NewMoney(cents)
	}
	return result
}
