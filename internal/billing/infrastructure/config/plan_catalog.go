package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

const placeholderPlanIDPrefix = "__PLACEHOLDER_"

type PlanCatalog struct {
	productIDs map[valueobjects.PlanCode]string
}

func NewPlanCatalog(cfg configs.KiwifyConfig) (*PlanCatalog, error) {
	missing := missingEnvVars(cfg)
	if len(missing) > 0 {
		return nil, fmt.Errorf("billing/plan_catalog: planos Kiwify nao configurados: %s", strings.Join(missing, ", "))
	}

	productIDs := map[valueobjects.PlanCode]string{
		valueobjects.PlanCodeMonthly:   cfg.ProductIDMonthly,
		valueobjects.PlanCodeQuarterly: cfg.ProductIDQuarterly,
		valueobjects.PlanCodeAnnual:    cfg.ProductIDAnnual,
	}

	return &PlanCatalog{productIDs: productIDs}, nil
}

func (c *PlanCatalog) Apply(ctx context.Context, repo interfaces.PlanRepository) error {
	if err := repo.ConfigureProductIDs(ctx, c.productIDs); err != nil {
		return fmt.Errorf("billing/plan_catalog: configurar IDs de produto Kiwify: %w", err)
	}
	return nil
}

func missingEnvVars(cfg configs.KiwifyConfig) []string {
	var missing []string
	if isUnsetPlanID(cfg.ProductIDMonthly) {
		missing = append(missing, "KIWIFY_PRODUCT_ID_MONTHLY")
	}
	if isUnsetPlanID(cfg.ProductIDQuarterly) {
		missing = append(missing, "KIWIFY_PRODUCT_ID_QUARTERLY")
	}
	if isUnsetPlanID(cfg.ProductIDAnnual) {
		missing = append(missing, "KIWIFY_PRODUCT_ID_ANNUAL")
	}
	return missing
}

func isUnsetPlanID(id string) bool {
	if id == "" {
		return true
	}
	return strings.HasPrefix(id, placeholderPlanIDPrefix)
}
