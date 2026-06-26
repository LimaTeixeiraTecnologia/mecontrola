package workflow

import (
	"strconv"
	"strings"
)

func ParseMoneyCents(text string) (int64, bool) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "R$", "")
	cleaned = strings.ReplaceAll(cleaned, "r$", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if cleaned == "" {
		return 0, false
	}

	lastComma := strings.LastIndex(cleaned, ",")
	lastDot := strings.LastIndex(cleaned, ".")

	var wholePart, fracPart string
	var sep rune
	if lastComma > lastDot {
		sep = ','
		wholePart = cleaned[:lastComma]
		fracPart = cleaned[lastComma+1:]
		wholePart = strings.ReplaceAll(wholePart, ".", "")
	} else if lastDot > lastComma {
		sep = '.'
		wholePart = cleaned[:lastDot]
		fracPart = cleaned[lastDot+1:]
		wholePart = strings.ReplaceAll(wholePart, ",", "")
	} else {
		wholePart = cleaned
		fracPart = ""
	}

	if wholePart == "" {
		wholePart = "0"
	}

	if len(fracPart) == 3 && sep == '.' {
		wholePart = wholePart + fracPart
		fracPart = ""
	}

	if len(fracPart) > 2 {
		return 0, false
	}

	switch len(fracPart) {
	case 0:
		fracPart = "00"
	case 1:
		fracPart = fracPart + "0"
	}

	wholePart = strings.TrimLeft(wholePart, "0")
	if wholePart == "" {
		wholePart = "0"
	}

	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil || whole < 0 {
		return 0, false
	}

	frac, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil || frac < 0 {
		return 0, false
	}

	return whole*100 + frac, true
}
