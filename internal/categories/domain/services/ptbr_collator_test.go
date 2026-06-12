package services_test

import (
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
)

func TestPTBRCollator_Less_IgnoresCase(t *testing.T) {
	c := services.NewPTBRCollator()

	assert.False(t, c.Less("Banco", "banco"))
	assert.False(t, c.Less("banco", "Banco"))
}

func TestPTBRCollator_Less_OrdersAlphabetically(t *testing.T) {
	c := services.NewPTBRCollator()

	assert.True(t, c.Less("aluguel", "banco"))
	assert.False(t, c.Less("banco", "aluguel"))
}

func TestPTBRCollator_ConcurrentLessIsSafe(t *testing.T) {
	c := services.NewPTBRCollator()
	inputs := [][2]string{
		{"aluguel", "banco"},
		{"banco", "supermercado"},
		{"supermercado", "aluguel"},
		{"Aluguel", "ALUGUEL"},
		{"agua", "AGUA"},
	}

	const goroutines = 32
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for i := range iterations {
				p := inputs[i%len(inputs)]
				_ = c.Less(p[0], p[1])
			}
		}()
	}
	wg.Wait()
}

func TestPTBRCollator_SortsList(t *testing.T) {
	c := services.NewPTBRCollator()
	items := []string{"banco", "Aluguel", "supermercado", "ALUGUEL"}
	sort.SliceStable(items, func(i, j int) bool { return c.Less(items[i], items[j]) })

	assert.Equal(t, "Aluguel", items[0])
	assert.Equal(t, "ALUGUEL", items[1])
	assert.Equal(t, "banco", items[2])
	assert.Equal(t, "supermercado", items[3])
}
