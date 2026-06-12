package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInstallmentCountOutOfRange = errors.New("transactions: installment count must be between 1 and 24")

type InstallmentCount struct {
	value int
}

func NewInstallmentCount(n int) (InstallmentCount, error) {
	if n < 1 || n > 24 {
		return InstallmentCount{}, fmt.Errorf("transactions: %d: %w", n, ErrInstallmentCountOutOfRange)
	}
	return InstallmentCount{value: n}, nil
}

func (ic InstallmentCount) Value() int {
	return ic.value
}
