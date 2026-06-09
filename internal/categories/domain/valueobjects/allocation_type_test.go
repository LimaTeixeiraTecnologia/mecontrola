package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type AllocationTypeSuite struct {
	suite.Suite
}

func TestAllocationTypeSuite(t *testing.T) {
	suite.Run(t, new(AllocationTypeSuite))
}

func (s *AllocationTypeSuite) TestParseAllocationType() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.AllocationType
		wantErr bool
	}{
		{name: "deve parsear consumption", input: "consumption", want: valueobjects.AllocationTypeConsumption, wantErr: false},
		{name: "deve parsear asset_allocation", input: "asset_allocation", want: valueobjects.AllocationTypeAssetAllocation, wantErr: false},
		{name: "deve retornar erro para allocation type invalido", input: "invalid", want: 0, wantErr: true},
		{name: "deve retornar erro para string vazia", input: "", want: 0, wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := valueobjects.ParseAllocationType(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, valueobjects.ErrInvalidAllocationType)
			} else {
				s.NoError(err)
				s.Equal(scenario.want, got)
			}
		})
	}
}

func (s *AllocationTypeSuite) TestAllocationTypeString() {
	scenarios := []struct {
		name           string
		allocationType valueobjects.AllocationType
		want           string
	}{
		{name: "consumption deve retornar consumption", allocationType: valueobjects.AllocationTypeConsumption, want: "consumption"},
		{name: "asset_allocation deve retornar asset_allocation", allocationType: valueobjects.AllocationTypeAssetAllocation, want: "asset_allocation"},
		{name: "zero value deve retornar vazio", allocationType: 0, want: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.allocationType.String())
		})
	}
}

func (s *AllocationTypeSuite) TestAllocationTypeIsValid() {
	scenarios := []struct {
		name           string
		allocationType valueobjects.AllocationType
		want           bool
	}{
		{name: "consumption eh valido", allocationType: valueobjects.AllocationTypeConsumption, want: true},
		{name: "asset_allocation eh valido", allocationType: valueobjects.AllocationTypeAssetAllocation, want: true},
		{name: "zero value eh invalido", allocationType: 0, want: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.allocationType.IsValid())
		})
	}
}
