package input

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"

type ConsumeMagicTokenInput struct {
	Token          string
	FromE164       string
	ActivationPath valueobjects.ActivationPath
}
