package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidAllocationType = errors.New("categories: invalid allocation type")

type AllocationType uint8

const (
	AllocationTypeConsumption AllocationType = iota + 1
	AllocationTypeAssetAllocation
)

func ParseAllocationType(s string) (AllocationType, error) {
	switch s {
	case "consumption":
		return AllocationTypeConsumption, nil
	case "asset_allocation":
		return AllocationTypeAssetAllocation, nil
	default:
		return 0, fmt.Errorf("categories: %q: %w", s, ErrInvalidAllocationType)
	}
}

func (a AllocationType) String() string {
	switch a {
	case AllocationTypeConsumption:
		return "consumption"
	case AllocationTypeAssetAllocation:
		return "asset_allocation"
	default:
		return ""
	}
}

func (a AllocationType) IsValid() bool {
	switch a {
	case AllocationTypeConsumption, AllocationTypeAssetAllocation:
		return true
	default:
		return false
	}
}
