package kiwify

import (
	"context"
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

// ErrPlanNotFound é retornado quando o kiwify_product_id não está mapeado em billing_plans.
var ErrPlanNotFound = errors.New("kiwify: plano não encontrado para o produto")

// BillingPlansLoader é o contrato mínimo para carregar planos do banco de dados.
// Segregado aqui para evitar dependência circular com infrastructure/repositories.
type BillingPlansLoader interface {
	LoadKiwifyProductPlans(ctx context.Context) (map[string]valueobjects.PlanCode, error)
}

// BillingPlansRegistry mantém cache em memória de kiwify_product_id → PlanCode.
// Populado na inicialização via SELECT billing_plans WHERE active = true AND kiwify_product_id IS NOT NULL.
type BillingPlansRegistry struct {
	plansByProductID map[string]valueobjects.PlanCode
}

// NewBillingPlansRegistry cria e popula o registry a partir do loader.
func NewBillingPlansRegistry(ctx context.Context, loader BillingPlansLoader) (*BillingPlansRegistry, error) {
	plans, err := loader.LoadKiwifyProductPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("billing plans registry: carregar planos: %w", err)
	}
	return &BillingPlansRegistry{plansByProductID: plans}, nil
}

// NewBillingPlansRegistryFromMap cria o registry a partir de um mapa pré-populado.
// Útil para testes e para bootstrap quando o banco já fornece os dados resolvidos.
func NewBillingPlansRegistryFromMap(plans map[string]valueobjects.PlanCode) *BillingPlansRegistry {
	return &BillingPlansRegistry{plansByProductID: plans}
}

// ParsePlanCodeFromKiwifyProductID resolve o PlanCode correspondente ao product ID da Kiwify.
// Retorna ErrPlanNotFound se o product ID não estiver registrado.
func (r *BillingPlansRegistry) ParsePlanCodeFromKiwifyProductID(id string) (valueobjects.PlanCode, error) {
	plan, ok := r.plansByProductID[id]
	if !ok {
		return valueobjects.PlanCodeUnknown, fmt.Errorf("%w: product_id=%q", ErrPlanNotFound, id)
	}
	return plan, nil
}
