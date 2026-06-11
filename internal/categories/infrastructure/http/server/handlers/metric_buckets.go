package handlers

func qLenBucket(normalized string) string {
	length := len(normalized)
	switch {
	case length >= 3 && length <= 4:
		return "3-4"
	case length >= 5 && length <= 8:
		return "5-8"
	case length >= 9 && length <= 16:
		return "9-16"
	case length >= 17 && length <= 32:
		return "17-32"
	case length >= 33:
		return "33+"
	default:
		return ""
	}
}
