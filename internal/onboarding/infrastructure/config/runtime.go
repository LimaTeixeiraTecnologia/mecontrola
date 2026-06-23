package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type OnboardingRuntimeConfig struct {
	CheckoutURLs        map[string]string
	KiwifyAllowedHosts  []string
	TrustedProxies      []string
	CheckoutCORSOrigins []string
	Messages            map[string]string
	TokenTTL            time.Duration
	OutreachGap         time.Duration
	MetaRetention       time.Duration
}

func NewOnboardingRuntimeConfig(cfg configs.OnboardingConfig, waCfg configs.WhatsAppConfig) (OnboardingRuntimeConfig, error) {
	checkoutURLs, err := parseCheckoutURLs(cfg.KiwifyCheckoutURLs)
	if err != nil {
		return OnboardingRuntimeConfig{}, err
	}

	if cfg.KiwifyCheckoutURLs != "" && len(checkoutURLs) == 0 {
		return OnboardingRuntimeConfig{}, fmt.Errorf("onboarding/runtime_config: ONBOARDING_KIWIFY_CHECKOUT_URLS sem entradas válidas")
	}

	messages := map[string]string{
		"welcome_activated":               waCfg.WelcomeActivated,
		"already_active":                  waCfg.AlreadyActive,
		"code_already_used_other_account": waCfg.CodeAlreadyUsed,
		"payment_still_processing_retry":  waCfg.PaymentProcessing,
		"code_expired_contact_support":    waCfg.CodeExpired,
		"code_invalid_check_again":        waCfg.CodeInvalid,
		"system_unavailable_retry":        waCfg.SystemUnavailable,
		"please_use_ativar_command":       waCfg.PleaseUseAtivar,
		"invalid_country":                 waCfg.InvalidCountry,
		"onboarding_intro":                waCfg.OnboardingIntro,
	}

	return OnboardingRuntimeConfig{
		CheckoutURLs:        checkoutURLs,
		KiwifyAllowedHosts:  parseCSV(cfg.KiwifyAllowedHosts),
		TrustedProxies:      parseCSV(cfg.TrustedProxies),
		CheckoutCORSOrigins: parseCSV(cfg.CheckoutCORSOrigins),
		Messages:            messages,
		TokenTTL:            time.Duration(cfg.TokenTTLDays) * 24 * time.Hour,
		OutreachGap:         time.Duration(cfg.OutreachGapHours) * time.Hour,
		MetaRetention:       time.Duration(cfg.MetaRetentionDays) * 24 * time.Hour,
	}, nil
}

func parseCheckoutURLs(raw string) (map[string]string, error) {
	result := make(map[string]string)
	normalized := strings.ReplaceAll(raw, ";", "\n")
	for line := range strings.SplitSeq(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("onboarding/runtime_config: chave duplicada em ONBOARDING_KIWIFY_CHECKOUT_URLS: %s", key)
		}
		result[key] = value
	}
	return result, nil
}

func parseCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	var result []string
	for item := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
