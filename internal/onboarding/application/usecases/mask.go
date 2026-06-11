package usecases

func maskMobile(mobile string) string {
	if len(mobile) < 4 {
		return "****"
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
