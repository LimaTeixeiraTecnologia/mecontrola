package usecases

type kiwifyTracking struct {
	S1  string `json:"s1"`
	Src string `json:"src"`
}

func extractFunnelToken(tracking kiwifyTracking) string {
	if tracking.S1 != "" {
		return tracking.S1
	}
	return tracking.Src
}

func ExtractFunnelTokenForTest(s1, src string) string {
	return extractFunnelToken(kiwifyTracking{S1: s1, Src: src})
}
