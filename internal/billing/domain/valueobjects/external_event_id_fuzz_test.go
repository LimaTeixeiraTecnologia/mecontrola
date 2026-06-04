package valueobjects_test

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func FuzzNewExternalEventIDCascade(f *testing.F) {
	f.Add([]byte(`{"id":"abc"}`))
	f.Add([]byte(`{"order":{"id":"xyz"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`not-json`))
	f.Add([]byte(`{"id":""}`))
	f.Add([]byte(`{"id":null}`))
	f.Add([]byte(`{"id":123}`))
	f.Add([]byte(`{"order":{"id":""}}`))
	f.Add([]byte(`{"id":"abc","order":{"id":"xyz"}}`))

	f.Fuzz(func(t *testing.T, input []byte) {
		// nunca deve panic
		_, _ = valueobjects.NewExternalEventIDCascade(input)
	})
}
