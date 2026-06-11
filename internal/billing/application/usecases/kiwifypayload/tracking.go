package kiwifypayload

func ExtractFunnel(p Payload) (token string, carrier string) {
	switch {
	case p.TrackingParameters.SCK != "":
		return p.TrackingParameters.SCK, "sck"
	case p.TrackingParameters.S1 != "":
		return p.TrackingParameters.S1, "s1"
	case p.TrackingParameters.Src != "":
		return p.TrackingParameters.Src, "src"
	default:
		return "", "none"
	}
}
