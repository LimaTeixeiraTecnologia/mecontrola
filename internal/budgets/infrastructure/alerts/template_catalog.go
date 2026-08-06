package alerts

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type CatalogEntry struct {
	Kind         services.ThresholdAlertKind
	Name         string
	LanguageCode string
	Category     services.TemplateCategory
	Approved     bool
}

type TemplateCatalog struct {
	entries map[services.ThresholdAlertKind]interfaces.AlertTemplate
}

func NewTemplateCatalog(entries []CatalogEntry) *TemplateCatalog {
	byKind := make(map[services.ThresholdAlertKind]interfaces.AlertTemplate, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" {
			continue
		}
		if !services.IsReleaseOneEmittable(entry.Kind) {
			continue
		}
		status := services.TemplateStatusPending
		if entry.Approved {
			status = services.TemplateStatusApproved
		}
		byKind[entry.Kind] = interfaces.AlertTemplate{
			Name:         entry.Name,
			LanguageCode: entry.LanguageCode,
			Category:     entry.Category,
			Status:       status,
		}
	}
	return &TemplateCatalog{entries: byKind}
}

func (c *TemplateCatalog) Lookup(kind services.ThresholdAlertKind) (interfaces.AlertTemplate, bool) {
	template, ok := c.entries[kind]
	return template, ok
}

type FallbackTimezoneResolver struct {
	fallback *time.Location
}

func NewFallbackTimezoneResolver(fallback *time.Location) *FallbackTimezoneResolver {
	return &FallbackTimezoneResolver{fallback: fallback}
}

func (r *FallbackTimezoneResolver) Resolve(_ context.Context, _ uuid.UUID) (*time.Location, error) {
	if r.fallback == nil {
		return time.UTC, nil
	}
	return r.fallback, nil
}

type DenyAllMarketingConsent struct{}

func NewDenyAllMarketingConsent() *DenyAllMarketingConsent {
	return &DenyAllMarketingConsent{}
}

func (DenyAllMarketingConsent) HasConsent(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}
