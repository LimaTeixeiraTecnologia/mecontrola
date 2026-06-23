package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrBudgetTotalMustBePositive = errors.New("budgets: total em centavos deve ser maior que zero para ativação")

var ErrBudgetAllocationSumMustBe10000 = errors.New("budgets: soma dos basis points deve ser exatamente 10000 para ativação")

var ErrBudgetAlreadyActive = errors.New("budgets: orçamento já está ativo")

var ErrBudgetNotDraft = errors.New("budgets: operação permitida apenas em rascunho")

var ErrBudgetNotActive = errors.New("budgets: operação permitida apenas em orçamento ativo")

type BudgetState uint8

const (
	BudgetStateDraft BudgetState = iota + 1
	BudgetStateActive
)

type Budget struct {
	id          uuid.UUID
	userID      uuid.UUID
	competence  valueobjects.Competence
	totalCents  int64
	state       BudgetState
	activatedAt *time.Time
	autoDraft   bool
	allocations []Allocation
	createdAt   time.Time
	updatedAt   time.Time
}

func NewBudget(userID uuid.UUID, competence valueobjects.Competence, totalCents int64, now time.Time) Budget {
	return Budget{
		id:         uuid.New(),
		userID:     userID,
		competence: competence,
		totalCents: totalCents,
		state:      BudgetStateDraft,
		createdAt:  now,
		updatedAt:  now,
	}
}

func NewAutoDraftBudget(userID uuid.UUID, competence valueobjects.Competence, now time.Time) Budget {
	return Budget{
		id:         uuid.New(),
		userID:     userID,
		competence: competence,
		totalCents: 0,
		state:      BudgetStateDraft,
		autoDraft:  true,
		createdAt:  now,
		updatedAt:  now,
	}
}

func HydrateBudget(
	id uuid.UUID,
	userID uuid.UUID,
	competence valueobjects.Competence,
	totalCents int64,
	state BudgetState,
	activatedAt *time.Time,
	autoDraft bool,
	allocations []Allocation,
	createdAt time.Time,
	updatedAt time.Time,
) Budget {
	return Budget{
		id:          id,
		userID:      userID,
		competence:  competence,
		totalCents:  totalCents,
		state:       state,
		activatedAt: activatedAt,
		autoDraft:   autoDraft,
		allocations: allocations,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

func (b Budget) ID() uuid.UUID                       { return b.id }
func (b Budget) UserID() uuid.UUID                   { return b.userID }
func (b Budget) Competence() valueobjects.Competence { return b.competence }
func (b Budget) TotalCents() int64                   { return b.totalCents }
func (b Budget) State() BudgetState                  { return b.state }
func (b Budget) ActivatedAt() *time.Time             { return b.activatedAt }
func (b Budget) AutoDraft() bool                     { return b.autoDraft }
func (b Budget) Allocations() []Allocation           { return b.allocations }
func (b Budget) CreatedAt() time.Time                { return b.createdAt }
func (b Budget) UpdatedAt() time.Time                { return b.updatedAt }

func (b Budget) IsDraft() bool  { return b.state == BudgetStateDraft }
func (b Budget) IsActive() bool { return b.state == BudgetStateActive }

func (b *Budget) SetAllocations(allocs []Allocation) {
	b.allocations = allocs
}

func (b *Budget) AddAllocation(a Allocation) {
	b.allocations = append(b.allocations, a)
}

func (b *Budget) RebalanceAllocations(allocs []Allocation, now time.Time) error {
	if b.state != BudgetStateActive {
		return ErrBudgetNotActive
	}
	sum := 0
	for i := range allocs {
		sum += allocs[i].BasisPoints()
	}
	if sum != 10000 {
		return fmt.Errorf("budgets: soma=%d: %w", sum, ErrBudgetAllocationSumMustBe10000)
	}
	b.allocations = allocs
	b.updatedAt = now
	return nil
}

func (b *Budget) Activate(now time.Time) error {
	if b.state == BudgetStateActive {
		return ErrBudgetAlreadyActive
	}
	if b.totalCents <= 0 {
		return ErrBudgetTotalMustBePositive
	}
	sum := 0
	for i := range b.allocations {
		sum += b.allocations[i].BasisPoints()
	}
	if sum != 10000 {
		return fmt.Errorf("budgets: soma=%d: %w", sum, ErrBudgetAllocationSumMustBe10000)
	}
	b.state = BudgetStateActive
	b.activatedAt = &now
	b.updatedAt = now
	return nil
}
