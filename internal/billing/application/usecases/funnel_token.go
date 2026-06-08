package usecases

type kiwifyTracking struct {
	SCK string `json:"sck"`
	S1  string `json:"s1"`
	Src string `json:"src"`
}

func extractFunnelToken(tracking kiwifyTracking) string {
	if tracking.SCK != "" {
		return tracking.SCK
	}
	if tracking.S1 != "" {
		return tracking.S1
	}
	return tracking.Src
}

func ExtractFunnelTokenForTest(sck, s1, src string) string {
	return extractFunnelToken(kiwifyTracking{SCK: sck, S1: s1, Src: src})
}
