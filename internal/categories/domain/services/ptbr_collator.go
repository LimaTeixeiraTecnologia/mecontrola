package services

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type PTBRCollator struct {
	pool sync.Pool
}

func NewPTBRCollator() *PTBRCollator {
	return &PTBRCollator{
		pool: sync.Pool{
			New: func() any {
				return collate.New(language.BrazilianPortuguese, collate.IgnoreCase)
			},
		},
	}
}

func (c *PTBRCollator) Less(a, b string) bool {
	cl := c.pool.Get().(*collate.Collator)
	defer c.pool.Put(cl)
	return cl.CompareString(a, b) < 0
}
