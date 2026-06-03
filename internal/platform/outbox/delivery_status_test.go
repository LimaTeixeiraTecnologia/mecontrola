package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type DeliveryStatusSuite struct {
	suite.Suite
}

func TestDeliveryStatus(t *testing.T) {
	suite.Run(t, new(DeliveryStatusSuite))
}

func (s *DeliveryStatusSuite) TestString() {
	s.Equal("pending", outbox.StatusPending.String())
	s.Equal("claimed", outbox.StatusClaimed.String())
	s.Equal("processed", outbox.StatusProcessed.String())
	s.Equal("dead_letter", outbox.StatusDeadLetter.String())
}

func (s *DeliveryStatusSuite) TestCanTransitionTo() {
	allStatuses := []outbox.DeliveryStatus{
		outbox.StatusPending,
		outbox.StatusClaimed,
		outbox.StatusProcessed,
		outbox.StatusDeadLetter,
	}

	scenarios := []struct {
		name   string
		from   outbox.DeliveryStatus
		to     outbox.DeliveryStatus
		expect bool
	}{
		// Transições válidas
		{"pending → claimed", outbox.StatusPending, outbox.StatusClaimed, true},
		{"claimed → processed", outbox.StatusClaimed, outbox.StatusProcessed, true},
		{"claimed → pending (reaper)", outbox.StatusClaimed, outbox.StatusPending, true},
		{"claimed → dead_letter", outbox.StatusClaimed, outbox.StatusDeadLetter, true},
		{"dead_letter → pending (re-enfileiramento)", outbox.StatusDeadLetter, outbox.StatusPending, true},
		// Transições inválidas
		{"pending → processed", outbox.StatusPending, outbox.StatusProcessed, false},
		{"pending → dead_letter", outbox.StatusPending, outbox.StatusDeadLetter, false},
		{"pending → pending", outbox.StatusPending, outbox.StatusPending, false},
		{"processed → qualquer", outbox.StatusProcessed, outbox.StatusPending, false},
		{"dead_letter → claimed", outbox.StatusDeadLetter, outbox.StatusClaimed, false},
		{"dead_letter → processed", outbox.StatusDeadLetter, outbox.StatusProcessed, false},
		{"dead_letter → dead_letter", outbox.StatusDeadLetter, outbox.StatusDeadLetter, false},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			got := sc.from.CanTransitionTo(sc.to)
			s.Equal(sc.expect, got)
		})
	}

	// Garantir cobertura total 4x4 via tabela separada
	s.Run("cobertura 4x4 origens x destinos", func() {
		type pair struct{ from, to outbox.DeliveryStatus }
		valid := map[pair]bool{
			{outbox.StatusPending, outbox.StatusClaimed}:    true,
			{outbox.StatusClaimed, outbox.StatusProcessed}:  true,
			{outbox.StatusClaimed, outbox.StatusPending}:    true,
			{outbox.StatusClaimed, outbox.StatusDeadLetter}: true,
			{outbox.StatusDeadLetter, outbox.StatusPending}: true,
		}
		for _, from := range allStatuses {
			for _, to := range allStatuses {
				expected := valid[pair{from, to}]
				got := from.CanTransitionTo(to)
				s.Equalf(expected, got, "from=%s to=%s", from.String(), to.String())
			}
		}
	})
}
