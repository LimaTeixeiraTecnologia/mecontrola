package handlers

import (
	"context"
)

type expireTokensUseCase interface {
	Execute(ctx context.Context) error
}

type TokenExpirationJob struct {
	usecase  expireTokensUseCase
	schedule string
}

func NewTokenExpirationJob(uc expireTokensUseCase, schedule string) *TokenExpirationJob {
	return &TokenExpirationJob{usecase: uc, schedule: schedule}
}

func (j *TokenExpirationJob) Name() string     { return "onboarding-token-expiration" }
func (j *TokenExpirationJob) Schedule() string { return j.schedule }

func (j *TokenExpirationJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
