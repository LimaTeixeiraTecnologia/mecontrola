package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	devkithttp "github.com/JailtonJunior94/devkit-go/pkg/httpclient"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	endpointCredits = "/api/v1/credits"
	endpointAuthKey = "/api/v1/auth/key"
)

type CreditsSnapshot struct {
	TotalCreditsUSD float64
	TotalUsageUSD   float64
	RemainingUSD    float64
	UsageDailyUSD   float64
	UsageWeeklyUSD  float64
	UsageMonthlyUSD float64
}

type CreditsClient struct {
	client *httpclient.Client
	apiKey string
}

func NewCreditsClient(client *httpclient.Client, apiKey string) *CreditsClient {
	return &CreditsClient{client: client, apiKey: apiKey}
}

func (c *CreditsClient) Fetch(ctx context.Context) (CreditsSnapshot, error) {
	var credits creditsResponse
	if err := c.get(ctx, endpointCredits, &credits); err != nil {
		return CreditsSnapshot{}, err
	}

	var authKey authKeyResponse
	if err := c.get(ctx, endpointAuthKey, &authKey); err != nil {
		return CreditsSnapshot{}, err
	}

	return CreditsSnapshot{
		TotalCreditsUSD: credits.Data.TotalCredits,
		TotalUsageUSD:   credits.Data.TotalUsage,
		RemainingUSD:    credits.Data.TotalCredits - credits.Data.TotalUsage,
		UsageDailyUSD:   authKey.Data.UsageDaily,
		UsageWeeklyUSD:  authKey.Data.UsageWeekly,
		UsageMonthlyUSD: authKey.Data.UsageMonthly,
	}, nil
}

func (c *CreditsClient) get(ctx context.Context, path string, out any) error {
	resp, err := c.client.Get(ctx, path,
		httpclient.WithHeader("Authorization", "Bearer "+c.apiKey),
		httpclient.WithRetry(openrouterRetryAttempts, openrouterRetryBackoff, devkithttp.DefaultRetryPolicy),
	)
	if err != nil {
		return fmt.Errorf("llm.credits: get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("llm.credits: get %s: %w: status=%d", path, ErrProviderUpstream, resp.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("llm.credits: decode %s: %w", path, err)
	}
	return nil
}

type creditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

type authKeyResponse struct {
	Data struct {
		UsageDaily   float64 `json:"usage_daily"`
		UsageWeekly  float64 `json:"usage_weekly"`
		UsageMonthly float64 `json:"usage_monthly"`
	} `json:"data"`
}
