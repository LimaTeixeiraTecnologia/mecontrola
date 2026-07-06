package output

import "github.com/google/uuid"

type ResolveCategoryForWriteOutput struct {
	RootCategoryID   uuid.UUID
	SubcategoryID    uuid.UUID
	Kind             string
	Path             string
	RootSlug         string
	SubcategorySlug  string
	CategoryName     string
	SubcategoryName  string
	EditorialVersion int64
	Deprecated       bool
}
