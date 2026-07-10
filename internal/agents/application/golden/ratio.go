package golden

type CategoryResult struct {
	Category Category
	Hits     int
	Total    int
	Failures []string
}

func (r CategoryResult) Ratio() float64 {
	if r.Total == 0 {
		return 1.0
	}
	return float64(r.Hits) / float64(r.Total)
}

func (r CategoryResult) PassesGate(threshold float64) bool {
	return r.Ratio() >= threshold
}

type CaseOutcome struct {
	Case   Case
	Passed bool
	Detail string
}

func AggregateByCategory(outcomes []CaseOutcome) []CategoryResult {
	index := make(map[Category]*CategoryResult)
	var order []Category
	for _, o := range outcomes {
		result, ok := index[o.Case.Category]
		if !ok {
			result = &CategoryResult{Category: o.Case.Category}
			index[o.Case.Category] = result
			order = append(order, o.Case.Category)
		}
		result.Total++
		if o.Passed {
			result.Hits++
		} else {
			result.Failures = append(result.Failures, o.Case.Name+": "+o.Detail)
		}
	}
	out := make([]CategoryResult, 0, len(order))
	for _, category := range order {
		out = append(out, *index[category])
	}
	return out
}
