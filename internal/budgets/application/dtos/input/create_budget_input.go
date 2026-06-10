package input

type AllocationInput struct {
	RootSlug    string
	BasisPoints int
}

type CreateBudgetInput struct {
	UserID      string
	Competence  string
	TotalCents  int64
	Allocations []AllocationInput
}
