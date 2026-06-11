package services

import (
	"fmt"
	"time"
)

func NewSaoPauloLocation() (*time.Location, error) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, fmt.Errorf("card.services: carregar America/Sao_Paulo: %w", err)
	}
	return loc, nil
}
