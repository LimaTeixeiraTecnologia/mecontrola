package usecases

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/kiwifypayload"
)

func ExtractFunnelTokenForTest(sck, s1, src string) string {
	p := kiwifypayload.Payload{}
	p.TrackingParameters.SCK = sck
	p.TrackingParameters.S1 = s1
	p.TrackingParameters.Src = src
	token, _ := kiwifypayload.ExtractFunnel(p)
	return token
}
