package checkout

import (
	"context"
	"net/url"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
)

type KiwifyURLBuilder struct {
	checkoutURLs map[string]string
	allowedHosts map[string]struct{}
}

func NewKiwifyURLBuilder(checkoutURLs map[string]string, allowedHosts []string) *KiwifyURLBuilder {
	hosts := make(map[string]struct{}, len(allowedHosts))
	for _, h := range allowedHosts {
		hosts[h] = struct{}{}
	}
	return &KiwifyURLBuilder{
		checkoutURLs: checkoutURLs,
		allowedHosts: hosts,
	}
}

func (b *KiwifyURLBuilder) Build(_ context.Context, planID, token string) (string, error) {
	raw, ok := b.checkoutURLs[planID]
	if !ok {
		return "", application.ErrUnknownPlan
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", application.ErrCheckoutUnavailable
	}

	if _, allowed := b.allowedHosts[u.Host]; !allowed {
		return "", application.ErrCheckoutUnavailable
	}

	q := u.Query()
	q.Set("sck", token)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
