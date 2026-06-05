package pii

import "unicode/utf8"

func MaskDisplayName(name string) string {
	if name == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError && size <= 1 {
		return "****"
	}
	if size == len(name) {
		return "*"
	}
	return string(r) + "****"
}
