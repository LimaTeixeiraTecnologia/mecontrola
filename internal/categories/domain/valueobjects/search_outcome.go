package valueobjects

type SearchOutcome uint8

const (
	SearchOutcomeUnknown SearchOutcome = iota
	SearchOutcomeNoMatch
	SearchOutcomeMatched
	SearchOutcomeAmbiguous
)

func ClassifyOutcome(candidatesCount int) SearchOutcome {
	switch {
	case candidatesCount <= 0:
		return SearchOutcomeNoMatch
	case candidatesCount == 1:
		return SearchOutcomeMatched
	default:
		return SearchOutcomeAmbiguous
	}
}

func (o SearchOutcome) String() string {
	switch o {
	case SearchOutcomeNoMatch:
		return "no_match"
	case SearchOutcomeMatched:
		return "matched"
	case SearchOutcomeAmbiguous:
		return "ambiguous"
	default:
		return ""
	}
}

func (o SearchOutcome) IsValid() bool {
	switch o {
	case SearchOutcomeNoMatch, SearchOutcomeMatched, SearchOutcomeAmbiguous:
		return true
	default:
		return false
	}
}
