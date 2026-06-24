package usecases

func sanitizeE164(e164 string) string {
	if len(e164) > 0 && e164[0] == '+' {
		return e164[1:]
	}
	return e164
}
